# Resume por caso los issues de gofhir (lista JSON del CLI) y de HL7 (Bundle de OperationOutcome).
import json,sys,os,re
P=sys.argv[1]; v=sys.argv[2]
def short(s): return re.sub(r'http://example.org/StructureDefinition/','',s)[:150]
out={}
for r in json.load(open(f"{P}/go-{v}.json")):
    n=os.path.basename(r["resource"])[:-5]
    out.setdefault(n,{})["go"]=[f'{i["severity"][0].upper()} {",".join(i.get("expression",[]))} | {short(i["diagnostics"])}' for i in r["issues"] if "dom-6" not in i["diagnostics"]]
hv={"4.0.1":"r4","5.0.0":"r5"}[v]
b=json.load(open(f"{P}/hl7-{hv}.json"))
for e in b.get("entry",[]):
    oo=e["resource"]; fn=None
    for x in oo.get("extension",[]):
        if x["url"].endswith("operationoutcome-file"): fn=x["valueString"]
    n=os.path.basename(fn)[:-5]
    out.setdefault(n,{})["hl7"]=[f'{i["severity"][0].upper()} {",".join(i.get("expression",[]))} | {short(i.get("details",{}).get("text",""))}' for i in oo.get("issue",[]) if "dom-6" not in i.get("details",{}).get("text","") and "All OK" not in i.get("details",{}).get("text","")]
for n in sorted(out):
    print(f"== {n}")
    for k in ("hl7","go"):
        for l in out[n].get(k,[]): print(f"  {k:3} {l}")
