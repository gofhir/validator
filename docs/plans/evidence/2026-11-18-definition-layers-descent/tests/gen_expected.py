#!/usr/bin/env python3
"""Genera pkg/validator/testdata/definition_layers/expected.json desde las salidas de HL7 validator_cli 7.0.4 (-tx n/a)
que deja run_hl7.sh en hl7/ (seis fuentes, R4, R4B y R5). Los valores esperados salen solo de HL7, nunca de gofhir,
con las excepciones declaradas abajo; cada una cita el plan (docs/plans/2026-11-18-definition-layers-descent.md) y la
especificación. Las instancias se nombran <fuente>/<instancia>.
Uso: python3 -I gen_expected.py <carpeta de trabajo de run_hl7.sh> <expected.json>"""
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
VERSIONS = ('4.0.1', '4.3.0', '5.0.0')
SOURCES = ('matrix', 'x2', 'p', 'g', 'probe', 'm1')

# Fuera del alcance (binding y cardinalidad van en la pieza siguiente, plan §1): sondas cuyo objeto es un binding o
# una cardinalidad. Se dejan fuera por nombre, no por el texto de sus informes.
OUT_OF_SCOPE = re.compile(r'(^|/)(ok|bad)-[a-z]+-(binding|min|max)$|/bad-cx?-|/bad-d-tp-m$|/(ok|bad)-s-deep$'
                          # la misma clave con otra expresión en un perfil de tipo y en el elemento: HL7 rechaza su
                          # snapshot (w1, w2) y el plan §2 lo deja fuera del alcance (plan §9).
                          r'|^p/m[89]-shadow$')

# Avisos de buenas prácticas de HL7 que no salen de ninguna StructureDefinition (InstanceValidator los escribe a mano
# para Observation): por la regla 5 (nada hardcodeado sin ADR) gofhir no los da. Se ignoran por su ID de mensaje.
HL7_ONLY = {'All_observations_should_have_a_subject', 'All_observations_should_have_a_performer',
            'All_observations_should_have_an_effectiveDateTime_or_an_effectivePeriod'}

TYPE_IDS = {'Validation_VAL_Profile_NoType', 'Extension_EXT_Type', 'Extension_PROF_Type'}
SIMPLE = {'Validation_VAL_Profile_NoMatch': 'profile-no-match', 'Validation_VAL_Profile_MultipleMatches': 'profile-multiple-matches',
          'Validation_VAL_Profile_MatchMultiple': 'slice-match-multiple', 'SLICING_CANNOT_BE_EVALUATED': 'slicing-cannot-be-evaluated',
          'Validation_VAL_Profile_NotAllowed': 'not-allowed', 'Validation_VAL_Profile_NotSlice': 'slice-no-match'}


def strip(loc):
    loc = re.sub(r'/\*[^*]*\*/', '', loc)                              # anotación /*Tipo/id*/ de HL7
    return re.sub(r'\.(\w+)\.ofType\((\w+)\)', lambda m: '.' + m.group(1) + m.group(2)[0].upper() + m.group(2)[1:], loc)


def msgid(i):
    return next((x.get('valueCode') for x in i.get('extension', []) if 'message-id' in x.get('url', '')), '')


def types_of(t):
    """Los tipos permitidos que nombra un informe de tipo no permitido, ordenados: «string,boolean»."""
    m = (re.search(r'allows for the types? \[?([^\]]*?)\]? but found', t) or
         re.search(r'Valid types at this point are (.*)$', t))
    return ','.join(sorted(x.strip() for x in m.group(1).split(',')))


def classify(i, name):
    """Una entrada de HL7 a (kind, severity, location, detail, text, source), o None si se ignora. source es el perfil
    que nombra «(defined in …)» en una constraint (plan §2, punto 8)."""
    sev, t, mid, loc = i['severity'], i['details']['text'], msgid(i), strip((i.get('expression') or [''])[0])
    if sev == 'information' or mid in HL7_ONLY:
        return None
    sev = 'error' if sev == 'fatal' else sev
    m = re.match(r"Constraint failed: (\S+?): '(.*?)'(?: \((?:validating against|defined in)|$)", t)
    if i['code'] == 'invariant' and m:
        src = re.search(r'\(defined in (\S+?)\)?(?: |$)', t)
        return ('constraint', sev, loc, m.group(1), m.group(2), src.group(1) if src else '')
    if mid == '_DT_Fixed_Wrong':      # fixed[x] y pattern[x]: HL7 informa la hoja distinta y el valor exigido
        return ('fixed', sev, loc, re.search(r"but is fixed to '(.*)' in the profile", t).group(1), '', '')
    if mid in TYPE_IDS:
        return ('type-not-allowed', sev, loc, types_of(t), '', '')
    if mid in SIMPLE:
        return (SIMPLE[mid], sev, loc, '', '', '')
    raise SystemExit('sin clasificar: %s %s %s' % (name, mid, t[:160]))


