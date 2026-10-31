import json,sys,os,glob,collections
sds={}
for pkg in sys.argv[1:]:
    for f in glob.glob(os.path.join(pkg,'package','*.json')):
        try: sd=json.load(open(f))
        except Exception: continue
        if sd.get('resourceType')=='StructureDefinition': sds.setdefault(sd['url'],[]).append((os.path.basename(pkg),sd))
def chain(sd):
    out=set(); cur=sd
    while cur and cur['url'] not in out:
        out.add(cur['url']); b=cur.get('baseDefinition')
        cur=sds[b][0][1] if b in sds else None
    return out
c=collections.Counter(); ex=[]
for url,lst in sds.items():
    for pkg,sd in lst:
        ch=chain(sd)
        for e in (sd.get('snapshot') or {}).get('element',[]):
            cr=e.get('contentReference','')
            if cr and not cr.startswith('#'):
                tgt=cr.split('#')[0]
                k='in-chain' if tgt in ch else ('type-url' if tgt=='http://hl7.org/fhir/StructureDefinition/'+sd['type'] or tgt==sd['type'] else 'outside')
                c[k]+=1
                if k=='outside' and len(ex)<10: ex.append((pkg,sd['url'],sd.get('kind'),e['id'],cr))
print(dict(c)); [print(x) for x in ex]
