#!/usr/bin/env python3
# Clasifica las diferencias de `hl7diff same` (same-base.jsonl y same-head.jsonl de cada grupo) en dos clases:
# las que solo difieren por el texto «(defined in …)» de una constraint (punto 8 del plan) y el resto, que se
# listan completas. Uso: same_classify.py <dir de trabajo de hl7diff same>
import collections, json, os, re, sys

DEF = re.compile(r" \(defined in [^)]*\)")
def lines(p):
    return collections.Counter(open(p).read().splitlines())
def strip(l):
    d = json.loads(l); d["diagnostics"] = DEF.sub("", d.get("diagnostics", "")); return json.dumps(d, sort_keys=True)

root = sys.argv[1]
tot_def = 0
for g in sorted(os.listdir(root)):
    b, h = os.path.join(root, g, "same-base.jsonl"), os.path.join(root, g, "same-head.jsonl")
    if not os.path.exists(b): continue
    cb, ch = lines(b), lines(h)
    if cb == ch: continue
    lost, new = cb - ch, ch - cb
    sl = collections.Counter(); sn = collections.Counter()
    for l, n in lost.items(): sl[strip(l)] += n
    for l, n in new.items(): sn[strip(l)] += n
    only_def = sum((sl & sn).values())
    tot_def += only_def
    rl, rn = sl - sn, sn - sl
    print(f"## {g}: solo «defined in» {only_def}, perdidas {sum(rl.values())}, nuevas {sum(rn.values())}")
    for l in sorted(rl): print("  -", l)
    for l in sorted(rn): print("  +", l)
print("total solo «defined in»:", tot_def)
