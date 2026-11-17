"""Generator of the probes for 2026-11-06-type-profile-layers (run: python3 -I gen.py).
Writes ig-<ver>/ (profiles, fhirVersion adjusted), inst/ (instances) and copies p1..p21 from the
2026-11-05 evidence. Deterministic; re-run overwrites."""
import json, os, shutil

HERE = os.path.dirname(os.path.abspath(__file__))
OLD = os.path.join(HERE, '..', '..', '2026-11-05-type-profile-alternatives', 'probe')
VERS = {'4.0.1': 'ig-4.0.1', '4.3.0': 'ig-4.3.0', '5.0.0': 'ig-5.0.0'}
NS = 'http://example.org/fhir/StructureDefinition/'
HL7 = 'http://hl7.org/fhir/StructureDefinition/'
DT = {'Quantity', 'Identifier', 'CodeableConcept', 'Reference'}

PROFILES = {}   # id -> function(ver) -> SD dict
def U(i): return NS + i

def sd(i, typ, elems, base=None, contained=None):
    def build(ver):
        d = {"resourceType": "StructureDefinition", "id": i, "url": U(i), "name": i.replace('-', '_'),
             "status": "draft", "fhirVersion": ver,
             "kind": "complex-type" if typ in DT or typ in ('string',) else "resource",
             "abstract": False, "type": typ, "baseDefinition": base or HL7 + typ,
             "derivation": "constraint", "differential": {"element": elems}}
        if contained: d["contained"] = contained
        return d
    PROFILES[i] = build
    return build

def el(path, **kw):
    d = {"id": kw.pop('id', path), "path": path}
    d.update(kw); return d

def cons(key, expr, human, sev='error'):
    return {"key": key, "severity": sev, "human": human, "expression": expr}

# ---- datatype profiles -------------------------------------------------------------------
sd('qa', 'Quantity', [el('Quantity.unit', min=1)])
sd('qb', 'Quantity', [el('Quantity.code', min=1)])
sd('qi2', 'Quantity', [el('Quantity', constraint=[cons('inv2-1', 'value > 10', 'value must exceed 10 (only in qi2)')])])
sd('qsev-a', 'Quantity', [el('Quantity', constraint=[cons('dup-1', 'value > 10', 'dup text', 'error')]), el('Quantity.unit', min=1)])
sd('qsev-b', 'Quantity', [el('Quantity', constraint=[cons('dup-1', 'value > 10', 'dup text', 'warning')]), el('Quantity.unit', min=1)])
sd('qtxt-a', 'Quantity', [el('Quantity', constraint=[cons('dup-1', 'value > 10', 'text A', 'error')]), el('Quantity.unit', min=1)])
sd('qtxt-b', 'Quantity', [el('Quantity', constraint=[cons('dup-1', 'value > 10', 'text B', 'error')]), el('Quantity.unit', min=1)])
sd('qdup-a', 'Quantity', [el('Quantity', constraint=[cons('dup-1', 'value > 10', 'dup text', 'error')]), el('Quantity.unit', min=1)])
sd('qdup-b', 'Quantity', [el('Quantity', constraint=[cons('dup-1', 'value > 10', 'dup text', 'error')]), el('Quantity.unit', min=1)])
sd('ida', 'Identifier', [el('Identifier.period', min=1)])
sd('idb', 'Identifier', [el('Identifier.value', min=1)])
sd('org-x', 'Organization', [el('Organization.active', min=1)])
sd('org-y', 'Organization', [el('Organization.contact', min=1)])
sd('idh-a', 'Identifier', [el('Identifier.assigner', type=[{"code": "Reference", "targetProfile": [U('org-x')]}])])
sd('idh-b', 'Identifier', [el('Identifier.assigner', type=[{"code": "Reference", "targetProfile": [U('org-y')]}])])
sd('qres-a', 'Quantity', [el('Quantity', constraint=[cons('ra-1', "%resource.status = 'final'", 'resource must be final (qres-a)')]),
                          el('Quantity.unit', min=1)])
sd('qres-b', 'Quantity', [el('Quantity', constraint=[cons('rb-1', "%resource.status = 'amended'", 'resource must be amended (qres-b)')])])
sd('ids-res', 'Identifier', [el('Identifier', constraint=[cons('sr-1', "%resource.resourceType = 'Observation'", 'resource must be an Observation (ids-res)')])])
sd('ids-root', 'Identifier', [el('Identifier', constraint=[cons('sk-1', "%rootResource.resourceType = 'Bundle'", 'root must be a Bundle (ids-root)')])])

