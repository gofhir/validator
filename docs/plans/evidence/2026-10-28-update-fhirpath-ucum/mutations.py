"""Mutaciones de #176 (plan §5). Uso: python3 -I mutations.py <copia del repo>.

Cada una se aplica sola sobre una copia limpia, corre los tests del incremento y tiene que hacerlos
fallar: KILLED. SURVIVED es un oráculo que no la ve.
"""
import pathlib, shutil, subprocess, sys

ROOT = pathlib.Path(sys.argv[1]).resolve()
TESTS = [
    ["go", "test", "./pkg/constraint", "-run", "TestHTMLChe", "-count=1"],
    ["go", "test", "./pkg/validator", "-run", "TestNarrativeAndUCUMAnnotation", "-count=1"],
]
WIRE = "\tevalCtx.SetHTMLChecker(v.html)\n"
CHECK = "\treturn validateXHTML(s), nil\n"
MUTATIONS = {
    "m1-no-checker": [("pkg/constraint/constraint.go", WIRE, "")],
    "m2-always-true": [("pkg/constraint/htmlchecks.go", CHECK, "\treturn true, nil\n")],
    "m3-global-function": [
        ("pkg/constraint/constraint.go", WIRE, ""),
        ("pkg/constraint/htmlchecks.go", "type narrativeHTML struct{}\n",
         "type narrativeHTML struct{}\n\nfunc init() {\n\tfuncs.Register(eval.FuncDef{Name: \"htmlChecks\", Fn: func(_ *eval.Context, in types.Collection, _ []any) (types.Collection, error) {\n"
         "\t\tif len(in) == 0 {\n\t\t\treturn types.Collection{}, nil\n\t\t}\n\t\ts, _ := in[0].(types.String)\n"
         "\t\treturn types.Collection{types.NewBoolean(validateXHTML(s.Value()))}, nil\n\t}})\n}\n"),
        ("pkg/constraint/htmlchecks.go", "\t\"golang.org/x/net/html\"\n",
         "\t\"github.com/gofhir/fhirpath/eval\"\n\t\"github.com/gofhir/fhirpath/funcs\"\n\t\"github.com/gofhir/fhirpath/types\"\n\t\"golang.org/x/net/html\"\n"),
    ],
    "m4-checker-errs": [("pkg/constraint/htmlchecks.go", CHECK, "\treturn false, context.Canceled\n")],
    "m5-ucum-v4.3.0": "go.mod",
    "m6-no-ucum-validate": [("pkg/ucumvalidator/ucumvalidator.go", "\tif err := v.ucum.Validate(code); err != nil {\n",
                             "\tif err := error(nil); err != nil {\n")],
}

def run(name, edits, work):
    if work.exists():
        shutil.rmtree(work)
    shutil.copytree(ROOT, work, ignore=shutil.ignore_patterns(".git", "hl7diff", "corpusrun"))
    if edits == "go.mod":
        subprocess.run(["go", "mod", "edit", "-require=github.com/gofhir/ucum/v4@v4.3.0"], cwd=work, check=True)
        subprocess.run(["go", "mod", "tidy"], cwd=work, check=True, capture_output=True)
        got = subprocess.run(["go", "list", "-m", "github.com/gofhir/ucum/v4"], cwd=work, capture_output=True, text=True).stdout.strip()
        print(f"  {name}: resolved {got}")
    else:
        for path, old, new in edits:
            f = work / path
            text = f.read_text()
            if text.count(old) != 1:
                return "NOT-APPLIED"
            f.write_text(text.replace(old, new))
    for cmd in TESTS:
        r = subprocess.run(cmd, cwd=work, capture_output=True, text=True)
        if "build failed" in r.stdout + r.stderr or "[setup failed]" in r.stdout:
            return "BUILD-FAILED"
        if r.returncode != 0:
            return "KILLED"
    return "SURVIVED"

work = pathlib.Path(sys.argv[2] if len(sys.argv) > 2 else "/tmp/m176").resolve()
for name, edits in MUTATIONS.items():
    print(name, run(name, edits, work / name), flush=True)
