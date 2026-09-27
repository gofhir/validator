"""Generate the Plan A / PR A0 decision probes: differential-only profiles plus instances.

Snapshots are NOT written here; they are produced by the HL7 validator (-snapshot) so that the
probes do not depend on gofhir's own snapshot generator. Every name is invented (acme).

Usage: python3 gen_acme_decisions.py <outdir>
"""
import json, os, sys

OUT = sys.argv[1]
os.makedirs(f"{OUT}/defs", exist_ok=True)
os.makedirs(f"{OUT}/instances", exist_ok=True)
B = "http://acme-health.test/fhir"
SDU = f"{B}/StructureDefinition"
NARR = {"status": "generated", "div": '<div xmlns="http://www.w3.org/1999/xhtml">x</div>'}


def write(kind, name, obj):
    with open(f"{OUT}/{kind}/{name}.json", "w") as f:
        json.dump(obj, f, indent=1)


def profile(pid, base_type, elements, base=None):
    write("defs", f"StructureDefinition-{pid}", {
        "resourceType": "StructureDefinition", "id": pid, "url": f"{SDU}/{pid}", "version": "0.2.0",
        "name": pid.replace("-", "").title(), "status": "draft", "fhirVersion": "4.0.1",
        "kind": "resource", "abstract": False, "type": base_type,
        "baseDefinition": base or f"http://hl7.org/fhir/StructureDefinition/{base_type}",
        "derivation": "constraint", "differential": {"element": elements}})


def extension_def(eid, value_type):
    write("defs", f"StructureDefinition-{eid}", {
        "resourceType": "StructureDefinition", "id": eid, "url": f"{SDU}/{eid}", "version": "0.2.0",
        "name": eid.replace("-", "").title(), "status": "draft", "fhirVersion": "4.0.1",
        "kind": "complex-type", "abstract": False, "context": [{"type": "element", "expression": "Patient"}],
        "type": "Extension", "baseDefinition": "http://hl7.org/fhir/StructureDefinition/Extension",
        "derivation": "constraint", "differential": {"element": [
            {"id": "Extension", "path": "Extension"},
            {"id": "Extension.extension", "path": "Extension.extension", "max": "0"},
            {"id": "Extension.url", "path": "Extension.url", "fixedUri": f"{SDU}/{eid}"},
            {"id": "Extension.value[x]", "path": "Extension.value[x]", "min": 1, "type": [{"code": value_type}]}]}})


def patient(name, prof, **fields):
    r = {"resourceType": "Patient", "id": name.lower().replace("_", "-")[:60], "meta": {"profile": [f"{SDU}/{prof}"]},
         "text": NARR}
    r.update(fields)
    write("instances", name, r)


def slicing(path, disc, rules="open", ordered=False):
    return {"id": path, "path": path, "slicing": {"discriminator": disc, "ordered": ordered, "rules": rules}}


def slice_(path, name, mn=0, mx="*", **extra):
    e = {"id": f"{path}:{name}", "path": path, "sliceName": name, "min": mn, "max": mx}
    e.update(extra)
    return e


SYS = f"{B}/sid/mrn"
SYS_B = f"{B}/sid/other"

# ---------------------------------------------------------------- Q1 multi-match per discriminator type
# value: both slices fix the same system; B additionally requires use=official (not a discriminator)
profile("mm-value", "Patient", [
    slicing("Patient.identifier", [{"type": "value", "path": "system"}]),
    slice_("Patient.identifier", "A", 0, "1"),
    {"id": "Patient.identifier:A.system", "path": "Patient.identifier.system", "min": 1, "fixedUri": SYS},
    slice_("Patient.identifier", "B", 1, "1"),
    {"id": "Patient.identifier:B.system", "path": "Patient.identifier.system", "min": 1, "fixedUri": SYS},
])
patient("Q1_value_one", "mm-value", identifier=[{"system": SYS, "value": "1"}])

