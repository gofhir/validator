"""Adversarial SD parser: each rule is an assumption of Plan A / Plan B."""
import json, os, re, sys, collections

roots = sys.argv[1:]
SDS = []  # (pkg, sd)
for root in roots:
    for dp, _, fs in os.walk(root):
        for f in fs:
            if not (f.startswith('StructureDefinition') and f.endswith('.json')):
                continue
            try:
                sd = json.load(open(os.path.join(dp, f)))
            except Exception:
                continue
            if sd.get('resourceType') != 'StructureDefinition':
                continue
            rel = os.path.relpath(dp, root).split(os.sep)[0]
            SDS.append((rel, sd))

# de-duplicate by (url, version)
seen = set(); uniq = []
for pkg, sd in SDS:
    k = (sd.get('url'), sd.get('version'))
    if k in seen:
        continue
    seen.add(k); uniq.append((pkg, sd))
SDS = uniq
URLS = {sd.get('url') for _, sd in SDS}
URLV = {f"{sd.get('url')}|{sd.get('version')}" for _, sd in SDS}
BYURL = {sd.get('url'): sd for _, sd in SDS}

R = collections.defaultdict(list)
def hit(rule, pkg, sd, detail):
    R[rule].append(f"{pkg} :: {sd.get('id')} :: {detail}")

def parent_id(i):
    last = i.rsplit('.', 1)[-1] if '.' in i else i
    if ':' in last:
        return i[: i.rfind(':')]           # slice -> its base element
    return i.rsplit('.', 1)[0] if '.' in i else None

def canon_resolvable(c):
    u = c.split('|')[0]
    return c in URLV or u in URLS

