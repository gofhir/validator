"""Mutaciones del incremento #183 (plan §2). Uso: python3 -I mutations.py <copia del repo> <dir>.

Cada una se aplica sola sobre una copia limpia, corre los tests de ChoiceType y tiene que hacerlos
fallar: KILLED. SURVIVED es un oráculo que no la ve.
"""
import pathlib, shutil, subprocess, sys

ROOT = pathlib.Path(sys.argv[1]).resolve()
TESTS = [["go", "test", "./pkg/registry", "-run", "TestChoiceType", "-count=1", "-timeout", "180s"]]
R = "pkg/registry/registry.go"
MUTATIONS = {
    "m1-ignore-reference": [(R, "if target, _ := r.ContentReference(e.sd, e); target != nil {", "if target, _ := r.ContentReference(e.sd, e); target != nil && e == nil {")],
    "m2-parent-from-snapshot": [(R, "\t\tif e == nil {\n\t\t\te = p\n\t\t}\n", "\t\te = p\n")],
    "m3-no-target-type": [(R, "\tif len(types) == 0 {\n\t\ttypes = e.Def.Type\n\t}\n", "")],
    "m4-no-cycle-guard": [(R, "\tif seen[e] {\n\t\treturn nil\n\t}\n", "")],
    "m6-base-types-first": [(R, "\ttypes := p.Def.Type\n\tif len(types) == 0 {\n\t\ttypes = e.Def.Type\n\t}\n", "\ttypes := e.Def.Type\n\tif len(types) == 0 {\n\t\ttypes = p.Def.Type\n\t}\n")],
    "m7-target-by-path": [(R, "e.sd.Tree().ByID(e.Def.ID + rel)", "e.sd.Tree().ByID(e.Def.Path + rel)")],
    "m8-rel-from-id": [(R, "node.Def.Path[len(p.Def.Path):]", "node.Def.ID[len(p.Def.ID):]")],
    "m9-no-base-path": [(R, "\tif node.Def.Base == nil || node.Def.Base.Path == \"\" {\n\t\treturn nil\n\t}\n\treturn r.definedIn(node.sd, node.Def.Base.Path)\n", "\treturn nil\n")],
    "m10-id-in-profiles": [(R, "\tif e.sd.Derivation != DerivationConstraint {\n\t\tif n := e.sd.Tree().ByID(e.Def.ID + rel); n != nil {", "\t{\n\t\tif n := e.sd.Tree().ByID(e.Def.ID + rel); n != nil {")],
}

def run(name, edits, work):
    if work.exists():
        shutil.rmtree(work)
    shutil.copytree(ROOT, work, ignore=shutil.ignore_patterns(".git", "hl7diff", "corpusrun", "docs"))
    for path, old, new in edits:
        f = work / path
        text = f.read_text()
        if text.count(old) != 1:
            return "NOT-APPLIED"
        f.write_text(text.replace(old, new))
    for cmd in TESTS:
        r = subprocess.run(cmd, cwd=work, capture_output=True, text=True)
        out = r.stdout + r.stderr
        if "build failed" in out or "[setup failed]" in out:
            return "BUILD-FAILED"
        if r.returncode != 0:
            return "KILLED"
    return "SURVIVED"

if __name__ == "__main__":
    base = pathlib.Path(sys.argv[2])
    for name, edits in MUTATIONS.items():
        print(f"{name}: {run(name, edits, base / name)}", flush=True)
