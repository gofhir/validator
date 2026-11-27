import sys,re
def L(f):
    o={};k=None
    for l in open(f):
        if l.startswith('##'): k=l[3:].strip(); o[k]=[]
        else: o[k].append(l.strip())
    return o
hd,d=sys.argv[1:3]; h=L(hd+'/hl7.txt'); B=L(d+'/base.txt'); F=L(d+'/filt.txt')
drop=("Extension '",'Slicing cannot be evaluated','not allowed in context')
def N(l,s):
    r=[]
    for x in l:
        if x[0]!=s or any(y in x for y in drop): continue
        p=x.split('|')[0][2:].strip(); p=re.sub(r'\.ofType\(\w+\)','',p); r.append(p)
    return r
for k in sorted(h):
  e=N(h[k],'E'); f=N(F.get(k,[]),'E'); ew=N(h[k],'W'); fw=N(F.get(k,[]),'W'); b=N(B.get(k,[]),'E')
  if bool(e)!=bool(f) or set(e)!=set(f) or set(ew)!=set(fw): print(f"{k:16} hl7E={len(e)} hoy={len(b)} protoE={len(f)} hl7W={len(ew)} protoW={len(fw)} | soloHL7={sorted(set(e)-set(f))} soloP={sorted(set(f)-set(e))}")
