"""Genera las sondas de #187: perfiles solo con diferencial (pkg-<versión>/package) y las instancias
(in/), las mismas en R4, R4B y R5. Uso: python3 -I gen.py (en esta carpeta)."""
import json, pathlib

B = "http://example.org/fhir/StructureDefinition/"
VERSIONS = {"4.0.1": "hl7.fhir.r4.core", "4.3.0": "hl7.fhir.r4b.core", "5.0.0": "hl7.fhir.r5.core"}
slc = {"discriminator": [{"type": "value", "path": "name"}], "rules": "open"}
code = [{"code": "code"}]
qty = [{"code": "Quantity"}]


def sd(id, typ, elems, kind="resource", ctx=None):
    s = {"resourceType": "StructureDefinition", "id": id, "url": B + id, "name": id.replace("-", "").title(),
         "status": "active", "kind": kind, "abstract": False, "type": typ,
         "baseDefinition": "http://hl7.org/fhir/StructureDefinition/" + typ, "derivation": "constraint",
         "differential": {"element": [{"id": typ, "path": typ}] + elems}}
    if ctx:
        s["context"] = ctx
    return s


def e(id, **kw):
    d = {"id": id, "path": ".".join(p.split(":")[0] for p in id.split("."))}
    d.update(kw)
    return d


def op(*extra, value=None):
    v = {"type": code}
    v.update(value or {})
    return [e("Parameters.parameter", slicing=slc), e("Parameters.parameter:op", sliceName="op"),
            e("Parameters.parameter:op.name", fixedString="op"), e("Parameters.parameter:op.value[x]", **v), *extra]


