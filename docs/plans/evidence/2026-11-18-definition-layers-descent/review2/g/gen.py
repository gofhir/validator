import json, os, sys
exec(open(sys.argv[1]).read().split("# --- perfil derivado")[0].replace('V = sys.argv[1] if len(sys.argv) > 1 else "4.0.1"','V="4.0.1"'))
IDT=lambda p:{"type":[{"code":"Identifier","profile":[B+p]}]}
# g1: el path ignorado cambia el tipo del foco de la invariante
K="($this is FHIR.string).not()"
T("g1-t","Identifier",[el("Identifier.value",**inv("gk",K))])
P("g1",[el("Patient.identifier",**IDT("g1-t")),el("Patient.identifier.value",**inv("gk",K))])
I("bad-g1","g1",{"identifier":[{"value":"x"}]})
# g3: forma D con recurso sostenido: el perfil derivado pide un perfil al recurso de la entrada
T2=lambda sid,typ,diff,**k: put("ig",sid,sd(sid,typ,diff,**k))
T2("q-pat","Patient",[el("Patient.birthDate",fixedDate="2000-01-01")])
EN={"discriminator":[{"type":"value","path":"fullUrl"}],"rules":"open"}
P("b0",[el("Bundle.entry",slicing=EN),el("Bundle.entry:a",sliceName="a"),el("Bundle.entry:a.id",short="i"),el("Bundle.entry:a.fullUrl",fixedUri="urn:uuid:11111111-1111-4111-8111-111111111111")],typ="Bundle")
P("b1",[el("Bundle.entry.resource",type=[{"code":"Patient","profile":[B+"q-pat"]}])],base=B+"b0",typ="Bundle")
BD=lambda d:{"type":"collection","entry":[{"fullUrl":"urn:uuid:11111111-1111-4111-8111-111111111111","resource":{"resourceType":"Patient","id":"p1","birthDate":d}}]}
I("bad-g3","b1",BD("1999-01-01"),rt="Bundle"); I("ok-g3","b1",BD("2000-01-01"),rt="Bundle")
# g3b: contained
P("c0",[el("Patient.contained",slicing={"discriminator":[{"type":"value","path":"id"}],"rules":"open"}),el("Patient.contained:a",sliceName="a"),el("Patient.contained:a.id",fixedId="c1")])
P("c1",[el("Patient.contained",type=[{"code":"Patient","profile":[B+"q-pat"]}])],base=B+"c0")
I("bad-g3c","c1",{"contained":[{"resourceType":"Patient","id":"c1","birthDate":"1999-01-01"}],"link":[{"type":"seealso","other":{"reference":"#c1"}}]})
# g4: extensiones con source distinto y sombra por perfil de tipo (primitivo)
SRC=lambda s:{"constraint":[{"key":"gs","severity":"error","human":"gs","expression":"$this.length() < 1","source":s}]}
put("ig","g4-s",sd("g4-s","string",[{"id":"string","path":"string","constraint":[{"key":"gs","severity":"error","human":"gs root","expression":"true","source":"urn:a"}]}],kind="primitive-type"))
TY={"type":[{"code":"string","profile":[B+"g4-s"]}]}
T("g4-t","Identifier",[el("Identifier.value",**TY,**SRC("urn:a"))])
P("g4",[el("Patient.identifier",**IDT("g4-t")),el("Patient.identifier.value",**TY,**SRC("urn:b"))])
I("bad-g4","g4",{"identifier":[{"value":"x"}]})
