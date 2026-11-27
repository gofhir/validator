# Sondas de la ronda 1 del code review (F5): una por mutación sobreviviente (F, L, B, K). R4. Uso: gen3.py <carpeta>
import json, os, sys
exec(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "gen.py")).read().split("# P2:")[0])
ROOT = {"id": "Patient", "path": "Patient"}
CT = [{"type": "element", "expression": "Patient"}]

# F: primitivo dividido (Patient.name.given:a). El elemento base y el slice declaran la misma clave (pfe-1) con otra
# expresión y otro texto en su extensión: el hijo del slice cede ante la del diferencial del slice, y la del elemento
# base llega al _given solo por la cadena SliceOf.
sd("pf-1", "Patient", C + "Patient", [ROOT,
   {"id": "Patient.name.given", "path": "Patient.name.given", "slicing": {"discriminator": [{"type": "value", "path": "$this"}], "rules": "open"}},
   {"id": "Patient.name.given.extension", "path": "Patient.name.given.extension", "constraint": [con("pfe-1", "url = 'urn:base'", human="pfe base")]},
   {"id": "Patient.name.given:a", "path": "Patient.name.given", "sliceName": "a", "min": 0, "max": "*", "fixedString": "A"},
   {"id": "Patient.name.given:a.extension", "path": "Patient.name.given.extension", "constraint": [con("pfe-1", "url = 'urn:slice'", human="pfe slice")]}])
inst("pf-i", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "pf-1"]},
   "name": [{"given": ["A"], "_given": [{"extension": [{"url": "urn:x", "valueString": "v"}]}]}]})

# L1: el slice declara dos perfiles de tipo y tiene sus hijos; el elemento base y el slice declaran la misma clave (plb-1)
# con otra expresión y otro texto en value: la del elemento base llega por la cadena SliceOf.
sd("pl-idpa", "Identifier", C + "Identifier", [{"id": "Identifier", "path": "Identifier"}], kind="complex-type")
sd("pl-idpb", "Identifier", C + "Identifier", [{"id": "Identifier", "path": "Identifier"}], kind="complex-type")
sd("pl-1", "Patient", C + "Patient", [ROOT,
   {"id": "Patient.identifier", "path": "Patient.identifier", "slicing": {"discriminator": [{"type": "value", "path": "system"}], "rules": "open"}},
   {"id": "Patient.identifier.value", "path": "Patient.identifier.value", "constraint": [con("plb-1", "startsWith('B')", human="plb base")]},
   {"id": "Patient.identifier:s", "path": "Patient.identifier", "sliceName": "s", "min": 0, "max": "*", "type": [{"code": "Identifier", "profile": [B + "pl-idpa", B + "pl-idpb"]}]},
   {"id": "Patient.identifier:s.system", "path": "Patient.identifier.system", "fixedUri": "urn:s"},
   {"id": "Patient.identifier:s.value", "path": "Patient.identifier.value", "constraint": [con("plb-1", "startsWith('C')", human="plb slice")]}])
inst("pl-i", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "pl-1"]}, "identifier": [{"system": "urn:s", "value": "X"}]})

# L2: el elemento declara dos perfiles de tipo y la extensión nombra su propia definición (pl-ext), que no es ninguno.
sd("pl-ext", "Extension", C + "Extension", [{"id": "Extension", "path": "Extension"},
   {"id": "Extension.url", "path": "Extension.url", "fixedUri": B + "pl-ext"},
   {"id": "Extension.value[x]", "path": "Extension.value[x]", "type": [{"code": "string"}], "constraint": [con("plx-1", "startsWith('Z')")]}], kind="complex-type", ctx=CT)
sd("pl-xa", "Extension", C + "Extension", [{"id": "Extension", "path": "Extension"}], kind="complex-type", ctx=CT)
sd("pl-xb", "Extension", C + "Extension", [{"id": "Extension", "path": "Extension"}], kind="complex-type", ctx=CT)
sd("pl-3", "Patient", C + "Patient", [ROOT,
   {"id": "Patient.extension", "path": "Patient.extension", "type": [{"code": "Extension", "profile": [B + "pl-xa", B + "pl-xb"]}]}])
inst("pl2-i", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "pl-3"]}, "extension": [{"url": B + "pl-ext", "valueString": "B"}]})

# B: el discriminador pattern lee el pattern que el perfil de tipo del slice declara para type.
sd("pb-idp", "Identifier", C + "Identifier", [{"id": "Identifier", "path": "Identifier"},
   {"id": "Identifier.type", "path": "Identifier.type", "patternCodeableConcept": {"coding": [{"system": "http://terminology.hl7.org/CodeSystem/v2-0203", "code": "MR"}]}}], kind="complex-type")
sd("pb-pat", "Patient", C + "Patient", [ROOT,
   {"id": "Patient.identifier", "path": "Patient.identifier", "slicing": {"discriminator": [{"type": "pattern", "path": "type"}], "rules": "open"}},
   {"id": "Patient.identifier.type", "path": "Patient.identifier.type", "short": "t"},
   {"id": "Patient.identifier:mrn", "path": "Patient.identifier", "sliceName": "mrn", "min": 0, "max": "*", "type": [{"code": "Identifier", "profile": [B + "pb-idp"]}],
    "constraint": [con("pbm-1", "value.startsWith('A')")]},
   {"id": "Patient.identifier:mrn.value", "path": "Patient.identifier.value", "short": "v"}])
inst("pb-i", {"resourceType": "Patient", "id": "a", "meta": {"profile": [B + "pb-pat"]},
   "identifier": [{"type": {"coding": [{"system": "http://terminology.hl7.org/CodeSystem/v2-0203", "code": "MR"}]}, "value": "X"}]})

# K: extensión compleja con un nivel más hondo que el de la definición base de Extension (Extension.extension.extension).
sd("pk-ext", "Extension", C + "Extension", [{"id": "Extension", "path": "Extension"},
   {"id": "Extension.extension", "path": "Extension.extension", "slicing": {"discriminator": [{"type": "value", "path": "url"}], "rules": "open"}},
   {"id": "Extension.extension:sub", "path": "Extension.extension", "sliceName": "sub", "min": 0, "max": "*"},
   {"id": "Extension.extension:sub.extension", "path": "Extension.extension.extension", "short": "deeper"},
   {"id": "Extension.extension:sub.url", "path": "Extension.extension.url", "fixedUri": "sub"},
   {"id": "Extension.url", "path": "Extension.url", "fixedUri": B + "pk-ext"},
   {"id": "Extension.value[x]", "path": "Extension.value[x]", "max": "0"}], kind="complex-type", ctx=CT)
inst("pk-i", {"resourceType": "Patient", "id": "a", "extension": [{"url": B + "pk-ext", "extension": [{"url": "sub", "extension": [{"url": "deep"}]}]}]})
# K, nivel raíz: la extensión no tiene ni sub-extensiones ni valor, y ext-1 falla en la raíz de pk-ext.
inst("pk2-i", {"resourceType": "Patient", "id": "a", "extension": [{"url": B + "pk-ext"}]})
