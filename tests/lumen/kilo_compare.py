#!/usr/bin/env python3
"""Compare Kilo chat completions; never expose credentials or raw response bodies."""
import json, os, pathlib, urllib.request, urllib.error, sys
ROOT="https://ai-gateway.kilo.ai/api/v1"
KEY=os.environ.get("KILO_API_KEY","")
OUT=pathlib.Path("artifacts/lumen-kilo-diagnostics.json")
OUT.parent.mkdir(parents=True,exist_ok=True)
result={"endpoint":ROOT+"/chat/completions","models":[]}
def save():
    OUT.write_text(json.dumps(result,indent=2)+"\n")
if not KEY:
    result["error"]="missing_secret"
    save()
    print("LUMEN_DIAGNOSTICS=FAIL missing_secret")
    sys.exit(1)
try:
    with urllib.request.urlopen(ROOT+"/models",timeout=25) as r:
        catalog=json.load(r).get("data",[])
except (urllib.error.URLError,ValueError) as e:
    result["error"]="catalog_unavailable"
    save()
    print("LUMEN_DIAGNOSTICS=FAIL catalog_unavailable")
    sys.exit(1)
ids=[m.get("id","") for m in catalog if isinstance(m,dict)]
target="nvidia/nemotron-3-super-120b-a12b:free"
# Prefer published zero-priced chat models from different providers.
models=[target]
for prefix in ("google/","meta-llama/","qwen/","deepseek/","openai/"):
    choices=[m for m in catalog if isinstance(m,dict) and str(m.get("id","")).startswith(prefix) and str(m.get("id","")).endswith(":free")]
    if choices:
        models.append(choices[0]["id"])
    if len(models)>=5:
        break
print("catalog_models="+str(len(ids)))
for model in models:
    row={"model":model,"listed":model in ids,"status":None,"response_kind":None}
    result["models"].append(row)
    if not row["listed"]:
        continue
    body=json.dumps({"model":model,"messages":[{"role":"user","content":"Reply PONG."}],"max_tokens":64,"stream":False}).encode()
    req=urllib.request.Request(ROOT+"/chat/completions",data=body,headers={"Authorization":"Bearer "+KEY,"Content-Type":"application/json"},method="POST")
    try:
        with urllib.request.urlopen(req,timeout=80) as resp:
            row["status"]=resp.status
            obj=json.load(resp)
            choices=obj.get("choices",[])
            row["response_kind"]="assistant_text" if choices and isinstance((choices[0].get("message") or {}).get("content"),str) and (choices[0].get("message") or {}).get("content").strip() else "no_text"
    except urllib.error.HTTPError as e:
        row["status"]=e.code
        try:
            obj=json.loads(e.read(8192))
            err=obj.get("error",{})
            row["error_code"]=str(err.get("code",""))[:80] if isinstance(err,dict) else ""
            row["error_type"]=str(err.get("type",""))[:80] if isinstance(err,dict) else ""
        except (ValueError,UnicodeError):
            pass
        row["response_kind"]="http_error"
    except (urllib.error.URLError,ValueError):
        row["response_kind"]="network_or_json_error"
    print("model="+model+" status="+str(row["status"])+" result="+str(row["response_kind"]))
save()
ok=any(r["response_kind"]=="assistant_text" for r in result["models"])
print("LUMEN_DIAGNOSTICS="+("PASS" if ok else "FAIL"))
sys.exit(0 if ok else 1)
