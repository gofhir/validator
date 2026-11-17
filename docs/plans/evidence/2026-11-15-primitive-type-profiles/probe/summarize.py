"""python3 -I summarize.py <ver>: una linea por instancia con (severity, message-id, location[, perfil]) de los issues que no son de narrativa/best practice."""
import json, sys, os
H = os.path.dirname(os.path.abspath(__file__))
ver = sys.argv[1]
d = json.load(open(os.path.join(H, 'hl7-%s-7.0.4.json' % ver)))
desc = dict(json.load(open(os.path.join(H, 'descriptions.json'))))
IGN = {'dom-6', 'All_observations_should_have_a_performer', 'All_observations_should_have_an_effectiveDateTime_or_an_effectivePeriod'}
out = {}
for e in d['entry']:
    r = e['resource']
    f = [x['valueString'] for x in r['extension'] if x['url'].endswith('operationoutcome-file')][0]
    n = os.path.basename(f)[:-5]
    items = []
    for i in r['issue']:
        mid = [x['valueCode'] for x in i.get('extension', []) if x['url'].endswith('message-id')]
        mid = mid[0] if mid else ''
        txt = i['details']['text']
        if mid in IGN or 'dom-6' in txt or 'Best Practice' in txt:
            continue
        items.append((i['severity'], mid, ','.join(i.get('expression', [])), txt))
    out[n] = items
if __name__ == '__main__':
    full = len(sys.argv) > 2
    for n in desc:
        print('## %s  -- %s' % (n, desc[n]))
        if not out.get(n):
            print('  (none)')
        for s, m, l, t in out.get(n, []):
            print('  %s %s @ %s%s' % (s, m, l, ('  :: ' + t) if full else ''))
