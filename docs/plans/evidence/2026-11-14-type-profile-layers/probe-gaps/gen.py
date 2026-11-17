"""Generador de las sondas de los huecos del oráculo de las mutaciones (3c, 3d, 10, 13a a 13c), 14-11-2026.
Uso: python3 -I gen.py. Escribe ig-<version>/ (los perfiles de ../probe-callers/ig-<version> más los nuevos) e inst/ (solo las
instancias nuevas). run.sh <version> corre HL7 validator_cli 7.0.4 (-tx n/a) sobre inst/ y deja hl7-<version>-7.0.4.json.
"""
import json, os, shutil

HERE = os.path.dirname(os.path.abspath(__file__))
BASE = os.path.join(HERE, '..', 'probe-callers')
VERS = ('4.0.1', '4.3.0', '5.0.0')
NS = 'http://example.org/fhir/StructureDefinition/'
HL7 = 'http://hl7.org/fhir/StructureDefinition/'
UCUM = 'http://unitsofmeasure.org'
DT = {'Quantity', 'Identifier', 'CodeableConcept', 'Reference', 'Range', 'Timing', 'Coding', 'Annotation'}
U = lambda i: NS + i
PROFILES = {}
INST = {}


def el(path, **kw):
    d = {"id": kw.pop('id', path), "path": path}
    d.update(kw)
    return d


def cons(key, expr, human, sev='error'):
    return {"key": key, "severity": sev, "human": human, "expression": expr}


def sd(i, typ, elems, ctx=None):
    def build(ver):
        d = {"resourceType": "StructureDefinition", "id": i, "url": U(i), "name": i.replace('-', '_'), "status": "draft",
             "fhirVersion": ver, "kind": "complex-type" if typ in DT or typ == 'Extension' else "resource", "abstract": False,
             "type": typ, "baseDefinition": HL7 + typ, "derivation": "constraint", "differential": {"element": elems}}
        if ctx:
            d["context"] = ctx
        return d
    PROFILES['StructureDefinition-%s.json' % i] = build


def obs(i, elem, typ, profiles):
    sd(i, 'Observation', [el(elem, type=[{"code": typ, "profile": [U(p) for p in profiles]}])])


def inst(name, prof, extra, what):
    INST[name] = (prof, extra, what)


# ---- profiles -----------------------------------------------------------------------------
# 3c y 3d: un valor Timing con dos perfiles; cada uno restringe el tipo de `Timing.repeat.bounds[x]` a uno distinto, y el
# segundo exige además `count`. El valor tiene un `boundsRange` (solo lo permite el segundo) sin `count`: no cumple ninguno
# y se camina por las dos capas. La extensión que va en `boundsRange` solo se alcanza por la segunda capa.
sd('tm-a', 'Timing', [el('Timing.repeat.bounds[x]', type=[{"code": "Duration"}])])
sd('tm-b', 'Timing', [el('Timing.repeat.bounds[x]', type=[{"code": "Range"}]), el('Timing.repeat.count', min=1)])
obs('obs-gt', 'Observation.effective[x]', 'Timing', ['tm-a', 'tm-b'])
sd('ext-rng', 'Extension', [el('Extension.url', fixedUri=U('ext-rng')), el('Extension.value[x]', type=[{"code": "integer"}])],
   ctx=[{"type": "element", "expression": "Range"}, {"type": "element", "expression": "Duration"}])
# ext-obsonly (contexto Observation.component) ya está en ../probe-callers/ig-<v>.

# 13a a 13c y 10: el %resource del dueño decide entre los perfiles de tipo de un slice (13a), de un hijo de un miembro de
# slice (13b, 13c) o del elemento que el slice divide (10), dentro de un contenido y de una entrada.
# cc-oc: la constraint de estado «final». cc-od: la de «amended» y, además, exige `text` (un hijo común, que el slicing comprueba
# para el miembro de un slice solo si la capa se elige): sin el hijo, el slicing no tendría nada de la segunda capa que informar.
sd('cc-oc', 'CodeableConcept', [el('CodeableConcept', constraint=[cons('ro-a', "%resource.status = 'final'", 'resource must be final (cc-oc)')])])
sd('cc-od', 'CodeableConcept', [
    el('CodeableConcept', constraint=[cons('ro-b', "%resource.status = 'amended'", 'resource must be amended (cc-od)')]),
    el('CodeableConcept.text', min=1)])
