# Errores de HL7 por caso: un Bundle de OperationOutcome por versión.
import json,sys,os,re
S=sys.argv[1]
for v in ("4.0.1","4.3.0","5.0.0"):
    b=json.load(open(f"{S}/hl7-{v}.json"))
    for e in b["entry"]:
        oo=e["resource"]; fn=[x["valueString"] for x in oo.get("extension",[]) if x["url"].endswith("operationoutcome-file")][0]
        errs=[f'{",".join(i.get("expression",[]))} | {re.sub("http://example.org/StructureDefinition/","",i["details"]["text"])[:110]}' for i in oo["issue"] if i["severity"] in("error","fatal")]
        print(f"{v} {os.path.basename(fn)[:-5]}")
        for l in errs: print("    "+l)
