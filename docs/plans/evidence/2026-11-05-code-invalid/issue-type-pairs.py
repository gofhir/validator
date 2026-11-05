import json, os, re, sys, glob, collections
W = sys.argv[1]
pairs = collections.Counter(); ex = {}
def norm(l):
    l = re.sub(r'\.ofType\([^)]*\)', '', l or '')
    l = re.sub(r'\.where\([^)]*\)', '', l)
    return l
for g in sorted(os.listdir(W)):
    d = os.path.join(W, g); head = os.path.join(d, 'head.jsonl')
    hs = sorted(glob.glob(os.path.join(d, 'hl7-*.json')), key=os.path.getmtime)
    if not os.path.isfile(head) or not hs: continue
    go = collections.defaultdict(list)
    for line in open(head):
        r = json.loads(line)
        if r.get('severity') not in ('error', 'warning'): continue
        loc = norm((r.get('expression') or [''])[0])
        go[(r['file'], loc)].append(r)
    b = json.load(open(hs[-1]))
    outs = [e['resource'] for e in b.get('entry', [])] if b.get('resourceType') == 'Bundle' else [b]
    for o in outs:
        fn = next((x.get('valueString') for x in o.get('extension', []) if x.get('url', '').endswith('/operationoutcome-file')), '')
        rel = fn.split('/go-validator-wt-registry/')[-1] if '/go-validator-wt-registry/' in fn else fn
        for i in o.get('issue', []):
            if i['severity'] not in ('error', 'warning'): continue
            loc = norm((i.get('expression') or i.get('location') or [''])[0])
            mid = next((x.get('valueCode') for x in i.get('extension', []) if 'message-id' in x.get('url', '')), '')
            cands = [r for (f, l), rs in go.items() if l == loc and (f == rel or f.endswith('/' + os.path.basename(rel))) for r in rs if r['severity'] == i['severity']]
            for r in cands:
                if r['code'] != i['code']:
                    k = (r['messageId'], r['code'], mid, i['code'])
                    pairs[k] += 1; ex.setdefault(k, (g, r['file'], loc))
for k, n in pairs.most_common():
    print(n, ' | '.join(k), ' e.g.', ex[k][0], ex[k][1].split('/')[-1], ex[k][2])
