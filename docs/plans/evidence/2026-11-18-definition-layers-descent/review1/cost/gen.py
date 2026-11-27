import json,os,sys
exec(open('../x/gen.py').read().split("# --- perfil derivado")[0].replace('V = sys.argv[1] if len(sys.argv) > 1 else "4.0.1"','V="4.0.1"'))
R=B+"e-r"
T("e-r","Extension",[el("Extension.extension",slicing=URL,**inv("ax-r0","url.exists()")),el("Extension.extension.id",**inv("ax-r1","$this.length() < 9")),
  el("Extension.extension:r",sliceName="r",type=[{"code":"Extension","profile":[R]}]),el("Extension.url",fixedUri=R),el("Extension.value[x]",max="0")],extra=EXTC)
P("c-e",[el("Patient.extension",slicing=URL),el("Patient.extension:r",sliceName="r",type=[{"code":"Extension","profile":[R]}]),el("Patient.extension:r.id",short="i")])
TR=B+"t-r"; TS2=B+"t-s"
T("t-r","Identifier",[el("Identifier.assigner.identifier",type=[{"code":"Identifier","profile":[TS2]}]),el("Identifier.value",**inv("ax-t1","$this.length() < 9"))])
T("t-s","Identifier",[el("Identifier.assigner.identifier",type=[{"code":"Identifier","profile":[TR]}]),el("Identifier.value",**inv("ax-t2","$this.length() < 8"))])
P("c-t",[el("Patient.identifier",type=[{"code":"Identifier","profile":[TR]}]),el("Patient.identifier.value",**inv("ax-t3","$this.length() < 7")),el("Patient.identifier.assigner.identifier.value",**inv("ax-t4","$this.length() < 6"))])
def ext(d): return {"url":R,"id":"i"+str(d),**({"extension":[ext(d-1),ext(d-1)]} if d>1 and BR else {"extension":[ext(d-1)]} if d>1 else {})}
def idn(d):
    o={"value":"v"}
    if d>1:o["assigner"]={"identifier":idn(d-1)}
    return o
BR=False
for d in (4,8,16,32,64):
    I(f"e{d:02}","c-e",{"extension":[ext(d)]}); I(f"t{d:02}","c-t",{"identifier":[idn(d)]})
BR=True
for d in (4,8,12): I(f"b{d:02}","c-e",{"extension":[ext(d)]})