CC2 = [{"code": "CodeableConcept", "profile": [U('cc-oc'), U('cc-od')]}]
PAT = {"coding": [{"system": "urn:probe:p", "code": "c"}]}
SLC = {"discriminator": [{"type": "pattern", "path": "$this"}], "rules": "open"}
# 13a: el slice declara los dos perfiles
sd('obs-s13a', 'Observation', [
    el('Observation.category', slicing=SLC),
    el('Observation.category', id='Observation.category:cat', sliceName='cat', type=CC2, patternCodeableConcept=PAT)])
# 13b y 13c: el slice despliega un hijo que declara los dos perfiles
sd('obs-s13b', 'Observation', [
    el('Observation.component', slicing={"discriminator": [{"type": "pattern", "path": "code"}], "rules": "open"}),
    el('Observation.component', id='Observation.component:c', sliceName='c'),
    el('Observation.component.code', id='Observation.component:c.code', type=CC2, patternCodeableConcept=PAT)])
# 10: el elemento que el slice divide declara los dos perfiles, y el slice no declara ninguno
sd('obs-s10', 'Observation', [
    el('Observation.category', slicing=SLC, type=CC2),
    el('Observation.category', id='Observation.category:cat', sliceName='cat', patternCodeableConcept=PAT)])

# Ronda 1 de la revisión: dos alternativas de Identifier declaran, para `assigner`, perfiles de Reference distintos y que no
# existen. El valor `assigner` se alcanza por las dos capas del Identifier (una sola vez); cada perfil faltante se informa.
sd('id-ua', 'Identifier', [el('Identifier.assigner', type=[{"code": "Reference", "profile": [U('missing-a')]}])])
sd('id-ub', 'Identifier', [el('Identifier.assigner', type=[{"code": "Reference", "profile": [U('missing-b')]}])])
obs('obs-unk', 'Observation.identifier', 'Identifier', ['id-ua', 'id-ub'])
inst('unk', 'obs-unk', {"identifier": [{"assigner": {"display": "x"}}]},
     'round 1: two Identifier alternatives declare different missing profiles on assigner; each is unknown')

# ---- instances ----------------------------------------------------------------------------
# la extensión va en el propio boundsRange, sin `low`: el perfil que solo permite Duration no lo tiene, y HL7 lo informaría
# aparte. ext-rng admite Range y Duration: HL7 mira la extensión también como un Duration (bajo el primer perfil).
LOW = lambda ext: {"repeat": {"boundsRange": {"extension": ext}}}
inst('gtctx', 'obs-gt', {"effectiveTiming": LOW([{"url": U('ext-obsonly'), "valueString": "s"}])},
     'extension context: no profile met (boundsRange is only the 2nd layer\'s, whose count is missing); the extension on it is not for a Range')
inst('gtval', 'obs-gt', {"effectiveTiming": LOW([{"url": U('ext-rng'), "valueString": "oops"}])},
     'extension value: same value; the extension on boundsRange has a string where it needs an integer')
inst('gtok', 'obs-gt', {"effectiveTiming": LOW([{"url": U('ext-rng'), "valueInteger": 3}])},
     'extension control: same value, a valid extension')

def cat(**k):
    return dict({"coding": [{"system": "urn:probe:p", "code": "c"}]}, **k)


def sub(rid, status, prof, kind, extra=None):
    o = {"resourceType": "Observation", "id": rid, "meta": {"profile": [U(prof)]}, "status": status, "code": {"text": "x"}}
    if kind == 'cat':
        o["category"] = [cat()]
    else:
        o["component"] = [{"code": cat()}]
    return o


