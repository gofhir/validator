import json,os,sys,re,collections
SDS=[]
for root in sys.argv[1:]:
  for dp,_,fs in os.walk(root):
    for f in fs:
      if f.startswith('StructureDefinition') and f.endswith('.json'):
        try: sd=json.load(open(os.path.join(dp,f)))
        except: continue
        SDS.append((os.path.relpath(dp,root).split(os.sep)[0],sd))
BYURL={sd.get('url'):sd for _,sd in SDS}
def fp(e):
  for k,v in e.items():
    if k.startswith(('fixed','pattern')): return v
  return None
def dig(val,segs):
  if not segs: return val is not None
  if isinstance(val,list): return any(dig(x,segs) for x in val)
  if isinstance(val,dict) and segs[0] in val: return dig(val[segs[0]],segs[1:])
  return False
def resolvable(byid,base,path):
  segs=path.split('.')
  for n in range(len(segs),-1,-1):
    eid=base+('.'+'.'.join(segs[:n]) if n else '')
    e=byid.get(eid)
    if e is not None and fp(e) is not None and dig(fp(e),segs[n:]): return 'inline'
  return None
res=collections.Counter(); bad=[]
for pkg,sd in SDS:
  snap=sd.get('snapshot',{}).get('element',[]); byid={e['id']:e for e in snap}
  for e in snap:
    sl=e.get('slicing')
    if not sl: continue
    slices=[x for x in snap if x.get('sliceName') and x['id'].rsplit(':',1)[0]==e['id']]
    for d in sl.get('discriminator',[]):
      if d.get('type') not in('value','pattern'): continue
      p=d.get('path','')
      if not re.fullmatch(r'[\w.\[\]$]+',p): res['function-path']+=len(slices); continue
      for s in slices:
        how=None
        if p=='$this':
          how='inline' if fp(s) is not None or any(k.startswith(s['id']+'.') and fp(v) is not None for k,v in byid.items()) else None
        else:
          how=resolvable(byid,s['id'],p)
        if not how:
          for t in s.get('type',[]):
            for pr in t.get('profile',[]):
              psd=BYURL.get(pr.split('|')[0])
              if psd:
                pb={x['id']:x for x in psd.get('snapshot',{}).get('element',[])}
                if (p=='$this' and any(fp(v) is not None for v in pb.values())) or (p!='$this' and resolvable(pb,psd['type'],p)): how='type.profile'
              elif how is None: how='profile-unresolvable'
        res[how or 'NONE']+=1
        if not how: bad.append(f"{pkg} :: {sd['id']} :: {s['id']} {d['type']}:{p}")
print(res); print(len(bad)); print('\n'.join(bad[:15]))
