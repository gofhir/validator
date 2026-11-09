import json,os
B='http://example.org/fhir/StructureDefinition/'
TXT={"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}
def sd(id,typ,elems):
  for e in elems:
    for c in e.get('constraint',[]): c.setdefault('source',B+id)
  return {"resourceType":"StructureDefinition","id":id,"url":B+id,"name":id.replace('-','').capitalize(),"status":"active","fhirVersion":"4.0.1","kind":"resource","abstract":False,"type":typ,"baseDefinition":"http://hl7.org/fhir/StructureDefinition/"+typ,"derivation":"constraint","differential":{"element":elems}}
def tp(path,*profs): return {"id":path,"path":path,"type":[{"code":"Reference","targetProfile":[B+p for p in profs]}]}
def meta(*p): return {"profile":[B+x for x in p]}
def entry(r): return {"fullUrl":f"http://ex.org/fhir/{r['resourceType']}/{r['id']}","resource":r}
def ref(r): return {"reference":r}
def obs(id,**kw):
  o={"resourceType":"Observation","id":id,"text":TXT,"status":"final","code":{"text":"x"}}; o.update(kw); return o
def pat(id,**kw):
  p={"resourceType":"Patient","id":id,"text":TXT}; p.update(kw); return p
def bundle(id,rs): return {"resourceType":"Bundle","id":id,"type":"collection","entry":[entry(r) for r in rs]}
def out(D,sds,insts):
  os.makedirs(D+'/ig',exist_ok=True); os.makedirs(D+'/inst',exist_ok=True)
  for s in sds: json.dump(s,open(f"{D}/ig/StructureDefinition-{s['id']}.json",'w'))
  for n,r in insts: r['id']=n; json.dump(r,open(f"{D}/inst/{n}.json",'w'),indent=1)