def obs_value(i, profiles, typ='Quantity'):
    return sd(i, 'Observation', [el('Observation.value[x]', type=[{"code": typ, "profile": [U(p) for p in profiles]}])])

# ---- probes ------------------------------------------------------------------------------
INST = {}     # name -> (profile id or None, extra fields, description)
DESC = []     # (probe, what)
def inst(name, prof, extra, what=None, resource='Observation'):
    INST[name] = (prof, extra, resource)
    if what: DESC.append((name, what))

Q = lambda **k: dict({"value": 5}, **k)
UCUM = "http://unitsofmeasure.org"

# 2. (c) callers
sd('obs-c1', 'Observation', [
    el('Observation.identifier', slicing={"discriminator": [{"type": "value", "path": "system"}], "rules": "open"},
       type=[{"code": "Identifier", "profile": [U('ida'), U('idb')]}]),
    el('Observation.identifier', id='Observation.identifier:s1', sliceName='s1'),
    el('Observation.identifier.system', id='Observation.identifier:s1.system', min=1, fixedUri='urn:probe:s1'),
    el('Observation.identifier.value', id='Observation.identifier:s1.value', min=1)])
sd('obs-c1d', 'Observation', [
    el('Observation.identifier', slicing={"discriminator": [{"type": "value", "path": "system"}], "rules": "open"},
       type=[{"code": "Identifier", "profile": [U('ida'), U('idb')]}]),
    el('Observation.identifier', id='Observation.identifier:s1', sliceName='s1',
       type=[{"code": "Identifier", "profile": [U('ida'), U('idb')]}]),
    el('Observation.identifier.system', id='Observation.identifier:s1.system', min=1, fixedUri='urn:probe:s1'),
    el('Observation.identifier.value', id='Observation.identifier:s1.value', min=1)])
S1 = "urn:probe:s1"
for p in ('obs-c1', 'obs-c1d'):
    k = p.replace('obs-', '')
    inst(k + 'a', p, {"identifier": [{"system": S1, "value": "v"}]}, 'slice s1 min value matches 2nd layer (idb) only; value matches idb, not ida (period)')
    inst(k + 'b', p, {"identifier": [{"system": S1, "period": {"start": "2020-01-01"}}]}, 'matches ida only; slice requires value (matches idb layer only)')
    inst(k + 'c', p, {"identifier": [{"system": S1, "use": "usual"}]}, 'matches neither layer; slice requires value')
    inst(k + 'n', p, {"identifier": [{"system": "urn:probe:other", "value": "v"}]}, 'outside the slice: node layers only (idb ok)')

# 4. slice leaf and reslice with several profiles (no unfolded children)
sd('obs-leaf', 'Observation', [
    el('Observation.identifier', slicing={"discriminator": [{"type": "pattern", "path": "$this"}], "rules": "open"}),
    el('Observation.identifier', id='Observation.identifier:s2', sliceName='s2',
       type=[{"code": "Identifier", "profile": [U('ida'), U('idb')]}],
       patternIdentifier={"system": "urn:probe:s2"})])
S2 = "urn:probe:s2"
inst('leafa', 'obs-leaf', {"identifier": [{"system": S2, "value": "v"}]}, 'slice leaf [ida,idb]: matches idb only')
inst('leafb', 'obs-leaf', {"identifier": [{"system": S2, "period": {"start": "2020-01-01"}}]}, 'slice leaf: matches ida only')
inst('leafc', 'obs-leaf', {"identifier": [{"system": S2, "use": "usual"}]}, 'slice leaf: neither')
sd('obs-reslice', 'Observation', [
    el('Observation.identifier', slicing={"discriminator": [{"type": "pattern", "path": "$this"}], "rules": "open"}),
    el('Observation.identifier', id='Observation.identifier:s3', sliceName='s3',
       patternIdentifier={"system": "urn:probe:s3"},
       slicing={"discriminator": [{"type": "value", "path": "use"}], "rules": "open"}),
    el('Observation.identifier', id='Observation.identifier:s3/ra', sliceName='s3/ra',
       type=[{"code": "Identifier", "profile": [U('ida'), U('idb')]}]),
    el('Observation.identifier.use', id='Observation.identifier:s3/ra.use', min=1, fixedCode='usual')])