def load(source, version):
    out = {}
    b = json.load(open(os.path.join(HERE, 'hl7', '%s-%s-7.0.4.json' % (source, version))))
    for e in b['entry']:
        o = e['resource']
        f = next(x['valueString'] for x in o['extension'] if x['url'].endswith('-file')).split('/')[-1][:-5]
        n = source + '/' + f
        if OUT_OF_SCOPE.search(n):
            continue
        out[n] = [c for c in (classify(i, n) for i in o['issue']) if c is not None]
    return out


def collapse(rows):
    """Divergencia declarada (plan §2, «Un mismo hecho en dos definiciones»): HL7 repite el informe cuando dos
    definiciones dicen lo mismo del mismo valor (el hijo desplegado y el perfil de tipo, el elemento de la extensión y
    su definición); gofhir da uno por ubicación, diagnóstico y parámetros."""
    seen, out = set(), []
    for r in rows:
        if r not in seen:
            seen.add(r)
            out.append(r)
    return out


# --- D2 (plan §2, divergencia; evidencia hl7-d2-snapshot/): el perfil derivado d2-<regla> solo redeclara el
# slice mrn, y el snapshot que genera HL7 pierde los hijos desplegados del elemento dividido que trae su base
# (Patient.identifier.value con dl-a, etc.), así que HL7 no aplica A. La especificación pide que un perfil derivado
# agregue reglas a su base sin quitarle ninguna (R4 structuredefinition-definitions.html#StructureDefinition.derivation,
# constraint: «This definition adds additional rules to an existing concrete type»; R4 profiling.html: «Profiles cannot
# break the rules established in the base specification»), y en el mismo perfil HL7 sí
# aplica A (m-sid-*). El valor esperado es el del control sid con la misma regla y la misma violación.
def spec_d2(name):
    m = re.match(r'matrix/m-d2-(\w+)-(\w+)$', name)
    return 'matrix/m-sid-%s-%s' % m.groups() if m else None


# --- D2b (plan §9; run_hl7.sh, known): HL7 no genera el snapshot de un perfil derivado que restringe un hijo del
# elemento dividido cuyo slice declara la base («Same id ... on multiple elements») y valida con un snapshot roto. El
# valor esperado es el de un control: el mismo diferencial en un solo perfil (ctl-*), o la sonda hermana s5e, que
# declara lo mismo que s5b en un solo perfil.
CONTROL = {'x2/bad-d-type': 'x2/ctl-d-type', 'x2/bad-d-type2': 'x2/ctl-d-type2', 'g/bad-g3': 'g/ctl-bad-g3',
           'g/ok-g3': 'g/ctl-ok-g3', 'p/s5b-ew': 'p/s5e-ew', 'p/s5b-we': 'p/s5e-we'}


# --- D2 con la regla sev (plan §9): m-d2-sev no sigue al control m-sid-sev. En m-sid las dos constraints están en un
# solo perfil y el hijo clonado cede ante la del slice (punto 7). En m-d2, la clave dl-s llega del snapshot base
# (d2b-sev), y el punto 7 la conserva, como hace HL7 en s5a-we y s5d-we (la base gana): queda la advertencia de A.
def spec_d2_sev(rows):
    return [('constraint', 'warning', r[2], r[3], r[4], 'http://example.org/dl/d2b-sev') if r[0] == 'constraint' and r[3] == 'dl-s'
            else r for r in rows]


# --- Operador is (plan §9; evidencia is-operator/): HL7 7.0.4 evalúa `$this is FHIR.string` como falso sobre un
# string, y `$this.is(FHIR.string)` como verdadero. FHIRPath N1 §6.3 define is(type) como «the same as the is
# operator», y Identifier.value es FHIR.string. Las sondas m1–m5 llevan las dos polaridades (n: la negación, p: la
# afirmación), así que el valor esperado de una es el que HL7 da para la otra, con su perfil de tipo. bad-g1 no tiene
# pareja: su invariante, `($this is FHIR.string).not()`, falla en Identifier.value.
def spec_is(name, hl7):
    m = re.match(r'(p/m[1-5]-)([np])$', name)
    if m:
        a, b = m.group(2), 'p' if m.group(2) == 'n' else 'n'
        return [r[:5] + (r[5].replace('-' + b, '-' + a) if r[5].startswith('http://example.org/p5/') else r[5],) for r in hl7[m.group(1) + b]]
    if name == 'g/bad-g1':
        return hl7[name] + [('constraint', 'error', 'Patient.identifier[0].value', 'gk', 'gk', '')]
    return None


# --- Cascada bajo un tipo no permitido (plan §9, bad-f-type): HL7 valida el contenido de un valor de tipo no permitido
# contra la definición del tipo permitido (los hijos de un Reference contra string) e informa cada hijo como no
# permitido. gofhir informa el tipo una vez. No hay definición para esos hijos: es el mismo hecho.
def drop_cascade(rows):
    under = [r[2] + '.' for r in rows if r[0] == 'type-not-allowed']
    return [r for r in rows if not (r[0] == 'not-allowed' and any(r[2].startswith(u) for u in under))]


