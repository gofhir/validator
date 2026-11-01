import json,sys,os,glob,collections
c=collections.Counter(); ex=[]
for pkg in sys.argv[1:]:
    for f in glob.glob(os.path.join(pkg,'package','*.json')):
        try: sd=json.load(open(f))
        except Exception: continue
        if sd.get('resourceType')!='StructureDefinition' or sd.get('derivation')!='specialization': continue
        els=(sd.get('snapshot') or sd.get('differential') or {}).get('element',[])
        ids={e.get('id',e['path']) for e in els}
        for e in els:
            i=e.get('id',e['path'])
            if '.' not in i: continue
            if not any(x.startswith(i+'.') for x in ids): continue
            codes={t['code'] for t in e.get('type',[])}
            k='backbone' if codes and codes<= {'BackboneElement','Element','Base','http://hl7.org/fhirpath/System.String'} else ('other:'+','.join(sorted(codes)))
            c[k if k=='backbone' else 'other']+=1
            if k!='backbone' and len(ex)<15: ex.append((os.path.basename(pkg),sd['url'],sd.get('kind'),i,k))
print(dict(c)); [print(x) for x in ex]
