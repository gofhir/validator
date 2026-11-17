"""python3 -I summarize.py <hl7 output.json> : one line per error/warning issue (dom-6 skipped), grouped by instance."""
import json, sys, re
b = json.load(open(sys.argv[1]))
outs = [e['resource'] for e in b.get('entry', [])] if b.get('resourceType') == 'Bundle' else [b]
rows = {}
for o in outs:
    f = next((x.get('valueString') for x in o.get('extension', []) if x.get('url', '').endswith('-file')), '?').split('/')[-1][:-5]
    rows.setdefault(f, [])
    for i in o.get('issue', []):
        t = i.get('details', {}).get('text', '')
        if i['severity'] in ('error', 'warning', 'fatal') and 'dom-6' not in t and not t.startswith(('Best Practice Recommendation', "Unable to validate code 'mg'")):
            mid = next((x.get('valueCode') for x in i.get('extension', []) if 'message-id' in x.get('url', '')), '')
            m = re.search(r'\((?:validating against|from) [^)]*\)', t)
            rows[f].append('%s %s %s @%s | %s' % (i['severity'][0], i['code'], mid.replace('Validation_VAL_', 'V_'), (i.get('expression') or [''])[0], t[:420]))
def key(s): return (not re.match(r'p\d+$', s), int(re.sub(r'\D', '', s) or 0) if re.match(r'p\d+$', s) else 0, s)
for f in sorted(rows, key=key):
    print('## %s' % f)
    for r in rows[f]: print('  ' + r)
    if not rows[f]: print('  (none)')
