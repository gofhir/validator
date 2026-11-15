"""python3 -I table.py : writes table.md (probe | what | HL7 R4 | R4B | R5) from hl7-<v>-7.0.4.json and descriptions.json."""
import json, re, collections
VERS = ['4.0.1', '4.3.0', '5.0.0']
OLDDESC = {  # p1..p21 (see 2026-11-05-type-profile-alternatives/probe/README.md)
 'p1': 'OR Simple/Money, meets Simple', 'p2': 'OR, meets Money', 'p3': 'OR, meets none (comparator <, UCUM mg)', 'p4': 'only SimpleQuantity, comparator <',
 'p5': 'OR, unit without system, none', 'p6': 'unknown + Simple, fails Simple', 'p7': 'unknown + Simple, meets Simple', 'p8': 'two unknown profiles',
 'p9': 'component.value[x] repeated, one fails', 'p10': 'extension value[x] with OR', 'p11': 'choice type, valueString', 'p12': 'choice type, valueQuantity meets none',
 'p13': 'OR, meets both', 'p14': 'string profiles, meets both', 'p15': 'string, none', 'p16': 'string, only upper', 'p17': 'versioned canonicals (|ver adapted), none',
 'p18': 'Simple or sq-unit, only Simple', 'p19': 'Simple or sq-unit, both', 'p20': 'Simple or CodeableConcept profile (other type)', 'p21': 'Simple or sq-unit, common violation'}
desc = dict(json.load(open('descriptions.json')))
desc.update(OLDDESC)
def sig(path):
    b = json.load(open(path)); out = {}
    for e in b['entry']:
        o = e['resource']
        f = next((x.get('valueString') for x in o.get('extension', []) if x.get('url', '').endswith('-file')), '?').split('/')[-1][:-5]
        items = []
        for i in o.get('issue', []):
            t = i.get('details', {}).get('text', '')
            if i['severity'] not in ('error', 'warning', 'fatal') or 'dom-6' in t or t.startswith(('Best Practice', "Unable to validate code 'mg'")) or 'ident-1' in t or 'dom-3' in t or 'ContactEntityType' in t or t.startswith('No code provided'):
                continue
            mid = next((x.get('valueCode') for x in i.get('extension', []) if 'message-id' in x.get('url', '')), '')
            mid = mid.replace('Validation_VAL_', '').replace('Profile_', '')
            m = re.match(r"Constraint failed: ([\w-]+):", t)
            if m: mid = m.group(1) + ('(w)' if i['severity'] == 'warning' else '')
            elif mid in ('Minimum', 'Maximum'):
                m2 = re.match(r'([\w.:]+): ', t); fr = re.search(r'\(from [^)|]*?([\w-]+)(?:\|[\d.]+)?\)', t)
                mid = '%s %s%s' % (mid, m2.group(1).split('.')[-1] if m2 else '', ' from ' + fr.group(1) if fr else '')
            elif not mid:
                mid = t[:30]
            elif mid == 'NotSlice': mid = 'NotSlice'
            nv = len(re.findall(r'validating against', t))
            items.append(mid + ('[x%d]' % nv if nv > 1 else ''))
        out[f] = items
    return out
S = {v: sig('hl7-%s-7.0.4.json' % v) for v in VERS}
def fmt(items): return ', '.join(items) if items else 'nothing'
def key(n):
    m = re.match(r'p(\d+)$', n); return (0, int(m.group(1)), '') if m else (1, 0, n)
rows = ['| Probe | What it tests | HL7 R4 (4.0.1) | R4B (4.3.0) | R5 (5.0.0) |', '|---|---|---|---|---|']
for n in sorted(S['4.0.1'], key=key):
    r = [fmt(S[v][n]) for v in VERS]
    same = r[0] == r[1] == r[2]
    rows.append('| %s | %s | %s | %s | %s |' % (n, desc.get(n, ''), r[0], 'same' if r[1] == r[0] else r[1], 'same' if r[2] == r[0] else r[2]))
open('table.md', 'w').write('\n'.join(rows) + '\n')
print(len(rows) - 2, 'rows;', sum(1 for n in S['4.0.1'] if not (S['4.0.1'][n] == S['4.3.0'][n] == S['5.0.0'][n])), 'differ across versions')