S3 = "urn:probe:s3"
inst('resla', 'obs-reslice', {"identifier": [{"system": S3, "use": "usual", "value": "v"}]}, 'reslice s3/ra [ida,idb]: matches idb only')
inst('reslb', 'obs-reslice', {"identifier": [{"system": S3, "use": "usual", "period": {"start": "2020-01-01"}}]}, 'reslice: matches ida only')
inst('reslc', 'obs-reslice', {"identifier": [{"system": S3, "use": "usual"}]}, 'reslice: neither')

# 3. invariant only in the 2nd profile (A1), both orders
obs_value('obs-a1', ['qa', 'qi2'])
obs_value('obs-a1r', ['qi2', 'qa'])
for p, s in (('obs-a1', 'a1'), ('obs-a1r', 'a1r')):
    inst(s + 'n', p, {"valueQuantity": Q(code='mg', system=UCUM)}, 'no unit (fails qa), value 5 (fails inv2-1): neither; inv2-1 only in qi2')
    inst(s + 'q', p, {"valueQuantity": Q(value=20, code='mg', system=UCUM)}, 'no unit (fails qa), value 20: only qi2 conforms')
    inst(s + 'p', p, {"valueQuantity": Q(unit='mg', code='mg', system=UCUM)}, 'unit present, value 5: only qa conforms')

# 5. recursive / reference / profile discriminator, both orders
for o, ps in (('1', ['idr-a', 'idr-b']), ('2', ['idr-b', 'idr-a'])):
    sd('obs-rec' + o, 'Observation', [el('Observation.identifier', type=[{"code": "Identifier", "profile": [U(p) for p in ps]}])])
sd('idr-a', 'Identifier', [el('Identifier.value', min=1),
    el('Identifier.assigner.identifier', type=[{"code": "Identifier", "profile": [U('idr-a'), U('idr-b')]}])])
sd('idr-b', 'Identifier', [el('Identifier.period', min=1)])
for o in '12':
    p = 'obs-rec' + o
    inst('rec%sa' % o, p, {"identifier": [{"value": "x", "assigner": {"display": "d", "identifier": {"value": "y"}}}]}, 'recursive: top and inner match idr-a')
    inst('rec%sb' % o, p, {"identifier": [{"period": {"start": "2020-01-01"}, "assigner": {"display": "d", "identifier": {"period": {"start": "2020-01-01"}}}}]}, 'recursive: top matches idr-b only (idr-b has no assigner rule)')
    inst('rec%sc' % o, p, {"identifier": [{"value": "x", "assigner": {"display": "d", "identifier": {"use": "usual"}}}]}, 'recursive: inner matches neither, so top fails idr-a; idr-b needs period: neither')
    inst('rec%sd' % o, p, {"identifier": [{"value": "x", "period": {"start": "2020-01-01"}, "assigner": {"display": "d", "identifier": {"use": "usual"}}}]}, 'recursive: top matches idr-b; idr-a fails by inner')
for o, ps in (('1', ['idh-a', 'idh-b']), ('2', ['idh-b', 'idh-a'])):
    sd('obs-ref' + o, 'Observation', [el('Observation.identifier', type=[{"code": "Identifier", "profile": [U(p) for p in ps]}])])
def orgs(**o): return [dict({"resourceType": "Organization", "id": "o1"}, **o)]
for o in '12':
    p = 'obs-ref' + o
    inst('ref%sx' % o, p, {"contained": orgs(name="N", active=True), "identifier": [{"value": "x", "assigner": {"reference": "#o1"}}]}, 'reference: contained Organization satisfies org-x only (active)')
    inst('ref%sy' % o, p, {"contained": orgs(name="N", contact=[{"purpose": {"text": "p"}}]), "identifier": [{"value": "x", "assigner": {"reference": "#o1"}}]}, 'reference: contained Organization satisfies org-y only (contact)')
    inst('ref%sn' % o, p, {"contained": orgs(name="N"), "identifier": [{"value": "x", "assigner": {"reference": "#o1"}}]}, 'reference: target satisfies neither (D2: failure exclusive to each group)')
