#!/usr/bin/env python3
"""tools/shortcut-runtime/extract_parameters.py

Fault-tolerant, sharded, resumable parameter encodings extractor for Apple Shortcuts.
Runs isolated worker processes with process-group watchdogs so that no single Apple private API
hang can block an extraction job indefinitely.
"""

import argparse
import datetime
import json
import os
import pathlib
import signal
import subprocess
import sys
import tempfile
import time


def get_shard_actions(all_actions, shard_index, shard_count):
    """Deterministically partition sorted actions into balanced shards using round-robin index."""
    sorted_actions = sorted(all_actions)
    return [ident for idx, ident in enumerate(sorted_actions) if idx % shard_count == shard_index]


def load_actions(actions_file=None):
    """Load action identifiers from JSON dict, JSON list, or plain text file."""
    if not actions_file or not os.path.exists(actions_file):
        return []
    path = pathlib.Path(actions_file)
    content = path.read_text(encoding="utf-8").strip()
    if not content:
        return []
    if path.suffix.lower() == ".json":
        data = json.loads(content)
        if isinstance(data, dict):
            # E.g. builtin-actions.json where keys are identifiers
            return sorted(data.keys())
        elif isinstance(data, list):
            # List of strings or list of dicts with identifier/shortcutActionIdentifier
            ids = set()
            for item in data:
                if isinstance(item, str):
                    ids.add(item)
                elif isinstance(item, dict):
                    i = item.get("identifier") or item.get("shortcutActionIdentifier") or item.get("WFWorkflowActionIdentifier")
                    if i:
                        ids.add(i)
            return sorted(ids)
    # Text file with one identifier per line
    return sorted(line.strip() for line in content.splitlines() if line.strip())


def run_worker_process(cmd, timeout_seconds):
    """Run child process in a new process group with hard timeout and process-group kill."""
    start_time = time.time()
    is_posix = os.name == "posix"

    try:
        proc = subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            start_new_session=is_posix
        )
    except Exception as e:
        return False, f"Failed to spawn worker: {e}", time.time() - start_time, True

    timed_out = False
    stdout, stderr = "", ""
    try:
        stdout, stderr = proc.communicate(timeout=timeout_seconds)
    except subprocess.TimeoutExpired:
        timed_out = True
        # Terminate entire process group
        if is_posix:
            try:
                pgid = os.getpgid(proc.pid)
                os.killpg(pgid, signal.SIGKILL)
            except Exception:
                try:
                    proc.kill()
                except Exception:
                    pass
        else:
            try:
                proc.kill()
            except Exception:
                pass
        try:
            stdout, stderr = proc.communicate(timeout=2.0)
        except Exception:
            pass

    elapsed = time.time() - start_time
    if timed_out:
        return False, f"Timed out after {timeout_seconds:.1f}s", elapsed, False

    if proc.returncode != 0:
        err_msg = stderr.strip() or stdout.strip() or f"Process exited with code {proc.returncode}"
        return False, err_msg, elapsed, False

    return True, stdout, elapsed, False


def prepare_jxa_runner(prelude_path, probe_script_path, temp_dir):
    """Combine JXA prelude and probe script into an executable temp file."""
    prelude = pathlib.Path(prelude_path).read_text(encoding="utf-8") if prelude_path and os.path.exists(prelude_path) else ""
    probe = pathlib.Path(probe_script_path).read_text(encoding="utf-8")
    combined = prelude + "\n\n" + probe
    runner_file = pathlib.Path(temp_dir) / "probe_runner.js"
    runner_file.write_text(combined, encoding="utf-8")
    return str(runner_file)


