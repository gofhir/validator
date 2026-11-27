import json, os, sys
exec(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "gen.py")).read().split("# P2:")[0])
# type profile with root and child constraints (not unrolled)
sd("q-idp", "Identifier", C + "Identifier", [{"id": "Identifier", "path": "Identifier", "constraint": [con("qr-1", "system.exists()")]},
   {"id": "Identifier.value", "path": "Identifier.value", "constraint": [con("qc-1", "startsWith('A')")]}], kind="complex-type")
sd("q-pat", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient"},
   {"id": "Patient.identifier", "path": "Patient.identifier", "type": [{"code": "Identifier", "profile": [B + "q-idp"]}]}])
inst("q-i", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "q-pat"]}, "identifier": [{"value": "B"}]})
# extension profile with root and value constraints
sd("q-ext", "Extension", C + "Extension", [{"id": "Extension", "path": "Extension", "constraint": [con("er-1", "extension.exists() or value.exists().not() or value.startsWith('A')")]},
   {"id": "Extension.url", "path": "Extension.url", "fixedUri": B + "q-ext"},
   {"id": "Extension.value[x]", "path": "Extension.value[x]", "type": [{"code": "string"}], "constraint": [con("ec-1", "startsWith('Z')")]}],
   kind="complex-type", ctx=[{"type": "element", "expression": "Patient"}, {"type": "element", "expression": "Resource"}])
sd("q-pat-ext", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient"},
   {"id": "Patient.extension", "path": "Patient.extension", "slicing": {"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}},
   {"id": "Patient.extension:e", "path": "Patient.extension", "sliceName": "e", "min": 0, "max": "1", "type": [{"code": "Extension", "profile": [B + "q-ext"]}]}])
inst("q-ext-slice", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "q-pat-ext"]}, "extension": [{"url": B + "q-ext", "valueString": "B"}]})
inst("q-ext-bare", {"resourceType": "Patient", "id": "a", "extension": [{"url": B + "q-ext", "valueString": "B"}]})
# R5/R4B: same p3-plain
sd("p3-idp", "Identifier", C + "Identifier", [{"id": "Identifier", "path": "Identifier"},
   {"id": "Identifier.value", "path": "Identifier.value", "constraint": [con("idp-1", "startsWith('A')")]}], kind="complex-type")
sd("p3-pat-plain", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient"},
   {"id": "Patient.identifier", "path": "Patient.identifier", "type": [{"code": "Identifier", "profile": [B + "p3-idp"]}]}])
inst("p3-pat-plain-i", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "p3-pat-plain"]}, "identifier": [{"system": "urn:x", "value": "B"}]})
sd("p2-pat", "Patient", C + "Patient", [{"id": "Patient", "path": "Patient"}, {"id": "Patient.active", "path": "Patient.active", "short": "x"}])
inst("p2-noprofile", {"resourceType": "Patient", "id": "a", "contact": [{"gender": "male"}]})
