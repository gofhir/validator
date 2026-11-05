# hl7diff same: baseline origin/main vs working tree

decisions: identical (15 files, 26 lines)

multimatch: identical (3 files, 9 lines)

core-probes: identical (7 files, 12 lines)

type-profile-probes: identical (47 files, 92 lines)

nested-resource-probes: identical (4 files, 6 lines)

review-probes: identical (5 files, 9 lines)

extension-probes: identical (14 files, 27 lines)

primitive-extension-probes: identical (5 files, 5 lines)

context-probes: identical (33 files, 131 lines)

context-eval-probes: identical (31 files, 55 lines)

context-scope-probes: identical (198 files, 369 lines)

interface-probes: identical (9 files, 35 lines)

interface-probes-r5: identical (16 files, 20 lines)

interface-probes-r4b: identical (9 files, 36 lines)

constraint-probes: identical (81 files, 162 lines)

invariant-probes: identical (61 files, 107 lines)

fixedpattern-probes: identical (50 files, 86 lines)

target-probes: identical (71 files, 104 lines)

nested-slicing-probes: identical (18 files, 23 lines)

json-shape-probes: identical (34 files, 103 lines)

binding-probes: identical (5 files, 10 lines)

binding-walk-probes: identical (14 files, 32 lines)

extension-walk-probes: identical (20 files, 42 lines)

binding-profile-probes: identical (10 files, 19 lines)

valueset-membership-probes: identical (43 files, 52 lines)

holder-profile-probes: identical (50 files, 93 lines)

reference-walk-probes: identical (56 files, 137 lines)

package-example-cpg-probes: identical (4 files, 10 lines)

package-example-cql-probes: identical (2 files, 3 lines)

unknown-codesystem-probes: identical (521 files, 1035 lines)

fullurl-probes: identical (48 files, 135 lines)

fullurl-probes-r4b: identical (1 files, 3 lines)

fullurl-probes-r5: identical (4 files, 8 lines)

held-resource-probes: identical (39 files, 146 lines)

choice-variant-probes: identical (27 files, 56 lines)

content-reference-probes: identical (32 files, 66 lines)

content-reference-probes-r4b: identical (32 files, 66 lines)

content-reference-probes-r5: identical (33 files, 42 lines)

empty-array-probes: identical (37 files, 95 lines)

empty-array-probes-r4b: identical (37 files, 103 lines)

empty-array-probes-r5: identical (37 files, 87 lines)

resolve-probes: identical (19 files, 31 lines)

resource-element-probes: identical (6 files, 17 lines)

canonical-probes: identical (17 files, 19 lines)

r5-probes: identical (8 files, 12 lines)

r4b-probes: identical (1 files, 1 lines)

narrative-ucum-probes: identical (49 files, 82 lines)

narrative-ucum-probes-r4b: identical (49 files, 82 lines)

narrative-ucum-probes-r5: identical (49 files, 82 lines)

deqm-probes: identical (15 files, 66 lines)

deqm-examples: identical (108 files, 451 lines)

uscore-probes: identical (7 files, 18 lines)

uscore-examples: identical (183 files, 409 lines)

ips: identical (44 files, 88 lines)

mcode-examples: identical (179 files, 471 lines)

aucore-examples: identical (65 files, 108 lines)

chcore-examples: identical (48 files, 76 lines)

clcore-examples: identical (20 files, 24 lines)

## r4-core-examples: 22415 lines differ (5301 files, 55193 baseline lines)

