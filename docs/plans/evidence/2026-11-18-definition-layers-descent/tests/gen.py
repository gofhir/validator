#!/usr/bin/env python3
"""Matriz de sondas del descenso por todas las definiciones (plan 2026-11-18, §2 v4, «Tests y datos»).

Cada caso es un valor que gobiernan dos definiciones, A y B, y una regla puesta en los hijos de cada una:
  portador (de dónde salen A y B) × regla × violación (ninguna, solo A, solo B, las dos).

Portadores (la forma del plan §1 entre paréntesis):
  sid   (S)  A = Patient.identifier (elemento dividido), B = su slice mrn.
  d1    (D)  B en el slice de un perfil base; A en el elemento dividido de un perfil derivado.
  d2    (D)  A en el elemento dividido del perfil base; B en el slice, en el perfil derivado.
  t2    (T2) A en el perfil de tipo de Patient.identifier; B en los hijos que el perfil de Patient despliega.
  sext  (S)  A = Patient.extension, B = su slice x (con un perfil de extensión sin reglas).
  t2ext (T2) A en el perfil de la extensión del slice x; B en el slice.
  pext  (P)  A = Patient.birthDate.extension, B = su slice x.
  own   (O)  A = Patient.extension (sin slices); B en la definición propia de la extensión (la que nombra url).
  held  (H)  A en el perfil del recurso que declara Bundle.entry.resource; B en el de su slice s.
Reglas: inv (invariante, mismo hijo), fixed, pattern, tprof (perfil de tipo de un hijo), cslice (slice de un hijo),
type (tipos permitidos; solo portadores de extensión), sev (la misma invariante, warning en A y error en B).
Además: profundidad 2 a 5 (extensiones anidadas) y arreglos.

Los perfiles se escriben con la fhirVersion del argumento. Los valores esperados NO salen de aquí: salen de HL7
(run_hl7.sh y gen_expected.py). Uso: python3 -I gen.py <dir> [fhirVersion]
"""
import json
import os
import sys

OUT = sys.argv[1]
V = sys.argv[2] if len(sys.argv) > 2 else "4.0.1"
B = "http://example.org/dl/"
FHIR = "http://hl7.org/fhir/StructureDefinition/"
# contexto de las extensiones: cualquier elemento y la raíz de un recurso (que no es un Element)
CTX = {"context": [{"type": "element", "expression": "Element"}, {"type": "element", "expression": "Resource"}]}

for d in ("ig", "inst"):
    os.makedirs(os.path.join(OUT, d), exist_ok=True)
DESC = []


def put(d, n, o):
    with open(os.path.join(OUT, d, n + ".json"), "w") as f:
        json.dump(o, f, indent=1)
        f.write("\n")


def sd(sid, typ, diff, base=None, kind="resource", extra=None):
    o = {"resourceType": "StructureDefinition", "id": sid, "url": B + sid,
         "name": "Dl" + "".join(p.title() for p in sid.split("-")), "status": "draft", "fhirVersion": V,
         "kind": kind, "abstract": False, "type": typ, "baseDefinition": base or FHIR + typ,
         "derivation": "constraint", "differential": {"element": diff}}
    o.update(extra or {})
    put("ig", sid, o)
    return B + sid


def el(i, **k):
    return dict(id=i, path=".".join(x.split(":")[0] for x in i.split(".")), **k)


def ext_sd(sid, diff):
    """Extensión simple cuyo valor es un Identifier (diff: elementos bajo Extension.value[x])."""
    return sd(sid, "Extension", [el("Extension", short=sid), el("Extension.extension", max="0"),
                                 el("Extension.url", fixedUri=B + sid)] + diff,
              kind="complex-type", extra=CTX)


def inv(key, letter, sev="error"):
    return {"constraint": [{"key": key, "severity": sev, "human": "no " + letter,
                            "expression": "$this.contains('%s').not()" % letter}]}


# --- perfiles de tipo y extensiones auxiliares
TPA = sd("tp-period", "Period", [el("Period", constraint=[{"key": "dl-tpa", "severity": "error", "human": "start",
                                                          "expression": "start.exists()"}])], kind="complex-type")
TPB = sd("tp-ref", "Reference", [el("Reference", constraint=[{"key": "dl-tpb", "severity": "error",
                                                             "human": "display", "expression": "display.exists()"}])],
         kind="complex-type")
