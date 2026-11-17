#!/usr/bin/env python3
"""Genera pkg/validator/testdata/type_profile_layers/expected.json desde las salidas de HL7 validator_cli 7.0.4
(-tx n/a) de ../probe/ (p1 a p21 y las sondas de la pieza 2) y ../probe-callers/ (una sonda por llamador) y
../probe-gaps/ (los huecos del oráculo de las mutaciones: 3c, 3d, 10, 13a a 13c).
Los valores esperados salen solo de HL7, nunca de gofhir, con las excepciones declaradas abajo (cada una cita el plan,
docs/plans/2026-11-06-type-profile-layers.md, y la especificación).
Uso: python3 -I gen_expected.py <expected.json>"""
import json, os, re, sys

HERE = os.path.dirname(os.path.abspath(__file__))
SRC = [os.path.join(HERE, '..', 'probe'), os.path.join(HERE, '..', 'probe-callers'), os.path.join(HERE, '..', 'probe-gaps')]
VERSIONS = ('4.0.1', '4.3.0', '5.0.0')


def strip(loc):
    loc = re.sub(r'/\*[^*]*\*/', '', loc)                              # anotación /*Tipo/id*/ de HL7
    return re.sub(r'\.(\w+)\.ofType\((\w+)\)', lambda m: '.' + m.group(1) + m.group(2)[0].upper() + m.group(2)[1:], loc)


def msgid(i):
    return next((x.get('valueCode') for x in i.get('extension', []) if 'message-id' in x.get('url', '')), '')


def leaf(path):      # «Quantity.unit» -> unit; «Observation.identifier:s1.value» -> value; «Quantity.extension:xi» -> extension
    return re.sub(r':[^.]*', '', path).split('.')[-1]


def classify(i, name):
    """Una entrada de HL7 a (kind, severity, location, detail, text), o None si se ignora."""
    sev, t, mid, loc = i['severity'], i['details']['text'], msgid(i), strip((i.get('expression') or [''])[0])
    if sev == 'warning':
        if mid == 'Validation_VAL_Profile_MultipleMatches':
            return ('profile-multiple-matches', 'warning', loc, '', '')
        m = re.match(r"Constraint failed: (\S+?): '", t)
        if i['code'] == 'invariant' and m and m.group(1) not in ('dom-6', 'ident-1'):
            return ('constraint', 'warning', loc, m.group(1), '')
        return None
    if sev not in ('error', 'fatal'):
        return None
    if mid == 'Validation_VAL_Profile_NoMatch':
        return ('profile-no-match', 'error', loc, '', '')
    if mid == 'Validation_VAL_Unknown_Profile':
        # HL7 añade «(validating against <perfil>)» cuando el perfil faltante lo declara otro perfil; el canónico es lo anterior
        return ('type-profile-not-found', 'error', loc, re.sub(r' \(validating against .*$', '', t.split('Unknown profile ', 1)[1].strip()), '')
    if mid in ('Validation_VAL_Profile_Minimum', 'Validation_VAL_Profile_Maximum'):
        return ('min' if mid.endswith('Minimum') else 'max', 'error', loc, leaf(t.split(': ')[0]), '')
    if mid in ('Validation_VAL_Profile_Minimum_SLICE', 'Validation_VAL_Profile_Minimum_SLICE_EXT'):
        return ('min', 'error', loc, leaf(re.match(r"Slice '([^']*)'", t).group(1)), '')
    if mid == 'Reference_REF_CantMatchChoice':
        return ('ref-target-profile', 'error', loc, '', '')
    if mid == 'Validation_VAL_Profile_NotSlice':
        return ('slice-no-match', 'error', loc, '', '')
    if mid == 'Extension_EXTP_Context_Wrong':
        return ('extension-context', 'error', loc, '', '')
    if mid in ('UNABLE_TO_INFER_CODESYSTEM', 'Terminology_TX_NoValid_16'):
        return ('binding', 'error', loc, '', '')
    if mid == 'CONTAINED_ORPHAN_DOM3':
        return None       # dom-3 de un contenido sin referenciar: lo informa el contenedor en gofhir y el contenido en HL7; no es de esta pieza
    if mid == 'Extension_PROF_Type':
        return ('type-not-allowed', 'error', loc, '', '')
    if mid == 'Extension_EXT_Type':
        # HL7 ubica el tipo que no cabe en la extensión; gofhir, en el valor (su propiedad «valueString»)
        return ('type-not-allowed', 'error', loc + '.value' + re.search(r'found type (\w+)', t).group(1).capitalize(), '', '')
    if mid == 'Type_Specific_Checks_DT_Integer_Valid' and name.startswith('gt'):
        return None       # lo mismo que Extension_EXT_Type (un string donde va un integer), dicho otra vez por HL7
    if mid == 'Validation_VAL_Profile_NotAllowed':
        return ('not-allowed', 'error', loc, '', '')
    if t.startswith('Unrecognized property'):
        # HL7 ubica el error del JSON en el elemento de elección («value[x]»); gofhir, en la propiedad
        prop = re.match(r"Unrecognized property '([^']*)'", t).group(1)
        return ('unknown-element', 'error', loc.replace('value[x]', 'valueQuantity') + '.' + prop, '', '')
    if t.startswith('Error parsing JSON: the primitive value must be a string'):
        return ('type-invalid', 'error', loc.replace('value[x]', 'valueQuantity'), '', '')
    m = re.match(r"Constraint failed: (\S+?): '(.*)'(?: \((?:validating against|defined in)|$)", t)
    if i['code'] == 'invariant' and m:
        text = m.group(2) if name in TEXT_CHECK else ''
        return ('constraint', 'error', loc, m.group(1), text)
    raise SystemExit('sin clasificar: %s %s %s' % (name, mid, t[:140]))


