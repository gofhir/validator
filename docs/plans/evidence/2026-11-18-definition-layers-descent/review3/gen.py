#!/usr/bin/env python3
"""Sondas adversariales del plan de descenso. Uso: python3 -I gen.py <fhirVersion> (desde x/<dir>)."""
import json, os, sys
V = sys.argv[1] if len(sys.argv) > 1 else "4.0.1"
B = "http://example.org/p5/"
def sd(sid, typ, diff, base=None, kind="resource", extra=None):
    o = {"resourceType": "StructureDefinition", "id": sid, "url": B + sid, "name": "Ax" + sid.replace("-", "").title(),
         "status": "draft", "fhirVersion": V, "kind": kind, "abstract": False, "type": typ,
         "baseDefinition": base or "http://hl7.org/fhir/StructureDefinition/" + typ, "derivation": "constraint",
         "differential": {"element": diff}}
    o.update(extra or {}); return o
def el(i, **k):
    p = ".".join(x.split(":")[0] for x in i.split("."))
    return dict(id=i, path=p, **k)
def inv(key, expr, sev="error"): return {"constraint": [{"key": key, "severity": sev, "human": key, "expression": expr}]}
for d in ("ig", "inst"): os.makedirs(d, exist_ok=True)
def put(d, n, o): json.dump(o, open(f"{d}/{n}.json", "w"), indent=1)
def P(sid, diff, **k): put("ig", sid, sd(sid, k.pop("typ", "Patient"), diff, **k))
def T(sid, typ, diff, **k): put("ig", sid, sd(sid, typ, diff, kind="complex-type", **k))
def I(name, prof, body, rt="Patient"):
    o = {"resourceType": rt, "id": name.replace("_", "-"), "meta": {"profile": [B + prof]}}; o.update(body); put("inst", name, o)
