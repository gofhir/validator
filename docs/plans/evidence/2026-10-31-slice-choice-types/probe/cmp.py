"""Compara por sonda los issues error, fatal y warning de HL7 y de gofhir en una versión.
Uso: python3 -I cmp.py <dir> <versión>. Falla si una sonda de in/ no tiene resultado en alguno."""
import json, pathlib, sys

d, v = pathlib.Path(sys.argv[1]), sys.argv[2]
probes = sorted(p.stem for p in (d / "in").glob("*.json"))
go = {}
for r in json.load(open(d / f"go-{v}.json")):
    name = r["resource"].split("/")[-1].removesuffix(".json")
    go[name] = [(i.get("severity"), (i.get("expression") or [""])[0], (i.get("diagnostics") or "")[:160])
                for i in r.get("issues") or [] if i.get("severity") in ("error", "fatal", "warning")]
hl7 = {}
for e in json.load(open(d / f"hl7-{v}.json")).get("entry", []):
    oo = e["resource"]
    src = next((x["valueString"] for x in oo.get("extension", []) if x["url"].endswith("operationoutcome-file")), "?")
    hl7[pathlib.Path(src).stem] = [(i.get("severity"), (i.get("expression") or [""])[0], (i.get("details", {}).get("text") or "")[:160])
                                   for i in oo.get("issue", []) if i.get("severity") in ("error", "fatal", "warning")]
missing = [p for p in probes if p not in go or p not in hl7]
for n in probes:
    print("##", n)
    for i in hl7.get(n, []):
        print("  HL7", *i, sep=" | ")
    for i in go.get(n, []):
        print("  GO ", *i, sep=" | ")
if missing:
    sys.exit(f"missing results: {missing}")
