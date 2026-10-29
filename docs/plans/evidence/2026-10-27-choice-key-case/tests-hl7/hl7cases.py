import json, os, sys
d = sys.argv[1]
for v in ["4.0.1", "4.3.0", "5.0.0"]:
    h = json.load(open(f"{d}/hl7-{v}.json"))
    ents = [e["resource"] for e in h["entry"]] if h["resourceType"] == "Bundle" else [h]
    print("########", v)
    rows = []
    for oo in ents:
        f = [x.get("valueString") for x in oo.get("extension", []) if "file" in x.get("url", "")]
        name = os.path.basename(f[0]) if f else "?"
        iss = [((i.get("expression") or [""])[0], i.get("details", {}).get("text", "")[:120]) for i in oo.get("issue", []) if i["severity"] in ("error", "fatal")]
        rows.append((name, iss))
    for name, iss in sorted(rows):
        print("==", name)
        for x in iss: print("  ", *x)
