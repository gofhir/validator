import json,sys,os,copy
D=sys.argv[1]; os.makedirs(D+'/ig',exist_ok=True); os.makedirs(D+'/inst',exist_ok=True)
B='http://example.org/fhir/StructureDefinition/'
def sd(id,typ,elems):
  return {"resourceType":"StructureDefinition","id":id,"url":B+id,"name":id.replace('-','').capitalize(),"status":"active","fhirVersion":"4.0.1","kind":"resource","abstract":False,"type":typ,"baseDefinition":"http://hl7.org/fhir/StructureDefinition/"+typ,"derivation":"constraint","differential":{"element":elems}}
def inv(typ,key,expr):
  return {"id":typ,"path":typ,"constraint":[{"key":key,"severity":"error","human":key,"expression":expr,"source":B}]}
sds=[
 sd('obs-pf-id','Observation',[inv('Observation','pfid-1',"performer.resolve().ofType(Organization).name = id")]),
 sd('obs-sub-hash','Observation',[inv('Observation','subh-1',"subject.resolve().ofType(Patient).name.family = id")]),
 sd('bun-sig-id','Bundle',[inv('Bundle','bsig-1',"signature.who.resolve().ofType(Patient).name.family = id"),]),
 sd('bun-sig-org','Bundle',[inv('Bundle','bsig-2',"signature.onBehalfOf.resolve().ofType(Organization).name = id")]),
 sd('org-rel','Organization',[inv('Organization','orel-1',"partOf.resolve().name = 'top'")]),
 sd('bun-tp','Bundle',[{"id":"Bundle.signature.onBehalfOf","path":"Bundle.signature.onBehalfOf","type":[{"code":"Reference","targetProfile":[B+"org-rel"]}]}]),
]
for s in sds:
  for e in s['differential']['element']:
    for c in e.get('constraint',[]): c['source']=B+s['id']
  json.dump(s,open(f"{D}/ig/StructureDefinition-{s['id']}.json",'w'))
