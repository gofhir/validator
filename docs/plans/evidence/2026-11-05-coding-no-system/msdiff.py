import sys,json,collections,glob,os
W=sys.argv[1]; tot=collections.Counter(); other=collections.Counter(); ex={}
for g in sorted(os.listdir(W)):
    b=os.path.join(W,g,'same-base.jsonl'); h=os.path.join(W,g,'same-head.jsonl')
    if not (os.path.exists(b) and os.path.exists(h)): continue
    def load(p):
        c=collections.Counter()
        for l in open(p):
            l=l.strip()
            if l: c[l]+=1
        return c
    B,H=load(b),load(h)
    for sign,d in (('+',H-B),('-',B-H)):
        for l,n in d.items():
            j=json.loads(l); k=(sign,j.get('severity'),j.get('messageId'))
            tot[k]+=n
            if j.get('messageId') not in ('BINDING_REQUIRED_NO_CODING','BINDING_EXTENSIBLE_NO_CODING'):
                other[(g,)+k]+=n; ex.setdefault((g,)+k,l[:300])
for k,n in sorted(tot.items()): print(n,k)
print('--- other')
for k,n in other.items(): print(n,k); print('   ',ex[k])
