#!/usr/bin/env python3
"""Cuenta, en los snapshots de los packages core, los hijos de un slice que redeclaran con otro valor lo que declara el
mismo hijo del elemento dividido. Uso: python3 -I core_scan.py"""
import glob, json, os, re
for p in ("hl7.fhir.r4.core#4.0.1", "hl7.fhir.r4b.core#4.3.0", "hl7.fhir.r5.core#5.0.0"):
    under = differ = bind = 0; ex = []
    for f in glob.glob(os.path.expanduser(f"~/.fhir/packages/{p}/package/StructureDefinition-*.json")):
        d = json.load(open(f)); els = d.get("snapshot", {}).get("element", [])
        ids = {e["id"]: e for e in els if "id" in e}
        for i, e in ids.items():
            if ":" not in i.rsplit(".", 1)[0]: continue
            fx = {k: v for k, v in e.items() if k.startswith(("fixed", "pattern"))}
            b = ids.get(re.sub(r":[^.]+", "", i))
            if fx: under += 1
            if not b: continue
            bx = {k: v for k, v in b.items() if k.startswith(("fixed", "pattern"))}
            if bx and bx != fx: differ += 1; ex.append((d["id"], i))
            if b.get("binding", {}).get("valueSet") and e.get("binding", {}).get("valueSet") != b["binding"]["valueSet"]:
                bind += 1; ex.append((d["id"], i, "binding"))
    print(p, "fixed/pattern bajo slices:", under, "| fixed/pattern distinto de la base:", differ, "| binding distinto:", bind, ex[:4])
