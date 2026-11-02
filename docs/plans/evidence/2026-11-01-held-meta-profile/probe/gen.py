"""Genera las sondas de #188: un recurso contenido (entrada de Bundle, contained, Parameters.parameter.resource,
Bundle dentro de Bundle) contra el meta.profile que declara, en la fase estructural. Perfiles solo con
diferencial (pkg-<versión>/package) e instancias (in/), las mismas en R4, R4B y R5.
Uso: python3 -I gen.py (en esta carpeta)."""
import json, pathlib

B = "http://example.org/fhir/StructureDefinition/"
VERSIONS = {"4.0.1": "hl7.fhir.r4.core", "4.3.0": "hl7.fhir.r4b.core", "5.0.0": "hl7.fhir.r5.core"}


def sd(id, typ, elems):
    return {"resourceType": "StructureDefinition", "id": id, "url": B + id, "name": id.replace("-", "").title(),
            "status": "active", "kind": "resource", "abstract": False, "type": typ,
            "baseDefinition": "http://hl7.org/fhir/StructureDefinition/" + typ, "derivation": "constraint",
            "differential": {"element": [{"id": typ, "path": typ}] + elems}}


def e(id, **kw):
    d = {"id": id, "path": id}
    d.update(kw)
    return d


SDS = [
    # value[x] de un parámetro solo admite code.
    sd("p-code", "Parameters", [e("Parameters.parameter.value[x]", type=[{"code": "code"}])]),
    # value[x] admite code o string.
    sd("p-cs", "Parameters", [e("Parameters.parameter.value[x]", type=[{"code": "code"}, {"code": "string"}])]),
    # Bundle cuyas entradas son Parameters de p-code (type.profile del elemento que lo contiene).
    # value[x] cortado por tipo, cerrado: solo el slice string (hallazgo 4 de la ronda adversarial).
    sd("o-ts", "Observation", [e("Observation.value[x]", slicing={"discriminator": [{"type": "type", "path": "$this"}], "rules": "closed"}),
                               {"id": "Observation.value[x]:valueString", "path": "Observation.value[x]", "sliceName": "valueString",
                                "type": [{"code": "string"}]}]),
    sd("b-hold", "Bundle", [e("Bundle.entry.resource", type=[{"code": "Parameters", "profile": [B + "p-code"]}])]),
]

TWO = [{"name": "x", "valueCode": "a", "valueString": "a"}]   # dos variantes; el perfil admite solo una
ONE = [{"name": "x", "valueString": "a"}]                      # una variante que solo la base admite
REV = [{"name": "x", "valueString": "a", "valueCode": "a"}]   # TWO con las claves en el orden inverso


def params(id, par, profiles=("p-code",)):
    p = {"resourceType": "Parameters", "id": id, "parameter": par}
    if profiles:
        p["meta"] = {"profile": [B + x if not x.startswith("http") else x for x in profiles]}
    return p


def bundle(id, *resources, prof=None):
    b = {"resourceType": "Bundle", "id": id, "type": "collection",
         "entry": [{"fullUrl": "urn:uuid:3f1c1b4e-2a5f-4c55-9d8e-0d7a8a6f1b%02d" % i, "resource": r} for i, r in enumerate(resources)]}
    if prof:
        b["meta"] = {"profile": [B + prof]}
    return b


def held_list(id, r):
    return {"resourceType": "List", "id": id, "status": "current", "mode": "working", "contained": [r],
            "entry": [{"item": {"reference": "#" + r["id"]}}]}


INSTANCES = [
    params("r-two", TWO),                                        # raíz: referencia del comportamiento esperado
    params("r-one", ONE),
    params("r-none-two", TWO, profiles=()),                      # control: sin perfil
    params("r-rev", REV),                                        # ¿orden o perfil decide la variante que sobra?
    params("r-none-rev", REV, profiles=()),
    bundle("b-rev", params("e1", REV)),
    bundle("b-two", params("e1", TWO)),
    bundle("b-one", params("e1", ONE)),
    bundle("b-none-two", params("e1", TWO, profiles=())),        # control en una entrada
    bundle("b-cs-two", params("e1", TWO, profiles=("p-cs",))),   # perfil que admite las dos variantes
    bundle("b-two-profiles", params("e1", TWO, profiles=("p-code", "p-cs"))),
    bundle("b-unresolved", params("e1", TWO, profiles=("http://example.org/fhir/StructureDefinition/nope",))),
    held_list("c-two", params("c1", TWO)),
    held_list("c-one", params("c1", ONE)),
    {"resourceType": "Parameters", "id": "pr-two", "parameter": [{"name": "r", "resource": params("pr1", TWO)}]},
    bundle("bb-two", bundle("inner", params("e1", TWO))),
    bundle("h-two", params("e1", TWO, profiles=()), prof="b-hold"),  # perfil por type.profile del contenedor
]



def obs(id, prof=None, **kw):
    o = {"resourceType": "Observation", "id": id, "status": "final", "code": {"text": "x"}}
    if prof:
        o["meta"] = {"profile": [B + prof]}
    o.update(kw)
    return o


EXT = "http://hl7.org/fhir/StructureDefinition/patient-mothersMaidenName"
# Hallazgos 1-4 de la ronda adversarial: hermanos "_", tres variantes, otros elementos y tipos.
INSTANCES += [
    obs("o-u1", valueString="a", _valueTime={"id": "t"}),
    obs("o-u2", _valueString={"id": "s"}, _valueTime={"id": "t"}),
    {"resourceType": "Patient", "id": "e-u1", "extension": [{"url": EXT, "valueString": "a", "_valueCode": {"id": "c"}}]},
    {"resourceType": "Patient", "id": "e-two", "extension": [{"url": EXT, "valueString": "a", "valueBoolean": True}]},
    params("r-three", [{"name": "x", "valueCode": "a", "valueString": "a", "valueBoolean": True}], profiles=()),
    obs("o-qs", valueQuantity={"value": 1}, valueString="a"),
    {"resourceType": "Patient", "id": "p-dec", "deceasedBoolean": True, "deceasedDateTime": "2020-01-01"},
    obs("o-timing", effectiveTiming={"repeat": {"boundsDuration": {"value": 1}, "boundsPeriod": {"start": "2020-01-01"}}}),
    {"resourceType": "Questionnaire", "id": "q-cr", "status": "draft", "item": [{"linkId": "1", "type": "group", "item": [
        {"linkId": "1.1", "type": "boolean", "enableWhen": [{"question": "1", "operator": "=", "answerBoolean": True, "answerString": "a"}]}]}]},
    obs("o-ts-two", prof="o-ts", valueString="a", valueBoolean=True),
]

here = pathlib.Path(__file__).parent
for v, core in VERSIONS.items():
    pkg = here / f"pkg-{v}" / "package"
    pkg.mkdir(parents=True, exist_ok=True)
    (pkg / "package.json").write_text(json.dumps({"name": "example.choice-variants", "version": "0.1.0", "fhirVersions": [v],
                                                  "dependencies": {core: v}}, indent=1))
    for s in SDS:
        s = dict(s, fhirVersion=v)
        (pkg / f"StructureDefinition-{s['id']}.json").write_text(json.dumps(s, indent=1))
inp = here / "in"
inp.mkdir(exist_ok=True)
for i in INSTANCES:
    (inp / f"{i['id']}.json").write_text(json.dumps(i, indent=1))
print(len(SDS), "perfiles,", len(INSTANCES), "instancias")