TXT={"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}
def meta(*p): return {"profile":[B+x for x in p]}
def obs_pf(id): # Observation whose "#o" must be its own contained o named id
  return {"resourceType":"Observation","id":id,"meta":meta('obs-pf-id'),"text":TXT,"contained":[{"resourceType":"Organization","id":"o","name":id}],"status":"final","code":{"text":"x"},"performer":[{"reference":"#o"}]}
def pat_hash(fam): # Patient whose contained Observation fam says "#" is the Patient with family fam
  return {"resourceType":"Patient","id":fam,"text":TXT,"contained":[{"resourceType":"Observation","id":fam,"meta":meta('obs-sub-hash'),"status":"final","code":{"text":"x"},"subject":{"reference":"#"}}],"name":[{"family":fam}]}
SIG={"type":[{"system":"urn:iso-astm:E1762-95:2013","code":"1.2.840.10065.1.12.1.1"}],"when":"2026-01-01T00:00:00Z","data":"AA=="}
def entry(r): return {"fullUrl":f"http://ex.org/fhir/{r['resourceType']}/{r['id']}","resource":r}
def W(n,r): r['id']=n if r['resourceType'] in ('Bundle','Parameters') else r['id']; json.dump(r,open(f"{D}/inst/{n}.json",'w'),indent=1)
# (a) contained Bundle: element (#, #o from b) and entries (#o from x1, # from cq's contained)
def d1a(rev):
  b={"resourceType":"Bundle","id":"bb","meta":meta('bun-sig-id','bun-sig-org'),"type":"collection",
     "entry":[entry(obs_pf('x1')),entry(pat_hash('cq'))],"signature":dict(SIG,who={"reference":"#"},onBehalfOf={"reference":"#o"})}
  o={"resourceType":"Organization","id":"o","name":"bb"}
  if rev: b['entry'].reverse()
  return {"resourceType":"Patient","id":"P","text":TXT,"contained":[b,o] if rev else [o,b],"name":[{"family":"bb"}],"generalPractitioner":[{"reference":"#o"}]}
r=d1a(False); r['id']='d1a1'; json.dump(r,open(D+'/inst/d1a1.json','w'),indent=1)
r=d1a(True); r['id']='d1a2'; json.dump(r,open(D+'/inst/d1a2.json','w'),indent=1)
# (a') D1 itself: "#o" from the contained Bundle reaches o; o's targetProfile resolves a relative reference
def d1a3(rev):
  b={"resourceType":"Bundle","id":"b","meta":meta('bun-tp'),"type":"collection","entry":[entry({"resourceType":"Organization","id":"o2","name":"top"})],
     "signature":dict(SIG,who={"reference":"#"},onBehalfOf={"reference":"#o"})}
  o={"resourceType":"Organization","id":"o","name":"x","partOf":{"reference":"Organization/o2"}}
  return {"resourceType":"Patient","text":TXT,"contained":[b,o] if rev else [o,b],"generalPractitioner":[{"reference":"#o"}]}
r=d1a3(False); r['id']='d1a3'; json.dump(r,open(D+'/inst/d1a3.json','w'),indent=1)
r=d1a3(True); r['id']='d1a4'; json.dump(r,open(D+'/inst/d1a4.json','w'),indent=1)
# (b) Parameters.parameter.resource
def d1b(rev):
  rs=[obs_pf('x1'),obs_pf('x2'),pat_hash('cq'),pat_hash('cq2')]
  if rev: rs.reverse()
  return {"resourceType":"Parameters","parameter":[{"name":f"p{i}","resource":x} for i,x in enumerate(rs)]}
r=d1b(False); r['id']='d1b1'; json.dump(r,open(D+'/inst/d1b1.json','w'),indent=1)
r=d1b(True); r['id']='d1b2'; json.dump(r,open(D+'/inst/d1b2.json','w'),indent=1)
# (c) Bundle entries
def d1c(rev):
  rs=[obs_pf('x1'),obs_pf('x2'),pat_hash('cq'),pat_hash('cq2')]
  if rev: rs.reverse()
  return {"resourceType":"Bundle","type":"collection","entry":[entry(x) for x in rs]}
r=d1c(False); r['id']='d1c1'; json.dump(r,open(D+'/inst/d1c1.json','w'),indent=1)
r=d1c(True); r['id']='d1c2'; json.dump(r,open(D+'/inst/d1c2.json','w'),indent=1)
# (a'') D1 inside a root Bundle: o's relative partOf resolves where P is (the root), not in b
def pin(rev,inner):
  b={"resourceType":"Bundle","id":"b","meta":meta('bun-tp'),"type":"collection","entry":[entry({"resourceType":"Organization","id":"o2","name":inner})],
     "signature":dict(SIG,who={"reference":"#"},onBehalfOf={"reference":"#o"})}
  o={"resourceType":"Organization","id":"o","name":"x","partOf":{"reference":"Organization/o2"}}
  return {"resourceType":"Patient","id":"P","text":TXT,"contained":[b,o] if rev else [o,b],"generalPractitioner":[{"reference":"#o"}]}
def rootb(rev,inner,outer,n):
  return {"resourceType":"Bundle","id":n,"type":"collection","entry":[entry(pin(rev,inner)),entry({"resourceType":"Organization","id":"o2","name":outer})]}
json.dump(rootb(False,'nested','top','d1a5'),open(D+'/inst/d1a5.json','w'),indent=1)
json.dump(rootb(True,'nested','top','d1a6'),open(D+'/inst/d1a6.json','w'),indent=1)
json.dump(rootb(False,'top','nested','d1a7'),open(D+'/inst/d1a7.json','w'),indent=1)
json.dump(rootb(True,'top','nested','d1a8'),open(D+'/inst/d1a8.json','w'),indent=1)
# absolute partOf: d1a9 = d1a3, d1a10 = d1a5, d1a11 = d1a7
for src,dst in (('d1a3','d1a9'),('d1a5','d1a10'),('d1a7','d1a11')):
  t=open(f"{D}/inst/{src}.json").read().replace('"Organization/o2"','"http://ex.org/fhir/Organization/o2"').replace(f'"id": "{src}"',f'"id": "{dst}"')
  open(f"{D}/inst/{dst}.json",'w').write(t)
# a speculative run: P slices contained by profile; slice B (bun-tp, min 1) holds b only if o's partOf resolves where P is
s=sd('pat-cs','Patient',[
 {"id":"Patient.contained","path":"Patient.contained","slicing":{"discriminator":[{"type":"profile","path":"$this"}],"rules":"open"}},
 {"id":"Patient.contained:B","path":"Patient.contained","sliceName":"B","min":1,"max":"1","type":[{"code":"Bundle","profile":[B+"bun-tp"]}]}])
json.dump(s,open(f"{D}/ig/StructureDefinition-pat-cs.json",'w'))
for n,inner,outer in (('d1a12','nested','top'),('d1a13','top','nested')):
  r=rootb(False,inner,outer,n)
  t=json.dumps(r,indent=1).replace('"Organization/o2"','"http://ex.org/fhir/Organization/o2"')
  r=json.loads(t); pt=r['entry'][0]['resource']; pt['meta']=meta('pat-cs'); del pt['contained'][1]['meta']
  json.dump(r,open(f"{D}/inst/{n}.json",'w'),indent=1)
