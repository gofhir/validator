#!/usr/bin/env python3
"""Compara los errores de HL7 y de gofhir por instancia. Uso: python3 -I compare.py <versión>"""
import json, sys
v = sys.argv[1]; h = {}
for e in json.load(open(f"hl7-{v}-7.0.4.json"))["entry"]:
    o = e["resource"]; n = [x["valueString"] for x in o["extension"] if x["url"].endswith("file")][0].rsplit("/", 1)[-1]
    h[n] = [((i.get("expression") or [""])[0], i["details"]["text"]) for i in o["issue"] if i["severity"] in ("error", "fatal")]
g = json.load(open(f"gofhir-ba63bba-{v}.json"))
for n in sorted(h):
    a, b = h[n], g.get(n, [])
    st = "IGUAL" if len(a) == len(b) else ("FALTA" if len(a) > len(b) else "SOBRA")
    print(f"{st:5} {n[:-5]:16} hl7={len(a)} go={len(b)}")
    if st != "IGUAL":
        for x in a: print("     H", x[0], "|", x[1][:95])
        for x in b: print("     G", x[0], "|", x[1][:95])
