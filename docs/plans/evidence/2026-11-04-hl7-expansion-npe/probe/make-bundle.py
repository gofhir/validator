# Genera bundle.json: 500 copias de vs.json con id y url distintos.
import json, os, sys
d = os.path.dirname(os.path.abspath(__file__))
vs = json.load(open(os.path.join(d, 'vs.json')))
entries = []
for i in range(int(sys.argv[1]) if len(sys.argv) > 1 else 500):
    v = json.loads(json.dumps(vs))
    v['id'], v['url'] = f'vs{i}', f'http://acme.test/vs{i}'
    entries.append({'fullUrl': f'http://acme.test/ValueSet/vs{i}', 'resource': v})
json.dump({'resourceType': 'Bundle', 'type': 'collection', 'entry': entries}, open(os.path.join(d, 'bundle.json'), 'w'))