class ParameterExtractor:
    def __init__(self, args):
        self.shard_index = args.shard_index
        self.shard_count = args.shard_count
        self.out_dir = pathlib.Path(args.out_dir)
        self.out_dir.mkdir(parents=True, exist_ok=True)
        self.action_timeout = float(args.action_timeout)
        self.param_timeout = float(args.param_timeout)
        self.resume = args.resume
        self.mock_worker = args.mock_worker

        self.prelude_path = args.prelude_path
        self.probe_script = args.probe_script or str(pathlib.Path(__file__).parent / "jxa" / "probe_single_action.js")
        self.meta_script = args.meta_script or str(pathlib.Path(__file__).parent / "jxa" / "inspect_actions_meta.js")

        self.checkpoint_path = pathlib.Path(args.checkpoint_file) if args.checkpoint_file else (
            self.out_dir / f"parameter-checkpoint-shard-{self.shard_index}.jsonl"
        )
        self.output_json_path = self.out_dir / f"parameter-encodings-shard-{self.shard_index}.json"
        self.summary_json_path = self.out_dir / f"parameter-shard-{self.shard_index}-summary.json"

        # State tracking
        self.completed_actions = {}
        self.timeouts = []
        self.errors = []
        self.action_parameters = {}
        self.action_output_names = {}
        self.action_defaults = {}
        self.unavailable_actions = set()
        self.parameter_classes = {}

    def load_checkpoint(self):
        """Load already completed actions if resume mode is enabled."""
        if not self.resume or not self.checkpoint_path.exists():
            return set()
        finished = set()
        try:
            with open(self.checkpoint_path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    try:
                        record = json.loads(line)
                        ident = record.get("action")
                        if ident:
                            finished.add(ident)
                    except Exception:
                        pass
        except Exception as e:
            print(f"[Warning] Failed to read checkpoint {self.checkpoint_path}: {e}", file=sys.stderr)
        return finished

    def record_checkpoint(self, record):
        """Append machine-readable progress checkpoint atomically."""
        record["timestamp"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        record["shardIndex"] = self.shard_index
        record["shardCount"] = self.shard_count
        with open(self.checkpoint_path, "a", encoding="utf-8") as f:
            f.write(json.dumps(record, sort_keys=True) + "\n")

    def build_worker_cmd(self, runner_script, action_id, out_temp_file, param_key=None):
        if self.mock_worker:
            cmd = [sys.executable, self.mock_worker, action_id, out_temp_file]
            if param_key:
                cmd.append(param_key)
            return cmd
        cmd = ["osascript", "-l", "JavaScript", runner_script, action_id, out_temp_file]
        if param_key:
            cmd.append(param_key)
        return cmd

    def probe_single_action(self, runner_script, meta_runner_script, action_id, temp_dir):
        """Probe an action with watchdog timeout; fall back to per-parameter probing if timed out."""
        out_temp = str(pathlib.Path(temp_dir) / f"{action_id}.json")
        cmd = self.build_worker_cmd(runner_script, action_id, out_temp)

        success, msg, elapsed, is_spawn_err = run_worker_process(cmd, self.action_timeout)

        if success and os.path.exists(out_temp):
            try:
                data = json.loads(pathlib.Path(out_temp).read_text(encoding="utf-8"))
                return data, elapsed
            except Exception as e:
                return {
                    "identifier": action_id,
                    "status": "error",
                    "error": f"Failed to parse worker output: {e}",
                    "parameters": []
                }, elapsed

        # If it timed out, attempt fine-grained per-parameter fallback
        timed_out = not success and "Timed out" in msg
        if timed_out and meta_runner_script:
            print(f"  [Fallback] Action {action_id} timed out. Attempting per-parameter isolation...")
            meta_temp = str(pathlib.Path(temp_dir) / f"{action_id}_meta.json")
            meta_cmd = self.build_worker_cmd(meta_runner_script, action_id, meta_temp)
            meta_ok, _, _, _ = run_worker_process(meta_cmd, timeout_seconds=6.0)

            param_keys = []
            if meta_ok and os.path.exists(meta_temp):
                try:
                    meta_data = json.loads(pathlib.Path(meta_temp).read_text(encoding="utf-8"))
                    param_keys = [p.get("key") for p in meta_data.get("parameters", []) if p.get("key")]
                except Exception:
                    pass

            survived_params = []
            timed_out_keys = []
            param_defaults = {}

            if param_keys:
                for pk in param_keys:
                    p_temp = str(pathlib.Path(temp_dir) / f"{action_id}_{pk}.json")
                    p_cmd = self.build_worker_cmd(runner_script, action_id, p_temp, param_key=pk)
                    p_ok, p_msg, p_el, _ = run_worker_process(p_cmd, self.param_timeout)
                    if p_ok and os.path.exists(p_temp):
                        try:
                            p_data = json.loads(pathlib.Path(p_temp).read_text(encoding="utf-8"))
                            for p in p_data.get("parameters", []):
                                if p.get("key") == pk:
                                    survived_params.append(p)
                            param_defaults.update(p_data.get("defaults", {}))
                            print(f"    param {pk}: OK ({p_el:.2f}s)")
                        except Exception:
                            timed_out_keys.append(pk)
                    else:
                        print(f"    param {pk}: TIMEOUT ({p_el:.2f}s)")
                        timed_out_keys.append(pk)

                return {
                    "identifier": action_id,
                    "status": "timeout",
                    "error": msg,
                    "parameters": survived_params,
                    "timedOutKeys": timed_out_keys,
                    "defaults": param_defaults,
                    "fallbackExecuted": True
                }, elapsed

        status = "timeout" if timed_out else "error"
        return {
            "identifier": action_id,
            "status": status,
            "error": msg,
            "parameters": []
        }, elapsed

    def update_aggregate_structures(self, result):
        ident = result.get("identifier")
        status = result.get("status", "unknown")
        is_missing = result.get("isMissing", False)

        if is_missing or status == "unavailable":
            self.unavailable_actions.add(ident)

        if result.get("outputName"):
            self.action_output_names[ident] = result["outputName"]

        if result.get("defaults"):
            self.action_defaults[ident] = result["defaults"]

        params = result.get("parameters", [])
        if params:
            self.action_parameters[ident] = params

        for p in params:
            pc = p.get("class")
            sc = p.get("singleStateClass")
            dflt = p.get("default")
            if not pc:
                continue
            entry = self.parameter_classes.setdefault(pc, {
                "stateClass": sc,
                "count": 0,
                "defaultExamples": [],
                "usedBy": []
            })
            entry["count"] += 1
            if sc and not entry.get("stateClass"):
                entry["stateClass"] = sc
            if ident not in entry["usedBy"] and len(entry["usedBy"]) < 5:
                entry["usedBy"].append(ident)
            if dflt is not None and len(entry["defaultExamples"]) < 3:
                try:
                    dumped = json.dumps(dflt, sort_keys=True)
                    if not any(json.dumps(ex, sort_keys=True) == dumped for ex in entry["defaultExamples"]):
                        entry["defaultExamples"].append(dflt)
                except Exception:
                    pass

    def run(self, all_actions):
        shard_actions = get_shard_actions(all_actions, self.shard_index, self.shard_count)
        total_actions = len(shard_actions)
        print(f"=== Parameter Shard {self.shard_index + 1}/{self.shard_count} ===")
        print(f"Total actions in shard: {total_actions} (from {len(all_actions)} global actions)")

        already_done = self.load_checkpoint()
        if already_done:
            print(f"Resuming: {len(already_done)} actions already recorded in checkpoint.")

        with tempfile.TemporaryDirectory() as temp_dir:
            runner_script = None
            meta_runner_script = None
            if not self.mock_worker:
                runner_script = prepare_jxa_runner(self.prelude_path, self.probe_script, temp_dir)
                meta_runner_script = prepare_jxa_runner(self.prelude_path, self.meta_script, temp_dir)

            shard_start = time.time()
            success_count = 0
            timeout_count = 0
            error_count = 0

            for i, action_id in enumerate(shard_actions, 1):
                if action_id in already_done:
                    print(f"[{self.shard_index}/{self.shard_count}] [{i}/{total_actions}] {action_id} SKIPPED (checkpoint)")
                    continue

                res, elapsed = self.probe_single_action(runner_script, meta_runner_script, action_id, temp_dir)
                status = res.get("status", "unknown")

                if status == "success":
                    success_count += 1
                    status_str = f"OK ({elapsed:.2f}s)"
                elif status == "timeout":
                    timeout_count += 1
                    status_str = f"TIMEOUT ({elapsed:.2f}s)"
                    self.timeouts.append({
                        "identifier": action_id,
                        "elapsedSeconds": elapsed,
                        "timedOutKeys": res.get("timedOutKeys", []),
                        "error": res.get("error")
                    })
                elif status == "unavailable":
                    status_str = f"UNAVAILABLE ({elapsed:.2f}s)"
                else:
                    error_count += 1
                    status_str = f"ERROR ({elapsed:.2f}s) - {res.get('error', '')[:80]}"
                    self.errors.append({
                        "identifier": action_id,
                        "elapsedSeconds": elapsed,
                        "error": res.get("error")
                    })

                print(f"[{self.shard_index}/{self.shard_count}] [{i}/{total_actions}] {action_id} {status_str}")

                self.update_aggregate_structures(res)
                self.completed_actions[action_id] = {
                    "status": status,
                    "elapsedSeconds": elapsed,
                    "parametersCount": len(res.get("parameters", [])),
                    "error": res.get("error")
                }

                # Flush checkpoint
                self.record_checkpoint({
                    "action": action_id,
                    "status": status,
                    "elapsedSeconds": elapsed,
                    "parametersCount": len(res.get("parameters", [])),
                    "timedOutKeys": res.get("timedOutKeys", [])
                })

            total_elapsed = time.time() - shard_start

            summary = {
                "shardIndex": self.shard_index,
                "shardCount": self.shard_count,
                "totalAssigned": total_actions,
                "completed": len(self.completed_actions),
                "successes": success_count,
                "timeouts": timeout_count,
                "errors": error_count,
                "unavailable": len(self.unavailable_actions),
                "parameterClasses": len(self.parameter_classes),
                "elapsedSeconds": round(total_elapsed, 2)
            }

            output_data = {
                "shardIndex": self.shard_index,
                "shardCount": self.shard_count,
                "actionsAttempted": total_actions,
                "parameterClasses": self.parameter_classes,
                "actionParameters": self.action_parameters,
                "actionOutputNames": self.action_output_names,
                "actionDefaults": self.action_defaults,
                "unavailableActions": sorted(self.unavailable_actions),
                "actionStatuses": self.completed_actions,
                "timeouts": self.timeouts,
                "errors": self.errors,
                "summary": summary
            }

            # Write atomically to output json
            temp_out = self.output_json_path.with_suffix(".tmp.json")
            temp_out.write_text(json.dumps(output_data, indent=2, sort_keys=True), encoding="utf-8")
            temp_out.replace(self.output_json_path)

            self.summary_json_path.write_text(json.dumps(summary, indent=2, sort_keys=True), encoding="utf-8")
            print(f"Shard {self.shard_index} complete: {json.dumps(summary)}")
            return output_data


def main():
    parser = argparse.ArgumentParser(description="Fault-tolerant sharded parameter encodings extractor")
    parser.add_argument("--shard-index", type=int, required=True, help="0-based shard index")
    parser.add_argument("--shard-count", type=int, required=True, help="Total number of shards")
    parser.add_argument("--actions-file", help="Path to JSON or text file containing action identifiers")
    parser.add_argument("--out-dir", default="out/data", help="Output directory")
    parser.add_argument("--checkpoint-file", help="Path to checkpoint.jsonl file")
    parser.add_argument("--action-timeout", type=float, default=20.0, help="Per-action timeout in seconds")
    parser.add_argument("--param-timeout", type=float, default=8.0, help="Per-parameter timeout in seconds for fallback")
    parser.add_argument("--resume", action="store_true", help="Resume from checkpoint file")
    parser.add_argument("--prelude-path", help="Path to jxa-prelude.js")
    parser.add_argument("--probe-script", help="Path to probe_single_action.js")
    parser.add_argument("--meta-script", help="Path to inspect_actions_meta.js")
    parser.add_argument("--mock-worker", help="Path to mock worker script for tests")
    args = parser.parse_args()

    all_actions = load_actions(args.actions_file)
    if not all_actions:
        print(f"Error: No actions loaded from {args.actions_file}", file=sys.stderr)
        sys.exit(1)

    extractor = ParameterExtractor(args)
    extractor.run(all_actions)


if __name__ == "__main__":
    main()
