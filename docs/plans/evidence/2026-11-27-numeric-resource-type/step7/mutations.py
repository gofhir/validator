"""Mutaciones del paso 7 (plan §5): cada una se aplica a una copia limpia del árbol y corre solo los tests nuevos."""
import os, re, shutil, subprocess, sys, time
M = os.path.dirname(os.path.abspath(__file__)); SRC = os.path.join(M, "src"); W = os.path.join(M, "work")
TESTS = [
 ("./internal/scopeindex/", "TestResolveOfANonResourceIsNotAFailure|TestFailureNamesTheResourceAndTheSite"),
 ("./pkg/validator/", "TestResourceTypeJSONType|TestBrokenResourceTypeNeverAbortsAScopeQuery|TestResolveScopedBrokenTarget|TestScopeOfNonResourceKeepsTheCallersScope"),
 ("./pkg/registry/", "TestResourceTypeKeyNeverHasAResourceBelow|TestResourceTypeGuardSeesAResourceBelow"),
]
MUTS = {
 "M1 Resolve registra MissingError por un no-recurso": ("internal/scopeindex/scopeindex.go", "if x.Err() != nil || !slicematch.IsResource(res) {", "if x.Err() != nil {"),
 "M2 ResolveScoped acepta entry rota por fullUrl": ("pkg/validator/slicematch.go", "if ok && !slicematch.IsResource(res) {", "if false && ok && !slicematch.IsResource(res) {"),
 "M3 indice acepta cualquier tipo": ("internal/scopeindex/scopeindex.go", "if slicematch.IsResource(v) && !sameMap(v, holder) {", "if _, has := v[resourceTypeKey]; has && !sameMap(v, holder) {"),
 "M4 sin diagnostico nuevo": ("pkg/structural/structural.go", "if _, isString := v.(string); !present || isString {", "if _, isString := v.(string); true || !present || isString {"),
 "M5 diagnostico solo en la raiz (contained)": ("pkg/structural/structural.go", 'ReportNoResourceType(result, resourceMap, fmt.Sprintf("%s[%d]", baseFhirPath, i))', 'result.AddErrorWithID(issue.DiagStructureNoResourceType, nil, fmt.Sprintf("%s[%d]", baseFhirPath, i))'),
 "M5b diagnostico solo en la raiz (elemento Resource)": ("pkg/structural/structural.go", "ReportNoResourceType(result, data, fhirPath)", "result.AddErrorWithID(issue.DiagStructureNoResourceType, nil, fhirPath)"),
 "M6 tipo JSON mal nombrado (Object/Array)": ("pkg/structural/structural.go", 'return "Object"', 'return "Array"'),
 "M6b null como Number": ("pkg/structural/structural.go", 'return "Null"', 'return "Number"'),
 "M7 guarda sin descendientes": ("pkg/registry/resource_type_guard_test.go", "for _, c := range slices.Concat(n.Children, n.Slices, []*ElementNode{n.Content}) {", "for _, c := range []*ElementNode{} {"),
 "M8 ScopeOf no conserva el Scope de quien pregunta": ("pkg/validator/slicematch.go", "return scope // lo que no es un recurso", "return slicematch.Scope{} // lo que no es un recurso"),
}
env = dict(os.environ, GOFLAGS="-count=1")
for name, (f, old, new) in MUTS.items():
    shutil.rmtree(W, ignore_errors=True); shutil.copytree(SRC, W, symlinks=True)
    p = os.path.join(W, f); s = open(p).read()
    if s.count(old) != 1: print(f"{name}: NO APLICA ({s.count(old)})"); continue
    open(p, "w").write(s.replace(old, new))
    t0 = time.time(); killed = []
    for pkg, run in TESTS:
        r = subprocess.run(["go", "test", pkg, "-run", run, "-failfast"], cwd=W, env=env, capture_output=True, text=True)
        if r.returncode != 0:
            fails = sorted(set(re.findall(r"--- FAIL: (\S+)", r.stdout))) or [r.stdout.strip().splitlines()[-1] if r.stdout.strip() else r.stderr.strip()[:120]]
            killed.append(f"{pkg} {fails[:3]}")
    print(f"{name}: {'MUERTA' if killed else 'VIVA'} ({time.time()-t0:.0f}s) {'; '.join(killed)}", flush=True)
