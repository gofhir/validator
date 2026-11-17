#!/usr/bin/env python3
"""Genera pkg/validator/testdata/primitive_type_profiles/expected.json desde las salidas de HL7 validator_cli 7.0.4
(-tx n/a) de ../probe/ (las 338 sondas de la pieza 3 de #193) y de dos sondas anteriores que el plan de la pieza 3 trae
(p14, p15 y p16 de ../../2026-11-14-type-profile-layers/probe/, y s22 de ../../2026-11-05-children-precedence/probe/review2/).
Los valores esperados salen solo de HL7, nunca de gofhir, con las excepciones declaradas abajo (cada una cita el plan,
docs/plans/2026-11-15-primitive-type-profiles.md, y la especificación).
Uso: python3 -I gen_expected.py <expected.json>"""
import json, os, re, sys

HERE = os.path.dirname(os.path.abspath(__file__))
EV = os.path.join(HERE, '..', '..')
VERSIONS = ('4.0.1', '4.3.0', '5.0.0')
# (carpeta, patrón del archivo de HL7 con {v}, nombres que se toman o None para todos)
SRC = [
    (os.path.join(EV, '2026-11-15-primitive-type-profiles', 'probe'), 'hl7-{v}-7.0.4.json', None),
    (os.path.join(EV, '2026-11-15-primitive-type-profiles', 'probe'), 'hl7-c14-{v}-7.0.4.json', None),  # c14: S1, ronda 2
    (os.path.join(EV, '2026-11-15-primitive-type-profiles', 'probe'), 'hl7-c15-{v}-7.0.4.json', None),  # c15: S1g y S1i, antes de la ronda 3
    (os.path.join(EV, '2026-11-15-primitive-type-profiles', 'probe'), 'hl7-c16-{v}-7.0.4.json', None),  # c16: T1 y T2, ronda 3
    (os.path.join(EV, '2026-11-15-primitive-type-profiles', 'probe'), 'hl7-c17-{v}-7.0.4.json', None),  # c17: un slice con dos perfiles, uno no resuelto (cierre de la ronda 3)
    (os.path.join(EV, '2026-11-14-type-profile-layers', 'probe'), 'hl7-{v}-7.0.4.json', {'p14', 'p15', 'p16'}),
    (os.path.join(EV, '2026-11-05-children-precedence', 'probe', 'review2'), 'hl7-s21-s22-{v}-7.0.4.json', {'s22'}),
]
DESC = [os.path.join(EV, '2026-11-15-primitive-type-profiles', 'probe', 'descriptions.json'),
        os.path.join(EV, '2026-11-14-type-profile-layers', 'probe', 'descriptions.json')]


def strip(loc):
    loc = re.sub(r'/\*[^*]*\*/', '', loc)                              # anotación /*Tipo/id*/ de HL7
    return re.sub(r'\.(\w+)\.ofType\((\w+)\)', lambda m: '.' + m.group(1) + m.group(2)[0].upper() + m.group(2)[1:], loc)


def msgid(i):
    return next((x.get('valueCode') for x in i.get('extension', []) if 'message-id' in x.get('url', '')), '')


def leaf(path):      # «string.extension:req» -> extension; «string.id» -> id
    return re.sub(r':[^.]*', '', path).split('.')[-1]


def classify(i, name):
    """Una entrada de HL7 a (kind, severity, location, detail), o None si se ignora."""
    sev, t, mid, loc = i['severity'], i['details']['text'], msgid(i), strip((i.get('expression') or [''])[0])
    if sev == 'warning':
        if mid == 'Validation_VAL_Profile_MultipleMatches':
            return ('profile-multiple-matches', 'warning', loc, '')
        m = re.match(r"Constraint failed: (\S+?): '", t)
        if i['code'] == 'invariant' and m and m.group(1) not in ('dom-6', 'ident-1'):
            return ('constraint', 'warning', loc, m.group(1))
        return None
    if sev not in ('error', 'fatal'):
        return None
    if mid == 'Validation_VAL_Profile_NoMatch':
        return ('profile-no-match', 'error', loc, '')
    if mid == 'Validation_VAL_Unknown_Profile':
        return ('type-profile-not-found', 'error', loc, re.sub(r' \(validating against .*$', '', t.split('Unknown profile ', 1)[1].strip()))
    if mid in ('Validation_VAL_Profile_Minimum', 'Validation_VAL_Profile_Maximum'):
        return ('min' if mid.endswith('Minimum') else 'max', 'error', loc, leaf(t.split(': ')[0]))
    if mid == 'Validation_VAL_Profile_Minimum_SLICE':
        return ('min', 'error', loc, leaf(re.match(r"Slice '([^']*)'", t).group(1)))
    if mid == 'Validation_VAL_Profile_NotSlice':   # c12: un elemento que ningún slice de un slicing cerrado admite
        return ('slice-no-match', 'error', loc, '')
    if mid.startswith('Terminology_TX_'):         # c12: un Coding fuera del ValueSet de un binding required
        return ('binding', 'error', loc, '')
    if mid == 'Reference_REF_BadTargetType':      # c12: una referencia a un recurso de un tipo que el targetProfile no admite
        return ('reference-target-type', 'error', loc, '')
    if mid == 'Extension_PROF_Type':               # c12: un valor de un tipo que el perfil de la extensión (ext-two-str) no admite
        return ('type-not-allowed', 'error', loc, '')
    if mid == 'SLICING_CANNOT_BE_EVALUATED':
        return ('slicing-cannot-be-evaluated', 'error', loc, '')
    m = re.match(r"Constraint failed: (\S+?): '(.*)'(?: \((?:validating against|defined in)|$)", t)
    if i['code'] == 'invariant' and m:
        return ('constraint', 'error', loc, m.group(1))
    raise SystemExit('sin clasificar: %s %s %s' % (name, mid, t[:140]))


