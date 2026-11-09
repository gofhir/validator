#!/usr/bin/env python3
"""Genera pkg/validator/testdata/canonical_scope/expected.json desde las salidas de HL7 validator_cli 7.0.4
(hl7-<sondas>-<version>-7.0.4.json). Los valores esperados salen solo de HL7, nunca de gofhir.
Uso: gen_expected.py <salida.json>"""
import json, re, sys

GROUPS = {
    'c1': 'sc8-sc10', 'c2': 'sc11-sc14', 'd1': 'd1a1-d1c2', 'd3': 'd3a1-d3c1', 'r1': 'r1a-r1b', 'r4': 'r4a-r4c',
}
VERSIONS = {'4.0.1': '', '4.3.0': 'v4.3.0/', '5.0.0': 'v5.0.0/'}
# HL7 informa un destino que no cumple una vez, en el primer referente (sc6 a sc9); gofhir en cada uno.
FIRST_REFERRER = {'sc8', 'sc9'}


def classify(text):
    if text.startswith('Unable to find a profile match for'):
        return 'target-profile'
    if 'minimum required' in text or 'a matching slice is required' in text:
        return 'min'
    m = re.match(r"Constraint failed: (\S+?):", text)
    if m:
        return 'constraint:' + m.group(1)
    if 'not in the value set' in text or 'not found in the value set' in text or 'System URI could not be determined' in text:
        return 'binding'
    raise SystemExit('sin clasificar: ' + text[:120])


def strip(loc):
    return re.sub(r'/\*[^*]*\*/', '', loc)


out = {}
for g, name in GROUPS.items():
    for v, sub in VERSIONS.items():
        b = json.load(open(f'{g}/{sub}hl7-{name}-{v}-7.0.4.json'))
        for e in b['entry']:
            o = e['resource']
            f = next(x['valueString'] for x in o['extension'] if x['url'].endswith('-file')).split('/')[-1][:-5]
            lines = out.setdefault(g, {}).setdefault(f, {'firstReferrer': f in FIRST_REFERRER, 'expected': {}})['expected'].setdefault(v, [])
            for i in o['issue']:
                if i['severity'] not in ('error', 'fatal'):
                    continue
                t = i['details']['text']
                lines.append({'kind': classify(t), 'location': strip(i['expression'][0])})
            lines.sort(key=lambda x: (x['kind'], x['location']))
json.dump(out, open(sys.argv[1], 'w'), indent=1, sort_keys=True)
