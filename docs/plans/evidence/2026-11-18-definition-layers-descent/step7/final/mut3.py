import subprocess, sys, shutil, time
F = "pkg/constraint/constraint.go"
M = {
 "21 reportedSource devuelve siempre c.Source": [("	owner := opts.owner\n	if owner == nil {", "	owner := opts.owner\n	if true {")],
 "22 sin la regla publishedSource": [("	if owner.Derivation == registry.DerivationConstraint {\n		if published", "	if false {\n		if published")],
 "23 sin la regla owner.URL": [("owner.SnapshotGenerated() {\n		return owner.URL", "owner.SnapshotGenerated() && false {\n		return owner.URL")],
 "27 owner.URL sin exigir snapshot generado": [(" && owner.SnapshotGenerated() {", " {")],
 "28 owner.URL sin exigir derivation constraint": [("c.Source == \"\" && owner.Derivation == registry.DerivationConstraint && ", "c.Source == \"\" && ")],
 "29 publishedSource sin buscar por elemento": [("				if _, dup := idx.byPath[pathRef{el.Path, ref}]; !dup {", "				if _, dup := idx.byPath[pathRef{el.Path, ref}]; false && !dup {")],
}
src = open(F).read()
for name, edits in M.items():
    s = src
    for a, b in edits:
        assert s.count(a) == 1, (name, a)
        s = s.replace(a, b)
    open(F, "w").write(s)
    t = time.time()
    r = subprocess.run(["go", "test", "-count=1", "-failfast", "-run", "TestConstraintDefinedIn|TestDescent|TestMutationProbe|TestHeldResource", "./pkg/validator"], capture_output=True, text=True)
    fails = [l.strip() for l in r.stdout.splitlines() if l.strip().startswith("--- FAIL")][:2]
    print(name, "KILLED" if r.returncode else "SURVIVED", fails, f"{time.time()-t:.0f}s")
    open(F, "w").write(src)
assert open(F).read() == src