SYS = {"discriminator": [{"type": "value", "path": "system"}], "rules": "open"}
URL = {"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}
EXTC = {"context": [{"type": "element", "expression": "Element"}, {"type": "element", "expression": "Patient"}]}
M = {"system": "urn:mrn"}
def slc(*extra):  # identifier sliced, slice mrn by system
    return [el("Patient.identifier", slicing=SYS)], [el("Patient.identifier:mrn", sliceName="mrn"), el("Patient.identifier:mrn.system", fixedUri="urn:mrn")]
head, tail = slc()

K=lambda k,e,s="error":inv(k,e,s)
EX="$this.length() < 2"
V2={"value":"xy"}
PAT={"discriminator":[{"type":"pattern","path":"$this"}],"rules":"open"}
# ---- punto 5: clon del hermano o herencia del padre
for n,a,b in (("we","warning","error"),("ew","error","warning")):
    # s5a: la clave viene del perfil padre en el elemento dividido; el slice nuevo la redeclara
    P("a0-"+n,[el("Patient.identifier.value",**K("k5",EX,a))])
    P("a1-"+n,head+tail+[el("Patient.identifier:mrn.value",**K("k5",EX,b))],base=B+"a0-"+n)
    I("s5a-"+n,"a1-"+n,{"identifier":[dict(M,**V2)]})
    # s5b: slice del padre sin hijos; el derivado restringe hijo y slice
    P("b0-"+n,[el("Patient.identifier",slicing=PAT),el("Patient.identifier:mrn",sliceName="mrn",patternIdentifier=M)])
    P("b1-"+n,[el("Patient.identifier.value",**K("k5",EX,a)),el("Patient.identifier:mrn"),el("Patient.identifier:mrn.value",**K("k5",EX,b))],base=B+"b0-"+n)
    I("s5b-"+n,"b1-"+n,{"identifier":[dict(M,**V2)]})
    # s5c: reslicing en el mismo diferencial; a no redeclara, a/b sí
    RU={"discriminator":[{"type":"value","path":"use"}],"rules":"open"}
    P("c-"+n,[el("Patient.identifier",slicing=SYS),el("Patient.identifier.value",**K("k5",EX,a)),el("Patient.identifier:a",sliceName="a",slicing=RU),el("Patient.identifier:a.system",fixedUri="urn:a"),
              el("Patient.identifier:a/b",sliceName="a/b"),el("Patient.identifier:a/b.use",fixedCode="usual"),el("Patient.identifier:a/b.system",fixedUri="urn:a"),el("Patient.identifier:a/b.value",**K("k5",EX,b))])
    I("s5c-"+n,"c-"+n,{"identifier":[{"system":"urn:a","use":"usual","value":"xy"}]})
    # s5d: tres niveles: clave en el abuelo, slice en el padre, redeclara el hijo
    P("d0-"+n,[el("Patient.identifier.value",**K("k5",EX,a))]); P("d1-"+n,head+tail,base=B+"d0-"+n)
    P("d2-"+n,[el("Patient.identifier:mrn"),el("Patient.identifier:mrn.value",**K("k5",EX,b))],base=B+"d1-"+n)
    I("s5d-"+n,"d2-"+n,{"identifier":[dict(M,**V2)]})
    # s5e: mismo diferencial, el caso que el plan corrige
    P("e-"+n,head+[el("Patient.identifier.value",**K("k5",EX,a))]+tail+[el("Patient.identifier:mrn.value",**K("k5",EX,b))])
    I("s5e-"+n,"e-"+n,{"identifier":[dict(M,**V2)]})
# ---- punto 3: foco, clave y capas
for n,k in (("n","($this is FHIR.string).not()"),("p","$this is FHIR.string")):
    IDT=lambda *p:{"type":[{"code":"Identifier","profile":[B+x for x in p]}]}
    P("k1-"+n,[el("Patient.identifier.value",**K("gk",k))]); I("m1-"+n,"k1-"+n,{"identifier":[{"value":"x"}]})
    T("tk-"+n,"Identifier",[el("Identifier.value",**K("gk",k))]); T("tz-"+n,"Identifier",[el("Identifier.use",short="u")])
    P("k2-"+n,[el("Patient.identifier",**IDT("tk-"+n))]); I("m2-"+n,"k2-"+n,{"identifier":[{"value":"x"}]})
    P("k3-"+n,[el("Patient.identifier",**IDT("tk-"+n)),el("Patient.identifier.value",**K("gk",k))]); I("m3-"+n,"k3-"+n,{"identifier":[{"value":"x"}]})
    P("k4-"+n,[el("Patient.identifier",**IDT("tz-"+n,"tk-"+n)),el("Patient.identifier.value",**K("gk",k))]); I("m4-"+n,"k4-"+n,{"identifier":[{"value":"x"}]})
    P("k5-"+n,[el("Patient.identifier",**IDT("tk-"+n,"tz-"+n)),el("Patient.identifier.value",**K("gk",k))]); I("m5-"+n,"k5-"+n,{"identifier":[{"value":"x"}]})
# dos claves, misma expresión y severidad (HL7 memoriza por expresión y severidad)
T("tq","Identifier",[el("Identifier.value",**K("kq",EX))])
P("q1",[el("Patient.identifier",type=[{"code":"Identifier","profile":[B+"tq"]}]),el("Patient.identifier.value",**K("kp",EX))]); I("m6-keys","q1",{"identifier":[{"value":"xy"}]})
P("q2",head+[el("Patient.identifier.value",**K("kb",EX))]+tail+[el("Patient.identifier:mrn.value",**K("ks",EX))]); I("m7-keys","q2",{"identifier":[dict(M,**V2)]})
# sombra: el dueño declara k y el perfil de tipo la reemplaza; un slice hermano la evalúa sin perfil
E1="$this.length() < 2"; E2="$this.length() < 9"
T("ts","Period",[el("Period",**K("kw",E2))])
P("w1",[el("Patient.identifier",slicing=SYS),el("Patient.identifier.period",type=[{"code":"Period","profile":[B+"ts"]}],**K("kw","start.empty()")),el("Patient.identifier:mrn",sliceName="mrn"),el("Patient.identifier:mrn.system",fixedUri="urn:mrn")])
I("m8-shadow","w1",{"identifier":[dict(M,period={"start":"2001"})]})
P("w2",[el("Patient.identifier.period",type=[{"code":"Period","profile":[B+"ts"]}],**K("kw","start.empty()"))]); I("m9-shadow","w2",{"identifier":[{"period":{"start":"2001"}}]})
# ---- punto 4: perfiles de recurso sostenido en la cadena SliceOf
def R(sid,typ,diff,**k): put("ig",sid,sd(sid,typ,diff,**k))
R("qa","Patient",[el("Patient.birthDate",fixedDate="2000-01-01")]); R("qb","Patient",[el("Patient.gender",fixedCode="male")])
R("qr","Patient",[el("Patient",**K("rr","%resource.id = 'c1' and %rootResource.id = 'root'"))])
PT=lambda *p:[{"code":"Patient","profile":[B+x for x in p]}]
RT={"discriminator":[{"type":"type","path":"$this"}],"rules":"open"}
RE={"discriminator":[{"type":"value","path":"id"}],"rules":"open"}
P("h1",[el("Patient.contained",slicing=RE,type=PT("qa")),el("Patient.contained:a",sliceName="a",type=PT("qb")),el("Patient.contained:a.id",fixedId="c1")])
L={"link":[{"type":"seealso","other":{"reference":"#c1"}}]}
for n,x in (("ok",{"birthDate":"2000-01-01","gender":"male"}),("base",{"birthDate":"1999-01-01","gender":"male"}),("sl",{"birthDate":"2000-01-01","gender":"female"}),("both",{"birthDate":"1999-01-01","gender":"female"})):
    I("h1-"+n,"h1",dict(L,contained=[dict({"resourceType":"Patient","id":"c1"},**x)]))
P("h2",[el("Patient.contained",slicing=RE),el("Patient.contained:a",sliceName="a",type=PT("qr")),el("Patient.contained:a.id",fixedId="c1")])
for n,i in (("ok","root"),("bad","other")):
    o=dict(L,contained=[{"resourceType":"Patient","id":"c1"}]); I("h2-"+n,"h2",o); d=json.load(open("inst/h2-%s.json"%n)); d["id"]=i; put("inst","h2-"+n,d)
P("h3",[el("Bundle.entry",slicing={"discriminator":[{"type":"value","path":"fullUrl"}],"rules":"open"}),el("Bundle.entry.resource",type=PT("qa")),el("Bundle.entry:a",sliceName="a"),el("Bundle.entry:a.fullUrl",fixedUri="urn:uuid:11111111-1111-4111-8111-111111111111"),el("Bundle.entry:a.resource",type=PT("qb"))],typ="Bundle")
for n,x in (("ok",{"birthDate":"2000-01-01","gender":"male"}),("base",{"birthDate":"1999-01-01","gender":"male"}),("sl",{"birthDate":"2000-01-01","gender":"female"})):
    I("h3-"+n,"h3",{"type":"collection","entry":[{"fullUrl":"urn:uuid:11111111-1111-4111-8111-111111111111","resource":dict({"resourceType":"Patient","id":"p1"},**x)}]},rt="Bundle")
P("h4",[el("Patient.contained",slicing=RE,type=PT("qa","qb")),el("Patient.contained:a",sliceName="a"),el("Patient.contained:a.id",fixedId="c1")])
I("h4-one","h4",dict(L,contained=[{"resourceType":"Patient","id":"c1","birthDate":"2000-01-01","gender":"female"}]))
