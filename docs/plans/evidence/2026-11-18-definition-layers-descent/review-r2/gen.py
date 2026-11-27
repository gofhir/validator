import json, os, sys
base = "http://example.org/r2/"
root = sys.argv[1]
def sd(id, typ, kind, diff, version, baseurl=None):
    return {"resourceType":"StructureDefinition","id":id,"url":base+id,"name":id.replace("-","").replace("_",""),"status":"active","fhirVersion":version,
      "kind":kind,"abstract":False,"type":typ,"baseDefinition":baseurl or "http://hl7.org/fhir/StructureDefinition/"+typ,"derivation":"constraint",
      "differential":{"element":diff}}
def rootel(t): return {"id":t,"path":t}
for d, ver in (("r4","4.0.1"),("r4b","4.3.0")):
    ig = os.path.join(root, d, "ig"); inst = os.path.join(root, d, "inst")
    os.makedirs(ig, exist_ok=True); os.makedirs(inst, exist_ok=True)
    def w(p, o):
        json.dump(o, open(p,"w"), separators=(",",":"))
    # rp: Reference type profile
    w(f"{ig}/rp-pat.json", sd("rp-pat","Patient","resource",[rootel("Patient"),{"id":"Patient.generalPractitioner","path":"Patient.generalPractitioner","type":[{"code":"Reference","profile":[base+"rp-ref"],"targetProfile":["http://hl7.org/fhir/StructureDefinition/Organization","http://hl7.org/fhir/StructureDefinition/Practitioner"]}]}],ver))
    w(f"{ig}/rp-ref.json", sd("rp-ref","Reference","complex-type",[{"id":"Reference","path":"Reference","short":"x"}],ver))
    w(f"{inst}/rp-i.json", {"resourceType":"Patient","id":"a","generalPractitioner":[{"reference":"#nope"}]})
    # bdl: Bundle profile, bdl-8
    w(f"{ig}/bdl-pat.json", sd("bdl-pat","Bundle","resource",[rootel("Bundle")],ver))
    w(f"{inst}/bdl-i.json", {"resourceType":"Bundle","id":"a","type":"collection","entry":[{"fullUrl":"http://example.org/Patient/1/_history/1","resource":{"resourceType":"Patient","id":"1"}}]})
    # tim: Observation with Timing type profile
    w(f"{ig}/tim-obs.json", sd("tim-obs","Observation","resource",[rootel("Observation"),{"id":"Observation.effective[x]","path":"Observation.effective[x]","type":[{"code":"Timing","profile":[base+"tim-tp"]}]}],ver))
    w(f"{ig}/tim-tp.json", sd("tim-tp","Timing","complex-type",[{"id":"Timing","path":"Timing","short":"x"}],ver))
    w(f"{inst}/tim-i.json", {"resourceType":"Observation","id":"a","status":"final","code":{"text":"x"},"effectiveTiming":{"repeat":{"offset":5,"when":["C"]}}})
    # tim2: explicit Observation profile, plain Timing
    w(f"{ig}/tim-plain.json", sd("tim-plain","Observation","resource",[rootel("Observation"),{"id":"Observation.effective[x]","path":"Observation.effective[x]","type":[{"code":"Timing"}]}],ver))
    w(f"{inst}/tim2-i.json", {"resourceType":"Observation","id":"a","status":"final","code":{"text":"x"},"effectiveTiming":{"repeat":{"offset":5,"when":["C"]}}})
    # vs
    w(f"{ig}/vs-pat.json", sd("vs-pat","Observation","resource",[rootel("Observation")],ver,"http://hl7.org/fhir/StructureDefinition/vitalsigns"))
    w(f"{inst}/vs-i.json", {"resourceType":"Observation","id":"a","status":"final","category":[{"coding":[{"system":"http://terminology.hl7.org/CodeSystem/observation-category","code":"vital-signs"}]}],"code":{"coding":[{"system":"http://loinc.org","code":"85353-1"}]},"subject":{"display":"x"},"effectiveDateTime":"2020","valueString":"x"})
    # con
    w(f"{ig}/con-pat.json", sd("con-pat","Condition","resource",[rootel("Condition")],ver))
    w(f"{inst}/con-i.json", {"resourceType":"Condition","id":"a","verificationStatus":{"coding":[{"system":"http://terminology.hl7.org/CodeSystem/condition-ver-status","code":"confirmed"}]},"category":[{"coding":[{"system":"http://terminology.hl7.org/CodeSystem/condition-category","code":"problem-list-item"}]}],"subject":{"display":"x"}})
    # que
    w(f"{ig}/que-pat.json", sd("que-pat","Questionnaire","resource",[rootel("Questionnaire")],ver))
    w(f"{inst}/que-i.json", {"resourceType":"Questionnaire","id":"a","status":"draft","item":[{"linkId":"1","type":"string","enableWhen":[{"question":"2","operator":"exists","answerString":"x"},{"question":"3","operator":"=","answerString":"y"}]},{"linkId":"2","type":"string"},{"linkId":"3","type":"string"}]})
    # ras
    w(f"{ig}/ras-pat.json", sd("ras-pat","RiskAssessment","resource",[rootel("RiskAssessment")],ver))
    w(f"{inst}/ras-i.json", {"resourceType":"RiskAssessment","id":"a","status":"final","subject":{"display":"x"},"prediction":[{"probabilityDecimal":150}]})
