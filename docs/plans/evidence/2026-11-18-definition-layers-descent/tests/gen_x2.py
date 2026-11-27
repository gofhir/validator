#!/usr/bin/env python3
"""Copia de ../review2/gen-ordered.py con las sondas corregidas el 25-11-2026 (plan §9): n-nest y q-cr con el diferencial
en el orden del snapshot base, p-base con un fixed del tipo del elemento, y controles ctl-d-type* en un solo perfil.
Sondas adversariales del plan de descenso. Uso: python3 -I gen.py <fhirVersion> (desde x/<dir>)."""
import json, os, sys
V = sys.argv[1] if len(sys.argv) > 1 else "4.0.1"
B = "http://example.org/ax/"
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
# --- perfil derivado: el hijo del elemento dividido se restringe en un perfil posterior (el slice no redeclara)
P("d0", head + [tail[0], el("Patient.identifier:mrn.use", short="u"), tail[1]])
for c, ch, b, val in [("min", "period", {"min": 1}, {"value": "v"}), ("fix", "value", {"fixedString": "BASE"}, {"value": "x"}),
                      ("inv", "value", inv("ax-d", "$this.length() < 2"), {"value": "xy"}), ("max", "assigner", {"max": "0"}, {"assigner": {"display": "x"}})]:
    P("d-" + c, [el("Patient.identifier." + ch, **b)], base=B + "d0"); I("bad-d-" + c, "d-" + c, {"identifier": [dict(M, **val)]})
# --- cardinalidad adversarial
T("tp-sd", "Identifier", [el("Identifier.period.start", min=1)])
P("c-tdeep", [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + "tp-sd"]}]), el("Patient.identifier.system", min=1)])
I("bad-c-tdeep", "c-tdeep", {"identifier": [{"system": "urn:x", "period": {"end": "2001"}}]})
P("c-tdeep2", [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + "tp-sd"]}]), el("Patient.identifier.period.end", short="e")])
I("bad-c-tdeep2", "c-tdeep2", {"identifier": [{"system": "urn:x", "period": {"end": "2001"}}]})
P("c-pmin", [el("Patient.birthDate.extension", min=1)])
I("bad-c-pmin", "c-pmin", {"birthDate": "2000-01-01", "_birthDate": {"id": "q"}})
E = B + "e0"
T("e0", "Extension", [el("Extension.extension", max="0"), el("Extension.url", fixedUri=E), el("Extension.value[x]", type=[{"code": "string"}])], extra=EXTC)
E1 = B + "e1"
T("e1", "Extension", [el("Extension.extension", max="0"), el("Extension.url", fixedUri=E1), el("Extension.value[x]", min=1, type=[{"code": "string"}], fixedString="F", **inv("ax-e", "$this.length() < 2"))], extra=EXTC)
PS = el("Patient.birthDate.extension", slicing=URL)
P("c-pdesc", [PS, el("Patient.birthDate.extension:e", sliceName="e"), el("Patient.birthDate.extension:e.url", fixedUri=E), el("Patient.birthDate.extension:e.value[x]", min=1)])
I("bad-c-pdesc", "c-pdesc", {"birthDate": "2000-01-01", "_birthDate": {"extension": [{"url": E, "id": "z"}]}})
# --- definición propia con el slice desplegado y sin perfil de tipo
RS = el("Patient.extension", slicing=URL)
P("o-own", [RS, el("Patient.extension:e", sliceName="e"), el("Patient.extension:e.id", short="i"), el("Patient.extension:e.url", fixedUri=E1)])
I("bad-o-min", "o-own", {"extension": [{"url": E1, "id": "z"}]})
I("bad-o-fix", "o-own", {"extension": [{"url": E1, "valueString": "G"}]})
I("bad-o-inv", "o-own", {"extension": [{"url": E1, "valueString": "FF"}]})
I("ok-o", "o-own", {"extension": [{"url": E1, "valueString": "F"}]})
# --- filtro: lo que el perfil de tipo aporta sin declararlo en el hijo
T("pp-fix", "Period", [el("Period.start", fixedDateTime="2000")]); T("pp-inv", "Period", [el("Period", **inv("ax-pp", "start.empty()"))])
for c in ("fix", "inv"):
    T("tp-low-" + c, "Identifier", [el("Identifier.period", type=[{"code": "Period", "profile": [B + "pp-" + c]}])])
    P("f-low-" + c, [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + "tp-low-" + c]}]), el("Patient.identifier.system", min=1)])
    I("bad-f-low-" + c, "f-low-" + c, {"identifier": [{"system": "urn:x", "period": {"start": "2001"}}]})
