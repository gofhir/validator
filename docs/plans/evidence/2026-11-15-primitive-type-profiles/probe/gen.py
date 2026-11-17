"""Sondas de la pieza 3 de #193 (perfiles de tipo en un valor primitivo), 15-11-2026.
Uso: python3 -I gen.py. Escribe ig-<version>/ e inst/ y descriptions.json. run.sh <version> corre HL7 validator_cli 7.0.4 (-tx n/a)."""
import json, os, re, shutil

HERE = os.path.dirname(os.path.abspath(__file__))
VERS = ('4.0.1', '4.3.0', '5.0.0')
NS = 'http://example.org/fhir/StructureDefinition/'
HL7 = 'http://hl7.org/fhir/StructureDefinition/'
U = lambda i: NS + i
PROFILES = {}   # file -> builder(ver)
INST = {}       # name -> (resource dict, description)
MISSING = U('missing-prim')
OMIT = object()


def el(path, **kw):
    d = {"id": kw.pop('id', path), "path": path}
    d.update(kw)
    return d


def cons(key, expr, human, sev='error'):
    return {"key": key, "severity": sev, "human": human, "expression": expr}


def sd(i, typ, kind, elems, ctx=None, base=None):
    def build(ver):
        d = {"resourceType": "StructureDefinition", "id": i, "url": U(i), "name": i.replace('-', '_'), "status": "draft",
             "fhirVersion": ver, "kind": kind, "abstract": False, "type": typ, "baseDefinition": base or HL7 + typ,
             "derivation": "constraint", "differential": {"element": elems}}
        if ctx:
            d["context"] = ctx
        return d
    PROFILES['StructureDefinition-%s.json' % i] = build


def prim(i, typ, elems):
    sd(i, typ, 'primitive-type', elems)


