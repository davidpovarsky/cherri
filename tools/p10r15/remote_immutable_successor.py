"""Advance an already immutable Remote Builder without historical P10 templates.

The predecessor receipt and current installed bytes are verified by
remote_stage_baseline before this renderer runs. This module does not grant
release acceptance and never reads or writes the installed tree.
"""
import hashlib

from remote_release_gates import TOOL_PATHS, is_hash, require, validate_tuple


def render_immutable_successor(templates, protected, previous_tuple, next_tuple,
                               tool_hashes, reader):
    validate_tuple(previous_tuple)
    validate_tuple(next_tuple)
    require(set(templates) == {"runner", "legacy"},
            "Incomplete installed immutable templates")
    require(set(tool_hashes) == TOOL_PATHS and
            all(is_hash(value) and protected.get(name) == value
                for name, value in tool_hashes.items()),
            "Installed immutable toolchain binding differs")
    for role in ("runner", "legacy"):
        text = templates[role]
        relative = "bin/cherri-remote-" + role + ".sh"
        require(isinstance(text, str) and
                hashlib.sha256(text.encode("utf-8")).hexdigest() == protected.get(relative),
                "Installed immutable wrapper digest differs: " + role)

    old_reader = reader(previous_tuple, tool_hashes)
    new_reader = reader(next_tuple, tool_hashes)
    runner = templates["runner"]
    require(runner.count(old_reader) == 1,
            "Installed immutable reader is missing, changed or duplicated")
    runner = runner.replace(old_reader, new_reader, 1)
    runner_sha = hashlib.sha256(runner.encode("utf-8")).hexdigest()
    old_label, label = previous_tuple["label"], next_tuple["label"]
    old_branch, branch = previous_tuple["sourceBranch"], next_tuple["sourceBranch"]
    old_runner_sha = protected["bin/cherri-remote-runner.sh"]
    replacements = (
        ('COMPILER="$BASE/' + old_label + '/runtime/bin/cherri"',
         'COMPILER="$BASE/' + label + '/runtime/bin/cherri"'),
        ('EXPECTED_RUNNER="' + old_runner_sha + '"',
         'EXPECTED_RUNNER="' + runner_sha + '"'),
        ("if x.get('label')!='" + old_label +
         "' or x.get('sourceBranch')!='" + old_branch + "':",
         "if x.get('label')!='" + label +
         "' or x.get('sourceBranch')!='" + branch + "':"),
        ("   printf 'CHERRI_REMOTE_SOURCE_BRANCH=" + old_branch + "\\n'",
         "   printf 'CHERRI_REMOTE_SOURCE_BRANCH=" + branch + "\\n'"),
        ('out="$BASE/outputs/${name}-' + old_label + '-legacy-${stamp}.shortcut"',
         'out="$BASE/outputs/${name}-' + label + '-legacy-${stamp}.shortcut"'),
        ("printf 'CHERRI_BRANCH=" + old_branch + "\\n'",
         "printf 'CHERRI_BRANCH=" + branch + "\\n'"),
    )
    lines = templates["legacy"].splitlines(keepends=True)
    for old, new in replacements:
        matches = [i for i, line in enumerate(lines) if line == old + "\n"]
        require(len(matches) == 1,
                "Installed immutable Legacy binding is missing, changed or duplicated")
        lines[matches[0]] = new + "\n"
    legacy = "".join(lines)
    return {"runner": runner, "legacy": legacy}