T("tp-sl", "Identifier", [el("Identifier.extension", slicing=URL), el("Identifier.extension:x", sliceName="x"), el("Identifier.extension:x.url", fixedUri=E),
                          el("Identifier.extension:x.value[x]", type=[{"code": "string"}], fixedString="F")])
P("f-slice", [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + "tp-sl"]}]), el("Patient.identifier.system", min=1)])
I("bad-f-slice", "f-slice", {"identifier": [{"system": "urn:x", "extension": [{"url": E, "valueString": "G"}]}]})
OBS = {"status": "final", "code": {"text": "x"}}
T("tp-ann", "Annotation", [el("Annotation.author[x]", type=[{"code": "string"}])])
P("f-type", [el("Observation.note", type=[{"code": "Annotation", "profile": [B + "tp-ann"]}]), el("Observation.note.time", short="t")], typ="Observation")
I("bad-f-type", "f-type", dict(OBS, note=[{"text": "t", "authorReference": {"display": "d"}}]), rt="Observation")
CS = el("Observation.component", slicing={"discriminator": [{"type": "value", "path": "code.text"}], "rules": "open"})
P("g0", [CS, el("Observation.component:a", sliceName="a"), el("Observation.component:a.id", short="i"), el("Observation.component:a.code.text", fixedString="A")], typ="Observation")
P("d-type", [el("Observation.component.value[x]", type=[{"code": "string"}])], base=B + "g0", typ="Observation")
I("bad-d-type", "d-type", dict(OBS, component=[{"code": {"text": "A"}, "valueInteger": 1}]), rt="Observation")
# --- severidad distinta entre capas, misma clave y expresión
EX = "$this.length() < 2"
for n, a, b in (("we", "warning", "error"), ("ew", "error", "warning")):
    P("v-s-" + n, head + [el("Patient.identifier.value", **inv("ax-v", EX, a))] + tail + [el("Patient.identifier:mrn.value", **inv("ax-v", EX, b))])
    I("bad-v-s-" + n, "v-s-" + n, {"identifier": [dict(M, value="xy")]})
    T("tp-v-" + n, "Identifier", [el("Identifier.value", **inv("ax-v", EX, a))])
    P("v-t-" + n, [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + "tp-v-" + n]}]), el("Patient.identifier.value", **inv("ax-v", EX, b))])
    I("bad-v-t-" + n, "v-t-" + n, {"identifier": [{"value": "xy"}]})
    P("v-d-" + n, [el("Patient.identifier.value", **inv("ax-w", EX, a))], base=B + "v-d0-" + n)
    P("v-d0-" + n, head + tail + [el("Patient.identifier:mrn.value", **inv("ax-w", EX, b))])
    I("bad-v-d-" + n, "v-d-" + n, {"identifier": [dict(M, value="xy")]})
# --- elección: homólogo renombrado
P("h-ch", [CS, el("Observation.component.value[x]", slicing={"discriminator": [{"type": "type", "path": "$this"}], "rules": "open"}),
           el("Observation.component.value[x]:valueString", sliceName="valueString", type=[{"code": "string"}], fixedString="BASE"),
           el("Observation.component:a", sliceName="a"), el("Observation.component:a.code.text", fixedString="A"),
           el("Observation.component:a.value[x]", type=[{"code": "string"}], fixedString="A")], typ="Observation")
