import json, sys, os, re, collections
W = sys.argv[1]
def short(msg):
    m = re.match(r'Constraint failed: (txt-\d|ele-1)', msg or '')
    if m: return m.group(1)
    return None
rows = collections.OrderedDict()
for g, v in [('narrative-ucum-probes', 'R4'), ('narrative-ucum-probes-r4b', 'R4B'), ('narrative-ucum-probes-r5', 'R5')]:
    d = os.path.join(W, g)
    hl7 = max((os.path.join(d, f) for f in os.listdir(d) if f.startswith('hl7-') and f.endswith('.json')), key=os.path.getmtime)
    for e in json.load(open(hl7))['entry']:
        oo = e['resource']; f = [x['valueString'] for x in oo.get('extension', []) if x['url'].endswith('operationoutcome-file')][0]
        if '/nx_' not in f: continue
        k = os.path.basename(f)[3:-5]
        out = []
        for i in oo.get('issue', []):
            if i['severity'] not in ('error', 'fatal'): continue
            mid = [x['valueCode'] for x in i.get('extension', []) if x['url'].endswith('operationoutcome-message-id')]
            txt = i.get('details', {}).get('text', '') or i.get('diagnostics', '')
            out.append(short(txt) or (mid[0] if mid else txt[:60]))
        rows.setdefault(k, {})[v + ' HL7'] = sorted(set(out))
    for line in open(os.path.join(d, 'head.jsonl')):
        r = json.loads(line)
        if '/nx_' not in r['file'] or r.get('severity') not in ('error', 'fatal'): continue
        k = os.path.basename(r['file'])[3:-5]
        rows.setdefault(k, {}).setdefault(v + ' ours', set()).add(short(r.get('diagnostics')) or r.get('messageId'))
for k in sorted(rows):
    r = rows[k]
    h = [tuple(r.get(v + ' HL7', [])) for v in ('R4', 'R4B', 'R5')]
    o = [tuple(sorted(r.get(v + ' ours', []))) for v in ('R4', 'R4B', 'R5')]
    hs = ' '.join(h[0]) if len(set(h)) == 1 else ' | '.join(' '.join(x) for x in h)
    os_ = ' '.join(o[0]) if len(set(o)) == 1 else ' | '.join(' '.join(x) for x in o)
    print(f'{k:16} HL7: {hs or "-":60} ours: {os_ or "-"}')
