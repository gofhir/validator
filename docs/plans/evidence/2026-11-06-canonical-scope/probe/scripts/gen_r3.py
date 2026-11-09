import json, os, sys
# Uso: gen_r3.py <r3> : perfiles e instancias de r3 (round 2 de la revision). r3a: un discriminador que
# termina en entry.resolve() contra bundle-p, que parte Bundle.signature por onBehalfOf.resolve() contra
# org-t (name obligatorio). r3b: un Bundle b2 suelto con signature.who absoluta.
D = sys.argv[1]
U = 'http://x/'
def sd(id, typ, elems):
  return {"resourceType": "StructureDefinition", "id": id, "url": U + id, "name": id.replace('-', '').capitalize(), "status": "active",
    "fhirVersion": "4.0.1", "kind": "resource", "abstract": False, "type": typ,
    "baseDefinition": "http://hl7.org/fhir/StructureDefinition/" + typ, "derivation": "constraint", "differential": {"element": elems}}
sds = [
 sd('comp-chain3', 'Composition', [
  {"id": "Composition.section", "path": "Composition.section", "slicing": {"discriminator": [{"type": "profile", "path": "entry.resolve()"}], "rules": "open"}},
  {"id": "Composition.section:B", "path": "Composition.section", "sliceName": "B", "min": 1, "max": "*"},
  {"id": "Composition.section:B.entry", "path": "Composition.section.entry", "type": [{"code": "Reference", "targetProfile": [U + "bundle-p"]}]}]),
 sd('bundle-p', 'Bundle', [
  {"id": "Bundle.signature", "path": "Bundle.signature", "slicing": {"discriminator": [{"type": "profile", "path": "onBehalfOf.resolve()"}], "rules": "open"}},
  {"id": "Bundle.signature:T", "path": "Bundle.signature", "sliceName": "T", "min": 1, "max": "1"},
  {"id": "Bundle.signature:T.onBehalfOf", "path": "Bundle.signature.onBehalfOf", "type": [{"code": "Reference", "targetProfile": [U + "org-t"]}]}]),
 sd('org-t', 'Organization', [{"id": "Organization.name", "path": "Organization.name", "min": 1}]),
 sd('bun-tpw', 'Bundle', [{"id": "Bundle.signature.who", "path": "Bundle.signature.who", "type": [{"code": "Reference", "targetProfile": [U + "pat-inner"]}]}]),
 sd('pat-inner', 'Patient', [{"id": "Patient", "path": "Patient", "constraint": [{"key": "inner-1", "severity": "error", "human": "inner patient", "expression": "name.family = 'inner'", "source": U + "pat-inner"}]}]),
]
SIG = {"type": [{"system": "urn:iso-astm:E1762-95:2013", "code": "1.2.840.10065.1.12.1.1"}], "when": "2026-01-01T00:00:00Z"}
def r3a(org):
  nested = {"resourceType": "Bundle", "id": "nested", "type": "collection",
    "signature": dict(SIG, who={"display": "x"}, onBehalfOf={"reference": "http://x/Organization/target"}),
    "entry": [{"fullUrl": "http://x/Organization/target", "resource": org}]}
  comp = {"resourceType": "Composition", "id": "c", "meta": {"profile": [U + "comp-chain3"]}, "status": "final", "type": {"text": "x"},
    "date": "2026-01-01", "author": [{"display": "x"}], "title": "x", "section": [{"entry": [{"reference": "http://x/Bundle/nested"}]}]}
  return {"resourceType": "Bundle", "type": "collection", "entry": [
    {"fullUrl": "http://x/Composition/c", "resource": comp}, {"fullUrl": "http://x/Bundle/nested", "resource": nested}]}
# b2 solo: su Patient inner es el destino de signature.who aunque quien lo contenga tenga otro Patient p
r3b = {"resourceType": "Bundle", "id": "r3b", "meta": {"profile": [U + "bun-tpw"]}, "type": "collection",
  "signature": dict(SIG, who={"reference": "http://ex.org/fhir/Patient/p"}, data="AA=="),
  "entry": [{"fullUrl": "http://ex.org/fhir/Patient/p", "resource": {"resourceType": "Patient", "id": "p", "name": [{"family": "inner"}]}}]}
insts = [('r3a1', r3a({"resourceType": "Organization", "id": "target", "name": "target"})),
         ('r3a2', r3a({"resourceType": "Organization", "id": "target"})), ('r3b', r3b)]
os.makedirs(D + '/ig', exist_ok=True); os.makedirs(D + '/inst', exist_ok=True)
for s in sds: json.dump(s, open(f"{D}/ig/StructureDefinition-{s['id']}.json", 'w'))
for n, r in insts:
  r['id'] = n; json.dump(r, open(f"{D}/inst/{n}.json", 'w'), indent=1)
