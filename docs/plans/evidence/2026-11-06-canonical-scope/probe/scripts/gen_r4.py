import copy, json, os, shutil, subprocess, sys, tempfile
# Uso: gen_r4.py <r4> : ronda 3 de la revision (consulta 1: el memo de Conforms y el Scope).
# bundle-p parte Bundle.signature, que no repite: HL7 no genera su snapshot a partir de un differential
# (r3 lo uso asi), de modo que aqui bundle-p trae su snapshot, copiado del Bundle de cada version, con la rebanada.
# Instancias: r4c = r3a1 (sin perfil exterior); r4a = r3a1 y r4b = r3a2, con el perfil exterior `outer`, que parte
# Bundle.entry por el perfil de `resource` (bundle-p): la entry de nested conforma con su Scope canonico, y la
# Composition (comp-chain3) pregunta por entry.resolve() contra bundle-p con el Scope de la cadena.
D = sys.argv[1]
U = 'http://x/'
CORE = {'4.0.1': 'hl7.fhir.r4.core#4.0.1', '4.3.0': 'hl7.fhir.r4b.core#4.3.0', '5.0.0': 'hl7.fhir.r5.core#5.0.0'}
def sd(id, typ, elems, ver):
  return {"resourceType": "StructureDefinition", "id": id, "url": U + id, "name": id.replace('-', '').capitalize(), "status": "active",
    "fhirVersion": ver, "kind": "resource", "abstract": False, "type": typ,
    "baseDefinition": "http://hl7.org/fhir/StructureDefinition/" + typ, "derivation": "constraint", "differential": {"element": elems}}
def bundle_p(ver):
  core = json.load(open(os.path.expanduser(f"~/.fhir/packages/{CORE[ver]}/package/StructureDefinition-Bundle.json")))
  els = copy.deepcopy(core['snapshot']['element'])
  out = []
  for e in els:
    out.append(e)
    if e['path'] == 'Bundle.signature':
      e['slicing'] = {"discriminator": [{"type": "profile", "path": "onBehalfOf.resolve()"}], "rules": "open"}
  # la rebanada T: copia de Bundle.signature y de sus descendientes, tras ellos
  idx = [i for i, e in enumerate(out) if e['path'] == 'Bundle.signature' or e['path'].startswith('Bundle.signature.')]
  tail = []
  for i in idx:
    c = copy.deepcopy(out[i]); c.pop('slicing', None)
    c['id'] = c['id'].replace('Bundle.signature', 'Bundle.signature:T', 1)
    if c['path'] == 'Bundle.signature':
      c['sliceName'] = 'T'; c['min'] = 1; c['max'] = '1'
    if c['path'] == 'Bundle.signature.onBehalfOf':
      c['type'] = [{"code": "Reference", "targetProfile": [U + "org-t"]}]
    tail.append(c)
  last = idx[-1]
  out[last + 1:last + 1] = tail
  s = sd('bundle-p', 'Bundle', [], ver)
  s['differential'] = {"element": [
    {"id": "Bundle.signature", "path": "Bundle.signature", "slicing": {"discriminator": [{"type": "profile", "path": "onBehalfOf.resolve()"}], "rules": "open"}},
    {"id": "Bundle.signature:T", "path": "Bundle.signature", "sliceName": "T", "min": 1, "max": "1"},
    {"id": "Bundle.signature:T.onBehalfOf", "path": "Bundle.signature.onBehalfOf", "type": [{"code": "Reference", "targetProfile": [U + "org-t"]}]}]}
  s['snapshot'] = {"element": out}
  return s
def sds(ver):
  return [
   sd('comp-chain3', 'Composition', [
    {"id": "Composition.section", "path": "Composition.section", "slicing": {"discriminator": [{"type": "profile", "path": "entry.resolve()"}], "rules": "open"}},
    {"id": "Composition.section:B", "path": "Composition.section", "sliceName": "B", "min": 1, "max": "*"},
    {"id": "Composition.section:B.entry", "path": "Composition.section.entry", "type": [{"code": "Reference", "targetProfile": [U + "bundle-p"]}]}], ver),
   bundle_p(ver),
   sd('org-t', 'Organization', [{"id": "Organization.name", "path": "Organization.name", "min": 1}], ver),
   sd('outer', 'Bundle', [
    {"id": "Bundle.entry", "path": "Bundle.entry", "slicing": {"discriminator": [{"type": "profile", "path": "resource"}], "rules": "open"}},
    {"id": "Bundle.entry:B", "path": "Bundle.entry", "sliceName": "B", "min": 1, "max": "*"},
    {"id": "Bundle.entry:B.resource", "path": "Bundle.entry.resource", "type": [{"code": "Resource", "profile": [U + "bundle-p"]}]}], ver)]
SIG = {"type": [{"system": "urn:iso-astm:E1762-95:2013", "code": "1.2.840.10065.1.12.1.1"}], "when": "2026-01-01T00:00:00Z"}
def inst(id, org, outer):
  nested = {"resourceType": "Bundle", "id": "nested", "type": "collection",
    "signature": dict(SIG, who={"display": "x"}, onBehalfOf={"reference": U + "Organization/target"}),
    "entry": [{"fullUrl": U + "Organization/target", "resource": org}]}
  comp = {"resourceType": "Composition", "id": "c", "meta": {"profile": [U + "comp-chain3"]}, "status": "final", "type": {"text": "x"},
    "date": "2026-01-01", "author": [{"display": "x"}], "title": "x", "section": [{"entry": [{"reference": U + "Bundle/nested"}]}]}
  r = {"resourceType": "Bundle", "id": id, "type": "collection", "entry": [
    {"fullUrl": U + "Composition/c", "resource": comp}, {"fullUrl": U + "Bundle/nested", "resource": nested}]}
  if outer: r["meta"] = {"profile": [U + "outer"]}
  return r
named = {"resourceType": "Organization", "id": "target", "name": "target"}
os.makedirs(D + '/inst', exist_ok=True)
for n, org, outer in (('r4a', named, True), ('r4b', {"resourceType": "Organization", "id": "target"}, True), ('r4c', named, False)):
  json.dump(inst(n, org, outer), open(f"{D}/inst/{n}.json", 'w'), indent=1)
for ver in CORE:
  O = D if ver == '4.0.1' else f"{D}/v{ver}"
  os.makedirs(O + '/ig', exist_ok=True)
  for s in sds(ver): json.dump(s, open(f"{O}/ig/StructureDefinition-{s['id']}.json", 'w'))
  t = tempfile.mkdtemp(); os.mkdir(t + '/package')
  for f in os.listdir(O + '/ig'): shutil.copy(f"{O}/ig/{f}", t + '/package/')
  pj = json.dumps({"name": "probe.scope", "version": "1.0.0", "fhirVersions": [ver]})
  open(t + '/package/package.json', 'w').write(pj)
  if ver != '4.0.1': open(O + '/package.json', 'w').write(pj)
  subprocess.check_call(['tar', 'czf', f"{O}/probe-r4.tgz", '-C', t, 'package']); shutil.rmtree(t)
