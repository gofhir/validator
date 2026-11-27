import json
B="http://example.org/hm/"
tb2={"resourceType":"StructureDefinition","id":"tb2","url":B+"tb2","name":"Tb2","status":"draft","fhirVersion":"4.0.1","kind":"complex-type","abstract":False,"type":"Identifier","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Identifier","derivation":"constraint","differential":{"element":[{"id":"Identifier","path":"Identifier"},{"id":"Identifier.value","path":"Identifier.value","constraint":[{"key":"gk","severity":"error","human":"tb2 text","expression":"true"}]}]}}
json.dump(tb2,open("ig/tb2.json","w"))
def sd(id,diff):
    return {"resourceType":"StructureDefinition","id":id,"url":B+id,"name":id.replace("-","").capitalize(),"status":"draft","fhirVersion":"4.0.1","kind":"resource","abstract":False,"type":"Patient","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","derivation":"constraint","differential":{"element":diff}}
gk=lambda t:[{"key":"gk","severity":"error","human":t,"expression":"$this.length() < 1"}]
tp=[{"code":"Identifier","profile":[B+"tb2"]}]
slicing={"discriminator":[{"type":"value","path":"system"}],"rules":"open"}
P={
 "ka3":[{"id":"Patient.identifier","path":"Patient.identifier","type":tp},{"id":"Patient.identifier.value","path":"Patient.identifier.value","constraint":gk("own text")}],
 "kc3":[{"id":"Patient.identifier","path":"Patient.identifier","slicing":slicing},
       {"id":"Patient.identifier:s1","path":"Patient.identifier","sliceName":"s1","type":tp},{"id":"Patient.identifier:s1.system","path":"Patient.identifier.system","fixedUri":"urn:s1"},
       {"id":"Patient.identifier:s1.value","path":"Patient.identifier.value","constraint":gk("slice text")}],
 "kd3":[{"id":"Patient.identifier","path":"Patient.identifier","slicing":slicing,"type":tp},
       {"id":"Patient.identifier:s1","path":"Patient.identifier","sliceName":"s1"},{"id":"Patient.identifier:s1.system","path":"Patient.identifier.system","fixedUri":"urn:s1"},
       {"id":"Patient.identifier:s1.value","path":"Patient.identifier.value","constraint":gk("slice text")}],
}
for k,d in P.items():
    json.dump(sd(k,d),open(f"ig/{k}.json","w"))
    inst={"resourceType":"Patient","id":k+"i","meta":{"profile":[B+k]},"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},"identifier":[{"system":"urn:s1","value":"x"}]}
    json.dump(inst,open(f"inst/{k}-i.json","w"),indent=1)