sd('obs-ref0', 'Observation', [el('Observation.identifier', type=[{"code": "Identifier", "profile": [U('idh-a')]}])])
inst('ref0x', 'obs-ref0', {"contained": orgs(name="N", active=True), "identifier": [{"value": "x", "assigner": {"reference": "#o1"}}]}, 'control, single profile idh-a: target satisfies org-x')
inst('ref0n', 'obs-ref0', {"contained": orgs(name="N"), "identifier": [{"value": "x", "assigner": {"reference": "#o1"}}]}, 'control, single profile idh-a: target does not satisfy org-x')
def bref(prof, **o):
    ob = {"resourceType": "Observation", "id": "ob1", "meta": {"profile": [U(prof)]}, "status": "final", "code": {"text": "x"},
          "identifier": [{"value": "x", "assigner": {"reference": "urn:uuid:00000000-0000-0000-0000-0000000000a2"}}]}
    return {"type": "collection", "entry": [{"fullUrl": "urn:uuid:00000000-0000-0000-0000-0000000000a1", "resource": ob},
            {"fullUrl": "urn:uuid:00000000-0000-0000-0000-0000000000a2", "resource": dict({"resourceType": "Organization", "id": "og1"}, **o)}]}
for o in '12':
    inst('refb%sx' % o, None, bref('obs-ref' + o, name="N", active=True), 'held in a Bundle entry: Organization satisfies org-x only', resource='Bundle')
    inst('refb%sy' % o, None, bref('obs-ref' + o, name="N", contact=[{"purpose": {"text": "p"}}]), 'held in a Bundle entry: satisfies org-y only', resource='Bundle')
    inst('refb%sn' % o, None, bref('obs-ref' + o, name="N"), 'held in a Bundle entry: satisfies neither', resource='Bundle')
for o, ps in (('1', ['ida', 'idb']), ('2', ['idb', 'ida'])):
    sd('obs-pd' + o, 'Observation', [
        el('Observation.identifier', slicing={"discriminator": [{"type": "profile", "path": "$this"}], "rules": "closed"}),
        el('Observation.identifier', id='Observation.identifier:sx', sliceName='sx',
           type=[{"code": "Identifier", "profile": [U(p) for p in ps]}])])
    p = 'obs-pd' + o
    inst('pd%sa' % o, p, {"identifier": [{"period": {"start": "2020-01-01"}}]}, 'profile discriminator: ida only')
    inst('pd%sb' % o, p, {"identifier": [{"value": "v"}]}, 'profile discriminator: idb only')
    inst('pd%sn' % o, p, {"identifier": [{"use": "usual"}]}, 'profile discriminator: neither (closed)')
    inst('pd%sab' % o, p, {"identifier": [{"value": "v", "period": {"start": "2020-01-01"}}]}, 'profile discriminator: both')

# 6. same canonical declared on two nodes
sd('obs-two', 'Observation', [
    el('Observation.value[x]', type=[{"code": "Quantity", "profile": [U('qa'), U('qb')]}]),
    el('Observation.component.value[x]', type=[{"code": "Quantity", "profile": [U('qa'), U('qb')]}])])
sd('obs-two-miss', 'Observation', [
    el('Observation.value[x]', type=[{"code": "Quantity", "profile": [U('qa'), U('nope-x')]}]),
    el('Observation.component.value[x]', type=[{"code": "Quantity", "profile": [U('qa'), U('nope-x')]}])])
bad = lambda: Q(system=UCUM)   # no unit, no code: fails qa and qb
inst('two', 'obs-two', {"valueQuantity": bad(), "component": [{"code": {"text": "c"}, "valueQuantity": bad()}]}, 'same two profiles on two nodes, both values fail both')
inst('twom', 'obs-two-miss', {"valueQuantity": bad(), "component": [{"code": {"text": "c"}, "valueQuantity": bad()}]}, 'same unresolved canonical on two nodes (Unknown_Profile per node?)')
inst('twom1', 'obs-two-miss', {"valueQuantity": bad(), "component": [{"code": {"text": "c"}, "valueQuantity": Q(unit='mg')}]}, 'unresolved canonical on two nodes, only the first value fails')