- sorted line 32949
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.contained[4].relationship.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":92,"Column":11}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"BINDING_EXTENSIBLE_NO_CODING","expression":["Claim.contained[4].relationship"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/subscriber-relationship', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = self)","location":{"Line":90,"Column":21}}
- sorted line 32950
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.diagnosis[0].diagnosisCodeableConcept.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":199,"Column":11}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.contained[4].relationship.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":92,"Column":11}}
- sorted line 32951
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.item[0].productOrService.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":223,"Column":11}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.diagnosis[0].diagnosisCodeableConcept.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":199,"Column":11}}
- sorted line 32952
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.payee.type.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":180,"Column":9}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.item[0].productOrService.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":223,"Column":11}}
- sorted line 32953
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.priority.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":172,"Column":7}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.payee.type.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":180,"Column":9}}
- sorted line 32954
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].detail[0].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHOEXAM'","location":{"Line":119,"Column":25}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100152.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.priority.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":172,"Column":7}}
- sorted line 32955
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].detail[1].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHODIAG'","location":{"Line":138,"Column":25}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].detail[0].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHOEXAM'","location":{"Line":119,"Column":25}}
- sorted line 32956
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].detail[2].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHOINITIAL'","location":{"Line":157,"Column":25}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].detail[1].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHODIAG'","location":{"Line":138,"Column":25}}
- sorted line 32957
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].detail[3].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHOMONTHS'","location":{"Line":176,"Column":25}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].detail[2].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHOINITIAL'","location":{"Line":157,"Column":25}}
- sorted line 32958
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].detail[4].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHOPERIODIC'","location":{"Line":190,"Column":25}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].detail[3].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHOMONTHS'","location":{"Line":176,"Column":25}}
- sorted line 32959
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHPLAN'","location":{"Line":99,"Column":21}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].detail[4].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHOPERIODIC'","location":{"Line":190,"Column":25}}
- sorted line 32960
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[1].bodySite.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://fdi.org/fhir/oraltoothcodes for '21'","location":{"Line":234,"Column":21}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[0].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'ORTHPLAN'","location":{"Line":99,"Column":21}}
- sorted line 32961
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[1].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for '21211'","location":{"Line":217,"Column":21}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[1].bodySite.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://fdi.org/fhir/oraltoothcodes for '21'","location":{"Line":234,"Column":21}}
- sorted line 32962
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[1].subSite[0].coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://fdi.org/fhir/oralsurfacecodes for 'L'","location":{"Line":243,"Column":23}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[1].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for '21211'","location":{"Line":217,"Column":21}}
- sorted line 32963
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[2].bodySite.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://fdi.org/fhir/oraltoothcodes for '36'","location":{"Line":275,"Column":21}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[1].subSite[0].coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://fdi.org/fhir/oralsurfacecodes for 'L'","location":{"Line":243,"Column":23}}
- sorted line 32964
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[2].detail[0].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for '27211'","location":{"Line":286,"Column":25}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[2].bodySite.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://fdi.org/fhir/oraltoothcodes for '36'","location":{"Line":275,"Column":21}}
- sorted line 32965
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[2].detail[1].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'lab'","location":{"Line":305,"Column":25}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[2].detail[0].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for '27211'","location":{"Line":286,"Column":25}}
- sorted line 32966
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[2].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for '27211'","location":{"Line":258,"Column":21}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[2].detail[1].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for 'lab'","location":{"Line":305,"Column":25}}
- sorted line 32967
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.fundsReserve.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":43,"Column":7}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODESYSTEM_NOT_FOUND","expression":["Claim.item[2].productOrService.coding[0].system"],"diagnostics":"CodeSystem is unknown and can't be validated: http://example.org/fhir/oralservicecodes for '27211'","location":{"Line":258,"Column":21}}
- sorted line 32968
  base: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.payee.type.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":51,"Column":9}}
  head: {"file":"/Users/robertoaraneda/.fhir/packages/hl7.fhir.r4.examples#4.0.1/package/Claim-100153.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Claim.fundsReserve.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":43,"Column":7}}

## coding-no-system-probes: 39 lines differ (12 files, 31 baseline lines)

