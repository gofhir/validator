#!/usr/bin/env python3
"""Matriz de erratas entre versiones (plan 2026-11-27-errata-cross-version.md, §5, «Matriz de versiones»).

  versión validada {4.0.1, 4.3.0, 5.0.0} × fhirVersion del perfil {4.0.1, 4.3.0, 5.0.0, sin declarar}
  × errata {ref-1, bdl-8, tim-9, vs-1, csd-0 (por expresión), tipo de Resource.id}
  × constraint {heredada, copiada al diferencial con el texto publicado de la versión del perfil}
  × instancia {la errata aplica, no aplica} × {perfil explícito, meta.profile}.

El texto de cada constraint (y el tipo de Patient.id) se lee de los packages oficiales fijados en PACKAGES, nunca de
gofhir. Un perfil sin fhirVersion toma, en su copia, el texto de la versión validada. Los valores esperados NO salen de
aquí: salen de HL7 validator_cli 7.0.4 (run_hl7.sh, gen_expected.py).

Escribe, bajo <out>:
  ig/<validada>/<perfil>.json   los perfiles (los de «none» copiados cambian con la versión validada)
  inst/<errata>-<aplica|no>.json            instancias sin meta.profile (perfil explícito)
  inst/meta-<errata>-<forma>-<pv>-<aplica|no>.json   instancias con meta.profile
  cases.json                    la lista de casos (nombre, errata, forma, pv, perfil, instancias)
Uso: python3 -I gen.py <out>
"""
import json
import os
import sys

OUT = sys.argv[1]
CACHE = os.path.expanduser('~/.fhir/packages')
PACKAGES = {'4.0.1': 'hl7.fhir.r4.core#4.0.1', '4.3.0': 'hl7.fhir.r4b.core#4.3.0', '5.0.0': 'hl7.fhir.r5.core#5.0.0'}
VERSIONS = ('4.0.1', '4.3.0', '5.0.0')
PVS = ('4.0.1', '4.3.0', '5.0.0', 'none')
FORMS = ('inh', 'copy')
ERRATA = ('ref1', 'bdl8', 'tim9', 'vs1', 'csd0', 'id')
BASE = 'http://example.org/ecv/'
FHIR = 'http://hl7.org/fhir/StructureDefinition/'
TAG = {'4.0.1': 'v401', '4.3.0': 'v430', '5.0.0': 'v500', 'none': 'none'}
DIV = {'status': 'generated', 'div': '<div xmlns="http://www.w3.org/1999/xhtml">x</div>'}


def official(version, sd):
    with open(os.path.join(CACHE, PACKAGES[version], 'package', 'StructureDefinition-%s.json' % sd)) as f:
        return json.load(f)['snapshot']['element']


def constraint(version, sd, element, key=None, contains=None):
    """La constraint de la definición oficial de version: por clave, o la que contiene un texto (csd-0 es cnl-0 en R5)."""
    for e in official(version, sd):
        if e['id'] != element:
            continue
        for c in e.get('constraint', []):
            if (key and c['key'] == key) or (contains and contains in c['expression']):
                return {k: c[k] for k in ('key', 'severity', 'human', 'expression')}
    raise SystemExit('%s %s %s %s no está en %s' % (sd, element, key or contains, '', version))


def id_type(version):
    for e in official(version, 'Patient'):
        if e['id'] == 'Patient.id':
            return e['type']
    raise SystemExit('Patient.id')


# errata -> (recurso del perfil, tipo del perfil de apoyo o None)
def carrier(version, err, form):
    """Los elementos del diferencial que copia la forma «copy», con el texto oficial de version."""
    if err == 'ref1':
        return {'Reference': [constraint(version, 'Reference', 'Reference', key='ref-1')]}
    if err == 'bdl8':
        return {'Bundle.entry': [constraint(version, 'Bundle', 'Bundle.entry', key='bdl-8')]}
    if err == 'tim9':
        return {'Timing.repeat': [constraint(version, 'Timing', 'Timing.repeat', key='tim-9')]}
    if err == 'vs1':
        return {'Observation.effective[x]': [constraint(version, 'vitalsigns', 'Observation.effective[x]', key='vs-1')]}
    if err == 'csd0':
        return {'CodeSystem': [constraint(version, 'CodeSystem', 'CodeSystem', contains='name.matches(')]}
    raise SystemExit(err)