# 7. %resource: contained resource and Bundle entry
obs_value('obs-res', ['qres-a', 'qres-b'])
def pq(): return {"resourceType": "Observation", "id": "c1", "meta": {"profile": [U('obs-res')]}, "status": "amended",
                  "code": {"text": "x"}, "valueQuantity": Q(code='mg', system=UCUM)}
inst('res1', 'obs-res', {"status": "final", "valueQuantity": Q(code='mg', system=UCUM), "contained": [pq()]},
     'top final + contained amended (contained has its own meta.profile); no unit: qres-a fails by unit, qres-b by status')
inst('res2', 'obs-res', {"status": "amended", "valueQuantity": Q(code='mg', system=UCUM)}, 'top amended: conforms qres-b only')
inst('res3', 'obs-res', {"status": "final", "valueQuantity": Q(unit='mg', code='mg', system=UCUM)}, 'top final with unit: conforms qres-a only')
inst('res5', 'obs-res', {"status": "amended", "valueQuantity": Q(code='mg', system=UCUM), "contained": [dict(pq(), status='final')]},
     'top amended + contained final (no unit): top conforms qres-b; contained fails both unless HL7 uses the contained resource as %resource')
def entry(i, st):
    return {"fullUrl": "urn:uuid:00000000-0000-0000-0000-00000000000%d" % i, "resource": {
        "resourceType": "Observation", "id": "e%d" % i, "meta": {"profile": [U('obs-res')]}, "status": st,
        "code": {"text": "x"}, "valueQuantity": Q(code='mg', system=UCUM)}}
inst('res4', None, {"entry": [entry(1, 'final'), entry(2, 'amended')], "type": "collection"},
     'Bundle: entry final (no unit) and entry amended (no unit); same datatype value under two %resource', resource='Bundle')

# 8. D2 is the ref probes above (held target under each layer). 9. D4
obs_value('obs-d4', ['nope-1', 'nope-2'])
inst('d4a', 'obs-d4', {"valueQuantity": {"value": 5, "bogus": 1, "unit": 7}}, 'two unresolved profiles, invalid base content (unknown element, wrong type)')
inst('d4b', 'obs-d4', {"valueQuantity": Q(unit='mg')}, 'two unresolved profiles, valid base content')
obs_value('obs-d4m', ['nope-1', 'qa'])
inst('d4c', 'obs-d4m', {"valueQuantity": {"value": 5, "bogus": 1, "unit": 7}}, 'one unresolved + qa, invalid base content')

# 10. IG canonical and contained SD
def igres(ver):
    return {"resourceType": "ImplementationGuide", "id": "igx", "url": "http://example.org/fhir/ImplementationGuide/igx",
            "name": "igx", "status": "draft", "packageId": "example.igx", "fhirVersion": [ver],
            "definition": {"resource": [{"reference": {"reference": "StructureDefinition/qa"}, "exampleBoolean": False}]}}
IGURL = "http://example.org/fhir/ImplementationGuide/igx"
sd('obs-igq', 'Observation', [el('Observation.value[x]', type=[{"code": "Quantity", "profile": [IGURL]}])])
sd('obs-igq2', 'Observation', [el('Observation.value[x]', type=[{"code": "Quantity", "profile": [IGURL, U('qb')]}])])
sd('obs-igmiss', 'Observation', [el('Observation.value[x]', type=[{"code": "Quantity", "profile": ["http://example.org/fhir/ImplementationGuide/nope", U('qb')]}])])
def cq(ver):
    return {"resourceType": "StructureDefinition", "id": "cq", "url": "http://example.org/fhir/StructureDefinition/cq-inner", "name": "cq",
            "status": "draft", "fhirVersion": ver, "kind": "complex-type", "abstract": False, "type": "Quantity", "baseDefinition": HL7 + "Quantity",
            "derivation": "constraint", "differential": {"element": [el('Quantity.unit', min=1)]}}
def mk_contained(ref, two):
    def b(ver):
        pr = [ref, U('qb')] if two else [ref]
        return {"resourceType": "StructureDefinition", "id": "obs-cont%d" % two, "url": U("obs-cont%d" % two), "name": "obs_cont%d" % two, "status": "draft",
                "fhirVersion": ver, "kind": "resource", "abstract": False, "type": "Observation", "baseDefinition": HL7 + "Observation", "derivation": "constraint",
                "contained": [cq(ver)], "differential": {"element": [el('Observation.value[x]', type=[{"code": "Quantity", "profile": pr}])]}}
    PROFILES["obs-cont%d" % two] = b
