import sys,re
def L(f):
    o={};k=None
    for l in open(f):
        if l.startswith('##'): k=l[3:].strip(); o[k]=[]
        else: o[k].append(l.strip())
    return o
d=sys.argv[1]; h=L(d+'/hl7.txt'); B=L(d+'/base.txt'); P=L(d+'/proto.txt'); F=L(d+'/filt.txt')
def norm(i):return i
drop=('Extension \'','Slicing cannot be evaluated','not allowed in context')
def errs(l,s='E'): return [x for x in l if x[0]==s and not any(y in x for y in drop)]
print(f"{'caso':18} hl7E  base protoE filtE | W: hl7 base proto filt")
for k in sorted(h):
    row=[len(errs(h[k])),len(errs(B.get(k,[]))),len(errs(P.get(k,[]))),len(errs(F.get(k,[])))]
    w=[len(errs(x.get(k,[]),'W')) for x in (h,B,P,F)]
    flag=''
    if bool(row[0])!=bool(row[1]): flag+=' BASE≠'
    if bool(row[0])!=bool(row[2]): flag+=' PROTO≠'
    if bool(row[0])!=bool(row[3]): flag+=' FILT≠'
    if row[2]!=row[3]or w[2]!=w[3]: flag+=' filtro-cambia'
    print(f"{k:18} {row[0]:4} {row[1]:5} {row[2]:5} {row[3]:5} | {w[0]} {w[1]} {w[2]} {w[3]} {flag}")
