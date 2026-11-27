import json
B="http://example.org/hm/"
tk3={"resourceType":"StructureDefinition","id":"tk3","url":B+"tk3","name":"Tk3","status":"draft","fhirVersion":"4.0.1","kind":"complex-type","abstract":False,"type":"Identifier","baseDefinition":B+"tb2","derivation":"constraint","differential":{"element":[{"id":"Identifier","path":"Identifier"},{"id":"Identifier.system","path":"Identifier.system","min":1}]}}
json.dump(tk3,open("ig/tk3.json","w"))
def sd(id,diff):
    return {"resourceType":"StructureDefinition","id":id,"url":B+id,"name":id.capitalize(),"status":"draft","fhirVersion":"4.0.1","kind":"resource","abstract":False,"type":"Patient","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","derivation":"constraint","differential":{"element":diff}}
def gk(t,e): return [{"key":"gk","severity":"error","human":t,"expression":e}]
P={
 "km1":[{"id":"Patient.identifier","path":"Patient.identifier","type":[{"code":"Identifier","profile":[B+"tz-10b",B+"tk-10b"]}]},{"id":"Patient.identifier.value","path":"Patient.identifier.value","constraint":gk("own text","true")}],
 "km2":[{"id":"Patient.identifier","path":"Patient.identifier","type":[{"code":"Identifier","profile":[B+"tz-10b",B+"tk3"]}]},{"id":"Patient.identifier.value","path":"Patient.identifier.value","constraint":gk("own text","$this.length() < 1")}],
}
for k,d in P.items():
    json.dump(sd(k,d),open(f"ig/{k}.json","w"))
    inst={"resourceType":"Patient","id":k+"i","meta":{"profile":[B+k]},"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},"identifier":[{"value":"x"}]}
    json.dump(inst,open(f"inst/{k}-i.json","w"),indent=1)