for prof, k, what in (('obs-s13a', 'cat', '13a: the slice declares both profiles'),
                      ('obs-s13b', 'comp', '13b/13c: a child of the slice member declares both profiles'),
                      ('obs-s10', 'cat', '10: the sliced element declares both profiles, the slice none')):
    n = prof.split('-')[-1]
    for a, b, i in (('amended', 'final', 1), ('final', 'amended', 2)):
        kind = 'cat' if k == 'cat' else 'comp'
        c = sub('c1', b, prof, kind)
        top = sub('top', a, prof, kind)
        top.update({"contained": [c], "hasMember": [{"reference": "#c1"}]})
        top.pop('resourceType'), top.pop('id')
        inst('%sc%d' % (n, i), prof, dict(top, id='%sc%d' % (n, i)), '%s: top %s, contained %s' % (what, a, b))
        # the same two resources as the entries of a Bundle
        e1, e2 = sub('e1', a, prof, kind), sub('e2', b, prof, kind)
        inst('%se%d' % (n, i), prof, {"resourceType": "Bundle", "id": '%se%d' % (n, i), "type": "collection", "entry": [
            {"fullUrl": "urn:uuid:00000000-0000-0000-0000-00000000000%d" % (i * 2), "resource": e1},
            {"fullUrl": "urn:uuid:00000000-0000-0000-0000-00000000000%d" % (i * 2 + 1), "resource": e2}]},
             '%s: Bundle entries, first %s, second %s' % (what, a, b))


# Ronda 2 de la revisión, 17a: dos perfiles de Annotation que no se pueden usar (no existen) en Observation.note y una nota sin
# `text`. Sin capas usables sigue el contenido base del tipo: el mínimo de `text`, y un TYPE_PROFILE_NOT_FOUND por perfil.
obs('obs-un17', 'Observation.note', 'Annotation', ['missing-a', 'missing-b'])
inst('un17', 'obs-un17', {"note": [{"time": "2020-01-01T00:00:00Z"}]},
     'round 2 (17a): two unavailable Annotation profiles on note, and a note with no text: the base type continues')

# Ronda 2, Owner del slicing más hondo (slicing.go ~202, ~221 y walkChildren): alternativas de Coding o de CodeableConcept
# que dependen de %resource.status, en un contenido «final» bajo un recurso «amended». Tres formas (Codex, round2_test.go):
# dpin = component:c.code.coding declara las alternativas de Coding en línea; dppr = component:c.code declara un perfil de
# CodeableConcept cuyo `coding` declara las alternativas de Coding; dpwk = Observation.code declara dos perfiles de
# CodeableConcept con un slice `coding:c` obligatorio y un patrón de código distinto en cada uno.
CDS = [{"code": "Coding", "profile": [U('dp-cd-final'), U('dp-cd-amended')]}]
sd('dp-cd-final', 'Coding', [el('Coding', constraint=[cons('dp-f', "%resource.status = 'final'", 'resource must be final (dp-cd-final)')])])
sd('dp-cd-amended', 'Coding', [
    el('Coding', constraint=[cons('dp-a', "%resource.status = 'amended'", 'resource must be amended (dp-cd-amended)')]),
    el('Coding.display', min=1)])
sd('dp-wrap', 'CodeableConcept', [el('CodeableConcept.coding', type=CDS)])
CSLC = {"discriminator": [{"type": "pattern", "path": "code"}], "rules": "open"}
PATD = {"coding": [{"system": "urn:review", "code": "c"}]}
sd('obs-dpin', 'Observation', [
    el('Observation.component', slicing=CSLC),
    el('Observation.component', id='Observation.component:c', sliceName='c'),
    el('Observation.component.code', id='Observation.component:c.code', patternCodeableConcept=PATD),
    el('Observation.component.code.coding', id='Observation.component:c.code.coding', type=CDS)])
sd('obs-dppr', 'Observation', [
    el('Observation.component', slicing=CSLC),
    el('Observation.component', id='Observation.component:c', sliceName='c'),
    el('Observation.component.code', id='Observation.component:c.code', type=[{"code": "CodeableConcept", "profile": [U('dp-wrap')]}],
       patternCodeableConcept=PATD)])
