#!/usr/bin/env python3
"""Sondas que endurecen las mutaciones 10, 15a y 19 (plan 2026-11-18-definition-layers-descent, §11). Uso: python3 -I gen.py <dir>."""
import json, os, sys
V = "4.0.1"
B = "http://example.org/hm/"
out = sys.argv[1]
os.makedirs(out + "/ig", exist_ok=True); os.makedirs(out + "/inst", exist_ok=True)
def sd(sid, typ, diff, kind="resource", extra=None):
    o = {"resourceType": "StructureDefinition", "id": sid, "url": B + sid, "name": "Hm" + sid.replace("-", "").title(),
         "status": "draft", "fhirVersion": V, "kind": kind, "abstract": False, "type": typ,
         "baseDefinition": "http://hl7.org/fhir/StructureDefinition/" + typ, "derivation": "constraint",
         "differential": {"element": diff}}
    o.update(extra or {}); return o
def put(d, n, o): json.dump(o, open(f"{out}/{d}/{n}.json", "w"), indent=1)
def el(i, **k): return dict(id=i, path=".".join(x.split(":")[0] for x in i.split(".")), **k)
def inv(key, expr, sev, src=None):
    c = {"key": key, "severity": sev, "human": key + " text", "expression": expr}
    if src: c["source"] = src
    return {"constraint": [c]}
def T(sid, typ, diff): put("ig", sid, sd(sid, typ, [dict(id=typ, path=typ)] + diff, kind="complex-type"))
def P(sid, diff, typ="Patient"): put("ig", sid, sd(sid, typ, diff))
def I(name, prof, body):
    o = {"resourceType": "Patient", "id": name, "meta": {"profile": [B + prof]}, "text": {"status": "generated", "div": "<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}}
    o.update(body); put("inst", name, o)
IDT = lambda *p: {"type": [{"code": "Identifier", "profile": [B + x for x in p]}]}
NOSYS = el  # ambas capas exigen system, que la instancia no trae: ninguna se cumple y se evalúan las dos
# --- 15a: dos perfiles de tipo con la misma clave, texto y fuente declarados, una advertencia y un error
for n, (a, b) in {"we": ("warning", "error"), "ew": ("error", "warning")}.items():
    T("sa-" + n, "Identifier", [el("Identifier.system", min=1), el("Identifier.value", **inv("k15", "$this.length() < 2", a, B + "common"))])
    T("sb-" + n, "Identifier", [el("Identifier.system", min=1), el("Identifier.value", **inv("k15", "$this.length() < 2", b, B + "common"))])
    P("p15-" + n, [el("Patient.identifier", **IDT("sa-" + n, "sb-" + n))])
    I("m15-" + n, "p15-" + n, {"identifier": [{"value": "xy"}]})
# --- 10: la misma clave y expresión en un hijo propio del perfil y en el de una capa de perfil de tipo, que ninguna cumple;
# el texto «(defined in …)» muestra cuál se evaluó primero: el hijo propio no lleva fuente, el de la capa lleva su perfil.
T("tz-10", "Identifier", [el("Identifier.system", min=1)])
T("tk-10", "Identifier", [el("Identifier.system", min=1), el("Identifier.value", **inv("gk", "$this.length() < 1", "error"))])
P("p10", [el("Patient.identifier", **IDT("tz-10", "tk-10")), el("Patient.identifier.value", **inv("gk", "$this.length() < 1", "error"))])
I("m10", "p10", {"identifier": [{"value": "x"}]})
# --- 19 (solo evidencia, ver §11 del plan): HL7 despliega en el slice con hijos propios solo el primer perfil de extensión; la prueba de la mutación 19 es de slicematch
def ext(sid, url):
    o = sd(sid, "Extension", [dict(id="Extension", path="Extension"), el("Extension.url", fixedUri=url), el("Extension.value[x]", type=[{"code": "code"}])], kind="complex-type")
    o["url"] = url; o["context"] = [{"type": "element", "expression": "Patient"}]; put("ig", sid, o)
ext("ext-a", B + "ext-a"); ext("ext-b", B + "ext-b")
P("p19", [el("Patient.extension", slicing={"discriminator": [{"type": "value", "path": "url"}], "rules": "closed"}),
          el("Patient.extension:either", sliceName="either", type=[{"code": "Extension", "profile": [B + "ext-a", B + "ext-b"]}]),
          el("Patient.extension:either.value[x]", type=[{"code": "code"}])])
I("m19-b", "p19", {"extension": [{"url": B + "ext-b", "valueCode": "x"}]})
I("m19-a", "p19", {"extension": [{"url": B + "ext-a", "valueCode": "x"}]})
I("m19-none", "p19", {"extension": [{"url": B + "ext-c", "valueCode": "x"}]})