for pkg, sd in SDS:
    snap = sd.get('snapshot', {}).get('element', [])
    if not snap:
        continue
    constraint = sd.get('derivation') == 'constraint'
    ids = [e.get('id') for e in snap]
    byid = {}
    for n, e in enumerate(snap):
        i = e.get('id')
        if i in byid:
            hit('R4 duplicate id', pkg, sd, i)
        byid[i] = (n, e)

    for n, e in enumerate(snap):
        i = e['id']
        p = parent_id(i)
        # R1 orphan
        if p and p not in byid:
            hit('R1 orphan (parent id missing)', pkg, sd, f"{i} -> {p}")
        # R5 order: parent before child
        if p and p in byid and byid[p][0] > n:
            hit('R5 child before parent', pkg, sd, i)
        sn = e.get('sliceName')
        if sn:
            last = i.rsplit('.', 1)[-1]
            idslice = last.split(':', 1)[1] if ':' in last else None
            if idslice != sn:
                hit('R3 sliceName != id slice segment', pkg, sd, f"{i} sliceName={sn}")
            base = byid.get(p, (None, {}))[1]
            if not base.get('slicing') and '/' not in (idslice or ''):
                hit('R2 slice whose base has no slicing', pkg, sd, i)
            if sn.startswith('@'):
                hit('R14 @default/@ slice (R5)', pkg, sd, i)
            if e.get('sliceIsConstraining'):
                hit('R14 sliceIsConstraining (R5)', pkg, sd, i)
            # R19 slice with own children AND type.profile (Plan A prec.1 vs Plan B profile-at-node)
            kids = [x for x in ids if x.startswith(i + '.') and x.count('.') == i.count('.') + 1]
            profs = [pr for t in e.get('type', []) for pr in t.get('profile', [])]
            if kids and profs:
                constrained = [k for k in kids if any(key.startswith(('fixed', 'pattern')) for key in byid[k][1])
                               or byid[k][1].get('min', 0) > 0 or byid[k][1].get('max') == '0']
                hit('R19 slice has inline children AND type.profile', pkg, sd,
                    f"{i} profile={profs[0].split('/')[-1]} constrainedKids={len(constrained)}")
        # R6 contentReference
        cr = e.get('contentReference')
        if cr:
            if not cr.startswith('#'):
                hit('R6 absolute contentReference', pkg, sd, f"{i} -> {cr}")
            tgt = cr.split('#', 1)[1]
            if tgt not in byid:
                hit('R6 contentReference target missing in snapshot', pkg, sd, f"{i} -> {cr}")
            elif ':' in tgt:
                hit('R6 contentReference target inside a slice', pkg, sd, f"{i} -> {cr}")
        # R8 / R7 slicing
        sl = e.get('slicing')
        if sl:
            if sl.get('ordered'):
                hit('R8 ordered slicing', pkg, sd, i)
            if sl.get('rules') == 'openAtEnd':
                hit('R8 openAtEnd', pkg, sd, i)
            if not sl.get('discriminator'):
                hit('R7 slicing without discriminator', pkg, sd, i)
            for d in sl.get('discriminator', []):
                dp_, dt = d.get('path', ''), d.get('type')
                if dt == 'position':
                    hit('R14 position discriminator (R5)', pkg, sd, i)
                if dt == 'pattern':
                    R['R7 type=pattern (count)'].append('')
                if dt == 'profile':
                    R['R7 type=profile (count)'].append('')
                if dt == 'exists':
                    R['R7 type=exists (count)'].append('')
                if 'resolve()' in dp_:
                    hit('R7 path uses resolve()', pkg, sd, f"{i} {dt}:{dp_}")
                if 'extension(' in dp_:
                    hit("R7 path uses extension('url')", pkg, sd, f"{i} {dt}:{dp_}")
                if 'ofType(' in dp_:
                    hit('R7 path uses ofType()', pkg, sd, f"{i} {dt}:{dp_}")
                if re.search(r'\w\(', dp_.replace('resolve()', '').replace('extension(', '').replace('ofType(', '')):
                    hit('R7 path uses other function', pkg, sd, f"{i} {dt}:{dp_}")
            # R17 D5: several slicing entries with the same path in one SD
        # R9/R10 type profiles
        for t in e.get('type', []):
            ps = t.get('profile', [])
            if len(ps) > 1:
                hit('R9 multiple type.profile (any-of)', pkg, sd, f"{i} {t.get('code')} x{len(ps)}")
            for pr in ps + t.get('targetProfile', []):
                if not canon_resolvable(pr):
                    R['R10 unresolvable profile/targetProfile'].append(f"{pkg} :: {pr}")
                elif '|' in pr and pr not in URLV:
                    hit('R18 versioned canonical, only another version loaded', pkg, sd, f"{i} -> {pr}")
        # R12 D1-class: required child of optional parent (slices only, constraint profiles)
        if constraint and ':' in i and e.get('min', 0) >= 1 and p and ':' not in i.rsplit('.', 1)[-1]:
            pe = byid.get(p, (None, {}))[1]
            if pe and pe.get('min', 0) == 0 and pe.get('max') != '0':
                R['R12 required child of optional parent inside a slice'].append(f"{pkg} :: {sd.get('id')} :: {i}")
            if pe and pe.get('max') == '0':
                hit('R12b required child of PROHIBITED parent', pkg, sd, i)
        # R21 renamed choice in snapshot id
        for seg in i.split('.')[1:]:
            b = seg.split(':')[0]
            if re.fullmatch(r'[a-z]+[A-Z]\w*', b):
                cand = re.sub(r'[A-Z]\w*$', '', b)
                prefix = i[: i.find(seg)]
                if (prefix + cand + '[x]') in byid:
                    hit('R21 renamed choice id in SNAPSHOT', pkg, sd, i)
                    break

    # R17 multiple slicing entries sharing a path
    cnt = collections.Counter(e['path'] for e in snap if e.get('slicing'))
    for pth, c in cnt.items():
        if c > 1:
            hit('R17 several slicings share one path (D5)', pkg, sd, f"{pth} x{c}")

    # R13 choice type-slicing: sliceName vs base+Type
    for e in snap:
        if e.get('sliceName') and e['path'].endswith('[x]'):
            base = e['path'].rsplit('.', 1)[-1][:-3]
            codes = [t.get('code', '') for t in e.get('type', [])]
            exp = [base + c[:1].upper() + c[1:] for c in codes]
            if len(codes) != 1:
                hit('R13 choice slice with != 1 type', pkg, sd, f"{e['id']} {codes}")
            elif e['sliceName'] not in exp:
                hit('R13 choice slice named differently from its type', pkg, sd, f"{e['id']} expected {exp}")

    # R15 value/pattern discriminator resolvability per slice
    for e in snap:
        sl = e.get('slicing')
        if not sl:
            continue
        slices = [x for x in snap if x.get('sliceName') and parent_id(x['id']) == e['id']]
        for d in sl.get('discriminator', []):
            if d.get('type') not in ('value', 'pattern'):
                continue
            path = d.get('path', '')
            for s in slices:
                ok = False
                if path == '$this':
                    ok = any(k.startswith(('fixed', 'pattern')) for k in s) or bool(
                        [pr for t in s.get('type', []) for pr in t.get('profile', [])])
                    # inline children with fixed/pattern also define $this
                    ok = ok or any(x['id'].startswith(s['id'] + '.') and any(k.startswith(('fixed', 'pattern')) for k in x) for x in snap)
                elif re.fullmatch(r'[\w.\[\]]+', path):
                    tid = s['id'] + '.' + path
                    te = byid.get(tid, (None, None))[1]
                    if te and any(k.startswith(('fixed', 'pattern')) for k in te):
                        ok = True
                    else:
                        for t in s.get('type', []):
                            for pr in t.get('profile', []):
                                psd = BYURL.get(pr.split('|')[0])
                                if psd:
                                    tp = psd.get('type', '') + '.' + path
                                    for pe in psd.get('snapshot', {}).get('element', []):
                                        if pe['id'] == tp and any(k.startswith(('fixed', 'pattern')) for k in pe):
                                            ok = True
                else:
                    ok = None  # function path: not statically decidable here
                if ok is False:
                    hit('R15 slice with no resolvable discriminator value', pkg, sd, f"{s['id']} {d.get('type')}:{path}")

# R11 cycles in the type.profile graph
G = collections.defaultdict(set)
for pkg, sd in SDS:
    for e in sd.get('snapshot', {}).get('element', []):
        for t in e.get('type', []):
            for pr in t.get('profile', []):
                u = pr.split('|')[0]
                if u != sd.get('url') or e['id'] != sd.get('type'):
                    G[sd.get('url')].add(u)
cycles = set()
def dfs(u, stack, onstack):
    for v in G.get(u, ()):
        if v in onstack:
            cyc = stack[stack.index(v):] + [v]
            cycles.add(' -> '.join(x.split('/')[-1] for x in cyc))
        elif len(stack) < 12 and v in G:
            stack.append(v); onstack.add(v); dfs(v, stack, onstack); onstack.discard(v); stack.pop()
for u in list(G):
    dfs(u, [u], {u})
for c in sorted(cycles)[:50]:
    R['R11 type.profile cycle'].append(c)

print(f"SDs analysed: {len(SDS)} (with snapshot: {sum(1 for _, s in SDS if s.get('snapshot'))})")
for rule in sorted(R):
    v = R[rule]
    print(f"\n### {rule}: {len(v)}")
    ex = [x for x in v if x][:6]
    for x in ex:
        print('   ', x)
