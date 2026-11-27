#!/usr/bin/env python3
"""Sondas por fase y por forma: qué definiciones paralelas aplica cada fase (revisión previa, 18-11-2026).

Formas: S (hijo del elemento dividido frente al mismo hijo redeclarado en el slice), T (perfil de tipo con hijos
desplegados en el perfil de la instancia), P (slice de la extensión de un primitivo). Cada caso tiene su perfil, una
instancia que viola y, si existe, una que cumple. Uso: python3 -I gen.py (desde probe/)."""
import json, os
B = "http://example.org/dl/"
def sd(sid, typ, diff, base=None, kind="resource", extra=None):
    o = {"resourceType": "StructureDefinition", "id": sid, "url": B + sid, "name": "Dl" + sid.replace("-", "").title(),
         "status": "draft", "fhirVersion": "4.0.1", "kind": kind, "abstract": False, "type": typ,
         "baseDefinition": base or "http://hl7.org/fhir/StructureDefinition/" + typ, "derivation": "constraint",
         "differential": {"element": diff}}
    o.update(extra or {})
    return o
def el(i, **k):
    p = ".".join(x.split(":")[0] for x in i.split("."))
    return dict(id=i, path=p, **k)
def req(vs): return {"binding": {"strength": "required", "valueSet": B + "vs-" + vs}}
def inv(key, expr): return {"constraint": [{"key": key, "severity": "error", "human": key, "expression": expr}]}
def cod(c): return {"coding": [{"system": B + "cs", "code": c}]}
os.makedirs("ig", exist_ok=True); os.makedirs("inst", exist_ok=True)
def put(d, n, o): json.dump(o, open(f"{d}/{n}.json", "w"), indent=1, ensure_ascii=False)
put("ig", "cs", {"resourceType": "CodeSystem", "id": "cs", "url": B + "cs", "status": "active", "content": "complete",
                 "concept": [{"code": "A"}, {"code": "B"}]})
for c in "AB":
    put("ig", "vs-" + c, {"resourceType": "ValueSet", "id": "vs-" + c, "url": B + "vs-" + c, "status": "active",
                          "compose": {"include": [{"system": B + "cs", "concept": [{"code": c}]}]}})
def pat(name, prof, body, ok):
    o = {"resourceType": "Patient", "id": name, "meta": {"profile": [B + prof]}}; o.update(body)
    put("inst", ("ok-" if ok else "bad-") + name, o)
M = {"system": "urn:mrn"}
# Forma S: (caso, hijo, base, slice, identifier que viola la base, identifier que cumple o None)
S = [("fixed", "value", {"fixedString": "BASE"}, {"fixedString": "MRN"}, {"value": "MRN"}, None),
     ("min", "period", {"min": 1}, {"min": 0}, {"value": "v"}, {"value": "v", "period": {"start": "2000"}}),
     ("max", "assigner", {"max": "0"}, {"max": "1"}, {"assigner": {"display": "x"}}, {"value": "v"}),
     ("binding", "type", req("A"), req("B"), {"type": cod("B")}, None),
     ("inv", "value", inv("dl-b", "$this.length() < 2"), inv("dl-s", "true"), {"value": "abcd"}, {"value": "a"})]
for c, ch, b, s, bad, ok in S:
    sid = "s-" + c
    put("ig", sid, sd(sid, "Patient", [
        el("Patient.identifier", slicing={"discriminator": [{"type": "value", "path": "system"}], "rules": "open"}),
        el("Patient.identifier." + ch, **b), el("Patient.identifier:mrn", sliceName="mrn"),
        el("Patient.identifier:mrn.system", fixedUri="urn:mrn"), el("Patient.identifier:mrn." + ch, **s)]))
    pat(sid, sid, {"identifier": [dict(M, **bad)]}, False)
    if ok: pat(sid, sid, {"identifier": [dict(M, **ok)]}, True)
sid = "s-deep"
put("ig", sid, sd(sid, "Patient", [
    el("Patient.identifier", slicing={"discriminator": [{"type": "value", "path": "system"}], "rules": "open"}),
    el("Patient.identifier.period.start", min=1), el("Patient.identifier:mrn", sliceName="mrn"),
    el("Patient.identifier:mrn.system", fixedUri="urn:mrn"), el("Patient.identifier:mrn.period.end", min=0)]))
pat(sid, sid, {"identifier": [dict(M, period={"end": "2001"})]}, False)
pat(sid, sid, {"identifier": [dict(M, period={"start": "2000"})]}, True)
# Forma T: el perfil de Identifier restringe; el perfil de Patient despliega otro hijo.
OKI = {"system": "urn:x"}
T = [("fixed", "value", {"fixedString": "F"}, {"value": "G"}, {"value": "F"}),
     ("min", "period", {"min": 1}, {}, {"period": {"start": "2000"}}),
     ("max", "assigner", {"max": "0"}, {"assigner": {"display": "x"}}, {}),
     ("binding", "type", req("A"), {"type": cod("B")}, {"type": cod("A")}),
     ("inv", "value", inv("dl-t", "$this.length() < 2"), {"value": "abcd"}, {"value": "a"})]
for c, ch, b, bad, ok in T:
    i, sid = "tid-" + c, "t-" + c
    put("ig", i, sd(i, "Identifier", [el("Identifier." + ch, **b)], kind="complex-type"))
    put("ig", sid, sd(sid, "Patient", [el("Patient.identifier", type=[{"code": "Identifier", "profile": [B + i]}]),
                                       el("Patient.identifier.system", min=1)]))
    pat(sid, sid, {"identifier": [dict(OKI, **bad)]}, False); pat(sid, sid, {"identifier": [dict(OKI, **ok)]}, True)
# Forma P: slices de la extensión de un primitivo.
E = B + "ext"
put("ig", "ext", sd("ext", "Extension", [el("Extension.extension", max="0"), el("Extension.url", fixedUri=E),
    el("Extension.value[x]", type=[{"code": "string"}, {"code": "code"}])], kind="complex-type",
    extra={"context": [{"type": "element", "expression": "Element"}]}))
SL = el("Patient.birthDate.extension", slicing={"discriminator": [{"type": "value", "path": "url"}], "rules": "open"})
def es(**k): return el("Patient.birthDate.extension:e", sliceName="e", type=[{"code": "Extension", "profile": [E]}], **k)
X = lambda **v: {"birthDate": "2000-01-01", "_birthDate": {"extension": [dict(url=E, **v)]}}
P = [("min", [SL, es(min=1)], {"birthDate": "2000-01-01"}, X(valueString="a")),
     ("max", [SL, es(max="0")], X(valueString="a"), {"birthDate": "2000-01-01"}),
     ("fixed", [SL, es(), el("Patient.birthDate.extension:e.value[x]", fixedString="F")], X(valueString="G"), X(valueString="F")),
     ("inv", [SL, es(), el("Patient.birthDate.extension:e.value[x]", **inv("dl-p", "$this.length() < 2"))], X(valueString="ab"), X(valueString="a")),
     ("binding", [SL, es(), el("Patient.birthDate.extension:e.value[x]", type=[{"code": "code"}], **req("A"))], X(valueCode="B"), X(valueCode="A"))]
for c, d, bad, ok in P:
    sid = "p-" + c
    put("ig", sid, sd(sid, "Patient", d)); pat(sid, sid, bad, False); pat(sid, sid, ok, True)