I("bad-h-ch", "h-ch", dict(OBS, component=[{"code": {"text": "A"}, "valueString": "A"}]), rt="Observation")
# --- slices anidados y reslicing
TS = {"discriminator": [{"type": "value", "path": "system"}], "rules": "open"}
CG = {"discriminator": [{"type": "value", "path": "gender"}], "rules": "open"}
P("n-nest", [el("Patient.contact", slicing=CG), el("Patient.contact.telecom", slicing=TS), el("Patient.contact.telecom.value", fixedString="C"),
             el("Patient.contact.telecom:p", sliceName="p"), el("Patient.contact.telecom:p.system", fixedCode="phone"), el("Patient.contact.telecom:p.value", fixedString="CP"),
             el("Patient.contact:m", sliceName="m"), el("Patient.contact:m.telecom", slicing=TS), el("Patient.contact:m.telecom.value", fixedString="M"),
             el("Patient.contact:m.telecom:p", sliceName="p"), el("Patient.contact:m.telecom:p.system", fixedCode="phone"), el("Patient.contact:m.telecom:p.value", fixedString="MP"),
             el("Patient.contact:m.gender", fixedCode="male")])  # gender va después de telecom en Patient.contact
I("bad-n-nest", "n-nest", {"contact": [{"gender": "male", "telecom": [{"system": "phone", "value": "MP"}]}]})
RU = {"discriminator": [{"type": "value", "path": "use"}], "rules": "open"}
P("r-fix", [el("Patient.identifier", slicing=SYS), el("Patient.identifier.value", fixedString="BASE"),
            el("Patient.identifier:a", sliceName="a", slicing=RU), el("Patient.identifier:a.system", fixedUri="urn:a"), el("Patient.identifier:a.value", fixedString="A"),
            el("Patient.identifier:a/b", sliceName="a/b"), el("Patient.identifier:a/b.use", fixedCode="usual"), el("Patient.identifier:a/b.system", fixedUri="urn:a"), el("Patient.identifier:a/b.value", fixedString="B")])
I("bad-r-fix", "r-fix", {"identifier": [{"system": "urn:a", "use": "usual", "value": "B"}]})
# --- slicing sin discriminador: la pertenencia es la conformidad con el slice
P("m-nod", [el("Patient.identifier", slicing={"rules": "open"}), el("Patient.identifier.value", fixedString="BASE"), el("Patient.identifier:m", sliceName="m", max="1"),
            el("Patient.identifier:m.system", fixedUri="urn:mrn"), el("Patient.identifier:m.value", fixedString="MRN")])
I("bad-m-nod", "m-nod", {"identifier": [dict(M, value="MRN")]})
P("m-min", [el("Patient.identifier", slicing={"rules": "closed", "discriminator": []} if False else {"rules": "closed"}), el("Patient.identifier.value", fixedString="BASE"),
            el("Patient.identifier:m", sliceName="m", min=1), el("Patient.identifier:m.system", fixedUri="urn:mrn"), el("Patient.identifier:m.value", fixedString="MRN")])
I("bad-m-min", "m-min", {"identifier": [dict(M, value="MRN")]})
# --- varios perfiles de tipo con hijos desplegados
for n, v in (("a", "F"), ("b", "H")): T("alt-" + n, "Identifier", [el("Identifier.value", fixedString=v)])
P("l-two", [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + "alt-a", B + "alt-b"]}]), el("Patient.identifier.system", min=1)])
I("bad-l-none", "l-two", {"identifier": [{"system": "urn:x", "value": "G"}]}); I("ok-l-a", "l-two", {"identifier": [{"system": "urn:x", "value": "F"}]})
T("alt-c", "Identifier", [el("Identifier.extension", slicing=URL), el("Identifier.extension.value[x]", type=[{"code": "string"}], fixedString="BASE"), el("Identifier.extension:x", sliceName="x"),
                          el("Identifier.extension:x.url", fixedUri=E), el("Identifier.extension:x.value[x]", type=[{"code": "string"}], fixedString="X")])