# --- Divergencias que agregan un informe a los de HL7 (plan §2, «Divergencias declaradas»):
#   - Reslicing (x2/bad-r-fix): HL7 aplica a/b y la base, no a. La especificación define a/b como «further slice a
#     slice» (R4 profiling.html, reslicing); que las reglas de a sigan vigentes en a/b es inferencia nuestra, declarada.
#     gofhir informa además el fixed «A» de a.
#   - contentReference (x2/bad-q-cr, B-D9 de 2026-09-27-profile-false-negatives.md): HL7 no aplica en item.item el
#     fixed de Questionnaire.item; gofhir sí, porque la contentReference trae las reglas de su destino.
#   - Identidad de invariante (p/m6-keys, p/m7-keys): HL7 memoriza una invariante por expresión y severidad, sin la
#     clave (InstanceValidator.java:8934-8937), y calla la segunda clave con la misma expresión. Son dos constraints
#     distintas (ElementDefinition.constraint.key identifica la constraint); gofhir informa las dos.
EXTRA = {
    'x2/bad-r-fix': (('fixed', 'error', 'Patient.identifier[0].value', 'A', '', ''),
                     'divergence (plan 2026-11-18, reslicing): the rules of slice a apply in its reslice a/b too (inference, declared)'),
    'x2/bad-q-cr': (('fixed', 'error', 'Questionnaire.item[0].item[0].text', 'T', '', ''),
                    'divergence B-D9 (plan 2026-09-27): the rules of the element a contentReference names apply to the '
                    'referring element (ElementDefinition.contentReference: «whose content rules should be applied»)'),
    'p/m6-keys': (('constraint', 'error', 'Patient.identifier[0].value', 'kp', 'kp', ''),
                  'divergence (plan 2026-11-18, invariant identity): two keys with the same expression are two constraints'),
    'p/m7-keys': (('constraint', 'error', 'Patient.identifier[0].value', 'kb', 'kb', ''),
                  'divergence (plan 2026-11-18, invariant identity): two keys with the same expression are two constraints'),
}


# --- dom-6 en un recurso contenido (vista y no resuelta desde 2026-10-11-held-resources-walker.md, «Out of step B»): HL7 no
# la evalúa en un recurso contenido; la especificación la pone en DomainResource (R4 domainresource.html, dom-6: «A
# resource should have narrative for robust management»), y un Patient contenido es un DomainResource. gofhir la informa.
DOM6 = 'A resource should have narrative for robust management'


def contained_dom6(work, name):
    source, f = name.split('/')
    o = json.load(open(os.path.join(work, source, '4.0.1', 'inst', f + '.json')))
    return [('constraint', 'warning', '%s.contained[%d]' % (o['resourceType'], i), 'dom-6', DOM6,
             'http://hl7.org/fhir/StructureDefinition/DomainResource')
            for i, c in enumerate(o.get('contained', [])) if 'text' not in c]


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
    work = sys.argv[1]
    desc = {'matrix/' + a: b for a, b in json.load(open(os.path.join(work, 'matrix', '4.0.1', 'descriptions.json')))}
    res = {}
    for v in VERSIONS:
        hl7 = {}
        for s in SOURCES:
            hl7.update(load(s, v))
        for n, rows in sorted(hl7.items()):
            note = ''
            ctl = spec_d2(n)
            if ctl:
                rows = hl7[ctl]
                note = ('divergence D2 (plan 2026-11-18): HL7 drops the base profile\'s sliced element children from the derived '
                        'snapshot; a derived profile keeps its base\'s constraints (spec), as the control %s' % ctl)
            if n in CONTROL:
                rows = hl7[CONTROL[n]]
                note = ('divergence D2b (plan 2026-11-18, §9): HL7 cannot generate this profile\'s snapshot (duplicate ids); '
                        'expected as the control %s, the same differential in one profile' % CONTROL[n])
            if re.match(r'matrix/m-d2-sev-', n):
                rows = spec_d2_sev(rows)
                note += '; the key comes from the base snapshot and the base keeps it (point 7, as HL7 in p/s5a-we, p/s5d-we)'
            spec = spec_is(n, hl7)
            if spec is not None:
                rows = spec
                note = ('divergence (plan 2026-11-18, §9): HL7 evaluates the operator `$this is FHIR.string` as false on a string '
                        'and the function is(FHIR.string) as true; FHIRPath N1 §6.3 makes them the same')
            cut = drop_cascade(rows)
            if cut != rows:
                rows = cut
                note = (note + '; ' if note else '') + 'divergence (plan 2026-11-18, §9): HL7 reports the children of a value of a type not allowed'
            if n in EXTRA:
                rows = rows + [EXTRA[n][0]]
                note = EXTRA[n][1]
            dom6 = contained_dom6(work, n)
            if dom6:
                rows = rows + dom6
                note = (note + '; ' if note else '') + 'divergence (spec): dom-6 applies to a contained DomainResource too'
            ent = res.setdefault(n, {'description': desc.get(n, n.split('/')[0] + ' probe'), 'expected': {}})
            ent['expected'][v] = [row(r) for r in sorted(collapse(rows))]
            if note:
                ent['note'] = note
    with open(sys.argv[2], 'w') as f:
        json.dump(res, f, indent=1, sort_keys=True)
        f.write('\n')
    print(len(res), 'instances', file=sys.stderr)


main()