E0 = ext_sd("e0", [el("Extension.value[x]", type=[{"code": "Identifier"}])])
ESTR = {}
for s in ("ea", "eb"):
    ESTR[s] = sd(s, "Extension", [el("Extension.extension", max="0"), el("Extension.url", fixedUri=B + s),
                                  el("Extension.value[x]", type=[{"code": "string"}])],
                 kind="complex-type", extra=CTX)


def rule(r, side, root):
    """Los elementos del diferencial que ponen la regla r del lado side ('a' o 'b') bajo root (un Identifier)."""
    a = side == "a"
    if r == "inv":
        return [el(root + ".value", **inv("dl-" + side, side))]
    if r == "sev":
        return [el(root + ".value", constraint=[{"key": "dl-s", "severity": "warning" if a else "error",
                                                 "human": "no s", "expression": "$this.contains('s').not()"}])]
    if r == "fixed":
        return [el(root + ".value", fixedString="fa")] if a else [el(root + ".assigner.display", fixedString="fb")]
    if r == "pattern":
        return ([el(root + ".assigner", patternReference={"display": "pa"})] if a
                else [el(root + ".period", patternPeriod={"start": "2000-01-01"})])
    if r == "tprof":
        return ([el(root + ".period", type=[{"code": "Period", "profile": [TPA]}])] if a
                else [el(root + ".assigner", type=[{"code": "Reference", "profile": [TPB]}])])
    if r == "cslice":
        s = "ea" if a else "eb"
        return [el(root + ".extension", slicing={"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}),
                el(root + ".extension:" + s, sliceName=s, type=[{"code": "Extension", "profile": [ESTR[s]]}]),
                el(root + ".extension:" + s + ".value[x]", fixedString=s)]
    raise ValueError(r)


def ident(r, viol):
    """El Identifier de la instancia para la regla r y la violación viol ('none', 'a', 'b', 'ab')."""
    va, vb = "a" in viol, "b" in viol
    o = {"system": "urn:mrn", "value": "z", "period": {"start": "2000-01-01"}, "assigner": {"display": "d"}}
    if r == "inv":
        o["value"] = {"none": "z", "a": "a", "b": "b", "ab": "ab"}[viol]
    elif r == "sev":
        o["value"] = "s" if viol != "none" else "z"
    elif r == "fixed":
        o["value"], o["assigner"]["display"] = ("zz" if va else "fa"), ("zz" if vb else "fb")
    elif r == "pattern":
        o["assigner"]["display"], o["period"]["start"] = ("zz" if va else "pa"), ("2001-01-01" if vb else "2000-01-01")
    elif r == "tprof":
        o["period"] = {"end": "2001-01-01"} if va else {"start": "2000-01-01"}
        o["assigner"] = {"identifier": {"value": "x"}} if vb else {"display": "d"}
    elif r == "cslice":
        o["extension"] = [{"url": ESTR["ea"], "valueString": "zz" if va else "ea"},
                          {"url": ESTR["eb"], "valueString": "zz" if vb else "eb"}]
    return o


VIOLS = ("none", "a", "b", "ab")
NARR = {"status": "generated", "div": '<div xmlns="http://www.w3.org/1999/xhtml">probe</div>'}
RULES = ("inv", "fixed", "pattern", "tprof", "cslice", "sev")
SLICE_SYS = {"discriminator": [{"type": "value", "path": "system"}], "rules": "open"}
SLICE_URL = {"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}


def viols(r):
    return ("none", "ab") if r == "sev" else VIOLS


def patient(name, prof, body, desc):
    o = {"resourceType": "Patient", "id": name, "meta": {"profile": [prof]}, "text": NARR}
    o.update(body)
    put("inst", name, o)
    DESC.append((name, desc))


MRN = [el("Patient.identifier:mrn", sliceName="mrn"), el("Patient.identifier:mrn.system", fixedUri="urn:mrn")]
ORDER = ("extension", "use", "type", "system", "value", "period", "assigner")


def ordered(root, elems):
    """Los elementos bajo root en el orden de los hijos de Identifier (el diferencial lo exige)."""
    def k(e):
        rest = e["id"][len(root) + 1:].split(".")[0].split(":")[0]
        return ORDER.index(rest)
    return sorted(elems, key=k)


def mrn(r):
    return MRN[:1] + ordered("Patient.identifier:mrn", MRN[1:] + rule(r, "b", "Patient.identifier:mrn"))
for r in RULES:
    # sid: el elemento dividido y su slice en un mismo perfil
    p = sd("sid-" + r, "Patient", [el("Patient.identifier", slicing=SLICE_SYS)] + rule(r, "a", "Patient.identifier")
           + mrn(r))
    for vi in viols(r):
        patient("m-sid-%s-%s" % (r, vi), p, {"identifier": [ident(r, vi)]}, "S: %s, violation %s" % (r, vi))
    # d1: B en el slice del perfil base, A en el elemento dividido del derivado
    base = sd("d1b-" + r, "Patient", [el("Patient.identifier", slicing=SLICE_SYS)] + mrn(r))
    p = sd("d1-" + r, "Patient", rule(r, "a", "Patient.identifier"), base=base)
    for vi in viols(r):
        patient("m-d1-%s-%s" % (r, vi), p, {"identifier": [ident(r, vi)]}, "D (A derived): %s, violation %s" % (r, vi))
    # d2: A en el elemento dividido del perfil base, B en el slice del derivado
    base = sd("d2b-" + r, "Patient", [el("Patient.identifier", slicing=SLICE_SYS)] + rule(r, "a", "Patient.identifier")
              + MRN)
    p = sd("d2-" + r, "Patient", [el("Patient.identifier:mrn", sliceName="mrn")] + rule(r, "b", "Patient.identifier:mrn"),
           base=base)
    for vi in viols(r):
        patient("m-d2-%s-%s" % (r, vi), p, {"identifier": [ident(r, vi)]}, "D (B derived): %s, violation %s" % (r, vi))
    # t2: A en el perfil de tipo, B en los hijos desplegados. Sin sev: la misma clave en el perfil de tipo y en el hijo
    # desplegado se funde al generar el snapshot (HL7 se queda con la del perfil de tipo), y eso queda fuera del
    # alcance (plan §2, «Fuera del alcance»).
    if r == "sev":
        continue
    tp = sd("tpi-" + r, "Identifier", [el("Identifier")] + rule(r, "a", "Identifier"), kind="complex-type")
    p = sd("t2-" + r, "Patient", [el("Patient.identifier", type=[{"code": "Identifier", "profile": [tp]}])]
           + rule(r, "b", "Patient.identifier"))
    for vi in viols(r):
        patient("m-t2-%s-%s" % (r, vi), p, {"identifier": [ident(r, vi)]}, "T2: %s, violation %s" % (r, vi))


def ext_rule(r, side, root, nested=False):
    """La regla bajo root, un elemento Extension: los tipos de su value[x], o un Identifier en value[x]. Con nested,
    B está dentro de A (un slice de un perfil de extensión solo puede achicar sus tipos)."""
    if r == "type":
        if nested:
            return [el(root + ".value[x]", type=[{"code": c} for c in (("string", "integer", "boolean") if side == "a"
                                                                         else ("string", "boolean"))])]
        return [el(root + ".value[x]", type=[{"code": "string"}, {"code": "integer" if side == "a" else "boolean"}])]
    return [el(root + ".value[x]", type=[{"code": "Identifier"}])] + rule(r, side, root + ".value[x]")


TYPE_VAL = {"none": ("valueString", "s"), "a": ("valueBoolean", True), "b": ("valueInteger", 1), "ab": ("valueDecimal", 1.5)}


def ext_value(r, vi, url):
    if r == "type":
        k, v = TYPE_VAL[vi]
        return {"url": url, k: v}
    return {"url": url, "valueIdentifier": ident(r, vi)}


ERULES = ("type",) + RULES
EANY = ext_sd("eany", [])          # extensión sin reglas y sin restringir value[x] (para la regla type)
for r in ERULES:
    e0 = EANY if r == "type" else E0
    # sext: el elemento dividido Patient.extension y su slice x
    p = sd("sext-" + r, "Patient", [el("Patient.extension", slicing=SLICE_URL)] + ext_rule(r, "a", "Patient.extension")
           + [el("Patient.extension:x", sliceName="x", type=[{"code": "Extension", "profile": [e0]}])]
           + ext_rule(r, "b", "Patient.extension:x"))
    for vi in viols(r):
        patient("m-sext-%s-%s" % (r, vi), p, {"extension": [ext_value(r, vi, e0)]}, "S (extension): %s, violation %s" % (r, vi))
    # t2ext: A en el perfil de la extensión, B en el slice (sin sev, como t2)
    if r != "sev":
        ea = sd("ext-a-" + r, "Extension", [el("Extension.extension", max="0"), el("Extension.url", fixedUri=B + "ext-a-" + r)]
                + ext_rule(r, "a", "Extension", nested=True), kind="complex-type",
                extra=CTX)
        p = sd("t2ext-" + r, "Patient", [el("Patient.extension", slicing=SLICE_URL),
                                        el("Patient.extension:x", sliceName="x", type=[{"code": "Extension", "profile": [ea]}])]
               + ext_rule(r, "b", "Patient.extension:x", nested=True))
        # con B dentro de A no hay valor que viole solo A
        for vi in (("none", "b", "ab") if r == "type" else viols(r)):
            patient("m-t2ext-%s-%s" % (r, vi), p, {"extension": [ext_value(r, vi, ea)]}, "T2 (extension): %s, violation %s" % (r, vi))
    # pext: la extensión de un primitivo, dividida
    p = sd("pext-" + r, "Patient", [el("Patient.birthDate.extension", slicing=SLICE_URL)]
           + ext_rule(r, "a", "Patient.birthDate.extension")
           + [el("Patient.birthDate.extension:x", sliceName="x", type=[{"code": "Extension", "profile": [e0]}])]
           + ext_rule(r, "b", "Patient.birthDate.extension:x"))
    for vi in viols(r):
        patient("m-pext-%s-%s" % (r, vi), p, {"birthDate": "2000-01-01", "_birthDate": {"extension": [ext_value(r, vi, e0)]}},
                "P: %s, violation %s" % (r, vi))
    # own: Patient.extension sin slices y la definición propia de la extensión
    eb = sd("ext-b-" + r, "Extension", [el("Extension.extension", max="0"), el("Extension.url", fixedUri=B + "ext-b-" + r)]
            + ext_rule(r, "b", "Extension"), kind="complex-type",
            extra=CTX)
    p = sd("own-" + r, "Patient", ext_rule(r, "a", "Patient.extension"))
    for vi in viols(r):
        patient("m-own-%s-%s" % (r, vi), p, {"extension": [ext_value(r, vi, eb)]}, "O: %s, violation %s" % (r, vi))


# --- held: perfiles del recurso sostenido en Bundle.entry.resource y en su slice
def held_rule(r, side):
    a = side == "a"
    if r == "inv":
        return [el("Patient.name.family", **inv("dl-" + side, side))]
    if r == "sev":
        return [el("Patient.name.family", constraint=[{"key": "dl-s", "severity": "warning" if a else "error",
                                                       "human": "no s", "expression": "$this.contains('s').not()"}])]
    if r == "fixed":
        return [el("Patient.name.family", fixedString="fa")] if a else [el("Patient.name.text", fixedString="fb")]
    if r == "pattern":
        return ([el("Patient.name", patternHumanName={"text": "pa"})] if a
                else [el("Patient.name.period", patternPeriod={"start": "2000-01-01"})])
    raise ValueError(r)


def held_patient(r, vi):
    va, vb = "a" in vi, "b" in vi
    n = {"family": "z", "text": "pa" if r == "pattern" else "t", "period": {"start": "2000-01-01"}}
    o = {"resourceType": "Patient", "id": "h", "text": NARR, "name": [n]}
    if r == "inv":
        n["family"] = {"none": "z", "a": "a", "b": "b", "ab": "ab"}[vi]
    elif r == "sev":
        n["family"] = "s" if vi != "none" else "z"
    elif r == "fixed":
        n["family"], n["text"] = ("zz" if va else "fa"), ("zz" if vb else "fb")
    elif r == "pattern":
        n["text"], n["period"]["start"] = ("zz" if va else "pa"), ("2001-01-01" if vb else "2000-01-01")
    return o


def bundle(name, prof, resources, desc):
    o = {"resourceType": "Bundle", "id": name, "meta": {"profile": [prof]}, "type": "collection",
         "entry": [{"fullUrl": "urn:uuid:%08d-0000-4000-8000-000000000000" % (i + 1), "resource": res}
                   for i, res in enumerate(resources)]}
    put("inst", name, o)
    DESC.append((name, desc))


for r in ("inv", "fixed", "pattern", "sev"):
    ha = sd("ha-" + r, "Patient", held_rule(r, "a"))
    hb = sd("hb-" + r, "Patient", held_rule(r, "b"))
    p = sd("held-" + r, "Bundle", [
        el("Bundle.entry", slicing={"discriminator": [{"type": "type", "path": "resource"}], "rules": "open"}),
        el("Bundle.entry.resource", type=[{"code": "Patient", "profile": [ha]}]),
        el("Bundle.entry:s", sliceName="s"),
        el("Bundle.entry:s.resource", type=[{"code": "Patient", "profile": [hb]}])])
    for vi in viols(r):
        bundle("m-held-%s-%s" % (r, vi), p, [held_patient(r, vi)], "H: %s, violation %s" % (r, vi))

# --- arreglos: varias repeticiones, solo algunas del slice
p = B + "sid-inv"
patient("m-arr-sid", p, {"identifier": [ident("inv", "b"), dict(ident("inv", "a"), system="urn:o"), ident("inv", "ab"),
                                        dict(ident("inv", "b"), system="urn:o")]},
        "S, array: slice members and others, each violating a different side")
patient("m-arr-sext", B + "sext-inv", {"extension": [ext_value("inv", "a", E0), ext_value("inv", "b", E0)]},
        "S (extension), array: two members of the slice")
patient("m-arr-pext", B + "pext-inv", {"birthDate": "2000-01-01",
                                       "_birthDate": {"extension": [ext_value("inv", "b", E0), ext_value("inv", "a", E0)]}},
        "P, array: two members of the slice")
bundle("m-arr-held", B + "held-inv", [held_patient("inv", "a"), held_patient("inv", "b"), held_patient("inv", "none")],
       "H, array: three entries in the slice")

# --- profundidad: el par (elemento dividido, slice) en una extensión compleja anidada d niveles
inner = sd("dep-2", "Extension", [
    el("Extension.extension", slicing=SLICE_URL), el("Extension.extension.value[x]", type=[{"code": "Identifier"}]),
    el("Extension.extension.value[x].value", **inv("dl-a", "a")),
    el("Extension.extension:n", sliceName="n", type=[{"code": "Extension", "profile": [E0]}]),
    el("Extension.extension:n.value[x]", type=[{"code": "Identifier"}]),
    el("Extension.extension:n.value[x].value", **inv("dl-b", "b")),
    el("Extension.url", fixedUri=B + "dep-2"), el("Extension.value[x]", max="0")],
    kind="complex-type", extra=CTX)
chain = {2: inner}
for d in (3, 4, 5):
    chain[d] = sd("dep-%d" % d, "Extension", [
        el("Extension.extension", slicing=SLICE_URL),
        el("Extension.extension:n", sliceName="n", min=1, type=[{"code": "Extension", "profile": [chain[d - 1]]}]),
        el("Extension.url", fixedUri=B + "dep-%d" % d), el("Extension.value[x]", max="0")],
        kind="complex-type", extra=CTX)


def nested(d, vi):
    leaf = {"url": E0, "valueIdentifier": ident("inv", vi)}
    o = {"url": chain[2], "extension": [leaf]}
    for k in range(3, d + 1):
        o = {"url": chain[k], "extension": [o]}
    return o


for d in (2, 3, 4, 5):
    p = sd("depth-%d" % d, "Patient", [el("Patient.extension", slicing=SLICE_URL),
                                      el("Patient.extension:x", sliceName="x", type=[{"code": "Extension", "profile": [chain[d]]}])])
    for vi in VIOLS:
        patient("m-depth%d-%s" % (d, vi), p, {"extension": [nested(d, vi)]}, "depth %d: inv, violation %s" % (d, vi))

with open(os.path.join(OUT, "descriptions.json"), "w") as f:
    json.dump(DESC, f, indent=1)
print(len(DESC), "instances,", len(os.listdir(os.path.join(OUT, "ig"))), "profiles", file=sys.stderr)
