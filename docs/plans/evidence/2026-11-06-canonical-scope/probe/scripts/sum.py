import json,sys,re
# usage: sum.py hl7|gofhir file
kind,path=sys.argv[1],sys.argv[2]
def clean(t): return re.sub(r'\s+',' ',t)[:150]
skip=('dom-6','performer','effective[x]','should have narrative')
if kind=='hl7':
  b=json.load(open(path))
  outs=[e['resource'] for e in b.get('entry',[])] if b.get('resourceType')=='Bundle' else [b]
  for o in outs:
    f=next((x.get('valueString') for x in o.get('extension',[]) if x.get('url','').endswith('-file')),'?').split('/')[-1]
    for i in o.get('issue',[]):
      t=i.get('details',{}).get('text','')
      if i['severity'] in ('error','fatal') or (i['severity']=='warning' and not any(s in t for s in skip)):
        print(f.replace('.json',''), i['severity'], (i.get('expression') or [''])[0], '|', clean(t))
else:
  s=open(path).read(); s=s if s.startswith('[') else s[s.index('\n[\n')+1:]
  for r in json.loads(s):
    f=r['resource'].split('/')[-1]
    for i in r.get('issues',[]):
      t=i.get('diagnostics','')
      if i['severity'] in ('error','fatal') or (i['severity']=='warning' and not any(x in t for x in skip)):
        print(f.replace('.json',''), i['severity'], (i.get('expression') or [''])[0], '|', clean(t))