# pattern: overlapping patterns on $this (A is a subset of B)
profile("mm-pattern", "Patient", [
    slicing("Patient.identifier", [{"type": "pattern", "path": "$this"}]),
    slice_("Patient.identifier", "A", 0, "1", patternIdentifier={"system": SYS}),
    slice_("Patient.identifier", "B", 1, "1", patternIdentifier={"system": SYS, "use": "official"}),
])
patient("Q1_pattern_one", "mm-pattern", identifier=[{"system": SYS, "use": "official", "value": "1"}])

# exists: both slices require period
profile("mm-exists", "Patient", [
    slicing("Patient.identifier", [{"type": "exists", "path": "period"}]),
    slice_("Patient.identifier", "A", 0, "1"),
    {"id": "Patient.identifier:A.period", "path": "Patient.identifier.period", "min": 1},
    slice_("Patient.identifier", "B", 1, "1"),
    {"id": "Patient.identifier:B.period", "path": "Patient.identifier.period", "min": 1},
])
patient("Q1_exists_one", "mm-exists", identifier=[{"system": SYS, "value": "1", "period": {"start": "2020-01-01"}}])

# type: Observation.component sliced by type of value[x]; both slices allow Quantity
profile("mm-type", "Observation", [
    slicing("Observation.component", [{"type": "type", "path": "value"}]),
    slice_("Observation.component", "A", 0, "1"),
    {"id": "Observation.component:A.value[x]", "path": "Observation.component.value[x]", "min": 1, "type": [{"code": "Quantity"}]},
    slice_("Observation.component", "B", 1, "1"),
    {"id": "Observation.component:B.value[x]", "path": "Observation.component.value[x]", "min": 1, "type": [{"code": "Quantity"}]},
])
write("instances", "Q1_type_one", {
    "resourceType": "Observation", "id": "q1-type-one", "meta": {"profile": [f"{SDU}/mm-type"]}, "text": NARR,
    "status": "final", "code": {"text": "x"},
    "component": [{"code": {"text": "c"}, "valueQuantity": {"value": 1}}]})

# profile: Bundle.entry sliced by profile of resource; two permissive Patient profiles
profile("pat-p1", "Patient", [{"id": "Patient", "path": "Patient"}])
profile("pat-p2", "Patient", [{"id": "Patient", "path": "Patient"}])
profile("mm-profile", "Bundle", [
    slicing("Bundle.entry", [{"type": "profile", "path": "resource"}]),
    slice_("Bundle.entry", "A", 0, "1"),
    {"id": "Bundle.entry:A.resource", "path": "Bundle.entry.resource", "min": 1,
     "type": [{"code": "Patient", "profile": [f"{SDU}/pat-p1"]}]},
    slice_("Bundle.entry", "B", 1, "1"),
    {"id": "Bundle.entry:B.resource", "path": "Bundle.entry.resource", "min": 1,
     "type": [{"code": "Patient", "profile": [f"{SDU}/pat-p2"]}]},
])
write("instances", "Q1_profile_one", {
    "resourceType": "Bundle", "id": "q1-profile-one", "meta": {"profile": [f"{SDU}/mm-profile"]}, "type": "collection",
    "entry": [{"fullUrl": f"{B}/Patient/p1", "resource": {"resourceType": "Patient", "id": "p1", "text": NARR}}]})

# ---------------------------------------------------------------- Q2 pinned version absent, other loaded
extension_def("ext-a", "string")
profile("ver-fallback", "Patient", [
    slicing("Patient.extension", [{"type": "value", "path": "url"}]),
    slice_("Patient.extension", "a", 0, "1", type=[{"code": "Extension", "profile": [f"{SDU}/ext-a|9.9.9"]}]),
])
patient("Q2_version_two_ext", "ver-fallback",
        extension=[{"url": f"{SDU}/ext-a", "valueString": "1"}, {"url": f"{SDU}/ext-a", "valueString": "2"}])

