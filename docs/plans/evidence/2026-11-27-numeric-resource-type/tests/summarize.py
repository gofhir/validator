"""Resume las salidas de HL7 7.0.4 en expected.json: por caso y versión, las issues sobre resourceType."""
import json, sys
out = {}
for v in ["4.0.1", "4.3.0", "5.0.0"]:
    d = json.load(open(f"hl7-{v}.json"))
    for e in d["entry"]:
        oo = e["resource"]
        src = next((x.get("valueString") for x in oo.get("extension", []) if x["url"].endswith("operationoutcome-file")), None)
        case = src.split("/")[-1].removesuffix(".json")
        iss = []
        for i in oo.get("issue", []):
            txt = i.get("details", {}).get("text", "")
            mid = next((x.get("valueCode") for x in i.get("extension", []) if x["url"].endswith("operationoutcome-message-id")), "")
            loc = (i.get("expression") or i.get("location") or [""])[0]
            if i["severity"] in ("fatal", "error"):
                iss.append({"severity": i["severity"], "id": mid, "expression": loc, "text": txt})
        out.setdefault(case, {})[v] = iss
json.dump(out, open("expected.json", "w"), indent=1, sort_keys=True)
for c in sorted(out):
    same = all(out[c][v] == out[c]["4.0.1"] for v in out[c])
    print(c, "same-across-versions" if same else "DIFFERS", [(i["severity"], i["id"], i["expression"]) for i in out[c]["4.0.1"]])
