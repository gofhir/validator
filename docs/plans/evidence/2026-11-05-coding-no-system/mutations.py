# Mutaciones de #192: se corren desde la raíz de una copia del repo con el cambio aplicado.
import subprocess
F='pkg/binding/binding.go'
muts=[
 ("M0 resolver verdict ignored (always Invalid)","\tif err != nil || res.Resolution == terminology.Unresolved {\n\t\treturn terminology.Unresolved\n\t}\n\treturn terminology.Invalid\n}","\tif false {\n\t\treturn terminology.Unresolved\n\t}\n\treturn terminology.Invalid\n}"),
 ("M1 rule removed","\tif system == \"\" {\n\t\treturn v.systemlessCodingInValueSet","\tif false {\n\t\treturn v.systemlessCodingInValueSet"),
 ("M2 rule applies to any system","\tif system == \"\" {\n\t\treturn v.systemlessCodingInValueSet","\tif system != \"x\" {\n\t\treturn v.systemlessCodingInValueSet"),
 ("M3 unresolved rejects","\t\treturn terminology.Unresolved\n\t}\n\treturn terminology.Invalid\n}","\t\treturn terminology.Invalid\n\t}\n\treturn terminology.Invalid\n}"),
 ("M4 label always system#code","\t\tlabel := cd\n\t\tif sys != \"\" {","\t\tlabel := cd\n\t\tif true {"),
 ("M5 extra no-code error (Codex r1)","if !anyValidInVS && binding.Strength == strengthRequired && len(codeLabels) > 0 {\n","if !anyValidInVS && binding.Strength == strengthRequired && len(codeLabels) > 0 {\n\t\tv.reportNoCode(ctx, binding, fhirPath, result)\n"),
 ("M6 strict membership kept","terminology.WithoutStrictMembership(ctx), \"\", code","ctx, \"\", code"),
 ("M7 authority error ignored","\tif err != nil || res.Resolution == terminology.Unresolved {\n\t\treturn terminology.Unresolved\n\t}\n\treturn terminology.Invalid\n}","\tif res.Resolution == terminology.Unresolved {\n\t\treturn terminology.Unresolved\n\t}\n\treturn terminology.Invalid\n}"),
]
orig=open(F).read()
try:
    for name,o,n in muts:
        assert orig.count(o)==1,name
        open(F,'w').write(orig.replace(o,n))
        r=subprocess.run(['go','test','./pkg/validator','-run','CodingNoSystem|VersionBindingVerdicts|BindingNoCode','-count=1'],capture_output=True,text=True)
        fails=[l.strip() for l in r.stdout.splitlines() if l.startswith('--- FAIL')][:3]
        print(f"{name} -> {'KILLED '+str(fails) if r.returncode else 'SURVIVED'}",flush=True)
finally:
    open(F,'w').write(orig)