T("alt-d", "Identifier", [el("Identifier.use", short="u")])
P("l-sp", [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + "alt-c", B + "alt-d"]}])])
I("ok-l-sp", "l-sp", {"identifier": [{"system": "urn:x", "extension": [{"url": E, "valueString": "X"}]}]})
# --- extensión con perfil de tipo y varios perfiles, uno es su propia definición (own)
T("e2", "Extension", [el("Extension.extension", max="0"), el("Extension.url", fixedUri=B + "e2"), el("Extension.value[x]", type=[{"code": "string"}])], extra=EXTC)
P("o-alt", [RS, el("Patient.extension:e", sliceName="e", type=[{"code": "Extension", "profile": [B + "e2", E1]}]), el("Patient.extension:e.url", short="u")])
I("bad-o-alt", "o-alt", {"extension": [{"url": B + "e2", "valueInteger": 1}]})
# --- filtro: el hijo del elemento dividido solo agrega un perfil de tipo, slices o un perfil de tipo en el padre
for c in ("fix", "inv"):
    P("d-low-" + c, [el("Patient.identifier.period", type=[{"code": "Period", "profile": [B + "pp-" + c]}])], base=B + "d0")
    I("bad-d-low-" + c, "d-low-" + c, {"identifier": [dict(M, period={"start": "2001"})]})
P("d-sl", [el("Patient.identifier.extension", slicing=URL), el("Patient.identifier.extension:x", sliceName="x"), el("Patient.identifier.extension:x.url", fixedUri=E),
           el("Patient.identifier.extension:x.value[x]", type=[{"code": "string"}], fixedString="F")], base=B + "d0")
I("bad-d-sl", "d-sl", {"identifier": [dict(M, extension=[{"url": E, "valueString": "G"}])]})
T("tp-f", "Identifier", [el("Identifier.value", fixedString="F")]); T("tp-m", "Identifier", [el("Identifier.period", min=1)])
for c, tp, val in (("f", "tp-f", {"value": "G"}), ("m", "tp-m", {"value": "G"})):
    P("d-tp-" + c, [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + tp]}])], base=B + "d0")
    I("bad-d-tp-" + c, "d-tp-" + c, {"identifier": [dict(M, **val)]})
# --- cardinalidad adversarial, segunda tanda
P("cx-nest", [el("Patient.contact", slicing=CG), el("Patient.contact.telecom", slicing=TS), el("Patient.contact.telecom.use", min=1),
              el("Patient.contact.telecom:p", sliceName="p"), el("Patient.contact.telecom:p.system", fixedCode="phone"), el("Patient.contact.telecom:p.rank", min=1),
              el("Patient.contact:m", sliceName="m"), el("Patient.contact:m.gender", fixedCode="male"), el("Patient.contact:m.telecom", slicing=TS), el("Patient.contact:m.telecom.period", min=1),
              el("Patient.contact:m.telecom:p", sliceName="p"), el("Patient.contact:m.telecom:p.system", fixedCode="phone"), el("Patient.contact:m.telecom:p.value", min=1)])
