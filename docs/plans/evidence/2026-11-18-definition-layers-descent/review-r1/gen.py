import json, os, sys
W = sys.argv[1]
V = sys.argv[2] if len(sys.argv) > 2 else "4.0.1"
os.makedirs(W + "/ig", exist_ok=True); os.makedirs(W + "/inst", exist_ok=True)
B = "http://example.org/rv8/"
def sd(name, typ, base, elems, kind="resource", ctx=None):
    d = {"resourceType": "StructureDefinition", "id": name, "url": B + name, "name": name.replace("-", ""),
         "status": "active", "fhirVersion": V, "kind": kind, "abstract": False, "type": typ,
         "baseDefinition": base, "derivation": "constraint", "differential": {"element": elems}}
    if ctx: d["context"] = ctx
    json.dump(d, open(f"{W}/ig/{name}.json", "w"), indent=1)
def inst(name, o):
    json.dump(o, open(f"{W}/inst/{name}.json", "w"), indent=1)
C = "http://hl7.org/fhir/StructureDefinition/"
def con(key, expr, sev="error", human=None):
    return {"key": key, "severity": sev, "human": human or key, "expression": expr}

# P2: explicit profile, pat-1 (core) suffix
sd("p2-pat", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient"}, {"id": "Patient.active", "path": "Patient.active", "short": "x"}])
inst("p2-noprofile", {"resourceType": "Patient", "id": "a", "contact": [{"gender": "male"}]})
inst("p2-meta", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "p2-pat"]}, "contact": [{"gender": "male"}]})

# P3: type profile on Identifier with a constraint on the child value; profile P unrolls (descends) or not
sd("p3-idp", "Identifier", C + "Identifier", [{"id": "Identifier", "path": "Identifier"},
   {"id": "Identifier.value", "path": "Identifier.value", "constraint": [con("idp-1", "startsWith('A')")]}], kind="complex-type")
sd("p3-pat-unroll", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient"},
   {"id": "Patient.identifier", "path": "Patient.identifier", "type": [{"code": "Identifier", "profile": [B + "p3-idp"]}]},
   {"id": "Patient.identifier.value", "path": "Patient.identifier.value", "min": 1}])
sd("p3-pat-plain", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient"},
   {"id": "Patient.identifier", "path": "Patient.identifier", "type": [{"code": "Identifier", "profile": [B + "p3-idp"]}]}])
for p in ["p3-pat-unroll", "p3-pat-plain"]:
    inst(p + "-i", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + p]}, "identifier": [{"system": "urn:x", "value": "B"}]})

# P5: chain of three profiles; constraint added in the first one
sd("p5-a", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient", "constraint": [con("pa-1", "name.exists()")]}])
sd("p5-b", "Patient", B + "p5-a", [{"id": "Patient", "path": "Patient"}, {"id": "Patient.active", "path": "Patient.active", "short": "b"}])
sd("p5-c", "Patient", B + "p5-b", [{"id": "Patient", "path": "Patient"}, {"id": "Patient.gender", "path": "Patient.gender", "short": "c"}])
inst("p5-c-i", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "p5-c"]}})
inst("p5-b-i", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "p5-b"]}})

# P7: slicing with no discriminator; slice fixes system and has an invariant
sd("p7-pat", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient"},
   {"id": "Patient.identifier", "path": "Patient.identifier", "slicing": {"rules": "open", "ordered": False}},
   {"id": "Patient.identifier:s1", "path": "Patient.identifier", "sliceName": "s1", "min": 0, "max": "1",
    "constraint": [con("s1-1", "value.startsWith('A')")]},
   {"id": "Patient.identifier:s1.system", "path": "Patient.identifier.system", "fixedUri": "urn:a"}])
inst("p7-match", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "p7-pat"]}, "identifier": [{"system": "urn:a", "value": "A1"}]})
inst("p7-inv", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "p7-pat"]}, "identifier": [{"system": "urn:a", "value": "B1"}]})
inst("p7-other", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "p7-pat"]}, "identifier": [{"system": "urn:b", "value": "B1"}]})
# P7b: closed slicing, slice restricted by a fixed system and min=1 on value with a type-level pattern on a child
sd("p7b-pat", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient"},
   {"id": "Patient.identifier", "path": "Patient.identifier", "slicing": {"rules": "closed", "ordered": False}},
   {"id": "Patient.identifier:s1", "path": "Patient.identifier", "sliceName": "s1", "min": 0, "max": "*"},
   {"id": "Patient.identifier:s1.system", "path": "Patient.identifier.system", "fixedUri": "urn:a"},
   {"id": "Patient.identifier:s1.value", "path": "Patient.identifier.value", "min": 1}])
inst("p7b-novalue", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "p7b-pat"]}, "identifier": [{"system": "urn:a"}]})
# P7c: two slices, no discriminator; s1 fixes system urn:a; s2 fixes nothing but requires use=official via invariant
sd("p7c-pat", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient"},
   {"id": "Patient.identifier", "path": "Patient.identifier", "slicing": {"rules": "closed", "ordered": False}},
   {"id": "Patient.identifier:s1", "path": "Patient.identifier", "sliceName": "s1", "min": 0, "max": "*"},
   {"id": "Patient.identifier:s1.system", "path": "Patient.identifier.system", "fixedUri": "urn:a"},
   {"id": "Patient.identifier:s2", "path": "Patient.identifier", "sliceName": "s2", "min": 0, "max": "*", "constraint": [con("s2-1", "use = 'official'")]}])
inst("p7c-s2", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "p7c-pat"]}, "identifier": [{"use": "official", "system": "urn:z", "value": "1"}]})
inst("p7c-none", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "p7c-pat"]}, "identifier": [{"use": "usual", "system": "urn:z", "value": "1"}]})

# P6: held resource, base entry.resource profile X + slice profile Y
sd("p6-x", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient", "constraint": [con("px-1", "active.exists()")]}])
sd("p6-y", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient", "constraint": [con("py-1", "gender.exists()")]}])
sd("p6-z", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient", "constraint": [con("pz-1", "birthDate.exists()")]}])
sd("p6-bundle", "Bundle", C + "Bundle", [{"id": "Bundle", "path": "Bundle"},
   {"id": "Bundle.entry", "path": "Bundle.entry", "slicing": {"discriminator": [{"type": "type", "path": "resource"}], "rules": "open"}},
   {"id": "Bundle.entry.resource", "path": "Bundle.entry.resource", "type": [{"code": "Patient", "profile": [B + "p6-x"]}]},
   {"id": "Bundle.entry:a", "path": "Bundle.entry", "sliceName": "a", "min": 0, "max": "*"},
   {"id": "Bundle.entry:a.resource", "path": "Bundle.entry.resource", "type": [{"code": "Patient", "profile": [B + "p6-y"]}]}])
inst("p6-i", {"resourceType": "Bundle", "id": "b", "meta": {"profile": [B + "p6-bundle"]}, "type": "collection",
   "entry": [{"fullUrl": "urn:uuid:1d5b3c2e-0000-4000-8000-000000000001", "resource": {"resourceType": "Patient", "id": "p"}}]})
