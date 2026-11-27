#!/usr/bin/env python3
"""Genera pkg/validator/testdata/errata_cross_version/expected.json desde las salidas de HL7 validator_cli 7.0.4
(-tx n/a -no-http-access) que deja run_hl7.sh en hl7/: la matriz de gen.py (meta.profile y perfil explícito) y la sonda
R3-P2 (x), en R4, R4B y R5. Los valores esperados salen solo de HL7, nunca de gofhir.

Nombres: meta/<celda>/<aplica|no>, explicit/<celda>/<aplica|no> y x/disp, con <celda> = <errata>-<forma>-<pv>.
Una celda que HL7 no puede cargar (su snapshot, o el de un perfil de apoyo, no se genera: snapshot-exclusions.tsv, o la
validación dice «has no snapshot») no tiene valor esperado: va en «excluded» con la salida de HL7 y el test no la compara.

Divergencias declaradas: DIVERGENCES (vs-1 en R5, ver abajo). Todo informe de HL7 se clasifica o se ignora por su ID de
mensaje, y uno que no se conoce detiene el script (sin clasificar), para que alguien lo mire.
Uso: python3 -I gen_expected.py <expected.json>"""
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
VERSIONS = ('4.0.1', '4.3.0', '5.0.0')

# Avisos que no salen de ninguna StructureDefinition: las buenas prácticas de Observation que InstanceValidator escribe a
# mano (por la regla 5, nada hardcodeado sin ADR, gofhir no los da) y la terminología que -tx n/a no resuelve.
IGNORED = {'All_observations_should_have_a_subject', 'All_observations_should_have_a_performer',
           'All_observations_should_have_an_effectiveDateTime_or_an_effectivePeriod',
           'Error_validating_code_running_without_terminology_services'}
NO_SNAPSHOT = 'Validation_VAL_Profile_NoSnapshot'
# Divergencia deliberada (versión, constraint) -> motivo: HL7 7.0.4 informa vs-1 en Observation.effectivePeriod con R5, y
# gofhir no, porque corrige vs-1 en R4, R4B y R5 con su corrección ya publicada (PR #142, docs/content/docs/error-reference/
# constraints.md, fila vs-1; fuente: US Core 9.0.0 us-core-vital-signs, que republica la constraint corregida, source
# vitalsigns, como AU Core 2.0.0 y mCODE 4.0.0). El valor esperado de esa celda es el de gofhir: sin el informe.
DIVERGENCES = {('5.0.0', 'vs-1'): 'gofhir corrects vs-1 in R5 (US Core 9.0.0 republishes it corrected); HL7 7.0.4 does not'}
ID_IDS = {'Resource_RES_ID_Malformed_Chars': 'id-malformed', 'Resource_RES_ID_Malformed_Length': 'id-malformed'}


def msgid(i):
    return next((x.get('valueCode') for x in i.get('extension', []) if 'message-id' in x.get('url', '')), '')


def classify(i, where):
    sev, t, mid = i['severity'], i['details']['text'], msgid(i)
    loc = (i.get('expression') or [''])[0]
    loc = re.sub(r'/\*[^*]*\*/', '', loc)
    loc = re.sub(r'\.(\w+)\.ofType\((\w+)\)', lambda m: '.' + m.group(1) + m.group(2)[0].upper() + m.group(2)[1:], loc)
    if sev == 'information' or mid in IGNORED or t.startswith('Unable to validate code without using server'):
        return None
    sev = 'error' if sev == 'fatal' else sev
    m = re.match(r"Constraint failed: (\S+?): '(.*?)'(?: \((?:validating against|defined in)|$)", t)
    if i['code'] == 'invariant' and m:
        src = re.search(r'\(defined in (\S+?)\)?(?: |$)', t)
        return ('constraint', sev, loc, m.group(1), m.group(2), src.group(1) if src else '')
    if mid in ID_IDS:
        return (ID_IDS[mid], sev, loc, '', '', '')
    raise SystemExit('sin clasificar: %s %s %s %s' % (where, mid, sev, t[:200]))