for st, code in (('final', 'c'), ('amended', 'other')):
    sd('dp-cc-' + st, 'CodeableConcept', [
        el('CodeableConcept', constraint=[cons('dp-cc-' + st[0], "%resource.status = '" + st + "'", 'resource must be ' + st + ' (dp-cc-' + st + ')')]),
        el('CodeableConcept.coding', slicing={"discriminator": [{"type": "pattern", "path": "code"}], "rules": "open"}),
        el('CodeableConcept.coding', id='CodeableConcept.coding:c', sliceName='c', min=1),
        el('CodeableConcept.coding.code', id='CodeableConcept.coding:c.code', patternCode=code)])
obs('obs-dpwk', 'Observation.code', 'CodeableConcept', ['dp-cc-final', 'dp-cc-amended'])
for n, k in (('dpin', 'comp'), ('dppr', 'comp'), ('dpwk', 'code')):
    c = {"resourceType": "Observation", "id": "c1", "meta": {"profile": [U('obs-' + n)]}, "status": "final", "code": {"text": "x"}}
    if k == 'comp':
        c["component"] = [{"code": PATD}]
    else:
        c["code"] = PATD
    inst(n + 'c1', None, {"status": "amended", "contained": [c], "hasMember": [{"reference": "#c1"}]},
         'round 2: ' + n + ': outer Observation amended (no profile), contained Observation final with the profile')

# Ronda 3 (mutación 15b, `definedIn` que mira solo la primera capa): dos perfiles de CodeableConcept que fallan una invariante de
# la raíz (así se caminan las dos capas), compartidos por `component.code` y por el slice `component:s.code`; solo el segundo asigna un
# perfil de Coding que exige `display`. El patrón es por coding: HL7 no evalúa un discriminador por `text`. El slicing no debe sumar un SLICING_CARDINALITY_MIN a ese CARDINALITY_MIN.
sd('d3-cod-b', 'Coding', [el('Coding.display', min=1)])
sd('d3-cc-a', 'CodeableConcept', [el('CodeableConcept', constraint=[cons('d3-fail', 'false', 'fail')])])
sd('d3-cc-b', 'CodeableConcept', [
    el('CodeableConcept', constraint=[cons('d3-fail', 'false', 'fail')]),
    el('CodeableConcept.coding', type=[{"code": "Coding", "profile": [U('d3-cod-b')]}])])
CCD = [{"code": "CodeableConcept", "profile": [U('d3-cc-a'), U('d3-cc-b')]}]
sd('obs-d3', 'Observation', [
    el('Observation.component', slicing={"discriminator": [{"type": "pattern", "path": "code"}], "rules": "open"}),
    el('Observation.component.code', type=CCD),
    el('Observation.component', id='Observation.component:s', sliceName='s'),
    el('Observation.component.code', id='Observation.component:s.code', type=CCD, patternCodeableConcept={"coding": [{"system": "urn:x", "code": "c"}]})])
inst('d3', 'obs-d3', {"component": [{"code": {"coding": [{"system": "urn:x", "code": "c"}]}}]},
     'round 3 (15b): two shared CodeableConcept layers, only the 2nd assigns a Coding profile that needs display; the slice adds nothing')

def write(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w') as f:
        json.dump(obj, f, indent=1)
        f.write('\n')


def main():
    for d in ['inst'] + ['ig-' + v for v in VERS]:
        shutil.rmtree(os.path.join(HERE, d), ignore_errors=True)
    for v in VERS:
        shutil.copytree(os.path.join(BASE, 'ig-' + v), os.path.join(HERE, 'ig-' + v))
        for f, b in PROFILES.items():
            write(os.path.join(HERE, 'ig-' + v, f), b(v))
    desc = []
    for n, (prof, extra, what) in INST.items():
        if extra.get("resourceType") == "Bundle":
            o = extra
        else:
            o = {"resourceType": "Observation", "id": n}
            if prof:
                o["meta"] = {"profile": [U(prof)]}
            o.update({"status": "final", "code": {"text": "x"}, "subject": {"display": "x"}})
            o.update(extra)
        write(os.path.join(HERE, 'inst', n + '.json'), o)
        desc.append((n, what))
    write(os.path.join(HERE, 'descriptions.json'), desc)
    print(len(PROFILES), 'profiles', len(INST), 'instances')


main()
