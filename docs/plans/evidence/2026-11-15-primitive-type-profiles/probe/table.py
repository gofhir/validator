"""python3 -I table.py > tables.md : tabla por caso con (severidad, message-id o clave de constraint, ubicacion) por version."""
import sys, os, json, subprocess, importlib.util
H = os.path.dirname(os.path.abspath(__file__))
def load(v):
    sys.argv = ['x', v]
    spec = importlib.util.spec_from_file_location('s' + v, os.path.join(H, 'summarize.py'))
    m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m); return m.out, m.desc
data = {}
for v in ('4.0.1', '4.3.0', '5.0.0'):
    data[v], desc = load(v)
def short(i):
    s, m, l, t = i
    if m.startswith('http://example.org/fhir/StructureDefinition/'):
        m = m.rsplit('/', 1)[1]
    if m.startswith('http://hl7.org/fhir/StructureDefinition/'):
        m = m.rsplit('/', 1)[1]
    if m == 'Details_for__matching_against_Profile_' and 'hasMember' in l:
        return None
    l = l.replace('Observation.value.ofType(', 'Obs.value.ofType(')
    return '%s `%s` @ `%s`' % ({'error': 'err', 'warning': 'warn', 'information': 'info'}.get(s, s), m or '(none)', l)
titles = {'c1': 'Caso 1: un solo perfil', 'c2': 'Caso 2: perfil que no resuelve', 'c3': 'Caso 3: dos perfiles, arrays, valueless',
          'c4': 'Caso 4: misma clave en ambos perfiles', 'c5': 'Caso 5: choice y slice por tipo', 'c6': 'Caso 6: %resource / %rootResource en un contenido',
          'c7': 'Caso 7: constraints de la raíz con extension, id, hasValue() y %context', 'c8': 'Caso 8: escalares iguales bajo padres y recursos distintos',
          'c9': 'Caso 9: slices de extensión obligatorios y prohibidos', 'c11': 'Caso 11: guardas del camino sin perfiles', 'c10': 'Caso 10 (A4): perfiles pareados por .value, y .value con id/extension',
          'c12': 'Caso 12: huecos de la corrida de mutaciones (plan §7)'}
cur = None
for n, d in desc.items():
    c = n.split('-')[0]
    if c != cur:
        cur = c
        print('\n## %s\n\n| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |\n|---|---|---|---|' % titles[c])
    res = {}
    for v in data:
        res[v] = [x for x in (short(i) for i in data[v].get(n, [])) if x]
    ref = res['4.0.1']
    txt = '<br>'.join(ref) if ref else 'nada'
    same = all(res[v] == ref for v in res)
    diff = 'igual' if same else 'R4B: ' + ('<br>'.join(res['4.3.0']) or 'nada') + ' / R5: ' + ('<br>'.join(res['5.0.0']) or 'nada')
    print('| `%s` | %s | %s | %s |' % (n, d.replace('|', '/'), txt, diff))
