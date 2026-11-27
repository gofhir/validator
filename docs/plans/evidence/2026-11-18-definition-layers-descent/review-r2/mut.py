import subprocess, sys, os, shutil, re
W = os.path.dirname(os.path.abspath(__file__)); R = W + "/../copy"; BK = W + "/bk"; OUT = W + "/out"
os.makedirs(BK, exist_ok=True); os.makedirs(OUT, exist_ok=True)
CT="pkg/constraint/tree.go"; CC="pkg/constraint/constraint.go"; SN="pkg/registry/snapshot.go"; SM="pkg/slicematch/slicematch.go"; SP="pkg/slicematch/path.go"
RS="pkg/constraint/report_scope.go"; LY="pkg/constraint/layers.go"; EX="pkg/validator/testdata/definition_layers/expected.json"
# id, sources, [(file, old, new, nth)], description
M = []
def m(i, src, edits, desc): M.append((i, src, [e if len(e)==4 else e+(1,) for e in edits], desc))
m("01","s7-2",[(CT,"for n := node; n != nil; n = n.SliceOf {\n\t\tadd(sd, n.Children)","for n := node; n != nil; n = nil {\n\t\tadd(sd, n.Children)")],"otherDefinitions chain walk n = n.SliceOf -> n = nil")
m("02","s7-2",[(CT,"if typeRoot != nil && typeSD.Derivation == registry.DerivationConstraint && typeSD != self {","if false && typeRoot != nil && typeSD.Derivation == registry.DerivationConstraint && typeSD != self {")],"otherDefinitions type-profile branch false &&")
m("03","s7-2",[(CT,"\tif self != nil {\n\t\tif root := self.Tree().Root(); root != nil {\n\t\t\tadd(self","\tif false && self != nil {\n\t\tif root := self.Tree().Root(); root != nil {\n\t\t\tadd(self")],"otherDefinitions own definition false &&")
m("04","s7-2",[(CT,"\t\tif opts.firstDescent(ext, fhirPath, n.Children) {","\t\tif false && opts.firstDescent(ext, fhirPath, n.Children) {")],"walkPrimitiveExtensions children loop false &&")
m("05","s7-2",[(CT,"if isObject && run.opts.firstDescent(","if isObject && !own && run.opts.firstDescent(")],"walkLayers layer walk guard !own restored")
m("06","s7-2",[(LY,"if _, done := o.descended[key]; done {","if _, done := o.descended[key]; done && false {")],"firstDescent done never true")
m("07","s7-2",[(CT,"\t\tif len(sets) > 0 {","\t\tif true {")],"walk opens scope always")
m("08","s7-2",[(CT,"\t\tif len(sets) > 0 {","\t\tif false && len(sets) > 0 {")],"walk never opens scope")
m("09a","s7-2",[(LY,'+ "\\x00" + string(result.Issues[i].Severity)','+ "\\x00" + ""')],"layers note(): identity drops severity")
m("09a2","s7-2",[(RS,"k := reportKey{at: fhirPath, id: identityOf(c), sev: c.Severity}","k := reportKey{at: fhirPath, id: identityOf(c)}")],"firstReport reportKey sev dropped")
m("09a3","s7-2",[(CT,"mk := memoKey{at: fhirPath, id: k, sev: c.Severity}","mk := memoKey{at: fhirPath, id: k}")],"pendingConstraints memoKey sev dropped")
m("09b","s7-2",[(CT,"mk := memoKey{at: fhirPath, id: k, sev: c.Severity}",'mk := memoKey{at: "", id: k, sev: c.Severity}')],"pendingConstraints memoKey location dropped")
m("10","s7-2",[(CT,"\tif isObject && own {\n\t\tv.walkOwn(obj, raw, fhirPath, childSD, children, self, opts, result)\n\t}\n\truns :=","\truns :="),(CT,"\tmergeLayers(opts, runs, result)\n\n\tif isObject && !wrongType {","\tmergeLayers(opts, runs, result)\n\tif isObject && own {\n\t\tv.walkOwn(obj, raw, fhirPath, childSD, children, self, opts, result)\n\t}\n\n\tif isObject && !wrongType {")],"walkLayers own-children walk moved after mergeLayers")
m("11a","s7-2",[(CT,"for h := holder; h != nil; h = h.SliceOf {","for h := holder; h != nil; h = nil {")],"validateNested holder chain h = nil")
m("11b","s7-2",[(CT,"if !opts.held.firstNested(key) {","if !opts.held.firstNested(key) && false {")],"validateNested nested pair dedup disabled")
m("12a","s7-2",[(CT,'if !firstReportKey(opts.ctx, typeNotAllowedKey(fhirPath)+"\\x00"+strings.Join(codes, ",")) {','if false && !firstReportKey(opts.ctx, typeNotAllowedKey(fhirPath)+"\\x00"+strings.Join(codes, ",")) {')],"checkType no dedup")
m("12b","s7-2",[(CT,'if !firstReportKey(opts.ctx, typeNotAllowedKey(fhirPath)+"\\x00"+strings.Join(codes, ",")) {','if !firstReportKey(opts.ctx, typeNotAllowedKey(fhirPath)+"\\x00"+strings.Join(codes, ",")+"\\x00"+sd.URL) {')],"checkType dedup key includes sd.URL")
m("13a","s7-2",[(SN,"func (b *snapshotBuilder) yieldCloned(idx int, id string, diff *ElementDefinition) error {\n","func (b *snapshotBuilder) yieldCloned(idx int, id string, diff *ElementDefinition) error {\n\tif true {\n\t\treturn nil\n\t}\n")],"yieldCloned no-op")
m("13b","s7-2",[(SN,"if !b.baseKeys[src][c.Key] {","if true {")],"yieldCloned also yields inherited keys")
m("14","s7-2",[(SN,"func stampConstraintSources(snapshot, base *Snapshot, baseURL string) {\n","func stampConstraintSources(snapshot, base *Snapshot, baseURL string) {\n\tif true {\n\t\treturn\n\t}\n")],"stampConstraintSources no-op")
m("15a","s7-2",[(RS,"if k := failureKey(&in[i]); failed[k] && !layered[k] {","if k := failureKey(&in[i]); failed[k] {"),(RS,"\tvar failed map[string]bool\n","\t_ = layered\n\tvar failed map[string]bool\n")],"CollapseSeverities collapses layered too")
m("15b","s7-2",[(RS,"func CollapseSeverities(ctx context.Context, in []issue.Issue) []issue.Issue {\n","func CollapseSeverities(ctx context.Context, in []issue.Issue) []issue.Issue {\n\tif true {\n\t\treturn in\n\t}\n")],"CollapseSeverities returns input")
m("16","s7-2",[(EX,'"description": "g probe",\n  "expected": {\n   "4.0.1": [\n    {\n     "detail": "gk",','"description": "g probe",\n  "expected": {\n   "4.0.1": [\n    {\n     "detail": "gX",')],"expected.json first detail gk -> gX")
m("17","s7-2",[(CC,"\treturn typeBelow(model, defPath)\n","\treturn model.TypeOf(model.ResolvePath(defPath))\n")],"typeOf uses full path TypeOf(ResolvePath) instead of typeBelow")
m("18","s7-2 (adapted: discriminatorsOf removed by F3; ownValue ignored in findValueSource/valueWithin)",[(SP,"if exp := ownValue(e.def); exp != nil {","if exp := ownValue(e.def); false && exp != nil {"),(SP,"if exp := ownValue(f.node); exp != nil {","if exp := ownValue(f.node); false && exp != nil {"),(SP,"if exp := ownValue(n); exp != nil {\n\t\tif len(path) == 0 {","if exp := ownValue(n); false && exp != nil {\n\t\tif len(path) == 0 {")],"ownValue ignored at all three discriminator value sources")
m("18b","s7-2 (adapted)",[(SP,"if exp := ownValue(f.node); exp != nil {","if exp := ownValue(f.node); false && exp != nil {"),(SP,"if exp := ownValue(n); exp != nil {\n\t\tif len(path) == 0 {","if exp := ownValue(n); false && exp != nil {\n\t\tif len(path) == 0 {")],"ownValue ignored on ancestors/descendants only")
m("19","s7-2",[(SP,"return overlayFixed(profiled, b.children), nil","return overlayFixed(profiled[:1], b.children), nil")],"children() overlayFixed only first type profile")
m("20","s7-2",[(SN,"func (b *snapshotBuilder) carryDeclaredValues(idx int, children []ElementDefinition) error {\n","func (b *snapshotBuilder) carryDeclaredValues(idx int, children []ElementDefinition) error {\n\tif true {\n\t\treturn nil\n\t}\n")],"carryDeclaredValues no-op")
m("21","s7-2,s7-3",[(CC,"\towner := opts.owner\n\tif owner == nil {","\towner := opts.owner\n\tif true {")],"reportedSource returns c.Source always")
m("22","s7-2,s7-3",[(CC,"if owner.Derivation == registry.DerivationConstraint && !opts.explicit {","if false && owner.Derivation == registry.DerivationConstraint && !opts.explicit {")],"reportedSource without publishedSource rule")
m("23","s7-2,s7-3",[(CC,"owner.SnapshotGenerated() {\n\t\treturn owner.URL","owner.SnapshotGenerated() && false {\n\t\treturn owner.URL")],"reportedSource without owner.URL rule")
m("24","s7-2",[(SN,"stampConstraintSources(snapshot, baseSnapshot, baseSD.URL)","stampConstraintSources(snapshot, baseSnapshot, baseSD.URL, sd.URL)"),(SN,"func stampConstraintSources(snapshot, base *Snapshot, baseURL string) {","func stampConstraintSources(snapshot, base *Snapshot, baseURL, profileURL string) {"),(SN,'inBase[sliceName.ReplaceAllString(e.ID, "")][k])','inBase[sliceName.ReplaceAllString(e.ID, "")][k], profileURL)')],"stampConstraintSources stamps own constraints with profile URL")
m("25","s7-2",[(CT,"if opts.divided(reflect.ValueOf(res).Pointer(), holder) {","if false && opts.divided(reflect.ValueOf(res).Pointer(), holder) {")],"validateNested without divided check")
m("26","s7-2",[(CT,"\t\tif opts.probe != nil {\n\t\t\topts.probe(walkprobe.Nested","\t\tif false && opts.probe != nil {\n\t\t\topts.probe(walkprobe.Nested")],"walkprobe.Nested never emitted")
m("27","s7-3",[(CC," && owner.SnapshotGenerated() {"," {")],"owner.URL rule without SnapshotGenerated")
m("28","s7-3",[(CC,"c.Source == \"\" && isRoot && owner.Derivation == registry.DerivationConstraint && ","c.Source == \"\" && isRoot && ")],"owner.URL rule without derivation constraint")
m("29","s7-3 (adapted: publishedSource lookup by element is now atPath/byPath)",[(CC,"if _, dup := idx.byPath[pathRef{el.Path, ref}]; !dup {","if _, dup := idx.byPath[pathRef{el.Path, ref}]; false && !dup {")],"publishedSource never indexes by element path")
m("A","r8",[(CT,"holder.Parent == nil || holder.Parent.SliceOf == nil || first.Parent == nil","holder.Parent == nil || first.Parent == nil")],"divided without base guard")
m("B","r8,r1",[(SN,'for _, prefix := range []string{"fixed", "pattern"} {','for _, prefix := range []string{"fixed"} {')],"carry no pattern")
m("C","r8",[(SN,'c.Source = cmp.Or(inBase[e.ID][k], inBase[sliceName.ReplaceAllString(e.ID, "")][k])','c.Source = inBase[e.ID][k]')],"stamp without slice fallback")
M.append(("D","r8",None,"OBSOLETE: published root branch removed as dead code (D equivalent; atPath no longer searches the root apart)"))
m("E2","r8:E,r1:E2",[(CT,"\tif definitions > 1 {\n\t\topts = opts.scoped()\n\t}","\tif false {\n\t\topts = opts.scoped()\n\t}")],"primitive ext never scoped (r8 E: scoped() removed; same edit)")
m("E","r1",[(CT,"\tif definitions > 1 {\n\t\topts = opts.scoped()\n\t}","\topts = opts.scoped()")],"primitive ext always scoped")
m("F","r8,r1",[(CT,"\tfor n := node; n != nil; n = n.SliceOf {\n\t\tif opts.firstDescent(ext","\tfor n := node; n != nil; n = nil {\n\t\tif opts.firstDescent(ext")],"prim-ext no SliceOf chain")
m("G","r8",[(CT,"\t\t\tif tp.SD == self {","\t\t\tif false && tp.SD == self {")],"alt-self ignored")
m("H","r8",[(CC,"\treturn typeBelow(model, defPath)\n",'\treturn ""\n')],"typeBelow off")
M.append(("I","r8",None,"OBSOLETE: discriminator kindOfValue (pattern-as-value) removed by F3 (discriminatorsOf/kindOfValue removed; sliceMatches returns false without discriminator)"))
m("J","r8",[(RS,"strings.TrimSuffix(is.Diagnostics, bestPracticeSuffix)","strings.TrimSuffix(is.Diagnostics, \"\")")],"collapse no trim best-practice suffix")
m("K","r8,r1",[(CC,"\t\t\t\tif _, dup := idx.any[ref]; !dup {\n\t\t\t\t\tidx.any[ref] = pc.Source\n\t\t\t\t}","\t\t\t\tidx.any[ref] = pc.Source")],"published last not first")
m("L","r8,r1",[(CT,"\tif isObject && !wrongType {\n\t\tv.walkSets(","\tif false {\n\t\tv.walkSets(")],"walkLayers no walkSets")
m("P","r1",[(CC,"\t\t_, rest, found = strings.Cut(rest, \".\")\n\t}","\t\tfound = false\n\t}")],"atPath no suffix-by-segment walk")
m("R1","r1",[(CC,"isRoot && owner.Derivation","(isRoot || !isRoot) && owner.Derivation")],"owner.URL rule for any element")
m("R2","r1",[(CT,"\t\tif typeSD == self {","\t\tif false {")],"self type-profile case dropped")
m("R3","r1",[(SM,"\tif len(req.Node.Def.Slicing.Discriminator) == 0 {\n\t\treturn false, nil\n\t}","")],"no-discriminator early return removed")
m("R4","r1",[(CC,"owner.Derivation == registry.DerivationConstraint && !opts.explicit {","owner.Derivation == registry.DerivationConstraint {")],"explicit flag ignored")

