import subprocess, sys, os
R = os.path.join(os.path.dirname(os.path.abspath(__file__)), "copy")
CT = "pkg/constraint/tree.go"; CC = "pkg/constraint/constraint.go"; SN = "pkg/registry/snapshot.go"; SM = "pkg/slicematch/slicematch.go"
M = {
 "F prim-ext-no-chain": (CT, "\tfor n := node; n != nil; n = n.SliceOf {\n\t\tif opts.firstDescent(ext", "\tfor n := node; n != nil; n = nil {\n\t\tif opts.firstDescent(ext"),
 "L walkLayers-no-sets": (CT, "\tif isObject && !wrongType {\n\t\tv.walkSets(", "\tif false {\n\t\tv.walkSets("),
 "B carry-no-pattern": (SN, 'for _, prefix := range []string{"fixed", "pattern"} {', 'for _, prefix := range []string{"fixed"} {'),
 "E prim-ext-no-scope": (CT, "\tif definitions > 1 {\n\t\topts = opts.scoped()\n\t}", "\topts = opts.scoped()"),
 "P no-suffix-mapping": (CC, "\t\t_, rest, found = strings.Cut(rest, \".\")\n\t}", "\t\tfound = false\n\t}"),
 "K published-last-not-first": (CC, "\t\t\t\tif _, dup := idx.any[ref]; !dup {\n\t\t\t\t\tidx.any[ref] = pc.Source\n\t\t\t\t}", "\t\t\t\tidx.any[ref] = pc.Source"),
 "R1 suffix-any-element": (CC, "isRoot && owner.Derivation", "(isRoot || !isRoot) && owner.Derivation"),
 "E2 prim-ext-never-scoped": (CT, "\tif definitions > 1 {\n\t\topts = opts.scoped()\n\t}", "\tif false {\n\t\topts = opts.scoped()\n\t}"),
 "R2 self-type-profile": (CT, "\t\tif typeSD == self {", "\t\tif false {"),
 "R3 no-discriminator-implicit-off": (SM, "\tif len(req.Node.Def.Slicing.Discriminator) == 0 {\n\t\treturn false, nil\n\t}", ""),
 "R4 explicit-ignored": (CC, "owner.Derivation == registry.DerivationConstraint && !opts.explicit {", "owner.Derivation == registry.DerivationConstraint {"),
}
only = sys.argv[1:]
for name, (f, old, new) in M.items():
    if only and not any(name.startswith(o) for o in only): continue
    p = os.path.join(R, f); s = open(p).read()
    if s.count(old) != 1: print(name, "ANCHOR", s.count(old)); continue
    open(p, "w").write(s.replace(old, new))
    try:
        pat = "TestReviewR1" if name.startswith("R") else "TestReviewR1|TestDescent|TestConstraintDefinedIn|TestMutationProbe"
        r = subprocess.run("go test -count=1 -failfast -run '" + pat + "' ./pkg/validator", shell=True, cwd=R, capture_output=True, text=True, timeout=1800)
        out = r.stdout + r.stderr
        fails = [l.strip() for l in out.splitlines() if "--- FAIL" in l][:2]
        if r.returncode == 0 and not name.startswith("R"):
            r = subprocess.run("go test -count=1 -failfast ./pkg/registry ./pkg/slicematch ./pkg/constraint ./pkg/validator", shell=True, cwd=R, capture_output=True, text=True, timeout=1800)
            out += r.stdout + r.stderr
            fails = [l.strip() for l in out.splitlines() if "--- FAIL" in l][:2]
        print(name, "KILLED" if r.returncode else "SURVIVES", fails, flush=True)
        open(os.path.join(os.path.dirname(R), "mut3-" + name.split()[0] + ".txt"), "w").write(out)
    finally:
        open(p, "w").write(s)
