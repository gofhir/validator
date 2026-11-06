import json,sys
b=json.load(open(sys.argv[1]))
outs=[e['resource'] for e in b.get('entry',[])] if b.get('resourceType')=='Bundle' else [b]
for o in outs:
  f=next((x.get('valueString') for x in o.get('extension',[]) if x.get('url','').endswith('-file')),'?').split('/')[-1]
  for i in o.get('issue',[]):
    t=i.get('details',{}).get('text','')
    if i['severity'] in ('error','warning') and 'dom-6' not in t:
      mid=next((x.get('valueCode') for x in i.get('extension',[]) if 'message-id' in x.get('url','')),'')
      print(f, i['severity'], i['code'], mid, (i.get('expression') or [''])[0], t[:60])