def sd(url_tail, name, pv, kind, typ, base, diff, version):
    o = {'resourceType': 'StructureDefinition', 'id': url_tail, 'url': BASE + url_tail, 'name': name, 'status': 'active'}
    if pv != 'none':
        o['fhirVersion'] = pv
    o.update({'kind': kind, 'abstract': False, 'type': typ, 'baseDefinition': base, 'derivation': 'constraint',
              'differential': {'element': diff}})
    return o


def build(version, err, form, pv):
    """Los perfiles (url_tail -> StructureDefinition) de una celda y la URL del que se valida. El texto copiado es el
    de la versión del perfil; sin versión, el de la validada."""
    text = version if pv == 'none' else pv
    cp = carrier(text, err, form) if (form == 'copy' and err != 'id') else {}
    tag = '%s-%s-%s' % (err, form, TAG[pv])
    nm = 'Ecv' + err.capitalize() + form.capitalize() + TAG[pv].capitalize()
    out = {}

    def own(path, extra=None):
        d = {'id': path, 'path': path}
        if path in cp:
            d['constraint'] = cp[path]
        d.update(extra or {})
        return d
    if err == 'ref1':
        out[tag + '-ref'] = sd(tag + '-ref', nm + 'Ref', pv, 'complex-type', 'Reference', FHIR + 'Reference',
                               [own('Reference', {'short': 'x'})], version)
        out[tag] = sd(tag, nm, pv, 'resource', 'Patient', FHIR + 'Patient',
                      [{'id': 'Patient', 'path': 'Patient'},
                       {'id': 'Patient.generalPractitioner', 'path': 'Patient.generalPractitioner',
                        'type': [{'code': 'Reference', 'profile': [BASE + tag + '-ref'],
                                  'targetProfile': [FHIR + 'Organization', FHIR + 'Practitioner']}]}], version)
    elif err == 'bdl8':
        out[tag] = sd(tag, nm, pv, 'resource', 'Bundle', FHIR + 'Bundle',
                      [{'id': 'Bundle', 'path': 'Bundle'}, own('Bundle.entry')], version)
    elif err == 'tim9':
        out[tag + '-tp'] = sd(tag + '-tp', nm + 'Tp', pv, 'complex-type', 'Timing', FHIR + 'Timing',
                              [{'id': 'Timing', 'path': 'Timing', 'short': 'x'}, own('Timing.repeat')], version)
        out[tag] = sd(tag, nm, pv, 'resource', 'Observation', FHIR + 'Observation',
                      [{'id': 'Observation', 'path': 'Observation'},
                       {'id': 'Observation.effective[x]', 'path': 'Observation.effective[x]',
                        'type': [{'code': 'Timing', 'profile': [BASE + tag + '-tp']}]}], version)
    elif err == 'vs1':
        out[tag] = sd(tag, nm, pv, 'resource', 'Observation', FHIR + 'vitalsigns',
                      [{'id': 'Observation', 'path': 'Observation'}, own('Observation.effective[x]')], version)
    elif err == 'csd0':
        out[tag] = sd(tag, nm, pv, 'resource', 'CodeSystem', FHIR + 'CodeSystem', [own('CodeSystem')], version)
    elif err == 'id':
        # inh: un diferencial que solo cambia min hereda el tipo (C1). copy: redeclara el tipo como lo publica la
        # versión del perfil (B1).
        el = {'id': 'Patient.id', 'path': 'Patient.id', 'min': 1}
        if form == 'copy':
            el = {'id': 'Patient.id', 'path': 'Patient.id', 'type': id_type(text)}
        out[tag] = sd(tag, nm, pv, 'resource', 'Patient', FHIR + 'Patient', [{'id': 'Patient', 'path': 'Patient'}, el], version)
    return out, BASE + tag


