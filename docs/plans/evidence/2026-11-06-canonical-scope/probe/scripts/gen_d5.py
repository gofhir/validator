import sys,json; sys.path.insert(0,sys.argv[2]); from common import *
D=sys.argv[1]
VS={"resourceType":"ValueSet","id":"vs-refer","url":"http://example.org/fhir/ValueSet/vs-refer","name":"VsRefer","status":"active","compose":{"include":[{"system":"http://hl7.org/fhir/link-type","concept":[{"code":"refer"}]}]}}
def lslice(id,tgt,extra=()):
  return sd(id,'Patient',[
   {"id":"Patient.link","path":"Patient.link","slicing":{"discriminator":[{"type":"profile","path":"other.resolve()"}],"rules":"open"}},
   {"id":"Patient.link:L","path":"Patient.link","sliceName":"L","min":0,"max":"*"},
   dict(tp("Patient.link.other",tgt),id="Patient.link:L.other"),
   {"id":"Patient.link:L.type","path":"Patient.link.type","binding":{"strength":"required","valueSet":"http://example.org/fhir/ValueSet/vs-refer"}}]+list(extra))
sds=[
 lslice('pat-l1','pat-b'),
 sd('pat-b','Patient',[tp("Patient.link.other","pat-l1")]),
 # s3: R's generalPractitioner... a second link value reaching R again in the reference phase
 sd('pat-c','Patient',[tp("Patient.link.other","pat-l1")]),
 sd('pat-l2','Patient',[
   {"id":"Patient.link","path":"Patient.link","slicing":{"discriminator":[{"type":"profile","path":"other.resolve()"}],"rules":"open"}},
   {"id":"Patient.link:L","path":"Patient.link","sliceName":"L","min":0,"max":"*"},
   dict(tp("Patient.link.other","pat-b2"),id="Patient.link:L.other"),
   {"id":"Patient.link:L.type","path":"Patient.link.type","binding":{"strength":"required","valueSet":"http://example.org/fhir/ValueSet/vs-refer"}},
   tp("Patient.generalPractitioner","prac-c")]),
 sd('pat-b2','Patient',[tp("Patient.link.other","pat-l2"),{"id":"Patient.gender","path":"Patient.gender","min":1}]),
 sd('prac-c','Practitioner',[{"id":"Practitioner","path":"Practitioner","constraint":[{"key":"pc-1","severity":"error","human":"pc-1","expression":"name.exists()"}]}]),
]
def lk(t,ty="seealso"): return {"other":ref(t),"type":ty}
insts=[]
# s1: R(pat-l1).link[0] -> R2; R2.link -> R (cycle through pat-b's targetProfile)
R=pat('R',meta=meta('pat-l1'),link=[lk("Patient/R2")]); R2=pat('R2',link=[lk("Patient/R")])
insts+= [('d5s1a',bundle('x',[R,R2])),('d5s1b',bundle('x',[R2,R]))]
# s2: as s1, R2 does not reach back: control
insts+= [('d5s2',bundle('x',[pat('R',meta=meta('pat-l1'),link=[lk("Patient/R2")]),pat('R2')]))]
# s3: R(pat-l1).link[0] -> R2 (discriminator cycle), link[1] -> R3 (pat-c reached by... element: no tp) ; R also checked as a target in the reference phase via R3
R=pat('R',meta=meta('pat-l1'),link=[lk("Patient/R2"),lk("Patient/R3")]); R2=pat('R2',link=[lk("Patient/R")]); R3=pat('R3',meta=meta('pat-c'),link=[lk("Patient/R")])
insts+= [('d5s3a',bundle('x',[R,R2,R3])),('d5s3b',bundle('x',[R3,R2,R]))]
# s4: pat-l2: the discriminator's target R2 (pat-b2) needs gender; R2 links back to R; R.gp -> pr (prac-c) checked in the reference phase
R=pat('R',meta=meta('pat-l2'),link=[lk("Patient/R2")],generalPractitioner=[ref("Practitioner/pr")])
R2=pat('R2',gender="male",link=[lk("Patient/R")])
pr={"resourceType":"Practitioner","id":"pr","text":TXT,"name":[{"family":"A"}]}
insts+= [('d5s4a',bundle('x',[R,R2,pr])),('d5s4b',bundle('x',[pr,R2,R]))]
# s5: R2 has no gender: the discriminator's target fails
R2n=pat('R2',link=[lk("Patient/R")])
insts+= [('d5s5',bundle('x',[pat('R',meta=meta('pat-l2'),link=[lk("Patient/R2")],generalPractitioner=[ref("Practitioner/pr")]),R2n,pr]))]
out(D,sds,insts)
json.dump(VS,open(f"{D}/ig/ValueSet-vs-refer.json",'w'))
# s6: R with two profiles: pat-l2's reference phase reaches R (pat-l1) again through R2
more=[sd('obs-l1','Observation',[tp("Observation.subject","pat-l1")]), sd('pat-b3','Patient',[tp("Patient.link.other","pat-l1","pat-l2")])]
for s in more: json.dump(s,open(f"{D}/ig/StructureDefinition-{s['id']}.json",'w'))
R=pat('R',meta=meta('pat-l1','pat-l2'),gender="male",link=[lk("Patient/R2")],generalPractitioner=[ref("Practitioner/pr")])
R2=pat('R2',gender="male",link=[lk("Patient/R")])
for n,r in (('d5s6a',bundle('x',[R,R2,pr])),('d5s6b',bundle('x',[pr,R2,R]))): r['id']=n; json.dump(r,open(f"{D}/inst/{n}.json",'w'),indent=1)
# s7: an Observation before R checks R against pat-l1 as a target, before R's own validation
o=obs('o',meta=meta('obs-l1'),subject=ref("Patient/R"))
R=pat('R',meta=meta('pat-l1'),link=[lk("Patient/R2")]); R2=pat('R2',link=[lk("Patient/R")])
for n,r in (('d5s7a',bundle('x',[o,R,R2])),('d5s7b',bundle('x',[R,R2,o]))): r['id']=n; json.dump(r,open(f"{D}/inst/{n}.json",'w'),indent=1)
