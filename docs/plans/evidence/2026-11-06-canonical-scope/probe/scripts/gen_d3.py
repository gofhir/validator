import sys; sys.path.insert(0,sys.argv[2]); from common import *
D=sys.argv[1]
pat_t=sd('pat-t','Patient',[
 {"id":"Patient.contained","path":"Patient.contained","slicing":{"discriminator":[{"type":"profile","path":"$this"}],"rules":"open"}},
 {"id":"Patient.contained:S","path":"Patient.contained","sliceName":"S","min":0,"max":"*","type":[{"code":"Observation","profile":[B+"obs-p"]}]},
 {"id":"Patient.gender","path":"Patient.gender","min":1},
 tp("Patient.link.other","pat-t")])
sds=[pat_t, sd('obs-p','Observation',[tp("Observation.subject","pat-t")]), sd('pat-any','Patient',[{"id":"Patient","path":"Patient"}]),
 sd('obs-za','Observation',[tp("Observation.subject","pat-t","pat-any"),tp("Observation.performer","pat-t")]),
 sd('obs-zb','Observation',[tp("Observation.subject","pat-t"),tp("Observation.performer","pat-t","pat-any")]),
 sd('obs-ra','Observation',[tp("Observation.hasMember","obs-za")]),
 sd('obs-rb','Observation',[tp("Observation.hasMember","obs-zb")])]
def p1(): return pat('p1',link=[{"other":ref("Patient/p2"),"type":"seealso"}])
def p2(sub="Patient/p1"): return pat('p2',contained=[obs('c',subject=ref(sub),focus=[ref("#")])],gender="male")
def case(v,first,sub="Patient/p1",order=0):
  z=obs('z',subject=ref("Patient/p1" if v=='a' else "Patient/p2"),performer=[ref("Patient/p2" if v=='a' else "Patient/p1")])
  r=obs('r1',meta=meta('obs-r'+v),hasMember=[ref("Observation/z")])
  rs=[r,z,p1(),p2(sub)]
  if order: rs=[rs[3],rs[2],rs[1],rs[0]]
  return bundle('x',rs)
insts=[('d3a1',case('a',1)),('d3a2',case('a',1,order=1)),('d3b1',case('b',1)),('d3b2',case('b',1,order=1)),
 ('d3c1',case('a',1,sub="Patient/p2"))]
out(D,sds,insts)
