# Diferencia como multiconjunto entre same-base.jsonl y same-head.jsonl de cada grupo de hl7diff,
# sin la ubicación (línea y columna). Resume por (lado, messageId, severidad) y guarda las líneas.
import json,sys,os,collections
C=sys.argv[1]; out=open(sys.argv[2],'w')
summary=collections.Counter()
for g in sorted(os.listdir(C)):
    b=os.path.join(C,g,'same-base.jsonl'); h=os.path.join(C,g,'same-head.jsonl')
    if not os.path.exists(b): continue
    def load(p):
        c=collections.Counter()
        for l in open(p):
            if not l.strip(): continue
            d=json.loads(l); d.pop('location',None); c[json.dumps(d,sort_keys=True)]+=1
        return c
    cb,ch=load(b),load(h)
    for side,diff in (('-',cb-ch),('+',ch-cb)):
        for k,n in diff.items():
            d=json.loads(k)
            summary[(g,side,d['messageId'],d['severity'])]+=n
            for _ in range(n): out.write(f"{g} {side} {k}\n")
for k,n in sorted(summary.items()): print(n,*k)