def instance(err, applies):
    n = 'a' if applies else 'n'
    if err == 'ref1':
        # R4 ref-1 falla en una referencia sin reference (solo display); HL7 la corrige con la de R5 (errata.go)
        gp = [{'display': 'x'}] if applies else [{'reference': 'Practitioner/1'}]
        return {'resourceType': 'Patient', 'id': 'p' + n, 'text': DIV, 'generalPractitioner': gp}
    if err == 'bdl8':
        e = {'resource': {'resourceType': 'Patient', 'id': 'p1', 'text': DIV}, 'request': {'method': 'POST', 'url': 'Patient'}}
        if not applies:
            e['fullUrl'] = 'urn:uuid:12345678-1234-4234-8234-123456789012'
        return {'resourceType': 'Bundle', 'id': 'b' + n, 'type': 'transaction', 'entry': [e]}
    if err == 'tim9':
        when = ['MORN', 'EVE'] if applies else ['MORN']
        return {'resourceType': 'Observation', 'id': 'o' + n, 'text': DIV, 'status': 'final', 'code': {'text': 'x'},
                'effectiveTiming': {'repeat': {'offset': 1, 'when': when}}}
    if err == 'vs1':
        o = {'resourceType': 'Observation', 'id': 'v' + n, 'text': DIV, 'status': 'final',
             'category': [{'coding': [{'system': 'http://terminology.hl7.org/CodeSystem/observation-category', 'code': 'vital-signs'}]}],
             'code': {'coding': [{'system': 'http://loinc.org', 'code': '8867-4', 'display': 'Heart rate'}]},
             'subject': {'reference': 'Patient/1'},
             'valueQuantity': {'value': 60, 'unit': '/min', 'system': 'http://unitsofmeasure.org', 'code': '/min'}}
        # vs-1 publicada («($this as dateTime)...») falla en un Period; corregida solo se aplica a un dateTime
        if applies:
            o['effectivePeriod'] = {'start': '2020-01-01', 'end': '2020-01-02'}
        else:
            o['effectiveDateTime'] = '2020-01-01'
        return o
    if err == 'csd0':
        o = {'resourceType': 'CodeSystem', 'id': 'c' + n, 'text': DIV, 'status': 'active', 'content': 'not-present'}
        if not applies:
            o['name'] = 'Good'
        return o
    if err == 'id':
        # R4 tipa Resource.id como string (errata.go typeErrata); R4B y R5 como id: un id con '_' no es un id
        return {'resourceType': 'Patient', 'id': 'a_b' if applies else 'a-b', 'text': DIV}
    raise SystemExit(err)


def write(path, obj):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, 'w') as f:
        json.dump(obj, f, indent=1, sort_keys=True)
        f.write('\n')


def main():
    cases = []
    for err in ERRATA:
        for app in (True, False):
            write(os.path.join(OUT, 'inst', '%s-%s.json' % (err, 'aplica' if app else 'no')), instance(err, app))
    for version in VERSIONS:
        for err in ERRATA:
            for form in FORMS:
                for pv in PVS:
                    sds, url = build(version, err, form, pv)
                    for tail, o in sds.items():
                        write(os.path.join(OUT, 'ig', version, tail + '.json'), o)
                    if version == VERSIONS[0]:
                        cases.append({'errata': err, 'form': form, 'pv': pv, 'profile': url,
                                      'name': '%s-%s-%s' % (err, form, TAG[pv])})
    for c in cases:
        for app in (True, False):
            o = instance(c['errata'], app)
            o['meta'] = {'profile': [c['profile']]}
            write(os.path.join(OUT, 'inst', 'meta-%s-%s.json' % (c['name'], 'aplica' if app else 'no')), o)
    write(os.path.join(OUT, 'cases.json'), cases)
    print(len(cases), 'celdas;', len(cases) * 4, 'instancias por versión (explícito y meta, aplica y no)', file=sys.stderr)


main()
