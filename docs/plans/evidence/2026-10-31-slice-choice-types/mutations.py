# Mutaciones de #187 (paso 7). Uso: python3 -I mutations.py <copia del repo>
# Aplica cada defecto a mano en la copia, corre los tests de los paquetes tocados, y revierte.
# Una mutación sobrevive si los tests siguen pasando: el oráculo que debía atraparla se endurece.
import subprocess
import sys
import time

ROOT = sys.argv[1]
PKGS = ["./pkg/constraint", "./pkg/extension", "./pkg/structural", "./pkg/validator"]

TREE = "pkg/constraint/tree.go"
STRUCT = "pkg/structural/structural.go"
EXT = "pkg/extension/extension.go"

MUTATIONS = [
    ("m1 sin tipo en las capas del slice (solo el elemento cortado)", TREE,
     "\t\tv.checkType(sd, n, typeCode, fhirPath, seen, opts, result)\n\t\tv.layer(",
     "\t\tif n.SliceOf == nil {\n\t\t\tv.checkType(sd, n, typeCode, fhirPath, seen, opts, result)\n\t\t}\n\t\tv.layer("),
    ("m2 sin tipo en ninguna capa del slice", TREE,
     "\t\tv.checkType(sd, n, typeCode, fhirPath, seen, opts, result)\n\t\tv.layer(",
     "\t\tv.layer("),
    ("m4 sin deduplicar (informa en cada capa)", TREE,
     "\tif seen[key] {\n\t\treturn\n\t}\n",
     ""),
    ("m5 payload: solo el primer tipo admitido", TREE,
     '"types":   strings.Join(codes, ", "),',
     '"types":   codes[0],'),
    ("m6 payload: perfil vacío", TREE,
     '"profile": sd.URL,',
     '"profile": "",'),
    ("m7 payload: tipo encontrado cambiado por el admitido", TREE,
     '"type":    typeCode,',
     '"type":    codes[0],'),
    ("m8 untyped siempre falso (t12)", TREE,
     "untyped := sd.Kind == kindResource && unrecognizedChoice(child, values, cv)",
     "untyped := false && unrecognizedChoice(child, values, cv)"),
    ("m9 untyped sin la condición de recurso", TREE,
     "untyped := sd.Kind == kindResource && unrecognizedChoice(child, values, cv)",
     "untyped := unrecognizedChoice(child, values, cv)"),
    ("m10 wrongType siempre falso (hijos del tipo propio)", TREE,
     "return typeCode != \"\" && len(node.Def.Type) > 0",
     "return false && typeCode != \"\" && len(node.Def.Type) > 0"),
    ("m11 checkValueless sin tipo", TREE,
     "\t\tv.checkType(sd, n, typeCode, fhirPath, seen, opts, result)\n\t\tcheck(n)",
     "\t\tcheck(n)"),
    ("m12 estructural: sin reconocer por la base", STRUCT,
     "\tnode := idx.sd.Tree().ByID(def.ID)\n",
     "\tnode := idx.sd.Tree().ByID(def.ID)\n\tnode = nil\n"),
    ("m13 estructural: toda variante de la base es el valor (sin baseOnly)", STRUCT,
     "\t\tif !allowed[basePath] {\n",
     "\t\tif true || !allowed[basePath] {\n"),
    ("m14 constraint: también compara con la definición propia del valor (doble informe)", TREE,
     " || judgedByExtension(sd, def, opts.own) {",
     " {"),
    ("m15 constraint: la definición propia nunca se marca", TREE,
     "\t\t\topts.own = self\n",
     "\t\t\t_ = self\n"),
    ("m16 extensión: sin el error de tipo", EXT,
     "\t\tif !allowed {\n",
     "\t\tif false && !allowed {\n"),
    ("m17 extensión: ruta con guion bajo en un primitivo", EXT,
     "inst, at := cv.Ext, extPath+\".\"+cv.Key",
     "inst, at := cv.Ext, extPath+\"._\"+cv.Key"),
    ("m18 checkType: el tipo nunca se declara (rechaza todo)", TREE,
     "\tif declaresType(def, typeCode) {\n\t\treturn\n\t}\n",
     ""),
    ("m19 constraint: omite toda la definición propia (own amplio de la ronda 1)", TREE,
     "\treturn p == nil || p.Parent == nil || (len(p.Def.Type) > 0 && p.Def.Type[0].Code == own.Type)",
     "\treturn true || p == nil"),
]


def run():
    t = time.time()
    p = subprocess.run(["go", "test", "-count=1", *PKGS], cwd=ROOT, capture_output=True, text=True)
    fails = [l for l in p.stdout.splitlines() if l.startswith("--- FAIL")]
    return p.returncode, fails, time.time() - t, p.stdout + p.stderr


code, fails, secs, out = run()
print(f"control: exit={code} ({secs:.0f}s)")
if code != 0:
    print(out[-3000:])
    sys.exit(1)

survivors = 0
for name, rel, old, new in MUTATIONS:
    path = f"{ROOT}/{rel}"
    src = open(path).read()
    if src.count(old) != 1:
        print(f"{name}: NO APLICA (el texto aparece {src.count(old)} veces)")
        survivors += 1
        continue
    open(path, "w").write(src.replace(old, new))
    try:
        code, fails, secs, out = run()
    finally:
        open(path, "w").write(src)
    if code == 0:
        survivors += 1
        print(f"{name}: SOBREVIVE ({secs:.0f}s)")
    elif not fails:
        print(f"{name}: NO COMPILA ({secs:.0f}s)\n{out[-800:]}")
        survivors += 1
    else:
        print(f"{name}: muere, {len(fails)} tests ({secs:.0f}s): " + ", ".join(f.split()[2] for f in fails[:4]))
print(f"sobreviven: {survivors}")