SDS = [
    sd("p-t1", "Parameters", [e("Parameters.parameter.value[x]", type=code)]),
    sd("p-t2", "Parameters", op()),
    sd("p-t3", "Parameters", [e("Parameters.parameter.part.value[x]", type=code)]),
    sd("p-t4", "Parameters", [e("Parameters.parameter.part", slicing=slc), e("Parameters.parameter.part:t", sliceName="t"),
                              e("Parameters.parameter.part:t.name", fixedString="t"), e("Parameters.parameter.part:t.value[x]", type=code)]),
    sd("o-t5", "Observation", [e("Observation.value[x]", type=qty)]),
    sd("p-t6", "Parameters", op(value={"fixedCode": "abc"})),
    sd("o-t7", "Observation", [e("Observation.component", slicing={"discriminator": [{"type": "pattern", "path": "code"}], "rules": "open"}),
                               e("Observation.component:sis", sliceName="sis"),
                               e("Observation.component:sis.code", patternCodeableConcept={"coding": [{"system": "http://loinc.org", "code": "8480-6"}]}),
                               e("Observation.component:sis.value[x]", type=qty)]),
    sd("p-t8", "Parameters", [e("Parameters.parameter", slicing=slc), e("Parameters.parameter.value[x]", type=code),
                              e("Parameters.parameter:op", sliceName="op"), e("Parameters.parameter:op.name", fixedString="op"),
                              e("Parameters.parameter:op.value[x]", type=code)]),
    sd("p-t9", "Parameters", [e("Parameters.parameter", slicing=slc), e("Parameters.parameter.value[x]", type=[{"code": "code"}, {"code": "string"}]),
                              e("Parameters.parameter:op", sliceName="op"), e("Parameters.parameter:op.name", fixedString="op"),
                              e("Parameters.parameter:op.value[x]", type=code)]),
    sd("ext-t10", "Extension", [e("Extension.extension", slicing={"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}),
                                e("Extension.extension:s", sliceName="s"), e("Extension.extension:s.url", fixedUri="s"),
                                e("Extension.extension:s.value[x]", type=[{"code": "Coding"}]),
                                e("Extension.url", fixedUri=B + "ext-t10"), e("Extension.value[x]", max="0")],
       kind="complex-type", ctx=[{"type": "element", "expression": "Patient"}]),
    sd("ext-t10b", "Extension", [e("Extension.extension", max="0"), e("Extension.url", fixedUri=B + "ext-t10b"),
                                 e("Extension.value[x]", type=[{"code": "Coding"}])],
       kind="complex-type", ctx=[{"type": "element", "expression": "Patient"}]),
    # H3: dos perfiles con tipos que no se contienen.
    sd("p-ha", "Parameters", [e("Parameters.parameter.value[x]", type=[{"code": "code"}, {"code": "string"}])]),
    sd("p-hb", "Parameters", [e("Parameters.parameter.value[x]", type=[{"code": "code"}, {"code": "boolean"}])]),
    # H5: binding requerido y cardinalidad en el slice; discriminador por tipo.
    sd("p-t16", "Parameters", op(value={"binding": {"strength": "required", "valueSet": "http://hl7.org/fhir/ValueSet/administrative-gender"}})),
    sd("p-t17", "Parameters", op(value={"min": 1})),
    sd("p-t18", "Parameters", [e("Parameters.parameter", slicing={"discriminator": [{"type": "type", "path": "value"}], "rules": "open"}),
                               e("Parameters.parameter:c", sliceName="c"), e("Parameters.parameter:c.value[x]", type=code)]),
    # Mutación m1 (paso 7): un slice del propio value[x] que recibe un valor de otro tipo (discriminador exists).
    sd("p-t19", "Parameters", [e("Parameters.parameter.value[x]", slicing={"discriminator": [{"type": "exists", "path": "system"}], "rules": "open"}),
                               e("Parameters.parameter.value[x]:q", sliceName="q", type=qty),
                               e("Parameters.parameter.value[x]:q.system", min=1)]),
    # Mutación m10 (ronda 1): el perfil despliega los hijos del value[x]; un valor de otro tipo se recorre con los de su tipo.
    sd("p-t20", "Parameters", [e("Parameters.parameter.value[x]", type=qty), e("Parameters.parameter.value[x].system", min=1)]),
    # Mutación m9 (ronda 1): un perfil de tipo de datos (no de recurso) que restringe un elemento de elección.
    sd("t-t21", "Timing", [e("Timing.repeat.bounds[x]", type=[{"code": "Period"}])], kind="complex-type"),
    sd("p-t21", "Parameters", [e("Parameters.parameter.value[x]", type=[{"code": "Timing", "profile": [B + "t-t21"]}])]),
]


def params(id, prof, par, profiles=None):
    return {"resourceType": "Parameters", "id": id, "meta": {"profile": profiles or [B + prof]}, "parameter": par}


def obs(id, prof, **kw):
    o = {"resourceType": "Observation", "id": id, "meta": {"profile": [B + prof]}, "status": "final", "code": {"text": "x"}}
    o.update(kw)
    return o


sis = {"coding": [{"system": "http://loinc.org", "code": "8480-6"}]}
q = {"value": 1, "unit": "mm[Hg]", "system": "http://unitsofmeasure.org", "code": "mm[Hg]"}
t2 = params("t2", "p-t2", [{"name": "op", "valueString": "a"}])
INS = [
    params("t1", "p-t1", [{"name": "x", "valueString": "a"}]),
    t2,
    params("t2ok", "p-t2", [{"name": "op", "valueCode": "a"}]),
    params("t2other", "p-t2", [{"name": "other", "valueString": "a"}]),
    params("t3", "p-t3", [{"name": "x", "part": [{"name": "y", "valueString": "a"}]}]),
    params("t4", "p-t4", [{"name": "x", "part": [{"name": "t", "valueString": "a"}]}]),
    obs("t5", "o-t5", valueString="a"),
    params("t6a", "p-t6", [{"name": "op", "valueCode": "zzz"}]),
    params("t6b", "p-t6", [{"name": "op", "valueString": "abc"}]),
    obs("t7", "o-t7", component=[{"code": sis, "valueString": "a"}]),
    obs("t7ok", "o-t7", component=[{"code": sis, "valueQuantity": q}]),
    params("t8", "p-t8", [{"name": "op", "valueString": "a"}]),
    params("t9", "p-t9", [{"name": "op", "valueString": "a"}]),
    {"resourceType": "Patient", "id": "t10", "extension": [{"url": B + "ext-t10", "extension": [{"url": "s", "valueString": "a"}]}]},
    {"resourceType": "Patient", "id": "t10b", "extension": [{"url": B + "ext-t10b", "valueString": "a"}]},
    # H1: solo la propiedad _ de un primitivo.
    params("t11", "p-t2", [{"name": "op", "_valueString": {"id": "v"}}]),
    # H4: dos variantes juntas; en un Bundle y en contained.
    params("t12", "p-t1", [{"name": "x", "valueCode": "a", "valueString": "a"}]),
    {"resourceType": "Bundle", "id": "t12b", "type": "collection", "entry": [{"fullUrl": "urn:uuid:3f1c1b4e-2a5f-4c55-9d8e-0d7a8a6f1b11", "resource": params("t12be", "p-t2", [{"name": "op", "valueString": "a"}])}]},
    {"resourceType": "List", "id": "t12c", "status": "current", "mode": "working", "contained": [params("c1", "p-t2", [{"name": "op", "valueString": "a"}])], "entry": [{"item": {"reference": "#c1"}}]},
    # H3: dos perfiles disjuntos.
    params("t13", None, [{"name": "x", "valueInteger": 1}], profiles=[B + "p-ha", B + "p-hb"]),
    # H5: tipo equivocado con fixed distinto, primitivo mal formado, binding, cardinalidad, discriminador por tipo.
    params("t14", "p-t6", [{"name": "op", "valueString": "zzz"}]),
    params("t15", "p-t2", [{"name": "op", "valueString": 123}]),
    params("t16", "p-t16", [{"name": "op", "valueString": "foo"}]),
    params("t17", "p-t17", [{"name": "op", "valueString": "a"}]),
    params("t18", "p-t18", [{"name": "x", "valueString": "a"}]),
    params("t18ok", "p-t18", [{"name": "x", "valueCode": "a"}]),
    params("t19", "p-t19", [{"name": "x", "valueCoding": {"system": "http://example.org/cs", "code": "a"}}]),
    params("t19ok", "p-t19", [{"name": "x", "valueQuantity": {"value": 1, "system": "http://unitsofmeasure.org", "code": "mg"}}]),
    params("t19none", "p-t19", [{"name": "x", "valueCoding": {"code": "a"}}]),
    params("t20", "p-t20", [{"name": "x", "valueCodeableConcept": {"coding": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/x"}]}]}}]),
    params("t21", "p-t21", [{"name": "x", "valueTiming": {"repeat": {"boundsPeriod": {"start": "2020-01-01"}, "boundsDuration": {"value": 1, "unit": "d", "system": "http://unitsofmeasure.org", "code": "d"}}}}]),
    params("t21one", "p-t21", [{"name": "x", "valueTiming": {"repeat": {"boundsDuration": {"value": 1, "unit": "d", "system": "http://unitsofmeasure.org", "code": "d"}}}}]),
]

here = pathlib.Path(".")
(here / "in").mkdir(exist_ok=True)
for i in INS:
    (here / "in" / f"{i['id']}.json").write_text(json.dumps(i, indent=1))
for v, core in VERSIONS.items():
    p = here / f"pkg-{v}" / "package"
    p.mkdir(parents=True, exist_ok=True)
    for s in SDS:
        (p / f"StructureDefinition-{s['id']}.json").write_text(json.dumps(s, indent=1))
    (p / "package.json").write_text(json.dumps({"name": "example.slicetypes", "version": "0.1.0", "fhirVersions": [v], "type": "fhir.ig", "dependencies": {core: v}}))