# E3 (plan §3f y §5): el mensaje de una invariante común a dos capas lleva el texto de la primera capa declarada.
TEXT_CHECK = {'txt1': "text A", 'txt2': "text B"}
# kinds donde HL7 repite el mismo informe por capa y por el tipo base: gofhir lo informa una vez (plan §2.5 y §2.7, p21)
COLLAPSE = {'min', 'max', 'binding', 'extension-context', 'type-not-allowed'}


def load(version):
    """HL7 por instancia: nombre -> lista de issues clasificados, de todas las fuentes."""
    out = {}
    for src in SRC:
        b = json.load(open(os.path.join(src, 'hl7-%s-7.0.4.json' % version)))
        for e in b['entry']:
            o = e['resource']
            f = next(x['valueString'] for x in o['extension'] if x['url'].endswith('-file')).split('/')[-1][:-5]
            rows = []
            for i in o['issue']:
                c = classify(i, f)
                if c is not None:
                    rows.append(c)
            out[f] = rows
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


def row(r, name):
    d = {'kind': r[0], 'severity': r[1], 'location': r[2]}
    if r[3]:
        d['detail'] = r[3]
    if name in TEXT_CHECK and r[0] == 'constraint' and r[3] == 'dup-1':
        d['text'] = TEXT_CHECK[name]
    return d


# --- E2 (plan §5): con la especificación (fhirpath.html: «%resource: the resource that contains the original node that is
# in %context»; «%rootResource»: el recurso contenedor, y para una entrada de Bundle, ella misma) el dueño del Identifier
# es el recurso que lo contiene: en `s` y `t` la Observation o1, en `h` la Observation o2 que resolve() halla.
SPEC = {'rempty': lambda o: False, 'rexists': lambda o: True, 'rid1': lambda o: o == 'o1', 'rid2': lambda o: o == 'o2',
        'rro1': lambda o: o == 'o1', 'rrid0': lambda o: False, 'rrid1': lambda o: False, 'rval': lambda o: False,
        'rrval': lambda o: False}