m("S","r2",[(SN,"\tcorrectSnapshot(cmp.Or(sd.errataVersion, sd.FHIRVersion, baseSD.errataVersion, baseSD.FHIRVersion), snapshot.Element)\n","\tstampConstraintSources(snapshot, baseSnapshot, baseSD.URL)\n\tcorrectSnapshot(cmp.Or(sd.errataVersion, sd.FHIRVersion, baseSD.errataVersion, baseSD.FHIRVersion), snapshot.Element)\n"),(SN,"\tstampConstraintSources(snapshot, baseSnapshot, baseSD.URL)\n\t// fixed[x]","\t// fixed[x]")],"stamp before correctSnapshot (R2-F1 reverted)")

STAGES = [
 ("1","go test -count=1 -failfast ./pkg/constraint ./pkg/registry ./pkg/slicematch ./pkg/slicing"),
 ("2","go test -count=1 -failfast -run 'TestDescent|TestMutationProbe|TestHeldResource|TestConstraintDefinedIn|TestReviewR1|TestReviewR2' ./pkg/validator"),
 ("3","go test -count=1 -failfast ./pkg/..."),
]
only = sys.argv[1:]
res = open(W + "/results.tsv", "a")
for i, src, edits, desc in M:
    if only and i not in only: continue
    if edits is None:
        res.write(f"{i}\t{src}\t-\t{desc}\tOBSOLETE\t-\t-\n"); res.flush(); continue
    files = sorted({e[0] for e in edits}); orig = {}
    for f in files:
        orig[f] = open(f"{R}/{f}").read(); open(f"{BK}/{i}-{f.replace('/','_')}", "w").write(orig[f])
    cur = dict(orig); locs = []; bad = None
    for f, old, new, nth in edits:
        s = cur[f]; c = s.count(old)
        if c < nth or (nth == 1 and c != 1): bad = f"ANCHOR {f} count={c}: {old[:50]!r}"; break
        idx = -1
        for _ in range(nth): idx = s.index(old, idx + 1)
        locs.append(f"{f}:{s[:idx].count(chr(10))+1}")
        cur[f] = s[:idx] + new + s[idx+len(old):]
    if bad:
        res.write(f"{i}\t{src}\t-\t{desc}\t{bad}\t-\t-\n"); res.flush(); continue
    status = "SURVIVED"; first = "-"; stage = "-"
    try:
        for f in files: open(f"{R}/{f}", "w").write(cur[f])
        b = subprocess.run("go build ./...", shell=True, cwd=R, capture_output=True, text=True)
        if b.returncode:
            status = "BUILD-FAIL"; first = b.stderr.strip().splitlines()[0][:150] if b.stderr.strip() else "?"
        else:
            for sn, cmd in STAGES:
                r = subprocess.run(cmd, shell=True, cwd=R, capture_output=True, text=True, timeout=1500)
                out = r.stdout + r.stderr
                open(f"{OUT}/{i}-s{sn}.txt", "w").write(out)
                if r.returncode:
                    status = "KILLED"; stage = sn
                    fl = [l.strip().split(" ")[2] for l in out.splitlines() if l.strip().startswith("--- FAIL")]
                    first = fl[0] if fl else ("BUILD/PANIC: " + (out.strip().splitlines() or ["?"])[-1][:100])
                    break
    finally:
        for f in files: open(f"{R}/{f}", "w").write(orig[f])
    for f in files:
        assert open(f"{R}/{f}").read() == open(f"{BK}/{i}-{f.replace('/','_')}").read(), f
    res.write(f"{i}\t{src}\t{';'.join(locs)}\t{desc}\t{status}\t{first}\t{stage}\n"); res.flush()
