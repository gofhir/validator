"""Generador de las sondas por llamador de la pieza 2 (capas por valor de `type.profile`), 14-11-2026.
Uso: python3 -I gen.py. Escribe ig-<version>/ (los perfiles de ../probe/ig-<version> más los nuevos) e inst/ (solo las
instancias nuevas). run.sh <version> corre HL7 validator_cli 7.0.4 (-tx n/a) sobre inst/ y deja hl7-<version>-7.0.4.json.
Cada sonda pone en el segundo perfil una regla de la fase que el llamador (a) del plan corre, de modo que solo una
corrida que baje por la segunda capa la informa."""
import json, os, shutil

HERE = os.path.dirname(os.path.abspath(__file__))
BASE = os.path.join(HERE, '..', 'probe')
VERS = ('4.0.1', '4.3.0', '5.0.0')
NS = 'http://example.org/fhir/StructureDefinition/'
HL7 = 'http://hl7.org/fhir/StructureDefinition/'
UCUM = 'http://unitsofmeasure.org'
DT = {'Quantity', 'Identifier', 'CodeableConcept', 'Reference', 'Range'}
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


def ext_slice(slice_name, ext_id, mn=0):
    return [el('Quantity.extension', slicing={"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}),
            el('Quantity.extension', id='Quantity.extension:' + slice_name, sliceName=slice_name, min=mn, max='1',
               type=[{"code": "Extension", "profile": [U(ext_id)]}])]


# ---- profiles -----------------------------------------------------------------------------
sd('ext-int', 'Extension', [el('Extension.url', fixedUri=U('ext-int')), el('Extension.value[x]', type=[{"code": "integer"}])],
   ctx=[{"type": "element", "expression": "Quantity"}])
sd('ext-obsonly', 'Extension', [el('Extension.url', fixedUri=U('ext-obsonly')), el('Extension.value[x]', type=[{"code": "string"}])],
   ctx=[{"type": "element", "expression": "Observation.component"}])
VS = "http://example.org/fhir/ValueSet/vs-gt"
sd('qbind', 'Quantity', [el('Quantity.comparator', binding={"strength": "required", "valueSet": VS})])
sd('qext', 'Quantity', ext_slice('xi', 'ext-int', 1))
sd('qctx', 'Quantity', ext_slice('xo', 'ext-obsonly'))
sd('cc-a', 'CodeableConcept', [el('CodeableConcept.text', min=1)])
sd('cc-b', 'CodeableConcept', [
    el('CodeableConcept.coding', slicing={"discriminator": [{"type": "value", "path": "system"}], "rules": "open"}),
    el('CodeableConcept.coding', id='CodeableConcept.coding:x', sliceName='x', min=1),
    el('CodeableConcept.coding.system', id='CodeableConcept.coding:x.system', min=1, fixedUri='urn:probe:x')])
sd('ra', 'Range', [el('Range.high', min=1)])
sd('rb', 'Range', [el('Range.low', min=1)])
# D1: la capa se elige por el %resource del dueño
sd('qoa', 'Quantity', [el('Quantity', constraint=[cons('ro-a', "%resource.status = 'final'", 'resource must be final (qoa)')])])
AM = cons('ro-b', "%resource.status = 'amended'", 'resource must be amended (qob)')
sd('qob-card', 'Quantity', [el('Quantity', constraint=[AM]), el('Quantity.code', min=1)])
sd('qob-bind', 'Quantity', [el('Quantity', constraint=[AM]), el('Quantity.comparator', binding={"strength": "required", "valueSet": VS})])
sd('qob-ext', 'Quantity', [el('Quantity', constraint=[AM])] + ext_slice('xi', 'ext-int', 1))
# B2: un ciclo por referencia entre dos Organization, con capas en cada vuelta
sd('cc-oa', 'CodeableConcept', [el('CodeableConcept', constraint=[cons('ro-a', "%resource.status = 'final'", 'resource must be final (cc-oa)')])])
sd('cc-ob', 'CodeableConcept', [
    el('CodeableConcept', constraint=[cons('ro-b', "%resource.status = 'amended'", 'resource must be amended (cc-ob)')]),
    el('CodeableConcept.coding', slicing={"discriminator": [{"type": "value", "path": "system"}], "rules": "open"}),
    el('CodeableConcept.coding', id='CodeableConcept.coding:x', sliceName='x', min=1),
    el('CodeableConcept.coding.system', id='CodeableConcept.coding:x.system', min=1, fixedUri='urn:probe:x')])
sd('cyc-a', 'Identifier', [el('Identifier.assigner', type=[{"code": "Reference", "targetProfile": [U('org-cyc')]}])])
sd('cyc-b', 'Identifier', [el('Identifier.period', min=1)])
sd('org-cyc', 'Organization', [el('Organization.active', min=1), el('Organization.identifier', type=[{"code": "Identifier", "profile": [U('cyc-a'), U('cyc-b')]}])])

obs('obs-kbind', 'Observation.value[x]', 'Quantity', ['qa', 'qbind'])
obs('obs-kext', 'Observation.value[x]', 'Quantity', ['qa', 'qext'])
obs('obs-kctx', 'Observation.value[x]', 'Quantity', ['qa', 'qctx'])
obs('obs-kslice', 'Observation.code', 'CodeableConcept', ['cc-a', 'cc-b'])
obs('obs-kucum', 'Observation.value[x]', 'Range', ['ra', 'rb'])
obs('obs-ocard', 'Observation.value[x]', 'Quantity', ['qoa', 'qob-card'])
obs('obs-obind', 'Observation.value[x]', 'Quantity', ['qoa', 'qob-bind'])
obs('obs-oext', 'Observation.value[x]', 'Quantity', ['qoa', 'qob-ext'])
obs('obs-oslice', 'Observation.code', 'CodeableConcept', ['cc-oa', 'cc-ob'])
sd('obs-cyc', 'Observation', [el('Observation.identifier', type=[{"code": "Identifier", "profile": [U('cyc-a'), U('cyc-b')]}])])


# E3, regresión con un solo perfil: la salida de un perfil de tipo no cambia (sin NoMatch, sin identidad entre capas)
obs('obs-single1', 'Observation.value[x]', 'Quantity', ['qsev-a'])
obs('obs-single2', 'Observation.value[x]', 'Quantity', ['qa'])
obs('obs-single3', 'Observation.value[x]', 'Quantity', ['qsev-b'])


def vs(ver):
    return {"resourceType": "ValueSet", "id": "vs-gt", "url": VS, "name": "vs_gt", "status": "draft",
            "compose": {"include": [{"system": "http://hl7.org/fhir/quantity-comparator", "concept": [{"code": ">"}]}]}}


# ---- instances ----------------------------------------------------------------------------
Q = lambda **k: dict({"value": 5}, **k)
EXT_BAD = [{"url": U('ext-int'), "valueString": "oops"}]
EXT_OK = [{"url": U('ext-int'), "valueInteger": 3}]
inst('kbind', 'obs-kbind', {"valueQuantity": Q(comparator='<', system=UCUM, code='mg')},
     'binding walk: qa fails (no unit); only qbind (2nd) binds comparator to {>}')
inst('kbindok', 'obs-kbind', {"valueQuantity": Q(comparator='>', system=UCUM, code='mg')},
     'binding walk: comparator in the set, no unit: only qbind conforms')
inst('kext', 'obs-kext', {"valueQuantity": Q(system=UCUM, code='mg')},
     'extension: qa fails (no unit); only qext (2nd) requires the extension slice xi (profile ext-int), absent')
inst('kextok', 'obs-kext', {"valueQuantity": Q(system=UCUM, code='mg', extension=EXT_OK)},
     'extension: required extension present, no unit: only qext conforms')
inst('kctx', 'obs-kctx', {"valueQuantity": Q(system=UCUM, code='mg', extension=[{"url": U('ext-obsonly'), "valueString": "s"}])},
     'extension context: qa fails; only qctx (2nd) declares the extension whose context is Observation.component')
inst('kslice', 'obs-kslice', {"code": {"coding": [{"system": "urn:probe:other", "code": "c"}]}},
     'slicing tree: cc-a fails (no text); only cc-b (2nd) requires the slice x')
inst('kslicep', 'obs-kslice', {"code": {"coding": [{"system": "urn:probe:x", "code": "c"}]}},
     'slicing tree: no text, slice x present: only cc-b conforms')
inst('kucum', 'obs-kucum', {"valueRange": {"low": {"value": 1, "system": UCUM, "code": "foo bar"}}},
     'UCUM: ra fails (no high); an invalid UCUM code inside the datatype')
inst('kucumb', 'obs-kucum', {"valueRange": {"low": {"value": 1, "system": UCUM, "code": "mg"}}},
     'UCUM control: valid code, ra fails')


inst('single1', 'obs-single1', {"valueQuantity": Q(system=UCUM, code='mg')}, 'single profile qsev-a: dup-1 (error) and unit min fail')
inst('single2', 'obs-single2', {"valueQuantity": {"id": "q1"}}, 'single profile qa: ele-1 and unit min fail')
inst('single3', 'obs-single3', {"valueQuantity": Q(system=UCUM, code='mg')}, 'single profile qsev-b: dup-1 (warning) and unit min fail')


def pq(i, st, **q):
    return {"resourceType": "Observation", "id": i, "status": st, "code": {"text": "x"}, "valueQuantity": Q(**q)}


# D1: dueño. Observation con un contenido que referencia; cada uno con su estado.
def owner(prof, top, cont, **q):
    c = pq('c1', cont, **q)
    c["meta"] = {"profile": [U(prof)]}
    d = dict(pq('top', top, **q), contained=[c], hasMember=[{"reference": "#c1"}])
    return d


for k, prof, q, what in (('card', 'obs-ocard', dict(unit='mg'), 'cardinality'),
                         ('bind', 'obs-obind', dict(comparator='<', unit='mg', system=UCUM, code='mg'), 'binding walk'),
                         ('ext', 'obs-oext', dict(unit='mg', system=UCUM, code='mg'), 'extension')):
    inst('o%s1' % k, prof, dict(owner(prof, 'amended', 'final', **q), id='o%s1' % k),
         'owner %s: top amended (qob chosen alone fails its rule: neither conforms), contained final (qoa conforms alone)' % what)
    inst('o%s2' % k, prof, dict(owner(prof, 'final', 'amended', **q), id='o%s2' % k),
         'owner %s: top final (qoa conforms alone), contained amended (neither conforms)' % what)

for a, b, k in (('amended', 'final', 1), ('final', 'amended', 2)):
    code = {"coding": [{"system": "urn:probe:other", "code": "c"}]}
    c = {"resourceType": "Observation", "id": "c1", "meta": {"profile": [U('obs-oslice')]}, "status": b, "code": code}
    inst('oslice%d' % k, 'obs-oslice', {"status": a, "code": code, "contained": [c], "hasMember": [{"reference": "#c1"}]},
         'owner slicing: top %s, contained %s; cc-ob (amended) requires the slice x' % (a, b))

ORG = lambda i, ident, active=True: dict({"resourceType": "Organization", "id": i, "name": "N", "identifier": [ident]},
                                         **({"active": True} if active else {}))
inst('bcyc1', 'obs-cyc', {"contained": [ORG('o1', {"value": "y", "assigner": {"reference": "#o2"}}),
                                        ORG('o2', {"value": "z", "assigner": {"reference": "#o1"}})],
                          "identifier": [{"value": "x", "assigner": {"reference": "#o1"}}]},
     'cycle through assigner references between two Organizations, layers [cyc-a, cyc-b] at each turn; nothing breaks the cycle')
inst('bcyc2', 'obs-cyc', {"contained": [ORG('o1', {"value": "y", "assigner": {"reference": "#o2"}}),
                                        ORG('o2', {"value": "z", "period": {"start": "2020-01-01"}, "assigner": {"reference": "#o1"}})],
                          "identifier": [{"value": "x", "assigner": {"reference": "#o1"}}]},
     'same cycle, o2 meets cyc-b by period')
inst('bcyc3', 'obs-cyc', {"contained": [ORG('o1', {"value": "y", "assigner": {"reference": "#o2"}}, active=False),
                                        ORG('o2', {"value": "z", "assigner": {"reference": "#o1"}})],
                          "identifier": [{"value": "x", "assigner": {"reference": "#o1"}}]},
     'same cycle, o1 lacks active (org-cyc fails for it), so the top fails cyc-a through o1 and cyc-b by period')


def write(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w') as f:
        json.dump(obj, f, indent=1)
        f.write('\n')


def main():
    for sub in ['inst'] + ['ig-' + v for v in VERS]:
        shutil.rmtree(os.path.join(HERE, sub), ignore_errors=True)
    for v in VERS:
        shutil.copytree(os.path.join(BASE, 'ig-' + v), os.path.join(HERE, 'ig-' + v))
        for f, b in PROFILES.items():
            write(os.path.join(HERE, 'ig-' + v, f), b(v))
        write(os.path.join(HERE, 'ig-' + v, 'ValueSet-vs-gt.json'), vs(v))
    desc = []
    for n, (prof, extra, what) in INST.items():
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