# ---------------------------------------------------------------- Q3 unresolvable slice profile
profile("unresolvable", "Patient", [
    slicing("Patient.extension", [{"type": "value", "path": "url"}]),
    slice_("Patient.extension", "missing", 1, "1", type=[{"code": "Extension", "profile": [f"{SDU}/ext-missing"]}]),
])
patient("Q3_unresolvable_present", "unresolvable", extension=[{"url": f"{SDU}/ext-missing", "valueString": "x"}])
patient("Q3_unresolvable_absent", "unresolvable")

# ---------------------------------------------------------------- Q4 ordered slicing out of order
profile("ordered", "Patient", [
    slicing("Patient.identifier", [{"type": "value", "path": "system"}], ordered=True),
    slice_("Patient.identifier", "first", 0, "1"),
    {"id": "Patient.identifier:first.system", "path": "Patient.identifier.system", "min": 1, "fixedUri": SYS},
    slice_("Patient.identifier", "second", 0, "1"),
    {"id": "Patient.identifier:second.system", "path": "Patient.identifier.system", "min": 1, "fixedUri": SYS_B},
])
patient("Q4_ordered_wrong", "ordered", identifier=[{"system": SYS_B, "value": "2"}, {"system": SYS, "value": "1"}])
patient("Q4_ordered_right", "ordered", identifier=[{"system": SYS, "value": "1"}, {"system": SYS_B, "value": "2"}])

# ---------------------------------------------------------------- Q5 openAtEnd with unmatched element first
# openAtEnd presupposes ordered slices (ElementDefinition.slicing.rules definition).
profile("open-at-end", "Patient", [
    slicing("Patient.identifier", [{"type": "value", "path": "system"}], rules="openAtEnd", ordered=True),
    slice_("Patient.identifier", "known", 0, "*"),
    {"id": "Patient.identifier:known.system", "path": "Patient.identifier.system", "min": 1, "fixedUri": SYS},
])
patient("Q5_openatend_wrong", "open-at-end", identifier=[{"system": f"{B}/sid/x", "value": "9"}, {"system": SYS, "value": "1"}])
patient("Q5_openatend_right", "open-at-end", identifier=[{"system": SYS, "value": "1"}, {"system": f"{B}/sid/x", "value": "9"}])

# ---------------------------------------------------------------- Q6 binding-based discriminator
CS = f"{B}/CodeSystem/kind"
write("defs", "CodeSystem-kind", {
    "resourceType": "CodeSystem", "id": "kind", "url": CS, "version": "0.2.0", "name": "Kind", "status": "draft",
    "content": "complete", "concept": [{"code": "a"}, {"code": "b"}, {"code": "c"}]})
write("defs", "ValueSet-kind-ab", {
    "resourceType": "ValueSet", "id": "kind-ab", "url": f"{B}/ValueSet/kind-ab", "version": "0.2.0", "name": "KindAB",
    "status": "draft", "compose": {"include": [{"system": CS, "concept": [{"code": "a"}, {"code": "b"}]}]}})
write("defs", "ValueSet-snomed-isa", {
    "resourceType": "ValueSet", "id": "snomed-isa", "url": f"{B}/ValueSet/snomed-isa", "version": "0.2.0",
    "name": "SnomedIsa", "status": "draft",
    "compose": {"include": [{"system": "http://snomed.info/sct", "filter": [{"property": "concept", "op": "is-a", "value": "404684003"}]}]}})
for pid, vs in (("binding-local", "kind-ab"), ("binding-external", "snomed-isa")):
    profile(pid, "Patient", [
        slicing("Patient.maritalStatus.coding", [{"type": "value", "path": "$this"}]),
        slice_("Patient.maritalStatus.coding", "inset", 1, "1",
               binding={"strength": "required", "valueSet": f"{B}/ValueSet/{vs}"}),
    ])
patient("Q6_binding_local_in", "binding-local", maritalStatus={"coding": [{"system": CS, "code": "a"}]})
patient("Q6_binding_local_out", "binding-local", maritalStatus={"coding": [{"system": CS, "code": "c"}]})
patient("Q6_binding_external", "binding-external",
        maritalStatus={"coding": [{"system": "http://snomed.info/sct", "code": "22298006"}]})

print("ok")
