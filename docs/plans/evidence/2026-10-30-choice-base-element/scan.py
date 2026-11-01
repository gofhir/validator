import json,sys,os,glob,collections
for pkg in sys.argv[1:]:
    notype=collections.Counter(); crs=collections.Counter(); ex=[]
    for f in glob.glob(os.path.join(pkg,'package','*.json')):
        try: sd=json.load(open(f))
        except Exception: continue
        if sd.get('resourceType')!='StructureDefinition': continue
        for e in (sd.get('snapshot') or {}).get('element',[]):
            cr=e.get('contentReference')
            if cr:
                crs['local' if cr.startswith('#') else 'url']+=1
                if not cr.startswith('#') and len(ex)<3: ex.append((sd['url'],e['id'],cr))
            elif '.' in e['path'] and not e.get('type'):
                notype['slice' if e.get('sliceName') else 'other']+=1
                if len(ex)<6: ex.append((sd['url'],e['id'],'NO TYPE'))
    print(os.path.basename(pkg),'notype',dict(notype),'contentReference',dict(crs),ex[:6])
