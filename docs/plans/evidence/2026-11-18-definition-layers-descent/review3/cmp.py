import sys,re
def L(f):
    o={};k=None
    for l in open(f):
        if l.startswith('##'): k=l[3:].strip(); o[k]=[]
        elif l.strip(): o[k].append(l.strip())
    return o
CARD=re.compile(r'[Mm]inimum (cardinality|required)|[Mm]ax(imum cardinality| allowed)|matching slice is required')
def NP(p):
    p=re.sub(r'/\*.*?\*/','',p.strip()); p=re.sub(r'\.ofType\(\w+\)','',p)
    return re.sub(r'\.(value|author|onset|deceased|effective)[A-Z][A-Za-z]*',r'.\1',p)
def N(ls):
    r=set()
    for x in ls:
        s=x[0]; p,_,t=x[2:].partition('|')
        if CARD.search(t): continue
        r.add((s,NP(p)))
    return r
h=L(sys.argv[1]); g=L(sys.argv[2]); n=0
txt=lambda ls,s,p:[x[2:].split('|',1)[1].strip()[:95] for x in ls if x[0]==s and NP(x[2:].split('|')[0])==p]
for k in sorted(h):
    a,b=N(h[k]),N(g.get(k,[]))
    if a==b: continue
    n+=1; print(k, 'VEREDICTO' if any(x[0]=='E' for x in a)!=any(x[0]=='E' for x in b) else '')
    for s,p in sorted(a-b): print('   soloHL7',s,p,txt(h[k],s,p)[:1])
    for s,p in sorted(b-a): print('   soloGo ',s,p,txt(g[k],s,p)[:1])
print('difieren',n,'de',len(h))
