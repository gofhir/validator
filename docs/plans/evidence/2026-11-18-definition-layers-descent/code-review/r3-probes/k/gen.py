import json
B="http://example.org/hm/"
def sd(id,diff):
    return {"resourceType":"StructureDefinition","id":id,"url":B+id,"name":id.replace("-","").capitalize(),"status":"draft","fhirVersion":"4.0.1","kind":"resource","abstract":False,"type":"Patient","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","derivation":"constraint","differential":{"element":diff}}
gk=lambda t:[{"key":"gk","severity":"error","human":t,"expression":"$this.length() < 1"}]
tp=[{"code":"Identifier","profile":[B+"tb-10b"]}]
slicing={"discriminator":[{"type":"value","path":"system"}],"rules":"open"}
P={
 "ka":[{"id":"Patient.identifier","path":"Patient.identifier","type":tp},{"id":"Patient.identifier.value","path":"Patient.identifier.value","constraint":gk("own text")}],
 "kb":[{"id":"Patient.identifier","path":"Patient.identifier","slicing":slicing},{"id":"Patient.identifier.value","path":"Patient.identifier.value","constraint":gk("base text")},
       {"id":"Patient.identifier:s1","path":"Patient.identifier","sliceName":"s1"},{"id":"Patient.identifier:s1.system","path":"Patient.identifier.system","fixedUri":"urn:s1"},
       {"id":"Patient.identifier:s1.value","path":"Patient.identifier.value","constraint":gk("slice text")}],
 "kc":[{"id":"Patient.identifier","path":"Patient.identifier","slicing":slicing},
       {"id":"Patient.identifier:s1","path":"Patient.identifier","sliceName":"s1","type":tp},{"id":"Patient.identifier:s1.system","path":"Patient.identifier.system","fixedUri":"urn:s1"},
       {"id":"Patient.identifier:s1.value","path":"Patient.identifier.value","constraint":gk("slice text")}],
 "kd":[{"id":"Patient.identifier","path":"Patient.identifier","slicing":slicing,"type":tp},
       {"id":"Patient.identifier:s1","path":"Patient.identifier","sliceName":"s1"},{"id":"Patient.identifier:s1.system","path":"Patient.identifier.system","fixedUri":"urn:s1"},
       {"id":"Patient.identifier:s1.value","path":"Patient.identifier.value","constraint":gk("slice text")}],
}
for k,d in P.items():
    json.dump(sd(k,d),open(f"ig/{k}.json","w"))
    inst={"resourceType":"Patient","id":k+"i","meta":{"profile":[B+k]},"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},"identifier":[{"system":"urn:s1","value":"x"}]}
    json.dump(inst,open(f"inst/{k}-i.json","w"),indent=1)