- sorted line 2
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"error","code":"code-invalid","messageId":"BINDING_REQUIRED_NO_CODING","expression":["AllergyIntolerance.clinicalStatus"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/allergyintolerance-clinical|4.0.1', and a coding from this value set is required (codes = active)","location":{"Line":1,"Column":90}}
- sorted line 3
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 4
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":223}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
- sorted line 5
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":223}}
- sorted line 6
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 7
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":219}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"processing","messageId":"BINDING_EXTENSIBLE_NO_CODING","expression":["Consent.category[0]"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/consent-category', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = 59284-0)","location":{"Line":1,"Column":178}}
- sorted line 8
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
- sorted line 9
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":219}}
- sorted line 10
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q13.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":251}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 11
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q13.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"processing","messageId":"BINDING_EXTENSIBLE_NO_CODING","expression":["Consent.category[0]"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/consent-category', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = acd)","location":{"Line":1,"Column":178}}
- sorted line 12
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":239}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
- sorted line 13
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q13.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":251}}
- sorted line 14
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q13.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 15
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":239}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":239}}
- sorted line 16
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 17
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"processing","messageId":"BINDING_EXTENSIBLE_NO_CODING","expression":["Consent.category[0]"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/consent-category', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = 59284-0)","location":{"Line":1,"Column":178}}
- sorted line 18
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q16.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":291}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
- sorted line 19
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q16.json","severity":"warning","code":"code-invalid","messageId":"CODING_NO_CODE_IN_CONCEPT","expression":["Consent.category[0].coding[0].code"],"diagnostics":"No code is provided for the code system 'http://terminology.hl7.org/CodeSystem/consentcategorycodes'"}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":239}}
- sorted line 20
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q16.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 21
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q16.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[1]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":261}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"processing","messageId":"BINDING_EXTENSIBLE_NO_CODING","expression":["Consent.category[0]"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/consent-category', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = acd)","location":{"Line":1,"Column":178}}

## coding-no-system-probes-r4b: 39 lines differ (12 files, 31 baseline lines)