# ---- primitive profiles --------------------------------------------------------------------
prim('ps-len', 'string', [el('string', constraint=[cons('len-1', 'length() <= 3', 'at most 3 characters (ps-len)')])])
prim('ps-len2', 'string', [el('string', constraint=[cons('len-1', 'length() <= 3', 'TEXT TWO: no more than 3 chars (ps-len2)')])])
prim('ps-len3', 'string', [el('string', constraint=[cons('len-1', 'length() <= 2', 'at most 2 characters (ps-len3)')])])
prim('ps-up', 'string', [el('string', constraint=[cons('up-1', '$this = $this.upper()', 'must be upper case (ps-up)')])])
prim('ps-max', 'string', [el('string.value', maxLength=3)])
prim('ps-fix', 'string', [el('string.value', fixedString='abc')])
prim('ps-pat', 'string', [el('string.value', patternString='abc')])
prim('ps-valc', 'string', [el('string.value', constraint=[cons('vc-1', 'length() <= 3', 'value constraint at most 3 (ps-valc)')])])
prim('ps-ext1', 'string', [el('string.extension', min=1)])
prim('ps-id1', 'string', [el('string.id', min=1)])
prim('ps-idc', 'string', [el('string.id', constraint=[cons('idc-1', 'length() <= 3', 'id at most 3 chars (ps-idc)')])])
prim('ps-sub', 'string', [
    el('string.extension', slicing={"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}),
    el('string.extension', id='string.extension:req', sliceName='req', min=1, type=[{"code": "Extension"}]),
    el('string.extension.url', id='string.extension:req.url', fixedUri=U('ext-req'))])
prim('ps-res', 'string', [el('string', constraint=[cons('res-1', "%resource.status = 'final'", 'resource must be final (ps-res)')])])
prim('ps-rroot', 'string', [el('string', constraint=[cons('rroot-1', "%rootResource.status = 'final'", 'root resource must be final (ps-rroot)')])])
prim('ps-hx', 'string', [el('string', constraint=[cons('hx-1', 'hasValue()', 'must have a value (ps-hx)')])])
prim('ps-rx', 'string', [el('string', constraint=[cons('rx-1', 'extension.exists()', 'must have an extension (ps-rx)')])])
prim('ps-ri', 'string', [el('string', constraint=[cons('ri-1', 'id.exists()', 'must have an id (ps-ri)')])])
prim('ps-rc', 'string', [el('string', constraint=[cons('rc-1', '%context.length() <= 3', 'context at most 3 chars (ps-rc)')])])
prim('ps-rcx', 'string', [el('string', constraint=[cons('rcx-1', '%context.extension.exists()', 'context has an extension (ps-rcx)')])])
prim('ps-nosub', 'string', [
    el('string.extension', slicing={"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}),
    el('string.extension', id='string.extension:bad', sliceName='bad', max='0', type=[{"code": "Extension"}]),
    el('string.extension.url', id='string.extension:bad.url', fixedUri=U('ext-other'))])
prim('ps-mixe', 'string', [el('string.extension', min=1), el('string.value', maxLength=3)])
prim('ps-mixi', 'string', [el('string.id', min=1), el('string.value', fixedString='abc')])
prim('pi-min', 'integer', [el('integer.value', minValueInteger=5)])
prim('pi-fix', 'integer', [el('integer', id='integer', constraint=[cons('fi-1', '$this = 7', 'must be 7 (pi-fix)')])])
prim('pi-fx', 'integer', [el('integer.value', fixedInteger=7)])
prim('pd-min', 'date', [el('date.value', minValueDate='2000-01-01')])
prim('pd-con', 'date', [el('date', constraint=[cons('dc-1', '$this >= @2000-01-01', 'date from 2000 (pd-con)')])])
prim('pc-bind', 'code', [el('code.value', binding={"strength": "required", "valueSet": U('vs-pc')})])
prim('pc-ext', 'code', [el('code', constraint=[cons('cx-1', "$this = 'male'", 'must be male (pc-ext)')])])

CS = lambda ver: {"resourceType": "CodeSystem", "id": "cs-pc", "url": "urn:probe:cs", "status": "draft", "content": "complete",
                  "caseSensitive": True, "concept": [{"code": c} for c in ('male', 'female', 'other')]}
VS = lambda ver: {"resourceType": "ValueSet", "id": "vs-pc", "url": U('vs-pc'), "status": "draft",
                  "compose": {"include": [{"system": "urn:probe:cs", "concept": [{"code": "male"}, {"code": "female"}]}]}}
PROFILES['CodeSystem-cs-pc.json'] = CS
PROFILES['ValueSet-vs-pc.json'] = VS

for _e in ('ext-req', 'ext-other'):
    sd(_e, 'Extension', 'complex-type', [el('Extension.url', fixedUri=U(_e)), el('Extension.value[x]', type=[{"code": "string"}])],
       ctx=[{"type": "element", "expression": "Element"}])

# ---- carriers -------------------------------------------------------------------------------
# a carrier is a profile of the owner (Observation, Patient or Extension) that declares the type profiles of one element
# tag = '+'.join of profile ids ('miss' = unresolved)
def purl(p):
    return MISSING if p == 'miss' else U(p)


def tagof(profs):
    return '+'.join(profs)


MADE = set()


def carrier(kind, typ, profs):
    t = tagof(profs)
    pid = '%s-%s' % (kind, t) if typ == 'string' else '%s-%s-%s' % (kind, typ, t)
    if pid in MADE:
        return pid
    MADE.add(pid)
    ty = [{"code": typ, "profile": [purl(p) for p in profs]}]
    if kind == 'o':     # Observation.value[x]
        sd(pid, 'Observation', 'resource', [el('Observation.value[x]', type=ty)])
    elif kind == 'g':   # Patient.name.given
        sd(pid, 'Patient', 'resource', [el('Patient.name.given', type=ty)])
    elif kind == 'b':   # Patient.birthDate
        sd(pid, 'Patient', 'resource', [el('Patient.birthDate', type=ty)])
    elif kind == 'x':   # Extension.value[x] (extension profile)
        sd(pid, 'Extension', 'complex-type', [el('Extension.url', fixedUri=U(pid)), el('Extension.value[x]', type=ty)],
           ctx=[{"type": "element", "expression": "Patient"}, {"type": "element", "expression": "Observation"}])
    elif kind == 'ge':  # Patient.gender
        sd(pid, 'Patient', 'resource', [el('Patient.gender', type=ty)])
    return pid


def obs(rid, prof, extra, **kw):
    o = {"resourceType": "Observation", "id": rid}
    if prof:
        o["meta"] = {"profile": [U(prof)]}
    o.update({"status": "final", "code": {"text": "x"}})
    o.update(extra)
    return o


def pat(rid, prof, extra):
    o = {"resourceType": "Patient", "id": rid}
    if prof:
        o["meta"] = {"profile": [U(prof)]}
    o.update(extra)
    return o


def add(name, res, what):
    INST[name] = (res, what)


def on_obs(name, profs, val, what, typ='string', ext=None, field='valueString'):
    p = carrier('o', typ, profs)
    e = {} if val is OMIT else {field: val}
    if ext is not None:
        e['_' + field] = ext
    add(name, obs(name, p, e), what)


def on_given(name, profs, given, what, ext=None):
    p = carrier('g', 'string', profs)
    n = {"given": given}
    if ext is not None:
        n["_given"] = ext
    add(name, pat(name, p, {"name": [n]}), what)


def on_bd(name, profs, val, what, ext=None):
    p = carrier('b', 'date', profs)
    e = {"birthDate": val}
    if ext is not None:
        e["_birthDate"] = ext
    add(name, pat(name, p, e), what)


def on_ext(name, profs, val, what, typ='string', field='valueString', ext=None):
    pid = carrier('x', typ, profs)
    e = {"url": U(pid), field: val}
    if ext is not None:
        e['_' + field] = ext
    add(name, pat(name, None, {"extension": [e]}), what)


SUBX = [{"url": U('ext-req'), "valueString": "q"}]
OTHX = [{"url": U('ext-other'), "valueString": "q"}]

# ---- case 1: one profile ----------------------------------------------------------------------
c1 = [  # (tag, profile, ok value, bad value)
    ('len', 'ps-len', 'ab', 'abcd'), ('max', 'ps-max', 'ab', 'abcd'), ('fix', 'ps-fix', 'abc', 'abd'), ('pat', 'ps-pat', 'abc', 'abd'),
    ('valc', 'ps-valc', 'ab', 'abcd'), ('up', 'ps-up', 'AB', 'ab')]
for tag, p, ok, bad in c1:
    for w, v in (('ok', ok), ('bad', bad)):
        on_obs('c1-%s-%s-obs' % (tag, w), [p], v, 'one profile %s on Observation.valueString=%r' % (p, v))
        on_given('c1-%s-%s-giv' % (tag, w), [p], [v], 'one profile %s on Patient.name.given[0]=%r' % (p, v))
        on_ext('c1-%s-%s-ext' % (tag, w), [p], v, 'one profile %s on Extension.valueString=%r' % (p, v))
for w, v in (('ok', 7), ('bad', 3)):
    on_obs('c1-imin-%s-obs' % w, ['pi-min'], v, 'one profile pi-min (minValue 5) on Observation.valueInteger=%r' % v, 'integer', field='valueInteger')
    on_ext('c1-imin-%s-ext' % w, ['pi-min'], v, 'pi-min on Extension.valueInteger=%r' % v, 'integer', 'valueInteger')
    on_obs('c1-ifx-%s-obs' % w, ['pi-fx'], 7 if w == 'ok' else 8, 'one profile pi-fx (fixed 7) on valueInteger=%r' % (7 if w == 'ok' else 8), 'integer', field='valueInteger')
    on_obs('c1-icon-%s-obs' % w, ['pi-fix'], 7 if w == 'ok' else 8, 'one profile pi-fix (root constraint $this=7) on valueInteger=%r' % (7 if w == 'ok' else 8), 'integer', field='valueInteger')
for w, v in (('ok', '2001-05-05'), ('bad', '1999-01-01')):
    on_bd('c1-dmin-%s-bd' % w, ['pd-min'], v, 'one profile pd-min (minValue 2000-01-01) on Patient.birthDate=%s' % v)
    on_bd('c1-dcon-%s-bd' % w, ['pd-con'], v, 'one profile pd-con (root constraint >= 2000) on Patient.birthDate=%s' % v)
for w, v in (('ok', 'male'), ('bad', 'other')):
    p = carrier('ge', 'code', ['pc-bind'])
    add('c1-bind-%s-gen' % w, pat('c1-bind-%s-gen' % w, p, {"gender": v}), 'pc-bind (required VS male|female) on Patient.gender=%s' % v)
    on_ext('c1-bind-%s-ext' % w, ['pc-bind'], v, 'pc-bind on Extension.valueCode=%s' % v, 'code', 'valueCode')
    p = carrier('ge', 'code', ['pc-ext'])
    add('c1-ccon-%s-gen' % w, pat('c1-ccon-%s-gen' % w, p, {"gender": v}), 'pc-ext (root constraint $this=male) on Patient.gender=%s' % v)
for w, ext in (('ok', [{"url": U('ext-other'), "valueString": "q"}]), ('bad', None)):
    on_obs('c1-ext1-%s-obs' % w, ['ps-ext1'], 'ab', 'ps-ext1 (string.extension min 1): _valueString=%s' % ('1 ext' if ext else 'absent'),
           ext=None if ext is None else {"extension": ext})
for w, ext in (('ok', SUBX), ('bad', OTHX), ('none', None)):
    on_obs('c1-sub-%s-obs' % w, ['ps-sub'], 'ab', 'ps-sub (required sub-extension ext-req): _valueString ext=%s' % w,
           ext=None if ext is None else {"extension": ext})
for w, ext in (('ok', {"id": "a1"}), ('bad', {"id": "abcdef"}), ('none', None)):
    on_obs('c1-idc-%s-obs' % w, ['ps-idc'], 'ab', 'ps-idc (string.id constraint length<=3): _valueString=%s' % json.dumps(ext), ext=ext)
for w, ext in (('ok', {"id": "a1"}), ('bad', None)):
    on_obs('c1-id1-%s-obs' % w, ['ps-id1'], 'ab', 'ps-id1 (string.id min 1): _valueString=%s' % json.dumps(ext), ext=ext)

# ---- case 2: unresolved profile -----------------------------------------------------------------
on_obs('c2-obs', ['miss'], 'ab', 'unresolved profile on Observation.valueString')
on_given('c2-giv', ['miss'], ['ab'], 'unresolved profile on Patient.name.given')
on_bd('c2-bd', ['miss'], '2001-05-05', 'unresolved profile on Patient.birthDate')
on_obs('c2-int', ['miss'], 5, 'unresolved profile on Observation.valueInteger', 'integer', field='valueInteger')
on_ext('c2-ext', ['miss'], 'ab', 'unresolved profile on Extension.valueString')
on_obs('c2-obs2', ['ps-len', 'miss'], 'ab', 'ps-len + unresolved on valueString (value conforms ps-len)')
on_obs('c2-obs3', ['ps-len', 'miss'], 'abcd', 'ps-len + unresolved on valueString (value fails ps-len)')
on_obs('c2-obs4', ['miss', 'miss'], 'ab', 'two unresolved (same url) on valueString')

# ---- case 3: two profiles ---------------------------------------------------------------------
P2 = ['ps-len', 'ps-up']
for tag, v in (('both', 'AB'), ('len', 'ab'), ('up', 'ABCDE'), ('none', 'abcde')):
    on_obs('c3-%s-obs' % tag, P2, v, 'ps-len+ps-up on valueString=%r' % v)
    on_given('c3-%s-giv' % tag, P2, [v], 'ps-len+ps-up on given[0]=%r' % v)
    on_ext('c3-%s-ext' % tag, P2, v, 'ps-len+ps-up on Extension.valueString=%r' % v)
on_given('c3-idx1', P2, ['AB', 'abcde'], 'two profiles, given[1] only fails both')
on_given('c3-idx1b', P2, ['AB', 'ABCDE'], 'two profiles, given[1] matches one')
on_given('c3-idx1c', P2, ['AB', 'AB'], 'two profiles, given[1] matches both')
on_given('c3-idx1one', ['ps-len'], ['AB', 'abcde'], 'ONE profile ps-len, given[1] only fails')
on_given('c3-al', P2, ['AB', 'abcde'], 'two profiles, _given aligned: [null,{ext}]', ext=[None, {"extension": OTHX}])
on_given('c3-al2', ['ps-ext1', 'ps-len'], ['AB', 'abcde'], 'ps-ext1+ps-len: _given [ {ext}, null ]', ext=[{"extension": OTHX}, None])
on_given('c3-vl', P2, [None, 'AB'], 'two profiles, given[0] null valueless with _given[0] ext', ext=[{"extension": OTHX}, None])
on_given('c3-vl1', ['ps-len'], [None, 'AB'], 'ONE profile ps-len, valueless given[0] with ext', ext=[{"extension": OTHX}, None])
on_given('c3-vl2', ['ps-ext1', 'ps-len'], [None, 'AB'], 'ps-ext1+ps-len, valueless given[0] with ext', ext=[{"extension": OTHX}, None])
on_given('c3-vl3', ['ps-ext1'], [None, 'AB'], 'ONE profile ps-ext1, valueless given[0] with ext, given[1] has no ext', ext=[{"extension": OTHX}, None])
on_given('c3-vl4', ['ps-ext1', 'ps-up'], [None], 'ps-ext1+ps-up, valueless given[0] WITHOUT ext (invalid anyway)', ext=[{"id": "z1"}])
on_given('c3-lon', P2, ['AB'], 'two profiles, _given longer than given: given[1] valueless', ext=[None, {"extension": OTHX}])
on_given('c3-lon1', ['ps-ext1'], ['AB'], 'ONE profile ps-ext1, _given longer than given: given[1] valueless', ext=[None, {"extension": OTHX}])
on_given('c3-lon2', ['ps-ext1', 'ps-len'], ['AB'], 'ps-ext1+ps-len, _given longer: given[1] valueless with ext', ext=[{"extension": OTHX}, {"extension": OTHX}])
on_obs('c3-vlo', P2, OMIT, 'two profiles, only _valueString (no valueString): ext only (valueless)', ext={"extension": OTHX})
on_obs('c3-vlo1', ['ps-ext1'], OMIT, 'ONE profile ps-ext1, only _valueString (no valueString): ext only', ext={"extension": OTHX})
on_bd('c3-bd-both', ['pd-min', 'pd-con'], '2001-05-05', 'two date profiles, both conform')
on_bd('c3-bd-one', ['pd-min', 'pd-con'], '1999-01-01', 'two date profiles, none conform')
on_obs('c3-i-both', ['pi-min', 'pi-fx'], 7, 'two integer profiles, both conform (7)', 'integer', field='valueInteger')
on_obs('c3-i-one', ['pi-min', 'pi-fx'], 6, 'two integer profiles, only pi-min conforms (6)', 'integer', field='valueInteger')
on_obs('c3-i-none', ['pi-min', 'pi-fx'], 3, 'two integer profiles, none conform (3)', 'integer', field='valueInteger')
on_obs('c3-max-both', ['ps-max', 'ps-pat'], 'abc', 'maxLength + pattern both conform', )
on_obs('c3-max-one', ['ps-max', 'ps-pat'], 'ab', 'maxLength conforms, pattern not', )
on_obs('c3-max-none', ['ps-max', 'ps-pat'], 'abcd', 'neither conforms')
on_obs('c3-3p', ['ps-len', 'ps-up', 'ps-max'], 'AB', 'three profiles, all conform')

# ---- case 4: same key in both profiles ----------------------------------------------------------
on_obs('c4-both-ok', ['ps-len', 'ps-len2'], 'ab', 'same key len-1, different message: conforms both')
on_obs('c4-both-bad', ['ps-len', 'ps-len2'], 'abcd', 'same key len-1, different message: fails both')
on_given('c4-giv-bad', ['ps-len', 'ps-len2'], ['abcd'], 'same key len-1, different message: fails both (given)')
on_obs('c4-expr-one', ['ps-len', 'ps-len3'], 'abc', 'same key len-1, different expression (<=3, <=2): "abc" fits only ps-len')
on_obs('c4-expr-bad', ['ps-len', 'ps-len3'], 'abcd', 'same key len-1, different expression: fails both')
on_obs('c4-same', ['ps-len', 'ps-len'], 'abcd', 'the same profile twice, fails')
on_obs('c4-same-ok', ['ps-len', 'ps-len'], 'ab', 'the same profile twice, conforms')

# ---- case 5: choices and slices by type -----------------------------------------------------------
def x_choice(i, profs, other=True):
    ty = [{"code": "string", "profile": [purl(p) for p in profs]}] + ([{"code": "integer"}] if other else [])
    sd(i, 'Extension', 'complex-type', [el('Extension.url', fixedUri=U(i)), el('Extension.value[x]', type=ty)],
       ctx=[{"type": "element", "expression": "Patient"}])


x_choice('xc2', ['ps-len', 'ps-up'])
x_choice('xc1', ['ps-len'])
for tag, v in (('both', 'AB'), ('none', 'abcde'), ('len', 'ab')):
    add('c5-xc2-' + tag, pat('c5-xc2-' + tag, None, {"extension": [{"url": U('xc2'), "valueString": v}]}), 'Extension.value[x] string|integer, profiles ps-len+ps-up, value %r' % v)
for tag, v in (('ok', 'ab'), ('bad', 'abcd')):
    add('c5-xc1-' + tag, pat('c5-xc1-' + tag, None, {"extension": [{"url": U('xc1'), "valueString": v}]}), 'Extension.value[x] string|integer, ONE profile ps-len, value %r' % v)
add('c5-xc2-int', pat('c5-xc2-int', None, {"extension": [{"url": U('xc2'), "valueInteger": 3}]}), 'Extension.value[x] string|integer: an integer value (no profile for integer)')
SLC = {"discriminator": [{"type": "type", "path": "$this"}], "rules": "open"}
for pid, profs in (('os2', ['ps-len', 'ps-up']), ('os1', ['ps-len'])):
    sd(pid, 'Observation', 'resource', [
        el('Observation.value[x]', slicing=SLC),
        el('Observation.value[x]', id='Observation.value[x]:valueString', sliceName='valueString',
           type=[{"code": "string", "profile": [purl(p) for p in profs]}]),
        el('Observation.value[x]', id='Observation.value[x]:valueInteger', sliceName='valueInteger', type=[{"code": "integer"}])])
for tag, v in (('both', 'AB'), ('none', 'abcde'), ('len', 'ab')):
    add('c5-os2-' + tag, obs('c5-os2-' + tag, 'os2', {"valueString": v}), 'slice by type valueString with profiles ps-len+ps-up, value %r' % v)
for tag, v in (('ok', 'ab'), ('bad', 'abcd')):
    add('c5-os1-' + tag, obs('c5-os1-' + tag, 'os1', {"valueString": v}), 'slice by type valueString with ONE profile ps-len, value %r' % v)


# 5b: Observation.value[x] with two types (string with profiles + integer), to tell the multi-type case from the Extension one
for pid, profs in (('oc2', ['ps-len', 'ps-up']), ('oc2r', ['ps-up', 'ps-len'])):
    sd(pid, 'Observation', 'resource', [el('Observation.value[x]', type=[{"code": "string", "profile": [purl(p) for p in profs]}, {"code": "integer"}])])
    for tag, v in (('both', 'AB'), ('none', 'abcde'), ('len', 'ab')):
        add('c5-%s-%s' % (pid, tag), obs('c5-%s-%s' % (pid, tag), pid, {"valueString": v}), 'Observation.value[x] string(%s)|integer, value %r' % ('+'.join(profs), v))
sd('oc1', 'Observation', 'resource', [el('Observation.value[x]', type=[{"code": "string", "profile": [U('ps-len')]}, {"code": "integer"}])])
for tag, v in (('ok', 'ab'), ('bad', 'abcd')):
    add('c5-oc1-' + tag, obs('c5-oc1-' + tag, 'oc1', {"valueString": v}), 'Observation.value[x] string(ps-len)|integer, value %r' % v)

# ---- case 6: %resource / %rootResource inside a contained resource ------------------------------------
def contained_case(tag, profs, top_status, c_status):
    p = carrier('o', 'string', profs)
    c = {"resourceType": "Observation", "id": "c1", "meta": {"profile": [U(p)]}, "status": c_status, "code": {"text": "x"}, "valueString": "ab"}
    t = obs('c6-' + tag, None, {"status": top_status, "contained": [c], "hasMember": [{"reference": "#c1"}]})
    add('c6-' + tag, t, 'contained Observation (%s) with %s on valueString; top status %s' % (c_status, '+'.join(profs), top_status))


for pr in ('ps-res', 'ps-rroot'):
    for ts in ('amended', 'final'):
        for cs in ('final', 'amended'):
            contained_case('%s-%s-%s' % (pr[3:], ts[0], cs[0]), [pr], ts, cs)
for ts, cs in (('amended', 'final'), ('final', 'amended')):
    contained_case('pair-%s-%s' % (ts[0], cs[0]), ['ps-res', 'ps-rroot'], ts, cs)
# control: the same profile on the top-level resource
for ts in ('final', 'amended'):
    p = carrier('o', 'string', ['ps-res'])
    add('c6-top-' + ts[0], obs('c6-top-' + ts[0], p, {"status": ts, "valueString": "ab"}), 'top-level Observation %s, ps-res' % ts)


# ---- case 7 (plan v2, item 7): root constraints that look at extension, id, hasValue() and %context ------------------------
def on_names(name, profs, names, what):
    p = carrier('g', 'string', profs)
    add(name, pat(name, p, {"name": names}), what)


XI = {"id": "a1", "extension": OTHX}
for tag, p in (('hx', 'ps-hx'), ('rx', 'ps-rx'), ('ri', 'ps-ri'), ('rc', 'ps-rc'), ('rcx', 'ps-rcx')):
    for w, v, ext in (('sc', 'ab', None), ('scx', 'ab', XI), ('vl', OMIT, XI), ('long', 'abcd', XI)):
        on_obs('c7-%s-%s-obs' % (tag, w), [p], v, 'root-constraint profile %s on Observation.valueString=%s, _valueString=%s' % (p, 'absent' if v is OMIT else repr(v), json.dumps(ext)), ext=ext)
        on_given('c7-%s-%s-giv' % (tag, w), [p], [None if v is OMIT else v], 'root-constraint profile %s on given[0]=%s, _given[0]=%s' % (p, 'null' if v is OMIT else repr(v), json.dumps(ext)), ext=None if ext is None else [ext])
for tag, profs in (('rxri', ['ps-rx', 'ps-ri']), ('hxrx', ['ps-hx', 'ps-rx']), ('rcrx', ['ps-rc', 'ps-rcx'])):
    for w, ext in (('x', {"extension": OTHX}), ('i', {"id": "a1"}), ('xi', XI), ('none', None)):
        on_obs('c7-%s-%s-obs' % (tag, w), profs, 'ab', '%s on valueString=ab, _valueString=%s' % ('+'.join(profs), json.dumps(ext)), ext=ext)
        on_obs('c7-%s-%s-vl' % (tag, w), profs, OMIT, '%s on a valueless valueString, _valueString=%s' % ('+'.join(profs), json.dumps(ext)), ext=ext or {"id": "z9"})

# ---- case 8: equal scalars under different parents, and in different resources, with different _x ---------------------------
OX = {"extension": OTHX}
for tag, profs, v in (('one', ['ps-ext1'], 'ab'), ('two', ['ps-ext1', 'ps-len'], 'abcd'), ('twoup', ['ps-ext1', 'ps-up'], 'AB'), ('sub', ['ps-sub', 'ps-len'], 'abcd')):
    t = '+'.join(profs)
    on_names('c8-par-%s-a' % tag, profs, [{"given": [v], "_given": [OX]}, {"given": [v]}], '%s, name[0].given[0]=name[1].given[0]=%r; only name[0] has an extension' % (t, v))
    on_names('c8-par-%s-b' % tag, profs, [{"given": [v]}, {"given": [v], "_given": [OX]}], '%s, name[0].given[0]=name[1].given[0]=%r; only name[1] has an extension' % (t, v))
    on_names('c8-par-%s-c' % tag, profs, [{"given": [v], "_given": [OX]}, {"given": [v], "_given": [OX]}], '%s, both names equal with an extension' % t)
    on_names('c8-par-%s-d' % tag, profs, [{"given": [v]}, {"given": [v]}], '%s, both names equal without extension' % t)
    on_names('c8-idx-%s' % tag, profs, [{"given": [v, v], "_given": [OX, None]}], '%s, given[0] and given[1] equal, only given[0] has an extension' % t)
    on_names('c8-idx-%s-r' % tag, profs, [{"given": [v, v], "_given": [None, OX]}], '%s, given[0] and given[1] equal, only given[1] has an extension' % t)
    p = carrier('g', 'string', profs)
    for w, a, b in (('ab', OX, None), ('ba', None, OX), ('aa', OX, OX), ('bb', None, None)):
        pa = {"resourceType": "Patient", "id": "pa", "meta": {"profile": [U(p)]}, "name": [dict({"given": [v]}, **({"_given": [a]} if a else {}))]}
        pb = {"resourceType": "Patient", "id": "pb", "meta": {"profile": [U(p)]}, "name": [dict({"given": [v]}, **({"_given": [b]} if b else {}))]}
        add('c8-res-%s-%s' % (tag, w), {"resourceType": "Bundle", "id": "c8-res-%s-%s" % (tag, w), "type": "collection",
                                         "entry": [{"fullUrl": "urn:uuid:3f1c0a2e-0000-4000-8000-00000000000a", "resource": pa}, {"fullUrl": "urn:uuid:3f1c0a2e-0000-4000-8000-00000000000b", "resource": pb}]},
            'Bundle of two Patients, %s, equal given %r; first has extension=%s, second has extension=%s' % (t, v, bool(a), bool(b)))

# ---- case 9: required and forbidden extension slices under a primitive profile ------------------------------------------------
SU = {"extension": SUBX}
OT = {"extension": OTHX}
BOTH = {"extension": SUBX + OTHX}
for pr in ('ps-sub', 'ps-nosub'):
    for w, ext in (('none', None), ('sub', SU), ('oth', OT), ('both', BOTH)):
        t = pr[3:]
        on_given('c9-%s-%s-giv' % (t, w), [pr], ['ab'], '%s on given[0]=ab, _given[0]=%s' % (pr, json.dumps(ext)), ext=None if ext is None else [ext])
        on_ext('c9-%s-%s-ext' % (t, w), [pr], 'ab', '%s on Extension.valueString=ab, _valueString=%s' % (pr, json.dumps(ext)), ext=ext)
    on_given('c9-%s-vl1' % pr[3:], [pr], ['ab', None], '%s: given=[ab,null], _given=[null,{sub ext}]: valueless member with the required ext, the scalar without _x' % pr, ext=[None, SU])
    on_given('c9-%s-vl2' % pr[3:], [pr], [None, 'ab'], '%s: given=[null,ab], _given=[{other ext},null]: valueless member with another ext' % pr, ext=[OT, None])
    on_given('c9-%s-vl3' % pr[3:], [pr], ['ab'], '%s: given=[ab], _given=[null,{sub ext}]: _given longer than given' % pr, ext=[None, SU])
    on_given('c9-%s-vl4' % pr[3:], [pr], ['ab'], '%s: given=[ab], _given=[null,{other ext}]: _given longer than given' % pr, ext=[None, OT])
    on_obs('c9-%s-vlo' % pr[3:], [pr], OMIT, '%s on a valueless valueString with another ext' % pr, ext=OT)
    on_obs('c9-%s-vlos' % pr[3:], [pr], OMIT, '%s on a valueless valueString with the sub ext' % pr, ext=SU)
for w, ext in (('none', None), ('sub', SU), ('oth', OT), ('both', BOTH)):
    on_obs('c9-pair-%s-obs' % w, ['ps-sub', 'ps-nosub'], 'ab', 'ps-sub+ps-nosub on valueString=ab, _valueString=%s' % json.dumps(ext), ext=ext)
    on_obs('c9-sublen-%s-obs' % w, ['ps-sub', 'ps-len'], 'abcd', 'ps-sub+ps-len on valueString=abcd, _valueString=%s' % json.dumps(ext), ext=ext)
    on_given('c9-sublen-%s-giv' % w, ['ps-sub', 'ps-len'], ['abcd', 'ab'], 'ps-sub+ps-len on given=[abcd,ab], _given[0]=%s' % json.dumps(ext), ext=None if ext is None else [ext, None])
on_given('c9-sublen-vl', ['ps-sub', 'ps-len'], ['ab', None], 'ps-sub+ps-len: given=[ab,null], _given=[null,{sub ext}]', ext=[None, SU])

# ---- case 10 (A4): pairs that differ only in .value, and .value mixed with id/extension -------------------------------------------
for tag, profs, vals in (('lenmax', ['ps-len', 'ps-max'], ('ab', 'abcd')), ('upfix', ['ps-up', 'ps-fix'], ('AB', 'ab')),
                         ('maxfix', ['ps-max', 'ps-fix'], ('abc', 'abcd')), ('valcpat', ['ps-valc', 'ps-pat'], ('abc', 'abcd'))):
    for n, v in enumerate(vals):     # n y no v en el nombre: en un disco sin distinción de mayúsculas «AB» y «ab» chocan
        on_obs('c10-%s-%d' % (tag, n), profs, v, '%s on valueString=%r (pair that differs only in .value)' % ('+'.join(profs), v))
        on_given('c10-%s-%d-giv' % (tag, n), profs, [v], '%s on given[0]=%r' % ('+'.join(profs), v))
for w, ext in (('x', OT), ('nox', None), ('xi', XI)):
    for v in ('abcd', 'ab'):
        on_obs('c10-mixe-%s-%s' % (w, v), ['ps-mixe'], v, 'ps-mixe (value maxLength 3 + extension min 1) on %r, _valueString=%s' % (v, json.dumps(ext)), ext=ext)
        on_obs('c10-mixelen-%s-%s' % (w, v), ['ps-mixe', 'ps-len'], v, 'ps-mixe+ps-len on %r, _valueString=%s' % (v, json.dumps(ext)), ext=ext)
        on_obs('c10-mixeup-%s-%s' % (w, v), ['ps-mixe', 'ps-up'], v, 'ps-mixe+ps-up on %r, _valueString=%s' % (v, json.dumps(ext)), ext=ext)
for w, ext in (('i', {"id": "a1"}), ('noi', None)):
    for v in ('abd', 'abc'):
        on_obs('c10-mixi-%s-%s' % (w, v), ['ps-mixi'], v, 'ps-mixi (value fixed abc + id min 1) on %r, _valueString=%s' % (v, json.dumps(ext)), ext=ext)
        on_obs('c10-mixiup-%s-%s' % (w, v), ['ps-mixi', 'ps-up'], v, 'ps-mixi+ps-up on %r, _valueString=%s' % (v, json.dumps(ext)), ext=ext)
        on_obs('c10-mixilen-%s-%s' % (w, v), ['ps-mixi', 'ps-len'], v, 'ps-mixi+ps-len on %r, _valueString=%s' % (v, json.dumps(ext)), ext=ext)


# ---- case 11: guards of the zero-profile path (no type profile on the element): what exists today must not change ----------------
add('c11-plain-obs', obs('c11-plain-obs', None, {"valueString": "abcd"}), 'no profile: Observation.valueString=abcd')
add('c11-plain-giv', pat('c11-plain-giv', None, {"name": [{"given": ["abcd", "ef"]}]}), 'no profile: given=[abcd,ef]')
add('c11-ext-giv', pat('c11-ext-giv', None, {"name": [{"given": ["ab"], "_given": [{"id": "a1", "extension": OTHX}]}]}), 'no profile: given[0] with id and extension')
add('c11-vl-giv', pat('c11-vl-giv', None, {"name": [{"given": [None, "ab"], "_given": [{"id": "z1"}, None]}]}), 'no profile: valueless given[0] with only an id (ele-1; R4B repeats it)')
add('c11-vlx-giv', pat('c11-vlx-giv', None, {"name": [{"given": [None, "ab"], "_given": [{"extension": OTHX}, None]}]}), 'no profile: valueless given[0] with an extension (valid)')
add('c11-vl-obs', obs('c11-vl-obs', None, {"_valueString": {"id": "z1"}}), 'no profile: valueless Observation.valueString with only an id')
add('c11-vl-bd', pat('c11-vl-bd', None, {"_birthDate": {"id": "z1"}}), 'no profile: valueless Patient.birthDate with only an id')
add('c11-lon-giv', pat('c11-lon-giv', None, {"name": [{"given": ["ab"], "_given": [None, {"id": "z1"}]}]}), 'no profile: _given longer than given, given[1] valueless with only an id')


# ---- case 12 (mutation gaps, 15-11-2026): inputs that kill the survivors of docs/plans/2026-11-15-primitive-type-profiles.md §7 -------------
# primitive profiles whose id/extension children constrain the sibling, and one with only a warning
prim('ps-extc', 'string', [el('string.extension', constraint=[cons('xc-1', "url = '%s'" % U('ext-req'), 'extension must be ext-req (ps-extc)')])])
prim('ps-extw', 'string', [el('string.extension', constraint=[cons('xw-1', "url = '%s'" % U('ext-req'), 'extension should be ext-req (ps-extw)', 'warning')])])
prim('ps-ext1b', 'string', [el('string.extension', min=1)])
# extensions reached only through a slice of a primitive profile: a base one, and profiles of it that restrict the value
sd('ext-code', 'Extension', 'complex-type', [el('Extension.url', fixedUri=U('ext-code')), el('Extension.value[x]', type=[{"code": "Coding"}])],
   ctx=[{"type": "element", "expression": "Element"}])
sd('ext-code-bind', 'Extension', 'complex-type', [el('Extension.url', fixedUri=U('ext-code')),
   el('Extension.value[x]', type=[{"code": "Coding"}], binding={"strength": "required", "valueSet": U('vs-pc')})], ctx=[{"type": "element", "expression": "Element"}], base=U('ext-code'))
sd('ext-code-ctx', 'Extension', 'complex-type', [el('Extension.url', fixedUri=U('ext-code'))],
   ctx=[{"type": "element", "expression": "Observation"}], base=U('ext-code'))
sd('ext-str', 'Extension', 'complex-type', [el('Extension.url', fixedUri=U('ext-str')), el('Extension.value[x]', type=[{"code": "string"}])],
   ctx=[{"type": "element", "expression": "Element"}])
sd('ext-str-con', 'Extension', 'complex-type', [el('Extension.url', fixedUri=U('ext-str')),
   el('Extension.value[x]', type=[{"code": "string"}], constraint=[cons('esc-1', '$this.length() <= 2', 'at most 2 characters (ext-str-con)')])],
   ctx=[{"type": "element", "expression": "Element"}], base=U('ext-str'))
sd('ext-ref', 'Extension', 'complex-type', [el('Extension.url', fixedUri=U('ext-ref')), el('Extension.value[x]', type=[{"code": "Reference"}])],
   ctx=[{"type": "element", "expression": "Element"}])
sd('ext-ref-t', 'Extension', 'complex-type', [el('Extension.url', fixedUri=U('ext-ref')),
   el('Extension.value[x]', type=[{"code": "Reference", "targetProfile": [HL7 + 'Patient']}])], ctx=[{"type": "element", "expression": "Element"}], base=U('ext-ref'))
for _id, _url, _pf in (('ps-xbind', 'ext-code', 'ext-code-bind'), ('ps-xfix', 'ext-str', 'ext-str-con'), ('ps-xctx', 'ext-code', 'ext-code-ctx'), ('ps-xref', 'ext-ref', 'ext-ref-t')):
    prim(_id, 'string', [
        el('string.extension', slicing={"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}),
        el('string.extension', id='string.extension:s', sliceName='s', type=[{"code": "Extension", "profile": [U(_pf)]}]),
        el('string.extension.url', id='string.extension:s.url', fixedUri=U(_url))])

XB = {"extension": [{"url": U('ext-code'), "valueCoding": {"system": "urn:probe:cs", "code": "other"}}]}      # not in vs-pc
XM = {"extension": [{"url": U('ext-code'), "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]}
XSB = {"extension": [{"url": U('ext-str'), "valueString": "abc"}]}     # longer than 2
XSM = {"extension": [{"url": U('ext-str'), "valueString": "ab"}]}
XR = {"extension": [{"url": U('ext-ref'), "valueReference": {"reference": "#c1"}}]}      # c1 is an Observation: not a Patient
XP = XR                                                                                  # the same with c1 a Patient
# M12 and M16: id/extension children of the profiles that are not chosen or not first
for tag, profs, xs in (('idc-up', ['ps-idc', 'ps-up'], (('bad', {"id": "abcdef"}), ('ok', {"id": "a1"}))),
                       ('extc-up', ['ps-extc', 'ps-up'], (('bad', OTHX), ('ok', SUBX))),
                       ('len-extw', ['ps-len', 'ps-extw'], (('bad', OTHX), ('ok', SUBX))),
                       ('extw-len', ['ps-extw', 'ps-len'], (('bad', OTHX), ('ok', SUBX)))):
    val = 'AB' if 'up' in tag else 'ab'
    for w, x in xs:
        ext = x if 'id' in x else {"extension": x}
        on_obs('c12-%s-%s-obs' % (tag, w), profs, val, '%s on valueString=%r, _valueString=%s' % ('+'.join(profs), val, json.dumps(ext)), ext=ext)
        on_given('c12-%s-%s-giv' % (tag, w), profs, [val], '%s on given[0]=%r, _given[0]=%s' % ('+'.join(profs), val, json.dumps(ext)), ext=[ext])
# M34: two profiles that require the same extension, _x with none
for w, ext in (('none', {"id": "a1"}), ('x', OT)):
    on_obs('c12-ext1x2-%s-obs' % w, ['ps-ext1', 'ps-ext1b'], 'ab', 'ps-ext1+ps-ext1b (both extension min 1), _valueString=%s' % json.dumps(ext), ext=ext)
    on_given('c12-ext1x2-%s-giv' % w, ['ps-ext1', 'ps-ext1b'], ['ab'], 'ps-ext1+ps-ext1b (both extension min 1), _given[0]=%s' % json.dumps(ext), ext=[ext])
# M18 (context, value), M35 (binding), M36 (reference): the profile of a slice of an extension, in a profile that is not chosen
for tag, pid, xs in (('bind', 'ps-xbind', (('bad', XB), ('ok', XM))), ('fix', 'ps-xfix', (('bad', XSB), ('ok', XSM))),
                     ('ctx', 'ps-xctx', (('x', XM),)), ('ref', 'ps-xref', (('bad', XR), ('ok', XP)))):
    for w, x in xs:
        for nm, profs, val in (('one', [pid], 'ab'), ('pair', ['ps-len', pid], 'ab'), ('both', ['ps-len', pid], 'abcd')):
            n = 'c12-xs-%s-%s-%s' % (tag, w, nm)
            if tag == 'ref':     # the reference is to a contained resource, which HL7 resolves: an Observation (bad) or a Patient (ok)
                c = {"resourceType": "Patient" if w == 'ok' else "Observation", "id": "c1"}
                if w != 'ok':
                    c.update({"status": "final", "code": {"text": "x"}})
                p = carrier('o', 'string', profs)
                add(n, obs(n, p, {"valueString": val, "_valueString": x, "contained": [c]}), '%s on valueString=%s, a reference to a contained %s' % ('+'.join(profs), val, c["resourceType"]))
                continue
            on_obs(n, profs, val, '%s on valueString=%s, _valueString=%s' % ('+'.join(profs), val, json.dumps(x)), ext=x)
        if tag != 'ref':
            on_given('c12-xs-%s-%s-giv' % (tag, w), ['ps-len', pid], ['ab'], 'ps-len+%s on given[0]=ab, _given[0]=%s' % (pid, json.dumps(x)), ext=[x])

# slices of Patient.name.given by type=profile (M23 to M26, M37, M04, M10), by value (M06, M21), and a HumanName profile (M20)
def given_slices(i, disc, rules, slices):
    elems = [el('Patient.name.given', slicing={"discriminator": [disc], "rules": rules})]
    for sn, profs, extra in slices:
        e = el('Patient.name.given', id='Patient.name.given:' + sn, sliceName=sn, type=[{"code": "string", "profile": [purl(p) for p in profs]}])
        e.update(extra)
        elems.append(e)
    sd(i, 'Patient', 'resource', elems)


PD = {"type": "profile", "path": "$this"}
given_slices('pgp-open', PD, 'open', [('a', ['ps-ext1'], {"min": 1}), ('b', ['ps-up'], {})])
given_slices('pgp-closed', PD, 'closed', [('a', ['ps-ext1'], {}), ('b', ['ps-up'], {})])
given_slices('pgp-type', PD, 'closed', [('a', ['pc-ext'], {}), ('b', ['ps-up'], {})])
given_slices('pgp-one', PD, 'closed', [('a', ['ps-ext1'], {})])
for cid in ('pgp-open', 'pgp-closed', 'pgp-one'):
    for tag, given, ext in (('x', ['ab'], [OX]), ('up', ['AB'], None), ('none', ['ab'], None),
                            ('mix', ['AB', 'ab', 'cd'], [None, OX, None])):
        n = 'c12-%s-%s' % (cid, tag)
        add(n, pat(n, cid, {"name": [dict({"given": given}, **({"_given": ext} if ext else {}))]}), '%s on given=%s, _given=%s' % (cid, given, json.dumps(ext)))
    # M04: the same value in two names, only one has the extension
    for tag, a, b in (('ab', OX, None), ('ba', None, OX)):
        n = 'c12-%s-names-%s' % (cid, tag)
        add(n, pat(n, cid, {"name": [dict({"given": ['ab']}, **({"_given": [a]} if a else {})), dict({"given": ['ab']}, **({"_given": [b]} if b else {}))]}),
            '%s: name[0] and name[1] with given ab; extension only in %s' % (cid, 'name[0]' if a else 'name[1]'))
for tag, given in (('male', ['male']), ('up', ['AB']), ('none', ['xx'])):
    n = 'c12-pgp-type-' + tag
    add(n, pat(n, 'pgp-type', {"name": [{"given": given}]}), 'pgp-type (slice a with a code profile on a string, b ps-up) on given=%s' % given)
# M02: two elements of the same name with the same two profiles
# (HL7 solo toma el differential en el orden del tipo base: text va antes que family)
sd('pft', 'Patient', 'resource', [el('Patient.name.text', type=[{"code": "string", "profile": [U('ps-ext1'), U('ps-len')]}]),
                                  el('Patient.name.family', type=[{"code": "string", "profile": [U('ps-ext1'), U('ps-len')]}])])
for tag, fx, tx in (('f', OX, None), ('t', None, OX), ('ft', OX, OX), ('none', None, None)):
    n = 'c12-pft-' + tag
    nm = {"family": "ab", "text": "ab"}
    if fx:
        nm["_family"] = fx
    if tx:
        nm["_text"] = tx
    add(n, pat(n, 'pft', {"name": [nm]}), 'pft: name.family and name.text both ab with ps-ext1+ps-len; _family=%s, _text=%s' % (json.dumps(fx), json.dumps(tx)))
# M06, M21: a slice by value whose type.profile is a primitive profile
for cid, profs in (('pgs1', ['ps-sub']), ('pgs2', ['ps-sub', 'ps-len']), ('pgs2r', ['ps-len', 'ps-sub'])):
    given_slices(cid, {"type": "value", "path": "$this"}, 'open', [('s', profs, {"patternString": "ab"})])
    for tag, given, ext in (('none', ['ab'], None), ('sub', ['ab'], [SU]), ('oth', ['ab'], [OT]), ('long', ['abcd'], None),
                            ('two', ['cd', 'ab'], [None, None]), ('twox', ['cd', 'ab'], [None, SU])):
        n = 'c12-%s-%s' % (cid, tag)
        add(n, pat(n, cid, {"name": [dict({"given": given}, **({"_given": ext} if ext else {}))]}), '%s (slice by pattern ab, profiles %s) on given=%s, _given=%s' % (cid, '+'.join(profs), given, json.dumps(ext)))
# M20: a Patient.name whose HumanName profile has a given with a primitive profile
sd('hn-sub', 'HumanName', 'complex-type', [el('HumanName.given', type=[{"code": "string", "profile": [U('ps-sub')]}])])
sd('hn-sublen', 'HumanName', 'complex-type', [el('HumanName.given', type=[{"code": "string", "profile": [U('ps-sub'), U('ps-len')]}])])
for cid, hn in (('pnh1', 'hn-sub'), ('pnh2', 'hn-sublen')):
    sd(cid, 'Patient', 'resource', [el('Patient.name', type=[{"code": "HumanName", "profile": [U(hn)]}])])
    for tag, given, ext in (('none', ['ab'], None), ('sub', ['ab'], [SU]), ('oth', ['ab'], [OT]), ('long', ['abcd'], None)):
        n = 'c12-%s-%s' % (cid, tag)
        add(n, pat(n, cid, {"name": [dict({"given": given}, **({"_given": ext} if ext else {}))]}), '%s (Patient.name profile %s) on given=%s, _given=%s' % (cid, hn, given, json.dumps(ext)))


# ---- case 12b: the second round of gaps (M04, M06, M18, M20, M21, M25, M37), slicing-only and path-walking inputs --------------------------------
# a closed slicing in a primitive profile: its error comes only from the slicing phase (the cardinality phase has nothing to report)
prim('ps-subc', 'string', [
    el('string.extension', slicing={"discriminator": [{"type": "value", "path": "url"}], "rules": "closed"}),
    el('string.extension', id='string.extension:req', sliceName='req', type=[{"code": "Extension"}]),
    el('string.extension.url', id='string.extension:req.url', fixedUri=U('ext-req'))])
sd('hn-subc', 'HumanName', 'complex-type', [el('HumanName.given', type=[{"code": "string", "profile": [U('ps-subc')]}])])
sd('hn-subclen', 'HumanName', 'complex-type', [el('HumanName.given', type=[{"code": "string", "profile": [U('ps-subc'), U('ps-len')]}])])
for cid, hn in (('pnhc1', 'hn-subc'), ('pnhc2', 'hn-subclen')):                                       # M20
    sd(cid, 'Patient', 'resource', [el('Patient.name', type=[{"code": "HumanName", "profile": [U(hn)]}])])
    for tag, given, ext in (('none', ['ab'], None), ('sub', ['ab'], [SU]), ('oth', ['ab'], [OT]), ('long', ['abcd'], [OT])):
        n = 'c12-%s-%s' % (cid, tag)
        add(n, pat(n, cid, {"name": [dict({"given": given}, **({"_given": ext} if ext else {}))]}), '%s (Patient.name profile %s) on given=%s, _given=%s' % (cid, hn, given, json.dumps(ext)))
for cid, profs in (('pgsc1', ['ps-subc']), ('pgsc2', ['ps-subc', 'ps-len']), ('pgsc2r', ['ps-len', 'ps-subc'])):   # M06, M21
    given_slices(cid, {"type": "value", "path": "$this"}, 'open', [('s', profs, {"patternString": "ab"})])
    for tag, given, ext in (('none', ['ab'], None), ('sub', ['ab'], [SU]), ('oth', ['ab'], [OT]), ('long', ['abcd'], [OT]),
                            ('two', ['cd', 'ab'], [OT, None]), ('twox', ['cd', 'ab'], [None, OT])):
        n = 'c12-%s-%s' % (cid, tag)
        add(n, pat(n, cid, {"name": [dict({"given": given}, **({"_given": ext} if ext else {}))]}), '%s (slice by pattern ab, profiles %s) on given=%s, _given=%s' % (cid, '+'.join(profs), given, json.dumps(ext)))
# M37: a constraint of the slice that a member matched by type=profile must meet
given_slices('pgp-con', PD, 'open', [('a', ['ps-ext1'], {"constraint": [cons('sc-1', "$this = 'zz'", 'a member of slice a must be zz')]}), ('b', ['ps-up'], {})])
for tag, given, ext in (('x', ['ab'], [OX]), ('up', ['AB'], None), ('none', ['ab'], None), ('mix', ['AB', 'ab'], [None, OX])):
    n = 'c12-pgp-con-' + tag
    add(n, pat(n, 'pgp-con', {"name": [dict({"given": given}, **({"_given": ext} if ext else {}))]}), 'pgp-con (slice a ps-ext1 with a constraint) on given=%s, _given=%s' % (given, json.dumps(ext)))
# M04, M25: a slicing of Patient.name by the profile of the primitive at its path `family`
for cid, rules in (('pnf-closed', 'closed'), ('pnf-open', 'open')):
    sd(cid, 'Patient', 'resource', [
        el('Patient.name', slicing={"discriminator": [{"type": "profile", "path": "family"}], "rules": rules}),
        el('Patient.name', id='Patient.name:a', sliceName='a', min=1 if rules == 'open' else 0),
        el('Patient.name.family', id='Patient.name:a.family', type=[{"code": "string", "profile": [U('ps-ext1')]}]),
        el('Patient.name', id='Patient.name:b', sliceName='b'),
        el('Patient.name.family', id='Patient.name:b.family', type=[{"code": "string", "profile": [U('ps-up')]}])])
    for tag, names in (('x', [{"family": "ab", "_family": OX}]), ('up', [{"family": "AB"}]), ('none', [{"family": "ab"}]),
                       ('ab', [{"family": "ab", "_family": OX}, {"family": "ab"}]), ('ba', [{"family": "ab"}, {"family": "ab", "_family": OX}]),
                       ('upx', [{"family": "AB"}, {"family": "ab", "_family": OX}])):
        n = 'c12-%s-%s' % (cid, tag)
        add(n, pat(n, cid, {"name": names}), '%s (Patient.name sliced by the profile of family) on %s' % (cid, json.dumps(names)))
# M18: the extension profile of a slice of a profile that is not chosen restricts the value's type
sd('ext-two', 'Extension', 'complex-type', [el('Extension.url', fixedUri=U('ext-two')), el('Extension.value[x]', type=[{"code": "string"}, {"code": "integer"}])],
   ctx=[{"type": "element", "expression": "Element"}])
sd('ext-two-str', 'Extension', 'complex-type', [el('Extension.url', fixedUri=U('ext-two')), el('Extension.value[x]', type=[{"code": "string"}])],
   ctx=[{"type": "element", "expression": "Element"}], base=U('ext-two'))
prim('ps-xstr', 'string', [
    el('string.extension', slicing={"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}),
    el('string.extension', id='string.extension:s', sliceName='s', type=[{"code": "Extension", "profile": [U('ext-two-str')]}]),
    el('string.extension.url', id='string.extension:s.url', fixedUri=U('ext-two'))])
for w, x in (('bad', {"extension": [{"url": U('ext-two'), "valueInteger": 5}]}), ('ok', {"extension": [{"url": U('ext-two'), "valueString": "q"}]})):
    for nm, profs, val in (('one', ['ps-xstr'], 'ab'), ('pair', ['ps-len', 'ps-xstr'], 'ab'), ('both', ['ps-len', 'ps-xstr'], 'abcd')):
        on_obs('c12-xstr-%s-%s' % (w, nm), profs, val, '%s on valueString=%s, _valueString=%s' % ('+'.join(profs), val, json.dumps(x)), ext=x)
    on_given('c12-xstr-%s-giv' % w, ['ps-len', 'ps-xstr'], ['ab'], 'ps-len+ps-xstr on given[0]=ab, _given[0]=%s' % json.dumps(x), ext=[x])


# ---- case 12c: the third round (M06, M20, M21): the slice's own children, a member that fails every profile, a slice of Patient.name with a given profile ------
for cid, profs in (('pgsd1', ['ps-subc']), ('pgsd2', ['ps-subc', 'ps-len']), ('pgsd2r', ['ps-len', 'ps-subc'])):   # a member (abcd) that fails the length profile
    given_slices(cid, {"type": "value", "path": "$this"}, 'open', [('s', profs, {"patternString": "abcd"})])
    for tag, given, ext in (('none', ['abcd'], None), ('sub', ['abcd'], [SU]), ('oth', ['abcd'], [OT])):
        n = 'c12-%s-%s' % (cid, tag)
        add(n, pat(n, cid, {"name": [dict({"given": given}, **({"_given": ext} if ext else {}))]}), '%s (slice by pattern abcd, profiles %s) on given=%s, _given=%s' % (cid, '+'.join(profs), given, json.dumps(ext)))
for cid, profs in (('pgsx1', ['ps-len']), ('pgsx2', ['ps-len', 'ps-up'])):   # the slice declares its own child (extension min 1) besides its type profiles
    elems = [el('Patient.name.given', slicing={"discriminator": [{"type": "value", "path": "$this"}], "rules": "open"}),
             el('Patient.name.given', id='Patient.name.given:s', sliceName='s', patternString='ab', type=[{"code": "string", "profile": [purl(p) for p in profs]}]),
             el('Patient.name.given.extension', id='Patient.name.given:s.extension', min=1)]
    sd(cid, 'Patient', 'resource', elems)
    for tag, given, ext in (('none', ['ab'], None), ('x', ['ab'], [OX]), ('two', ['cd', 'ab'], [OX, None])):
        n = 'c12-%s-%s' % (cid, tag)
        add(n, pat(n, cid, {"name": [dict({"given": given}, **({"_given": ext} if ext else {}))]}), '%s (slice s by pattern ab with its own extension min 1, profiles %s) on given=%s, _given=%s' % (cid, '+'.join(profs), given, json.dumps(ext)))
# M20: a slice of Patient.name (by use) whose given has a primitive profile with a required slice
sd('pnhs', 'Patient', 'resource', [
    el('Patient.name', slicing={"discriminator": [{"type": "value", "path": "use"}], "rules": "open"}),
    el('Patient.name', id='Patient.name:off', sliceName='off'),
    el('Patient.name.use', id='Patient.name:off.use', fixedCode='official'),
    el('Patient.name.given', id='Patient.name:off.given', type=[{"code": "string", "profile": [U('ps-sub')]}])])
sd('pnhs2', 'Patient', 'resource', [
    el('Patient.name', slicing={"discriminator": [{"type": "value", "path": "use"}], "rules": "open"}),
    el('Patient.name', id='Patient.name:off', sliceName='off'),
    el('Patient.name.use', id='Patient.name:off.use', fixedCode='official'),
    el('Patient.name.given', id='Patient.name:off.given', type=[{"code": "string", "profile": [U('ps-subc'), U('ps-len')]}])])
sd('pnhs3', 'Patient', 'resource', [
    el('Patient.name', slicing={"discriminator": [{"type": "value", "path": "use"}], "rules": "open"}),
    el('Patient.name', id='Patient.name:off', sliceName='off'),
    el('Patient.name.use', id='Patient.name:off.use', fixedCode='official'),
    el('Patient.name.given', id='Patient.name:off.given', type=[{"code": "string", "profile": [U('ps-subc')]}])])
for cid in ('pnhs', 'pnhs2', 'pnhs3'):
    for tag, names in (('off-none', [{"use": "official", "given": ["ab"]}]), ('off-sub', [{"use": "official", "given": ["ab"], "_given": [SU]}]),
                       ('off-oth', [{"use": "official", "given": ["abcd"], "_given": [OT]}]), ('usual', [{"use": "usual", "given": ["ab"]}]),
                       ('both', [{"use": "usual", "given": ["ab"]}, {"use": "official", "given": ["ab"]}])):
        n = 'c12-%s-%s' % (cid, tag)
        add(n, pat(n, cid, {"name": names}), '%s (Patient.name sliced by use; the official one has a given profile) on %s' % (cid, json.dumps(names)))


# ---- case 13 (code review round 1, 15-11-2026): R1 the decimal's text decides the choice among profiles; R2 unrolled children and unresolved profiles ------
# R1: 1.50 is not 1.5 (json.html#primitive): precision() and toString() read the text the JSON spells. The sentinel "DEC:1.50" is written as the number.
prim('pdec-p2', 'decimal', [el('decimal', constraint=[cons('dp-1', 'precision() = 2', 'precision is 2 (pdec-p2)')])])
prim('pdec-p3', 'decimal', [el('decimal', constraint=[cons('dp-3', 'precision() = 3', 'precision is 3 (pdec-p3)')])])
prim('pdec-ts', 'decimal', [el('decimal', constraint=[cons('dt-1', "toString() = '1.50'", 'text is 1.50 (pdec-ts)')])])
prim('pdec-ts2', 'decimal', [el('decimal', constraint=[cons('dt-2', "toString() = '1.500'", 'text is 1.500 (pdec-ts2)')])])
for tag, profs, val in (('p2-one', ['pdec-p2'], 'DEC:1.50'), ('p2-one5', ['pdec-p2'], 'DEC:1.5'),
                        ('p2-pair', ['pdec-p2', 'pdec-p3'], 'DEC:1.50'), ('p2-pairr', ['pdec-p3', 'pdec-p2'], 'DEC:1.50'),
                        ('p2-pair5', ['pdec-p2', 'pdec-p3'], 'DEC:1.5'), ('p2-pair3', ['pdec-p2', 'pdec-p3'], 'DEC:1.500'),
                        ('ts-one', ['pdec-ts'], 'DEC:1.50'), ('ts-pair', ['pdec-ts', 'pdec-ts2'], 'DEC:1.50'),
                        ('ts-pairr', ['pdec-ts2', 'pdec-ts'], 'DEC:1.50'), ('ts-pair3', ['pdec-ts', 'pdec-ts2'], 'DEC:1.500'),
                        ('ts-pair1', ['pdec-ts', 'pdec-ts2'], 'DEC:1.5')):
    on_ext('c13-dec-' + tag, profs, val, '%s on Extension.valueDecimal=%s' % ('+'.join(profs), val[4:]), 'decimal', 'valueDecimal')
# R2: a profile that declares the children of the element (here given.extension) and a type profile that does not resolve
for tag, elems in (('ext', [el('Patient.name.given.extension', max='1')]),
                   ('extmin', [el('Patient.name.given.extension', min=1)]),
                   ('id', [el('Patient.name.given.id', max='1')])):
    sd('pgu-' + tag, 'Patient', 'resource', [el('Patient.name.given', type=[{"code": "string", "profile": [MISSING]}])] + elems)
    for itag, n in (('none', {"given": ["ab"]}), ('x', {"given": ["ab"], "_given": [{"extension": [{"url": U('ext-other'), "valueString": "q"}]}]})):
        nm = 'c13-pgu-%s-%s' % (tag, itag)
        add(nm, pat(nm, 'pgu-' + tag, {"name": [n]}), 'pgu-%s (given has a type profile that does not resolve, and the child %s) on %s' % (tag, tag, json.dumps(n)))
# R2 on a datatype: Patient.name with an unresolved HumanName profile and an unrolled child (family)
for tag, elems in (('fam', [el('Patient.name.family', min=1)]), ('use', [el('Patient.name.use', max='1')])):
    sd('pnu-' + tag, 'Patient', 'resource', [el('Patient.name', type=[{"code": "HumanName", "profile": [MISSING]}])] + elems)
    for itag, n in (('fam', {"family": "x"}), ('giv', {"given": ["a"]})):
        nm = 'c13-pnu-%s-%s' % (tag, itag)
        add(nm, pat(nm, 'pnu-' + tag, {"name": [n]}), 'pnu-%s (name has a HumanName profile that does not resolve, and the child %s) on %s' % (tag, tag, json.dumps(n)))
# R2 with two profiles, one that does not resolve and one that does, besides the child
sd('pgu-two', 'Patient', 'resource', [el('Patient.name.given', type=[{"code": "string", "profile": [MISSING, U('ps-len')]}]), el('Patient.name.given.extension', max='1')])
for itag, n in (('ok', {"given": ["ab"]}), ('bad', {"given": ["abcd"]})):
    nm = 'c13-pgu-two-' + itag
    add(nm, pat(nm, 'pgu-two', {"name": [n]}), 'pgu-two (given has an unresolved profile and ps-len, and the child extension) on %s' % json.dumps(n))


# controls of R2: the same children with no type profile declared, to tell what HL7 reports of them by itself
sd('pgc-extmin', 'Patient', 'resource', [el('Patient.name.given.extension', min=1)])
sd('pnc-fam', 'Patient', 'resource', [el('Patient.name.family', min=1)])
for itag, n in (('none', {"given": ["ab"]}), ('x', {"given": ["ab"], "_given": [{"extension": [{"url": U('ext-other'), "valueString": "q"}]}]})):
    nm = 'c13-pgc-extmin-' + itag
    add(nm, pat(nm, 'pgc-extmin', {"name": [n]}), 'control: pgc-extmin (given.extension min 1, no type profile) on %s' % json.dumps(n))
nm = 'c13-pnc-fam-giv'
add(nm, pat(nm, 'pnc-fam', {"name": [{"given": ["a"]}]}), 'control: pnc-fam (name.family min 1, no type profile) on a name with no family')


# ---- case 14 (code review round 2, S1, 15-11-2026): the same constraint key declared by the owner and by a type profile, or by a chain of profiles ------
# the owner and its type profile declare sev-1 with the same text, with severity (and in some probes expression) apart; HL7 tells which report stays
SEV = lambda sev, expr='false': [cons('sev-1', expr, 'sev-1 %s (%s)' % (expr, sev), sev)]
prim('s14-pe', 'string', [el('string', constraint=SEV('error'))])
prim('s14-pw', 'string', [el('string', constraint=SEV('warning'))])
prim('s14-pe2', 'string', [el('string', constraint=SEV('error', '1 = 2'))])
sd('s14-he', 'HumanName', 'complex-type', [el('HumanName', constraint=SEV('error'))])
sd('s14-hw', 'HumanName', 'complex-type', [el('HumanName', constraint=SEV('warning'))])
sd('s14-he2', 'HumanName', 'complex-type', [el('HumanName', constraint=SEV('error', '1 = 2'))])
# (a) the primitive: owner Patient.name.given (+ type profile)
for tag, osev, oexpr, prof in (('a1', 'warning', 'false', 's14-pe'), ('a2', 'error', 'false', 's14-pw'), ('a3', 'warning', 'false', 's14-pe2'),
                               ('a4', 'error', 'false', 's14-pe'), ('a5', 'warning', 'false', None), ('a6', None, None, 's14-pe')):
    pid = 's14-%s' % tag
    ty = [{"code": "string"}]
    if prof:
        ty[0]["profile"] = [U(prof)]
    e = el('Patient.name.given', type=ty)
    if osev:
        e["constraint"] = SEV(osev, oexpr)
    sd(pid, 'Patient', 'resource', [e])
    add('c14-' + tag, pat('c14-' + tag, pid, {"name": [{"given": ["ab"]}]}),
        '%s (Patient.name.given: %s, type profile %s) on given=["ab"]' % (pid, ('sev-1 %s `%s`' % (osev, oexpr)) if osev else 'no constraint', prof or 'none'))
# (b) the datatype: owner Patient.name (+ HumanName profile)
for tag, osev, oexpr, prof in (('b1', 'warning', 'false', 's14-he'), ('b2', 'error', 'false', 's14-hw'), ('b3', 'warning', 'false', 's14-he2'),
                               ('b4', 'error', 'false', 's14-he'), ('b5', 'warning', 'false', None), ('b6', None, None, 's14-he')):
    pid = 's14-%s' % tag
    ty = [{"code": "HumanName"}]
    if prof:
        ty[0]["profile"] = [U(prof)]
    e = el('Patient.name', type=ty)
    if osev:
        e["constraint"] = SEV(osev, oexpr)
    sd(pid, 'Patient', 'resource', [e])
    add('c14-' + tag, pat('c14-' + tag, pid, {"name": [{"family": "x"}]}),
        '%s (Patient.name: %s, type profile %s) on name=[{"family": "x"}]' % (pid, ('sev-1 %s `%s`' % (osev, oexpr)) if osev else 'no constraint', prof or 'none'))
# (c) no type profiles: a slice of Patient.name (by use) that re-declares the constraint of the profile it derives from, with another severity
for tag, bsev, ssev, sexpr in (('c1', 'warning', 'error', 'false'), ('c2', 'error', 'warning', 'false'), ('c3', 'warning', 'error', '1 = 2'), ('c4', 'error', 'error', 'false')):
    base, pid = 's14-%sb' % tag, 's14-%s' % tag
    sd(base, 'Patient', 'resource', [el('Patient.name', constraint=SEV(bsev))], base=HL7 + 'Patient')
    sd(pid, 'Patient', 'resource', [
        el('Patient.name', slicing={"discriminator": [{"type": "value", "path": "use"}], "rules": "open"}),
        el('Patient.name', id='Patient.name:off', sliceName='off', constraint=SEV(ssev, sexpr)),
        el('Patient.name.use', id='Patient.name:off.use', fixedCode='official')], base=U(base))
    for itag, names in (('off', [{"use": "official", "family": "x"}]), ('usual', [{"use": "usual", "family": "x"}])):
        n = 'c14-%s-%s' % (tag, itag)
        add(n, pat(n, pid, {"name": names}), '%s (base: Patient.name sev-1 %s; slice off re-declares sev-1 %s `%s`) on %s' % (pid, bsev, ssev, sexpr, json.dumps(names)))
# (c') the same without a slice: the derived profile re-declares the constraint on Patient.name itself
for tag, bsev, ssev in (('c5', 'warning', 'error'), ('c6', 'error', 'warning')):
    base, pid = 's14-%sb' % tag, 's14-%s' % tag
    sd(base, 'Patient', 'resource', [el('Patient.name', constraint=SEV(bsev))], base=HL7 + 'Patient')
    sd(pid, 'Patient', 'resource', [el('Patient.name', constraint=SEV(ssev))], base=U(base))
    n = 'c14-%s' % tag
    add(n, pat(n, pid, {"name": [{"family": "x"}]}), '%s (base: Patient.name sev-1 %s; derived re-declares sev-1 %s `false`, no slice) on name=[{"family": "x"}]' % (pid, bsev, ssev))
# (d) a resource profile and its base declare the same key and expression on the root with another severity
for tag, bsev, dsev, dexpr in (('d1', 'warning', 'error', 'false'), ('d2', 'error', 'warning', 'false'), ('d3', 'warning', 'error', '1 = 2'), ('d4', 'error', 'error', 'false')):
    base, pid = 's14-%sb' % tag, 's14-%s' % tag
    sd(base, 'Patient', 'resource', [el('Patient', constraint=SEV(bsev))], base=HL7 + 'Patient')
    sd(pid, 'Patient', 'resource', [el('Patient', constraint=SEV(dsev, dexpr))], base=U(base))
    n = 'c14-%s' % tag
    add(n, pat(n, pid, {"name": [{"family": "x"}]}), '%s (base: Patient sev-1 %s; derived sev-1 %s `%s`) on a Patient' % (pid, bsev, dsev, dexpr))


# ---- case 15 (code review round 3, mutations S1g and S1i, 16-11-2026): the owner's key repeated by TWO type profiles, and in a slice's base --------------
# S1g: Patient.name.given declares sev-1 and so do both of its type profiles (they conform, one does, none does; severity and expression apart)
prim('s15-ok', 'string', [el('string', constraint=SEV('error', 'true'))])
prim('s15-ok2', 'string', [el('string', constraint=SEV('warning', 'length() <= 3'))])
prim('s15-bad', 'string', [el('string', constraint=SEV('error', 'false'))])
prim('s15-bad2', 'string', [el('string', constraint=SEV('warning', '1 = 2'))])
sd('s15-hok', 'HumanName', 'complex-type', [el('HumanName', constraint=SEV('error', 'true'))])
sd('s15-hbad', 'HumanName', 'complex-type', [el('HumanName', constraint=SEV('error', 'false'))])
sd('s15-hbad2', 'HumanName', 'complex-type', [el('HumanName', constraint=SEV('warning', '1 = 2'))])
for tag, osev, profs in (('g1', 'warning', ['s15-ok', 's15-ok2']), ('g2', 'warning', ['s15-ok', 's15-bad']), ('g2r', 'warning', ['s15-bad', 's15-ok']),
                         ('g3', 'warning', ['s15-bad', 's15-bad2']), ('g3r', 'warning', ['s15-bad2', 's15-bad']), ('g4', 'error', ['s15-bad', 's15-bad2']),
                         ('g5', 'warning', ['s15-bad', 's15-ok2'])):
    pid = 's15-' + tag
    sd(pid, 'Patient', 'resource', [el('Patient.name.given', type=[{"code": "string", "profile": [U(p) for p in profs]}], constraint=SEV(osev, 'false'))])
    add('c15-' + tag, pat('c15-' + tag, pid, {"name": [{"given": ["ab"]}]}),
        '%s (Patient.name.given: sev-1 %s `false`; type profiles %s) on given=["ab"]' % (pid, osev, ' + '.join(profs)))
for tag, osev, profs in (('h1', 'warning', ['s15-hok', 's14-he']), ('h2', 'warning', ['s15-hbad', 's15-hbad2']), ('h3', 'warning', ['s15-hok', 's15-hbad'])):
    pid = 's15-' + tag
    sd(pid, 'Patient', 'resource', [el('Patient.name', type=[{"code": "HumanName", "profile": [U(p) for p in profs]}], constraint=SEV(osev, 'false'))])
    add('c15-' + tag, pat('c15-' + tag, pid, {"name": [{"family": "x"}]}),
        '%s (Patient.name: sev-1 %s `false`; type profiles %s) on name=[{"family": "x"}]' % (pid, osev, ' + '.join(profs)))
# S1i: the key is declared by the element a slice divides (not by the slice), and the slice declares the type profile
SLH = {"slicing": {"discriminator": [{"type": "value", "path": "use"}], "rules": "open"}}
def slc_given(sev, prof):
    return [el('Patient.name', **SLH), el('Patient.name.given', constraint=SEV(sev, 'false')),
            el('Patient.name', id='Patient.name:off', sliceName='off'),
            el('Patient.name.use', id='Patient.name:off.use', fixedCode='official'),
            el('Patient.name.given', id='Patient.name:off.given', type=[{"code": "string", "profile": [U(prof)]}])]
sd('s15-i1', 'Patient', 'resource', slc_given('warning', 's14-pe'))
sd('s15-i2', 'Patient', 'resource', slc_given('error', 's14-pw'))
sd('s15-i3', 'Patient', 'resource', [
    el('Patient.name', slicing={"discriminator": [{"type": "value", "path": "use"}], "rules": "open"}, constraint=SEV('warning', 'false')),
    el('Patient.name', id='Patient.name:off', sliceName='off', type=[{"code": "HumanName", "profile": [U('s14-he')]}]),
    el('Patient.name.use', id='Patient.name:off.use', fixedCode='official')])
sd('s15-i4', 'Patient', 'resource', [
    el('Patient.name', slicing={"discriminator": [{"type": "value", "path": "use"}], "rules": "open"}, constraint=SEV('error', 'false')),
    el('Patient.name', id='Patient.name:off', sliceName='off', type=[{"code": "HumanName", "profile": [U('s14-hw')]}]),
    el('Patient.name.use', id='Patient.name:off.use', fixedCode='official')])
for pid, what in (('s15-i1', 'given sev-1 warning on the sliced element, string profile s14-pe on the slice'),
                  ('s15-i2', 'given sev-1 error on the sliced element, string profile s14-pw on the slice'),
                  ('s15-i3', 'name sev-1 warning on the sliced element, HumanName profile s14-he on the slice'),
                  ('s15-i4', 'name sev-1 error on the sliced element, HumanName profile s14-hw on the slice')):
    for itag, names in (('off', [{"use": "official", "given": ["ab"], "family": "x"}]), ('usual', [{"use": "usual", "given": ["ab"], "family": "x"}])):
        n = 'c15-%s-%s' % (pid[4:], itag)
        add(n, pat(n, pid, {"name": names}), '%s (%s) on %s' % (pid, what, json.dumps(names)))


# ---- case 16 (code review round 3, T1 and T2, 16-11-2026): a valueless primitive, and a type profile that does not resolve --------------------------
# T1: the case-14 primitives (a1..a6) on given=[null] with only _given (an extension); T2: the owner declares [resolved profile, missing profile] in both orders
EXTX = {"extension": [{"url": U('ext-other'), "valueString": "q"}]}
for tag in ('a1', 'a2', 'a3', 'a4', 'a5', 'a6'):
    n = 'c16-v' + tag[1]
    add(n, pat(n, 's14-' + tag, {"name": [{"given": [None], "_given": [EXTX]}]}),
        's14-%s (as in case 14) on given=[null] with _given=[{extension ext-other}] (no value)' % tag)
for pid, osev, profs in (('s16-u1', 'warning', ['s14-pe', 's16-missing']), ('s16-u2', 'error', ['s14-pw', 's16-missing']), ('s16-u3', 'warning', ['s16-missing', 's14-pe'])):
    sd(pid, 'Patient', 'resource', [el('Patient.name.given', type=[{"code": "string", "profile": [U(p) for p in profs]}], constraint=SEV(osev, 'false'))])
    n = 'c16-' + pid[-2:]
    add(n, pat(n, pid, {"name": [{"given": ["ab"]}]}), '%s (Patient.name.given: sev-1 %s `false`; type profiles %s, the last not loaded) on given=["ab"]' % (pid, osev, ' + '.join(profs)))
    add(n + 'v', pat(n + 'v', pid, {"name": [{"given": [None], "_given": [EXTX]}]}), '%s (as above) on given=[null] with _given=[{extension ext-other}] (no value)' % pid)


# ---- case 17 (closing round 3, 16-11-2026): the key on the element a slice divides, and the slice declares [resolved profile, missing profile] ----
# Mide la rama de shadowedByType con dos o más perfiles declarados y un dueño que no es el propio (un slice).
for pid, osev, profs in (('s17-k1', 'warning', ['s14-pe', 's16-missing']), ('s17-k2', 'error', ['s14-pw', 's16-missing']), ('s17-k3', 'warning', ['s16-missing', 's14-pe'])):
    sd(pid, 'Patient', 'resource', [el('Patient.name', **SLH), el('Patient.name.given', constraint=SEV(osev, 'false')),
                                     el('Patient.name', id='Patient.name:off', sliceName='off'),
                                     el('Patient.name.use', id='Patient.name:off.use', fixedCode='official'),
                                     el('Patient.name.given', id='Patient.name:off.given', type=[{"code": "string", "profile": [U(p) for p in profs]}])])
    for itag, names in (('off', [{"use": "official", "given": ["ab"], "family": "x"}]), ('usual', [{"use": "usual", "given": ["ab"], "family": "x"}])):
        n = 'c17-%s-%s' % (pid[4:], itag)
        add(n, pat(n, pid, {"name": names}), '%s (given sev-1 %s on the sliced element; the slice declares %s, the missing one not loaded) on %s' % (pid, osev, ' + '.join(profs), json.dumps(names)))


def write(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w') as f:
        # un número que el JSON debe escribir con su texto (1.50): el centinela "DEC:<texto>" se vuelve el número
        f.write(re.sub(r'"DEC:([0-9.]+)"', r'\1', json.dumps(obj, indent=1)))
        f.write('\n')


def main():
    for d in ['inst'] + ['ig-' + v for v in VERS]:
        shutil.rmtree(os.path.join(HERE, d), ignore_errors=True)
    for v in VERS:
        write(os.path.join(HERE, 'ig-' + v, 'ImplementationGuide-igx.json'), {
            "resourceType": "ImplementationGuide", "id": "igx", "url": "http://example.org/fhir/ImplementationGuide/igx", "name": "igx",
            "status": "draft", "packageId": "example.igx", "fhirVersion": [v],
            "definition": {"resource": [{"reference": {"reference": "StructureDefinition/qa"}, "exampleBoolean": False}]}})
        for f, b in PROFILES.items():
            write(os.path.join(HERE, 'ig-' + v, f), b(v))
    desc = []
    for n, (res, what) in INST.items():
        write(os.path.join(HERE, 'inst', n + '.json'), res)
        desc.append((n, what))
    write(os.path.join(HERE, 'descriptions.json'), desc)
    print(len(PROFILES), 'profiles/resources', len(INST), 'instances')


main()
