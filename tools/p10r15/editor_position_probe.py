#!/usr/bin/env python3
"""Run the actual Foundation position helper; no SwiftUI/C ABI/Apple claim."""
import argparse
import hashlib
import json
from pathlib import Path
import shutil
import sys

from editor_publication_probe import invoke, successful, completion_types

NAMES = ("start", "hebrew_end", "astral_end", "crlf_next_line", "woman_end",
         "zwj_end", "laptop_end", "combining_base", "combining_end",
         "no_selection", "negative_offset", "past_end", "selection_not_caret",
         "empty_start", "empty_past_end", "lf_next_line", "lf_astral_end")

def sha(data):
    return hashlib.sha256(data).hexdigest()

def inspect(data):
    rows = [json.loads(line) for line in data.decode("utf-8", errors="strict").splitlines()]
    if len(rows) != 18 or tuple(row.get("name") for row in rows[:-1]) != NAMES:
        raise ValueError("incomplete or foreign actual position control inventory")
    for row in rows[:-1]:
        if row.get("event") != "control" or row.get("result") not in {"PASS", "FAIL"} or row.get("scope") != "actual Foundation UTF16 helper; no UI" or type(row.get("observation")) is not dict:
            raise ValueError("invalid actual position control")
    summary = rows[-1]
    if summary != {"event": "summary", "controls": 17, "failed": sum(row["result"] == "FAIL" for row in rows[:-1]), "uiExecuted": False} or type(summary["failed"]) is not int or type(summary["controls"]) is not int or summary.get("uiExecuted") is not False:
        raise ValueError("position summary does not preserve actual outcome")
    return rows

def main():
    p=argparse.ArgumentParser()
    p.add_argument("--repo",type=Path,required=True)
    p.add_argument("--output",type=Path,required=True)
    args=p.parse_args()
    repo,output=args.repo.resolve(),args.output.resolve()
    output.mkdir(parents=True,exist_ok=False)
    compiler=shutil.which("swiftc")
    if not compiler:raise RuntimeError("Swift compiler unavailable; no position control executed")
    paths=["tools/p10r15/editor_position_probe.py","tools/p10r15/editor_position_probe.swift",
           "tools/p10r15/editor_publication_probe.py","ios/CherriApp/Compiler/CherriCompiler.swift",
           "ios/CherriApp/Editor/CherriCompletionRequest.swift"]
    sources={name:(repo/name).read_bytes() for name in paths}
    item=completion_types(sources[paths[3]].decode("utf-8"))
    generated=output/"position-generated.swift"
    generated.write_text("import Foundation\n"+item+"\n"+sources[paths[1]].decode("utf-8"),encoding="utf-8",newline="\n")
    binary=output/("position-probe.exe" if sys.platform=="win32" else "position-probe")
    command=[compiler,"-parse-as-library",str(generated),str(repo/paths[4]),"-o",str(binary)]
    compiled=invoke(command,repo,output,"compile",90)
    receipt={"schema_version":1,"scope":"actual Foundation UTF16 helper; no UI","uiExecuted":False,"formalRequirementsProved":[],
             "sources":[{"path":name,"bytes":len(data),"sha256":sha(data)}for name,data in sources.items()],
             "generated":{"path":generated.name,"bytes":generated.stat().st_size,"sha256":sha(generated.read_bytes())},
             "compile":compiled,"execution":None,"rows":[]}
    if successful(compiled):
        receipt["binary"]={"bytes":binary.stat().st_size,"sha256":sha(binary.read_bytes())}
        receipt["execution"]=invoke([str(binary)],repo,output,"execute",20)
        try:receipt["rows"]=inspect((output/"execute.stdout").read_bytes())
        except (ValueError,UnicodeError) as error:receipt["intake_error"]=str(error)
    receipt["sources_unchanged"]=all((repo/name).read_bytes()==data for name,data in sources.items())
    receipt["passed"]=bool(successful(compiled) and receipt["execution"] and successful(receipt["execution"]) and receipt["rows"] and receipt["rows"][-1]["failed"]==0 and receipt["sources_unchanged"])
    (output/"receipt.json").write_text(json.dumps(receipt,indent=2)+"\n",encoding="utf-8",newline="\n")
    print(json.dumps({"passed":receipt["passed"],"rows":receipt["rows"],"receipt":str(output/"receipt.json")}))
    return 0 if receipt["passed"] else 1

if __name__=="__main__":
    sys.exit(main())