mk_contained('#cq', 0); mk_contained('#cq', 1)
for n, p in (('ig1', 'obs-igq'), ('ig2', 'obs-igq2'), ('ig3', 'obs-igmiss'), ('ct1', 'obs-cont0'), ('ct2', 'obs-cont1')):
    inst(n + 'n', p, {"valueQuantity": Q(system=UCUM)}, 'IG/contained canonical in type.profile: no unit, no code (fails qa/qb/cq)')
    inst(n + 'u', p, {"valueQuantity": Q(unit='mg', system=UCUM)}, 'IG/contained canonical: unit present, no code (qa/cq ok, qb fails)')
    inst(n + 'c', p, {"valueQuantity": Q(code='mg', system=UCUM)}, 'IG/contained canonical: code present, no unit (qb ok)')

# 11. E2: profile discriminator ending in a datatype, nested Bundle. Identifier profiles with %resource / %rootResource.
E2P = {'rempty': '%resource.empty()', 'rexists': '%resource.exists()', 'rid1': "%resource.id = 'o1'", 'rid2': "%resource.id = 'o2'",
       'rro1': "%rootResource.id = 'o1'", 'rrid0': "%rootResource.id = 'b0'", 'rrid1': "%rootResource.id = 'b1'",
       'rval': "%resource.value = 'v'", 'rrval': "%rootResource.value = 'v'"}
for k, ex in E2P.items():
    sd('ids-' + k, 'Identifier', [el('Identifier', constraint=[cons('e2-' + k, ex, 'identifier profile ids-%s: %s' % (k, ex))])])
    sd('obs-e2s-' + k, 'Observation', [      # discriminated by profile
        el('Observation.identifier', slicing={"discriminator": [{"type": "profile", "path": "$this"}], "rules": "closed"}),
        el('Observation.identifier', id='Observation.identifier:sx', sliceName='sx', type=[{"code": "Identifier", "profile": [U('ids-' + k)]}])])
    sd('obs-e2t-' + k, 'Observation', [      # plain type.profile, no slicing (control)
        el('Observation.identifier', type=[{"code": "Identifier", "profile": [U('ids-' + k)]}])])
def eobs(i, prof):
    return {"resourceType": "Observation", "id": i, "meta": {"profile": [U(prof)]}, "status": "final", "code": {"text": "x"},
            "identifier": [{"value": "v"}]}
def wrap(o, depth):
    r = o
    for d in range(depth):
        r = {"resourceType": "Bundle", "id": "b%d" % (depth - 1 - d), "type": "collection", "entry": [{"fullUrl": "urn:uuid:00000000-0000-0000-0000-00000000000%d" % d, "resource": r}]}
    return r
for k in E2P:
    for kind, pre in (('s', 'obs-e2s-'), ('t', 'obs-e2t-')):
        for where, depth in (('x', 0), ('b', 1), ('n', 2)):
            o = eobs('o1', pre + k)
            if depth == 0: inst('e2%s%s%s' % (kind, k, where), pre + k, {"id": "o1", "identifier": [{"value": "v"}]}, 'e2 %s: Identifier profile ids-%s, %s' % ('profile-discriminator' if kind == 's' else 'plain type.profile', k, {'x': 'standalone Observation', 'b': 'Observation in a Bundle', 'n': 'Observation in a Bundle in a Bundle'}[where]))
            else: inst('e2%s%s%s' % (kind, k, where), None, wrap(o, depth), 'e2 %s: Identifier profile ids-%s, %s' % ('profile-discriminator' if kind == 's' else 'plain type.profile', k, {'b': 'Observation in a Bundle', 'n': 'Observation in a Bundle in a Bundle'}[where]), resource='Bundle')
