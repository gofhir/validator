import json, os, sys
V = sys.argv[1] if len(sys.argv) > 1 else "4.0.1"
B = "http://example.org/c2/"
def sd(sid, typ, diff, base=None):
    return {"resourceType": "StructureDefinition", "id": sid, "url": B + sid, "name": "C" + sid.replace("-", "").title(),
         "status": "draft", "fhirVersion": V, "kind": "resource", "abstract": False, "type": typ,
         "baseDefinition": base or "http://hl7.org/fhir/StructureDefinition/" + typ, "derivation": "constraint",
         "differential": {"element": diff}}
def el(i, **k):
    return dict(id=i, path=".".join(x.split(":")[0] for x in i.split(".")), **k)
def inv(key, expr, sev="error"): return {"constraint": [{"key": key, "severity": sev, "human": key+" "+sev+" "+expr, "expression": expr}]}
for d in ("ig", "inst"): os.makedirs(d, exist_ok=True)
def put(d, n, o): json.dump(o, open(f"{d}/{n}.json", "w"), indent=1)
def P(sid, diff, base=None): put("ig", sid, sd(sid, "Patient", diff, B+base if base else None))
def I(name, prof, body):
    o = {"resourceType": "Patient", "id": name, "meta": {"profile": [B + prof]}}; o.update(body); put("inst", name, o)
F="Patient.name.family"
E1="$this.length() < 2"; E2="$this.startsWith('a')"
fam=lambda v:{"name":[{"family":v}]}
# severidad, dos direcciones, misma expresion, perfil derivado
P("s0e",[el(F,**inv("k",E1,"error"))]); P("s1w",[el(F,**inv("k",E1,"warning"))],"s0e"); I("sev-ew","s1w",fam("xy"))
P("s0w",[el(F,**inv("k",E1,"warning"))]); P("s1e",[el(F,**inv("k",E1,"error"))],"s0w"); I("sev-we","s1e",fam("xy"))
# otra expresion bajo la misma clave
P("x0",[el(F,**inv("k",E1))]); P("x1",[el(F,**inv("k",E2))],"x0")
I("exp-viol-old","x1",fam("ab")); I("exp-viol-new","x1",fam("b")); I("exp-viol-both","x1",fam("bc")); I("exp-ok","x1",fam("a"))
# cadena de tres
P("c1",[el(F,short="s")],"x0"); P("c2",[el(F,**inv("k",E2,"warning"))],"c1"); P("c3",[el(F,short="t")],"c2")
I("chain-viol-old","c3",fam("ab")); I("chain-viol-new","c3",fam("b"))
# clave del core redeclarada
P("core",[el("Patient.contact",**inv("pat-1","name.exists()","warning"))]); I("core-pat1","core",{"contact":[{"gender":"male"}]})
# slice en el mismo perfil, otra expresion
SYS={"discriminator":[{"type":"value","path":"system"}],"rules":"open"}
P("sl",[el("Patient.identifier",slicing=SYS),el("Patient.identifier.value",**inv("k",E1)),el("Patient.identifier:m",sliceName="m"),el("Patient.identifier:m.system",fixedUri="urn:m"),el("Patient.identifier:m.value",**inv("k",E2))])
I("sl-viol-base","sl",{"identifier":[{"system":"urn:m","value":"ab"}]}); I("sl-viol-slice","sl",{"identifier":[{"system":"urn:m","value":"b"}]}); I("sl-viol-both","sl",{"identifier":[{"system":"urn:m","value":"bc"}]})