def entries(path):
    """nombre de archivo -> informes de HL7, de una salida de HL7"""
    out = {}
    b = json.load(open(path))
    # con una sola instancia HL7 escribe el OperationOutcome sin Bundle
    for o in ([e['resource'] for e in b['entry']] if 'entry' in b else [b]):
        f = next(x['valueString'] for x in o['extension'] if x['url'].endswith('-file')).split('/')[-1][:-5]
        out[f] = o['issue']
    return out


def exclusions(version):
    """perfil -> motivo, de lo que HL7 no puede generar en version"""
    out = {}
    for line in open(os.path.join(HERE, 'hl7', 'snapshot-exclusions.tsv')):
        v, name, origin, msg = line.rstrip('\n').split('\t', 3)
        if v == version and name != '*':
            out[name] = '%s: %s' % (origin, msg)
    return out


def cell_of(profile):
    """un perfil de apoyo (-ref, -tp) pertenece a la celda del perfil que lo usa"""
    return re.sub(r'-(ref|tp)$', '', profile)


def row(r):
    d = {'kind': r[0], 'severity': r[1], 'location': r[2]}
    if r[3]:
        d['detail'] = r[3]
    if r[4]:
        d['text'] = r[4]
    if r[5]:
        d['source'] = r[5]
    return d


def main():
    res, excluded = {}, {}
    for v in VERSIONS:
        ex = {}
        for prof, why in exclusions(v).items():
            ex.setdefault(cell_of(prof), why)
        cells = sorted({re.sub(r'^meta-', '', n)[:-len(s)] for n in entries(os.path.join(HERE, 'hl7', v + '-meta.json'))
                        for s in ('-aplica', '-no') if n.endswith(s)})
        for mode in ('meta', 'explicit'):
            for cell in cells:
                for app, suffix in (('applies', 'aplica'), ('not', 'no')):
                    name = '%s/%s/%s' % (mode, cell, app)
                    if mode == 'meta':
                        issues = entries(os.path.join(HERE, 'hl7', v + '-meta.json'))['meta-%s-%s' % (cell, suffix)]
                    else:
                        err = cell.split('-')[0]
                        issues = entries(os.path.join(HERE, 'hl7', 'explicit', v, cell + '.json'))['%s-%s' % (err, suffix)]
                    if any(msgid(i) == NO_SNAPSHOT for i in issues) and cell not in ex:
                        ex[cell] = 'validation: HL7 has no snapshot for a profile of the cell'
                    rows = [c for c in (classify(i, v + ' ' + name) for i in issues if msgid(i) != NO_SNAPSHOT) if c is not None]
                    rows = [r for r in rows if (v, r[3]) not in DIVERGENCES]
                    ent = res.setdefault(name, {'expected': {}})
                    ent['expected'][v] = [row(r) for r in sorted(set(rows))]
        excluded[v] = ex
        xs = entries(os.path.join(HERE, 'hl7', 'x-%s.json' % v))
        rows = [c for c in (classify(i, v + ' x') for i in xs['disp']) if c is not None]
        res.setdefault('x/disp', {'expected': {}})['expected'][v] = [row(r) for r in sorted(set(rows))]
    for v in VERSIONS:
        for cell in excluded[v]:
            for mode in ('meta', 'explicit'):
                for app in ('applies', 'not'):
                    res['%s/%s/%s' % (mode, cell, app)]['expected'].pop(v, None)
    res = {'cases': res, 'excluded': excluded}
    with open(sys.argv[1], 'w') as f:
        json.dump(res, f, indent=1, sort_keys=True)
        f.write('\n')
    print(len(res['cases']), 'instances;', sum(len(e) for e in excluded.values()), 'excluded cells', file=sys.stderr)


main()