# resolve(): hasMember sliced by a profile discriminator on resolve().identifier; o1 -> o2, same Bundle (h) or nested Bundle (hn)
for k in E2P:
    sd('obs-e2h-' + k, 'Observation', [
        el('Observation.hasMember', slicing={"discriminator": [{"type": "profile", "path": "resolve().identifier"}], "rules": "closed"}),
        el('Observation.hasMember', id='Observation.hasMember:sx', sliceName='sx', type=[{"code": "Reference", "targetProfile": [U('obs-e2t-' + k)]}])])
    top = {"resourceType": "Observation", "id": "o1", "meta": {"profile": [U('obs-e2h-' + k)]}, "status": "final", "code": {"text": "x"},
           "hasMember": [{"reference": "urn:uuid:00000000-0000-0000-0000-000000000009"}]}
    inner = {"resourceType": "Observation", "id": "o2", "status": "final", "code": {"text": "x"}, "identifier": [{"value": "v"}]}
    ents = [{"fullUrl": "urn:uuid:00000000-0000-0000-0000-000000000008", "resource": top},
            {"fullUrl": "urn:uuid:00000000-0000-0000-0000-000000000009", "resource": inner}]
    inst('e2h' + k, None, {"id": "b0", "type": "collection", "entry": ents}, 'e2 resolve(): hasMember discriminated by profile of resolve().identifier (ids-%s); target in the same Bundle' % k, resource='Bundle')
    inst('e2hn' + k, None, {"id": "b0", "type": "collection", "entry": [{"fullUrl": "urn:uuid:00000000-0000-0000-0000-000000000007",
         "resource": {"resourceType": "Bundle", "id": "b1", "type": "collection", "entry": ents}}]}, 'e2 resolve(): same, both Observations in a Bundle nested in a Bundle (ids-%s)' % k, resource='Bundle')

# 12. E3/F1: same invariant, different severity / text; ele-1 inherited by both layers
for nm, ps in (('sev1', ['qsev-a', 'qsev-b']), ('sev2', ['qsev-b', 'qsev-a']), ('txt1', ['qtxt-a', 'qtxt-b']), ('txt2', ['qtxt-b', 'qtxt-a']), ('dup', ['qdup-a', 'qdup-b'])):
    obs_value('obs-' + nm, ps)
    inst(nm, 'obs-' + nm, {"valueQuantity": Q(code='mg', system=UCUM)}, 'dup-1 and unit min fail in both layers: %s' % ps)
obs_value('obs-ele', ['qa', 'qb'])
inst('ele1', 'obs-ele', {"valueQuantity": {"id": "q1"}}, 'Quantity with only an id: ele-1 fails, inherited by both layers (qa, qb also fail by cardinality)')
inst('ele2', 'obs-ele', {"valueQuantity": {"id": "q1", "value": 5}}, 'Quantity id+value, no unit/code: ele-1 ok; NoMatch only')

# ---- p1..p21 copied from the 2026-11-05 evidence ------------------------------------------
def write(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w') as f: json.dump(obj, f, indent=1); f.write('\n')

def main():
    for sub in ['inst'] + list(VERS.values()):
        shutil.rmtree(os.path.join(HERE, sub), ignore_errors=True)
    for ver, d in VERS.items():
        for i, b in PROFILES.items():
            write(os.path.join(HERE, d, 'StructureDefinition-%s.json' % i), b(ver))
        write(os.path.join(HERE, d, 'ImplementationGuide-igx.json'), igres(ver))
        for f in sorted(os.listdir(os.path.join(OLD, 'ig'))):
            if f.startswith('StructureDefinition-'):
                x = json.load(open(os.path.join(OLD, 'ig', f)))
                x['fhirVersion'] = ver
                x = json.loads(json.dumps(x).replace('|4.0.1', '|' + ver))   # p17: versioned canonicals follow the version
                write(os.path.join(HERE, d, 'old-' + f), x)
    os.makedirs(os.path.join(HERE, 'inst'), exist_ok=True)
    for n in range(1, 22):
        shutil.copy(os.path.join(OLD, 'inst', 'p%d.json' % n), os.path.join(HERE, 'inst', 'p%d.json' % n))
    for n, (prof, extra, res) in INST.items():
        o = {"resourceType": res, "id": n}
        if res == 'Observation':
            if prof: o["meta"] = {"profile": [U(prof)]}
            o.update({"status": "final", "code": {"text": "x"}, "subject": {"display": "x"}})
        o.update(extra)
        write(os.path.join(HERE, 'inst', n + '.json'), o)
    write(os.path.join(HERE, 'descriptions.json'), DESC)
    print(len(PROFILES), 'profiles', len(INST), 'instances')
main()
