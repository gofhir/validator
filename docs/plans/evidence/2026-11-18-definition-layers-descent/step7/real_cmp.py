#!/usr/bin/env python3
# Compara la salida JSON de dos binarios de gofhir-validator sobre los mismos archivos: veredicto por
# archivo e informes (severidad, ubicación, mensaje) como multiconjunto. Uso: real_cmp.py nombre base.json head.json
import collections, json, sys

def load(p):
    out = {}
    for r in json.load(open(p)):
        iss = collections.Counter()
        for i in r.get("issues") or []:
            loc = ",".join(i.get("expression") or i.get("location") or [])
            iss[(i.get("severity"), loc, i.get("diagnostics") or i.get("message"))] += 1
        out[r["resource"]] = (r["valid"], iss)
    return out

name, b, h = sys.argv[1], load(sys.argv[2]), load(sys.argv[3])
assert b.keys() == h.keys(), "distintos archivos"
verd = [f for f in b if b[f][0] != h[f][0]]
new = lost = 0
samples = []
for f in sorted(b):
    n, l = h[f][1] - b[f][1], b[f][1] - h[f][1]
    new += sum(n.values()); lost += sum(l.values())
    for k in list(n)[:2]: samples.append(("+", f.rsplit("/", 1)[-1], k))
    for k in list(l)[:2]: samples.append(("-", f.rsplit("/", 1)[-1], k))
tb = sum(sum(v[1].values()) for v in b.values()); th = sum(sum(v[1].values()) for v in h.values())
print(f"{name} archivos {len(b)} issues base {tb} head {th} nuevos {new} perdidos {lost} veredictos cambiados {len(verd)}")
for f in verd: print("  veredicto", f.rsplit("/", 1)[-1], b[f][0], "->", h[f][0])
for s in samples[:30]: print("  ", *s)
