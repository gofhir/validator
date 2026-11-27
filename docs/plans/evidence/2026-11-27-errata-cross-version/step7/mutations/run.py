import subprocess, time, shutil, sys, re
R='/Volumes/dev/tmp/claude/claude-501/-Users-robertoaraneda-projects-personal-opensource-go-validator/11d0cc65-98da-4af5-bcc8-65c4743d06bd/scratchpad/'
C=R+'mut-copy/pkg/registry/'; W=R+'mutwork/'
E='errata.go'; S='snapshot.go'
refB=re.search(r'const refOneCorrected = `(.*)`',open(W+E).read()).group(1)
M={}
M[1]=("patchRaw does nothing",[(E,"func (e *ElementDefinition) patchRaw() {\n","func (e *ElementDefinition) patchRaw() {\n\tif true {\n\t\treturn\n\t}\n")])
M[2]=("patchRaw rewrites whole raw via json.Marshal(struct)",[(E,"func (e *ElementDefinition) patchRaw() {\n","func (e *ElementDefinition) patchRaw() {\n\tif raw, err := json.Marshal(e); err == nil {\n\t\te.raw = raw\n\t\te.keepPrefixedValues()\n\t}\n\tif true {\n\t\treturn\n\t}\n")])
M[3]=("post-merge correctSnapshot(profileVersion)",[(S,"\tinheritAddedConstraints(snapshot, baseSnapshot)\n","\tinheritAddedConstraints(snapshot, baseSnapshot)\n\tcorrectSnapshot(profileVersion, snapshot.Element)\n")])
M[4]=("post-merge correctSnapshot(profile) + correctSnapshot(root)",[(S,"\tinheritAddedConstraints(snapshot, baseSnapshot)\n","\tinheritAddedConstraints(snapshot, baseSnapshot)\n\tcorrectSnapshot(profileVersion, snapshot.Element)\n\tcorrectSnapshot(cmp.Or(baseSD.errataVersion, baseSD.FHIRVersion), snapshot.Element)\n")])
M[5]=("correctPlaced disabled",[(E,"	selected := *placed\n","	if true {\n		return\n	}\n	selected := *placed\n")])
M[6]=("correctPlaced identifies by placed's own path/base",[(E,"selected.Path, selected.Base = base.Path, base.Base","selected.Path, selected.Base = placed.Path, placed.Base")])
M[7]=("addConstraints also writes added constraint into raw",[(E,"				e.Constraint = append(e.Constraint, er.constraint)\n","""				e.Constraint = append(e.Constraint, er.constraint)
				if m, err := rawToMap(e.raw); err == nil && len(m) > 0 {
					var cs []json.RawMessage
					_ = unmarshalMember(m, "constraint", &cs)
					b, _ := json.Marshal(er.constraint)
					cs = append(cs, b)
					m["constraint"], _ = json.Marshal(cs)
					if raw, err := json.Marshal(m); err == nil {
						e.raw = raw
						e.keepPrefixedValues()
					}
				}
""")])
M[8]=("addConstraints post-merge with profileVersion",[(S,"addConstraints(r.rootVersion(ctx, sd), snapshot.Element)","addConstraints(profileVersion, snapshot.Element)")])
M[9]=("rootVersion returns immediate base's version",[(S,"	seen := map[*StructureDefinition]bool{}\n","	seen := map[*StructureDefinition]bool{}\n	orig := sd\n"),(S,'if sd.Derivation == "specialization" || sd.BaseDefinition == "" {','if sd != orig || sd.BaseDefinition == "" {')])
M[10]=("inRelease always true",[(E,'return fhirVersion != "" && versionorder.SameRelease(erratum, fhirVersion)','_ = versionorder.SameRelease\n\treturn true')])
M[11]=("remove R4 ref-1 erratum (key renamed)",[(E,'fhirVersion: fhirR4, path: "Reference", key: "ref-1"','fhirVersion: fhirR4, path: "Reference", key: "ref-1x"')])
M[12]=("remove R4 tim-9 erratum (key renamed)",[(E,'fhirVersion: fhirR4, path: "Timing.repeat", key: "tim-9"','fhirVersion: fhirR4, path: "Timing.repeat", key: "tim-9x"')])
M[13]=("R4B ref-1 corrected text 1 char ('#' -> '##')",[(E,'	fhirVersion: fhirR4B, from: "http://hl7.org/fhir/StructureDefinition/Reference", key: "ref-1",\n	published: `reference.startsWith(\'#\').not() or (reference.substring(1).trace(\'url\') in %rootResource.contained.id.trace(\'ids\')) or (reference=\'#\' and %rootResource!=%resource)`,\n	corrected: refOneCorrected,','	fhirVersion: fhirR4B, from: "http://hl7.org/fhir/StructureDefinition/Reference", key: "ref-1",\n	published: `reference.startsWith(\'#\').not() or (reference.substring(1).trace(\'url\') in %rootResource.contained.id.trace(\'ids\')) or (reference=\'#\' and %rootResource!=%resource)`,\n	corrected: `'+refB.replace("reference='#'","reference='##'")+'`,')])
M[14]=("patchRaw skips keepPrefixedValues",[(E,"	e.keepPrefixedValues() // la caché","	// la caché")])
M[15]=("correctPlaced skips Type correction",[(E,"	selected.Type = slices.Clone(placed.Type)\n	for i := range selected.Type {\n		selected.Type[i].Extension = slices.Clone(selected.Type[i].Extension)\n	}\n","	selected.Type = nil\n"),(E,"placed.Constraint, placed.Type, placed.raw = selected.Constraint, selected.Type, selected.raw","placed.Constraint, placed.raw = selected.Constraint, selected.raw")])
M[16]=("correctPlaced uses validated version (r.fhirVersion)",[(S,"version, _ := ctx.Value(profileVersionKey{}).(string)","_, _ = ctx.Value(profileVersionKey{}).(string)\n\t\tversion := b.r.fhirVersion")])
M[17]=("setString adds an absent member with an empty value",[(E,"	if !ok && value == \"\" {\n","	if false && !ok && value == \"\" {\n")])
M[18]=("rootVersion returns empty on a broken chain",[(S,"\t\t\tbreak\n\t\t}\n\t\turl, version := ParseCanonical(sd.BaseDefinition)","\t\t\treturn deepest\n\t\t}\n\t\turl, version := ParseCanonical(sd.BaseDefinition)"),(S,"\treturn deepest\n}\n","\treturn \"\"\n}\n")])
for k in (8,9,18): M.pop(k,None)
M[19]=("inheritAddedConstraints does nothing",[(E,"func inheritAddedConstraints(snapshot, base *Snapshot) {\n","func inheritAddedConstraints(snapshot, base *Snapshot) {\n\tif true {\n\t\treturn\n\t}\n")])
M[20]=("inherit every base constraint the merged element lacks",[(E,"if isAddedConstraint(c) && !slices","if !slices")])
M[21]=("no slice-stripped match",[(E,"\t\tif b == nil {\n\t\t\tb = inBase[sliceName.ReplaceAllString(e.ID, \"\")]\n\t\t}\n","")])
M[22]=("isAddedConstraint by key only",[(E,"return er.constraint.Key == c.Key && er.constraint.Expression == c.Expression","return er.constraint.Key == c.Key")])
which=[int(x) for x in sys.argv[1:]] or sorted(M)
for n in which:
    name,edits=M[n]
    for f in (E,S): shutil.copy(W+f,C+f)
    for f,o,nw in edits:
        t=open(C+f).read()
        assert t.count(o)==1,(n,o[:40],t.count(o))
        open(C+f,'w').write(t.replace(o,nw))
    t0=time.time()
    p=subprocess.run("cd %smut-copy && gofmt -l pkg/registry >/dev/null; go test -count=1 -failfast -run 'ErrataCrossVersion|Errata' ./pkg/registry ./pkg/validator 2>&1"%R,shell=True,capture_output=True,text=True)
    d=time.time()-t0
    out=p.stdout
    open(W+'out%d.txt'%n,'w').write(out)
    if p.returncode==0: res='SURVIVED(targeted)'
    else:
        fails=re.findall(r'--- FAIL: (\S+)',out)
        res='KILLED: '+(fails[0] if fails else 'build/other: '+out.strip().splitlines()[0][:80])
    for f in (E,S): shutil.copy(W+f,C+f)
    print(n,name,'|',res,'|','%.0fs'%d,flush=True)