- sorted line 2
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"error","code":"code-invalid","messageId":"BINDING_REQUIRED_NO_CODING","expression":["AllergyIntolerance.clinicalStatus"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/allergyintolerance-clinical|4.3.0', and a coding from this value set is required (codes = active)","location":{"Line":1,"Column":90}}
- sorted line 3
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 4
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":223}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
- sorted line 5
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":223}}
- sorted line 6
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 7
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":219}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"processing","messageId":"BINDING_EXTENSIBLE_NO_CODING","expression":["Consent.category[0]"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/consent-category', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = 59284-0)","location":{"Line":1,"Column":178}}
- sorted line 8
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q11.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
- sorted line 9
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":219}}
- sorted line 10
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q13.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":251}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 11
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q13.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"processing","messageId":"BINDING_EXTENSIBLE_NO_CODING","expression":["Consent.category[0]"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/consent-category', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = acd)","location":{"Line":1,"Column":178}}
- sorted line 12
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":239}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q12.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
- sorted line 13
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q13.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":251}}
- sorted line 14
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q13.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 15
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":239}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":239}}
- sorted line 16
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 17
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"processing","messageId":"BINDING_EXTENSIBLE_NO_CODING","expression":["Consent.category[0]"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/consent-category', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = 59284-0)","location":{"Line":1,"Column":178}}
- sorted line 18
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q16.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":291}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q14.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":189}}
- sorted line 19
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q16.json","severity":"warning","code":"code-invalid","messageId":"CODING_NO_CODE_IN_CONCEPT","expression":["Consent.category[0].coding[0].code"],"diagnostics":"No code is provided for the code system 'http://terminology.hl7.org/CodeSystem/consentcategorycodes'"}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"code-invalid","messageId":"BINDING_TEXT_ONLY_WARNING","expression":["Consent.policyRule"],"diagnostics":"No code provided, and a code should be provided from the value set 'http://hl7.org/fhir/ValueSet/consent-policy' (extensible)","location":{"Line":1,"Column":239}}
- sorted line 20
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q16.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Consent"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 21
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q16.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Consent.category[0].coding[1]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":261}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q15.json","severity":"warning","code":"processing","messageId":"BINDING_EXTENSIBLE_NO_CODING","expression":["Consent.category[0]"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/consent-category', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = acd)","location":{"Line":1,"Column":178}}

## coding-no-system-probes-r5: 17 lines differ (6 files, 13 baseline lines)

- sorted line 2
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"error","code":"code-invalid","messageId":"BINDING_REQUIRED_NO_CODING","expression":["AllergyIntolerance.clinicalStatus"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/allergyintolerance-clinical|5.0.0', and a coding from this value set is required (codes = active)","location":{"Line":1,"Column":90}}
- sorted line 3
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 4
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q2.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q1.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
- sorted line 5
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q2.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q2.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 6
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q3.json","severity":"error","code":"code-invalid","messageId":"BINDING_REQUIRED_NO_CODING","expression":["AllergyIntolerance.clinicalStatus"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/allergyintolerance-clinical|5.0.0', and a coding from this value set is required (codes = bogus)","location":{"Line":1,"Column":90}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q2.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
- sorted line 7
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q3.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q3.json","severity":"error","code":"code-invalid","messageId":"BINDING_REQUIRED_NO_CODING","expression":["AllergyIntolerance.clinicalStatus"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/allergyintolerance-clinical|5.0.0', and a coding from this value set is required (codes = bogus)","location":{"Line":1,"Column":90}}
- sorted line 8
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q3.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q3.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 9
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q4.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Patient"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q3.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
- sorted line 10
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q4.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Patient.maritalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":64}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q4.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Patient"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 11
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q5.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Patient"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q4.json","severity":"warning","code":"processing","messageId":"BINDING_EXTENSIBLE_NO_CODING","expression":["Patient.maritalStatus"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/marital-status', and a coding should come from this value set unless it has no suitable code (note that the validator cannot judge what is suitable) (codes = M)","location":{"Line":1,"Column":52}}
- sorted line 12
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q5.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Patient.communication[0].language.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":77}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q4.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Patient.maritalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":64}}
- sorted line 13
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q6.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q5.json","severity":"error","code":"code-invalid","messageId":"BINDING_REQUIRED_NO_CODING","expression":["Patient.communication[0].language"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/all-languages|5.0.0', and a coding from this value set is required (codes = en)","location":{"Line":1,"Column":65}}
- sorted line 14
  base: {"file":"testdata/m12-slice-scoping/probes/nsys_q6.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q5.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["Patient"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 15
  base: 
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q5.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["Patient.communication[0].language.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":77}}
- sorted line 16
  base: 
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q6.json","severity":"error","code":"code-invalid","messageId":"BINDING_REQUIRED_NO_CODING","expression":["AllergyIntolerance.clinicalStatus"],"diagnostics":"None of the codings provided are in the value set 'http://hl7.org/fhir/ValueSet/allergyintolerance-clinical|5.0.0', and a coding from this value set is required (codes = active)","location":{"Line":1,"Column":90}}
- sorted line 17
  base: 
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q6.json","severity":"warning","code":"invariant","messageId":"CONSTRAINT_FAILED","expression":["AllergyIntolerance"],"diagnostics":"Constraint failed: dom-6: 'A resource should have narrative for robust management' (defined in http://hl7.org/fhir/StructureDefinition/DomainResource) (Best Practice Recommendation)","location":{"Line":1,"Column":2}}
- sorted line 18
  base: 
  head: {"file":"testdata/m12-slice-scoping/probes/nsys_q6.json","severity":"warning","code":"processing","messageId":"CODING_NO_SYSTEM","expression":["AllergyIntolerance.clinicalStatus.coding[0]"],"diagnostics":"Coding has no system. A code with no system has no defined meaning, and it cannot be validated. A system should be provided","location":{"Line":1,"Column":102}}

**DIFFERENT**