I("bad-cx-nest", "cx-nest", {"contact": [{"gender": "male", "telecom": [{"system": "phone"}]}]})
P("cx-t2", [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + "tp-m"]}]), el("Patient.identifier.period", min=0)])
I("bad-cx-t2", "cx-t2", {"identifier": [{"value": "v"}]})
T("tp-mx", "Identifier", [el("Identifier.assigner", max="0")])
P("cx-t3", [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + "tp-mx"]}]), el("Patient.identifier.assigner", max="1")])
I("bad-cx-t3", "cx-t3", {"identifier": [{"assigner": {"display": "x"}}]})
P("cx-pown", [PS, el("Patient.birthDate.extension:e", sliceName="e"), el("Patient.birthDate.extension:e.id", short="i"), el("Patient.birthDate.extension:e.url", fixedUri=E1)])
I("bad-cx-pown", "cx-pown", {"birthDate": "2000-01-01", "_birthDate": {"extension": [{"url": E1, "id": "z"}]}})
P("cx-sltp", [head[0], el("Patient.identifier:mrn", sliceName="mrn", type=[{"code": "Identifier", "profile": [B + "tp-m"]}]), el("Patient.identifier:mrn.system", fixedUri="urn:mrn"), el("Patient.identifier:mrn.period", min=0)])
I("bad-cx-sltp", "cx-sltp", {"identifier": [dict(M, value="v")]})
P("cx-pd", [el("Patient.birthDate.extension", max="0")], base=B + "cx-p0"); P("cx-p0", [el("Patient.birthDate.id", short="i")])
I("bad-cx-pd", "cx-pd", {"birthDate": "2000-01-01", "_birthDate": {"extension": [{"url": E, "valueString": "x"}]}})
# --- primitivo: fixed/invariante en un hijo desplegado del primitivo, y en la extensión por definición propia
P("p-own", [PS, el("Patient.birthDate.extension:e", sliceName="e"), el("Patient.birthDate.extension:e.id", short="i"), el("Patient.birthDate.extension:e.url", fixedUri=E1)])
I("bad-p-own-fix", "p-own", {"birthDate": "2000-01-01", "_birthDate": {"extension": [{"url": E1, "valueString": "G"}]}})
P("p-base", [el("Patient.birthDate.id", fixedString="zz")])  # Element.id es string: fixedId no es de su tipo
I("bad-p-id", "p-base", {"birthDate": "2000-01-01", "_birthDate": {"id": "q"}})
P("p-inv", [el("Patient.birthDate.extension", **inv("ax-pe", "url.startsWith('urn:')"))])
I("bad-p-einv", "p-inv", {"birthDate": "2000-01-01", "_birthDate": {"extension": [{"url": E, "valueString": "x"}]}})
# --- arreglo: un elemento en el slice y otro fuera, y dos en el slice
P("s-arr", head + [el("Patient.identifier.value", fixedString="BASE")] + tail + [el("Patient.identifier:mrn.value", fixedString="MRN")])
I("bad-s-arr", "s-arr", {"identifier": [dict(M, value="MRN"), {"system": "urn:o", "value": "BASE"}, dict(M, value="BASE")]})
# --- contentReference con hijo desplegado en el destino y slice debajo
P("q-cr", [el("Questionnaire.item", slicing={"discriminator": [{"type": "value", "path": "type"}], "rules": "open"}), el("Questionnaire.item.text", fixedString="T"),
           el("Questionnaire.item:g", sliceName="g"), el("Questionnaire.item:g.text", fixedString="G"), el("Questionnaire.item:g.type", fixedCode="group")], typ="Questionnaire")  # text va antes que type
I("bad-q-cr", "q-cr", {"status": "draft", "item": [{"linkId": "1", "type": "group", "text": "G", "item": [{"linkId": "2", "type": "group", "text": "X"}]}]}, rt="Questionnaire")
# --- checkType en dos recorridos de la misma definición
P("g1", [CS, el("Observation.component:a", sliceName="a"), el("Observation.component:a.code.text", fixedString="A"), el("Observation.component:a.value[x]", type=[{"code": "string"}])], typ="Observation")
P("d-type2", [el("Observation.component.value[x]", type=[{"code": "string"}])], base=B + "g1", typ="Observation")
I("bad-d-type2", "d-type2", dict(OBS, component=[{"code": {"text": "A"}, "valueInteger": 1}]), rt="Observation")

# --- controles (25-11-2026, plan §9): HL7 no genera el snapshot de d-type ni de d-type2 (perfil derivado que restringe
# un hijo del elemento dividido cuyo slice declara la base: «Same id ... on multiple elements»). El mismo diferencial
# en un solo perfil da el valor esperado.
P("ctl-d-type", [CS, el("Observation.component.value[x]", type=[{"code": "string"}]), el("Observation.component:a", sliceName="a"), el("Observation.component:a.id", short="i"),
                 el("Observation.component:a.code.text", fixedString="A")], typ="Observation")
I("ctl-d-type", "ctl-d-type", dict(OBS, component=[{"code": {"text": "A"}, "valueInteger": 1}]), rt="Observation")
P("ctl-d-type2", [CS, el("Observation.component.value[x]", type=[{"code": "string"}]), el("Observation.component:a", sliceName="a"), el("Observation.component:a.code.text", fixedString="A"),
                  el("Observation.component:a.value[x]", type=[{"code": "string"}])], typ="Observation")
I("ctl-d-type2", "ctl-d-type2", dict(OBS, component=[{"code": {"text": "A"}, "valueInteger": 1}]), rt="Observation")
