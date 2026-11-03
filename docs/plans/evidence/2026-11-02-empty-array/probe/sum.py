# Resume los errores de HL7 (Bundle de OperationOutcome) y de gofhir (salida JSON del CLI).
import json, sys, os
def hl7(path, v):
    d = json.load(open(path))
    for e in d["entry"]:
        oo = e["resource"]
        name = "?"
        for x in oo.get("extension", []):
            if x["url"].endswith("operationoutcome-file"):
                name = os.path.basename(x["valueString"])
        print("==", v, name)
        for i in oo.get("issue", []):
            if i["severity"] in ("error", "fatal"):
                loc = (i.get("expression") or i.get("location") or ["?"])[0]
                print("  ", loc, "|", i["code"], "|", i.get("details", {}).get("text", "")[:110])
def gofhir(path, v):
    s = open(path).read()
    for r in json.loads(s[s.index("[") if s.startswith("[") else s.index("\n[") + 1:]):
        print("==", v, os.path.basename(r["resource"]))
        for i in r.get("issues") or []:
            if i["severity"] in ("error", "fatal"):
                print("  ", (i.get("expression") or ["?"])[0], "|", i["code"], "|", i["diagnostics"][:110])
{"hl7": hl7, "gofhir": gofhir}[sys.argv[1]](sys.argv[2], sys.argv[3])
