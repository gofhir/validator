import sys,json,collections,glob,os,re
W=sys.argv[1]; res=collections.Counter(); miss=[]
def norm(e): return re.sub(r'\.value\.ofType\((\w)(\w*)\)', lambda m:'.value'+m.group(1).upper()+m.group(2), e)
for g in sorted(os.listdir(W)):
    b=os.path.join(W,g,'same-base.jsonl'); h=os.path.join(W,g,'same-head.jsonl')
    if not os.path.exists(h): continue
    B=collections.Counter(l.strip() for l in open(b) if l.strip()); H=collections.Counter(l.strip() for l in open(h) if l.strip())
    new=[json.loads(l) for l in (H-B).elements()]
    if not new: continue
    hl7={}
    for f in glob.glob(os.path.join(W,g,'hl7-*.json')):
        d=json.load(open(f)); outs=[e['resource'] for e in d.get('entry',[])] if d.get('resourceType')=='Bundle' else [d]
        for o in outs:
            fn=next((x.get('valueString') for x in o.get('extension',[]) if x.get('url','').endswith('-file')),'')
            for i in o.get('issue',[]):
                mid=next((x.get('valueCode') for x in i.get('extension',[]) if 'message-id' in x.get('url','')),'')
                for e in i.get('expression') or []:
                    hl7.setdefault((os.path.basename(fn),norm(e)),set()).add((i['severity'],mid))
    for j in new:
        k=(os.path.basename(j['file']),j['expression'][0])
        got=hl7.get(k,set())
        want='error' if j['severity']=='error' else 'warning'
        ok=any(s==want and m.startswith('Terminology_TX_NoValid') for s,m in got)
        res[(g,j['messageId'],ok)]+=1
        if not ok: miss.append((g,k,sorted(got)))
for k,n in sorted(res.items()): print(n,k)
print('--- unmatched'); [print(m) for m in miss[:15]]
