import json,sys,collections
n=sys.argv[1]
def L(m):
    t=open(f'{n}.{m}.out').read(); d=json.loads(t[t.index('\n[')+1:] if not t.startswith('[') else t)
    c=collections.Counter()
    for r in d:
        for i in r.get('issues') or []:
            e=i.get('expression') or ['']
            c[(r['resource'].split('/')[-1],i['severity'],e[0] if isinstance(e,list) else e,(i.get('diagnostics') or i.get('message') or '')[:140])]+=1
    return c
b,u=L('base'),L('v')
print(n,'issues base',sum(b.values()),'proto',sum(u.values()),'nuevos',sum((u-b).values()),'perdidos',sum((b-u).values()))
for k in list((u-b).elements())[:8]: print('  NUEVO',k)
for k in list((b-u).elements())[:8]: print('  PERDIDO',k)