def e2_overrides(hl7):
    """name -> filas esperadas por la especificación, para las instancias e2s* (discriminador por perfil) y e2h*
    (discriminador resolve().identifier, en el mismo Bundle o en uno anidado)."""
    parsed = {}
    for n in hl7:
        m = re.match(r'e2(s|hn|h)(r.+)$', n)
        if not m:
            continue
        kind, rest = m.groups()
        where = ''
        if kind == 's':
            where, rest = rest[-1], rest[:-1]
        if rest in SPEC:
            parsed[n] = (kind, rest, where)
    notslice = {}     # la ubicación del error de slice cerrado, de las propias salidas de HL7
    for n, (kind, rest, where) in parsed.items():
        for r in hl7[n]:
            if r[0] == 'slice-no-match':
                notslice.setdefault((kind, where), r[2])
    out = {}
    for n, (kind, rest, where) in parsed.items():
        owner = 'o2' if kind in ('h', 'hn') else 'o1'
        out[n] = [] if SPEC[rest](owner) else [('slice-no-match', 'error', notslice[(kind, where)], '', '')]
    return out


def main():
    res = {}
    desc = {}
    for src in SRC:
        p = os.path.join(src, 'descriptions.json')
        if os.path.exists(p):
            desc.update({a: b for a, b in json.load(open(p))})
    div = []
    for v in VERSIONS:
        hl7 = load(v)
        e2 = e2_overrides(hl7)
        for n, rows in sorted(hl7.items()):
            note = ''
            if n == 'p12':
                # p12: tipo de elección. HL7 usa solo el primer perfil (InstanceValidator.java:8072 y :8085); la
                # especificación (ElementDefinition.type.profile: «the content must conform to at least one of them»)
                # lo pide para el conjunto: con el mismo valor y los mismos perfiles que p3, p12 da lo que p3.
                rows, note = hl7['p3'], 'divergence p12: a choice type uses all its profiles (spec), as p3'
            elif n == 'p20':
                # p20: un perfil de otro tipo cuenta como no cumplido y no se recorre (plan §2.7, #193 pieza 2).
                rows = [r for r in rows if r[0] != 'not-allowed' and not (r[0] == 'min' and r[3] == 'text')]
                note = 'divergence p20: a profile of another type is not walked'
            elif n == 'bcyc3':
                # bcyc3: HL7 no informa el perfil de destino de Identifier.assigner hacia un contenido que no cumple (ni con
                # un solo perfil de tipo ni con el targetProfile puesto en el perfil de Observation: evidencia en
                # ../probe-callers/hl7-target-in-datatype/, plan §«Después de la implementación»). La especificación
                # (ElementDefinition.type.targetProfile) lo pide: NoMatch con el mínimo de cada capa y el destino.
                rows = [('min', 'error', 'Observation.identifier[0].period', 'period', ''),
                        ('profile-no-match', 'error', 'Observation.identifier[0]', '', ''),
                        ('ref-target-profile', 'error', 'Observation.identifier[0].assigner', '', '')]
                note = 'divergence bcyc3: the target profile of a reference in a datatype is reported (spec); HL7 reports nothing'
            elif n == 'kucum':
                # HL7 con -tx n/a no valida un código UCUM (solo avisa). El código «foo bar» no es UCUM válido
                # (ucum.org/ucum, §2): el valor esperado es el de la especificación de UCUM.
                rows, note = [('ucum', 'error', 'Observation.valueRange.low.code', '', '')], 'expected by the UCUM spec: HL7 with -tx n/a only warns'
            elif n in e2:
                if sorted(e2[n]) != sorted(rows):
                    div.append((v, n))
                    note = 'divergence E2: expected by the spec (owner resource), HL7 uses the datatype'
                rows = e2[n]
            elif n == 'p21':
                note = 'divergence p21: the cardinality common to both layers is reported once'
            rows = collapse(rows)
            ent = res.setdefault(n, {'description': desc.get(n, ''), 'expected': {}})
            ent['expected'][v] = [row(r, n) for r in sorted(rows)]
            if note:
                ent['note'] = note
    with open(sys.argv[1], 'w') as f:
        json.dump(res, f, indent=1, sort_keys=True)
        f.write('\n')
    print(len(res), 'instances; E2 rows where the spec differs from HL7:', len(div), sorted(set(n for _, n in div))[:60], file=sys.stderr)


main()
