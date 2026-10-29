import json, sys, os, re
d = sys.argv[1]
for v in ["4.0.1", "4.3.0", "5.0.0"]:
    h = json.load(open(f"{d}/hl7-{v}.json"))
    hl = {}
    for e in h.get("entry", []):
        oo = e["resource"]
        f = next((x.get("valueString") for x in oo.get("extension", []) if "file" in x.get("url", "")), "?")
        hl[os.path.basename(f)] = [(i["severity"], (i.get("expression") or [""])[0], i.get("details", {}).get("text", "")[:110])
                                   for i in oo.get("issue", []) if i["severity"] in ("error", "fatal")]
    g = json.load(open(f"{d}/go-{v}.json"))
    gl = {os.path.basename(r["resource"]): [(i["severity"], (i.get("expression") or [""])[0], i.get("diagnostics", "")[:110])
                                         for i in r.get("issues") or [] if i["severity"] in ("error", "fatal")] for r in g}
    print(f"######## {v}")
    for f in sorted(set(hl) | set(gl)):
        print("==", f)
        for x in hl.get(f, []): print("  HL7", *x)
        for x in gl.get(f, []): print("  GO ", *x)