# kinds donde HL7 repite el mismo informe por capa y por el tipo base: gofhir lo informa una vez (pieza 2, plan §2.5 y §2.7)
COLLAPSE = {'min', 'max'}


def load(version):
    """HL7 por instancia: nombre -> lista de issues clasificados, de todas las fuentes."""
    out = {}
    for folder, pat, only in SRC:
        b = json.load(open(os.path.join(folder, pat.format(v=version))))
        for e in b['entry']:
            o = e['resource']
            f = next(x['valueString'] for x in o['extension'] if x['url'].endswith('-file')).split('/')[-1][:-5]
            if only is not None and f not in only:
                continue
            out[f] = [c for c in (classify(i, f) for i in o['issue']) if c is not None]
    return out


def collapse(rows):
    seen, out = set(), []
    for r in rows:
        if r[0] in COLLAPSE:
            if r in seen:
                continue
            seen.add(r)
        out.append(r)
    return out


# --- A6 (plan §3): un tipo de elección con varios tipos. HL7 aplica solo el primer perfil de `type.profile`
# (InstanceValidator.java:8072 y :8085, como en p12 de la pieza 2). La especificación (R4 elementdefinition.html#
# ElementDefinition.type.profile: «If any profiles are specified, then the content must conform to at least one of them»)
# lo pide para todo el conjunto, en cualquier orden. El valor esperado es el de la sonda del mismo valor y los mismos
# perfiles (ps-len y ps-up) en un elemento de un solo tipo: c3-<v>-ext para la extensión y c3-<v>-obs para la Observation.
SPEC_CHOICE = {}
for _tag in ('both', 'none', 'len'):
    SPEC_CHOICE['c5-xc2-' + _tag] = 'c3-%s-ext' % _tag
    SPEC_CHOICE['c5-oc2-' + _tag] = 'c3-%s-obs' % _tag
    SPEC_CHOICE['c5-oc2r-' + _tag] = 'c3-%s-obs' % _tag


# --- A7 (plan §8, ronda 1, R2): con un perfil de tipo que no se resuelve, HL7 no aplica los hijos que el perfil de la instancia
# despliega para el valor (c13-pgu-extmin-none y c13-pnu-fam-giv no informan el mínimo; los controles sin perfil de tipo,
# c13-pgc-extmin-none y c13-pnc-fam-giv, sí). La especificación (ElementDefinition.min, snapshot del perfil) lo pide siempre. El
# valor esperado es el de HL7 más los `min` del control.
SPEC_CHILDREN = {'c13-pgu-extmin-none': 'c13-pgc-extmin-none', 'c13-pnu-fam-giv': 'c13-pnc-fam-giv'}


def row(r):
    d = {'kind': r[0], 'severity': r[1], 'location': r[2]}
    if r[3]:
        d['detail'] = r[3]
    return d


def main():
    res = {}
    desc = {}
    for p in DESC:
        desc.update({a: b for a, b in json.load(open(p))})
    for v in VERSIONS:
        hl7 = load(v)
        for n, rows in sorted(hl7.items()):
            note = ''
            if n in SPEC_CHOICE:
                src = hl7[SPEC_CHOICE[n]]
                rows, note = src, 'divergence A6 (plan 2026-11-15, §3): a choice type with several types uses all its profiles (spec), as %s' % SPEC_CHOICE[n]
            if n in SPEC_CHILDREN:
                rows = rows + [r for r in hl7[SPEC_CHILDREN[n]] if r[0] == 'min']
                note = 'divergence A7 (plan 2026-11-15, §8 R2): the children the instance profile unrolls apply with an unresolved type profile (spec), as the control %s' % SPEC_CHILDREN[n]
            rows = collapse(rows)
            ent = res.setdefault(n, {'description': desc.get(n, ''), 'expected': {}})
            ent['expected'][v] = [row(r) for r in sorted(rows)]
            if note:
                ent['note'] = note
    with open(sys.argv[1], 'w') as f:
        json.dump(res, f, indent=1, sort_keys=True)
        f.write('\n')
    print(len(res), 'instances', file=sys.stderr)


main()
