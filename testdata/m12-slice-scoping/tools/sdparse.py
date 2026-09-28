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

def fixed_or_pattern(e):
    for k, v in e.items():
        if k.startswith(('fixed', 'pattern')):
            return v
    return None


def value_contains(val, segs):
    if not segs:
        return val is not None
    if isinstance(val, list):
        return any(value_contains(x, segs) for x in val)
    if isinstance(val, dict) and segs[0] in val:
        return value_contains(val[segs[0]], segs[1:])
    return False


def value_in_tree(byid, base, path):
    segs = path.split('.')
    for n in range(len(segs), -1, -1):
        e = byid.get(base + ('.' + '.'.join(segs[:n]) if n else ''))
        if e is not None:
            v = fixed_or_pattern(e[1] if isinstance(e, tuple) else e)
            if v is not None and value_contains(v, segs[n:]):
                return True
    return False


def discriminator_source(byid, s, path):
    if not re.fullmatch(r'[\w.\[\]$]+', path):
        return 'function'
    if path == '$this':
        if fixed_or_pattern(s) is not None or any(
                k.startswith(s['id'] + '.') and fixed_or_pattern(v[1]) is not None for k, v in byid.items()):
            return 'inline'
    elif value_in_tree(byid, s['id'], path):
        return 'inline'
    unresolvable = False
    for t in s.get('type', []):
        for pr in t.get('profile', []):
            psd = BYURL.get(pr.split('|')[0])
            if psd is None:
                unresolvable = True
                continue
            pb = {x['id']: x for x in psd.get('snapshot', {}).get('element', [])}
            if path == '$this':
                if any(fixed_or_pattern(v) is not None for v in pb.values()):
                    return 'type.profile'
            elif value_in_tree(pb, psd.get('type', ''), path):
                return 'type.profile'
    return 'unresolvable' if unresolvable else 'NONE'


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
            # A missing base is R1's orphan; R2 is only an existing base without slicing.
            if p in byid and not byid[p][1].get('slicing') and '/' not in (idslice or ''):
                hit('R2 slice whose existing base has no slicing', pkg, sd, i)
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
                    R['R10 unresolvable profile/targetProfile (element references)'].append(f"{pkg} :: {pr}")
                    if pr not in R['R10b distinct unresolvable canonicals']:
                        R['R10b distinct unresolvable canonicals'].append(pr)
                elif '|' in pr and pr not in URLV:
                    hit('R18 versioned canonical, only another version loaded (element references)', pkg, sd, f"{i} -> {pr}")
                    if pr not in R['R18b distinct versioned canonicals with only another version loaded']:
                        R['R18b distinct versioned canonicals with only another version loaded'].append(pr)
        # R12 D1 triggers. slicing.go's validateSliceChildren looks up the LAST segment of every
        # descendant of the outermost enclosing slice on that slice's member, whether or not the
        # elements in between are present. A required element is falsely reported missing when
        # some element between it and that slice is optional (R12) or prohibited (R12b), unless
        # the slice has a required direct child with the same name, which is always present and
        # masks the miscount (e.g. the url of a nested extension). Direct children are counted at
        # the right level. R12c is D1b: a required value[x] anywhere below a slice is looked up by
        # the literal key "value[x]", which never exists, so it is always reported missing. R12c
        # counts resource profiles only: slicing does not run inside extension definitions today
        # (plan B, D6), so their value[x] leaves are not reached yet.
        if constraint and ':' in i and e.get('min', 0) >= 1 and p and ':' not in i.rsplit('.', 1)[-1]:
            segs = i.split('.')
            k = next(n for n, sg in enumerate(segs) if ':' in sg)
            outer = '.'.join(segs[:k + 1])
            name = segs[-1]
            entry = f"{pkg} :: {sd.get('id')} :: {i} (outermost slice {outer})"
            if name.endswith('[x]'):
                if sd.get('kind') == 'resource':
                    R['R12c D1b: required value[x] below a slice of a resource profile (literal key never found)'].append(entry)
            else:
                between = ['.'.join(segs[:m]) for m in range(len(segs) - 1, k + 1, -1)]
                defs = [byid[b][1] for b in between if b in byid]
                twin = byid.get(outer + '.' + name)
                masked = twin is not None and twin[1].get('min', 0) >= 1
                if not masked and any(d.get('max') == '0' for d in defs):
                    R['R12b D1 trigger: required element under a PROHIBITED element below a slice'].append(entry)
                elif not masked and any(d.get('min', 0) == 0 for d in defs):
                    R['R12 D1 trigger: required element under an optional element below a slice'].append(entry)
        # R21 renamed choice in a snapshot id: a segment b + Suffix where the snapshot has b[x]
        # and Suffix is the title-cased code of one of b[x]'s types (FHIR choice naming).
        segs = i.split('.')
        for k in range(1, len(segs)):
            name = segs[k].split(':')[0]
            prefix = '.'.join(segs[:k]) + '.'
            for j in range(1, len(name)):
                choice = byid.get(prefix + name[:j] + '[x]')
                if choice is None:
                    continue
                titles = {t.get('code', '')[:1].upper() + t.get('code', '')[1:] for t in choice[1].get('type', [])}
                if name[j:] in titles:
                    hit('R21 renamed choice id in SNAPSHOT', pkg, sd, i)
                    break

    # R17 multiple slicing entries sharing a path
    cnt = collections.Counter(e['path'] for e in snap if e.get('slicing'))
    shared = [(pth, c) for pth, c in cnt.items() if c > 1]
    for pth, c in shared:
        hit('R17 (SD, path) pairs where several slicings share one path (D5)', pkg, sd, f"{pth} x{c}")
    if shared:
        hit('R17b SDs with at least one path shared by several slicings (D5)', pkg, sd, f"{len(shared)} path(s)")

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

    # R15 where each value/pattern discriminator of each slice gets its value from:
    #   inline        fixed/pattern on the slice, on the discriminated element, or on an ancestor
    #                 of it inside the slice, whose value contains the rest of the path
    #   type.profile  the same, inside a profile the slice's type declares
    #   unresolvable  the slice's type.profile is not in the corpus
    #   function      the path uses a FHIRPath function (resolve(), extension(), ofType())
    #   NONE          none of the above (e.g. values reached through nested slices, or bindings)
    for e in snap:
        sl = e.get('slicing')
        if not sl:
            continue
        slices = [x for x in snap if x.get('sliceName') and parent_id(x['id']) == e['id']]
        for d in sl.get('discriminator', []):
            if d.get('type') not in ('value', 'pattern'):
                continue
            path = d.get('path', '')
            for s_ in slices:
                src = discriminator_source(byid, s_, path)
                R['R15 value source per (slice, discriminator): ' + src].append(f"{pkg} :: {sd.get('id')} :: {s_['id']} {d.get('type')}:{path}")
                entry = f"{pkg} :: {sd.get('id')} :: {s_['id']}"
                if src == 'NONE' and entry not in R['R15b distinct slices with a discriminator of unknown source']:
                    R['R15b distinct slices with a discriminator of unknown source'].append(entry)

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
    ex = [x for x in v if x][:10]
    for x in ex:
        print('   ', x)
