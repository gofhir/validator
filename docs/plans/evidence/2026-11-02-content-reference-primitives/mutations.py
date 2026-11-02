# Mutaciones de #189 (paso 7). Uso: python3 -I mutations.py <copia del repo>
# Aplica cada defecto a mano en la copia, corre los tests de los paquetes tocados, y revierte.
# Una mutación sobrevive si los tests siguen pasando: el oráculo que debía atraparla se endurece.
import subprocess
import sys
import time

ROOT = sys.argv[1]
PKGS = ["./internal/tools/hl7diff", "./pkg/registry", "./pkg/primitive", "./pkg/validator"]

PRIM = "pkg/primitive/primitive.go"
TREE = "pkg/registry/tree.go"
FAM = "internal/tools/hl7diff/families.json"

MUTATIONS = [
    ("m1 primitivos: no sigue la contentReference", PRIM,
     "\t\tv.validateContentReference(data, resolved.elemDef, fhirPath, currentIdx, ctx, result)\n", "\n"),
    ("m2 primitivos: sigue la referencia aunque el snapshot desenrolle los hijos", PRIM,
     "\t\tif idx.parents[elemDef.Path] {", "\t\tif false && idx.parents[elemDef.Path] {"),
    ("m3 primitivos: el destino como contenedor en línea, sin despachar por tipo", PRIM,
     "\t\t\tv.validateComplexElement(data, &resolvedElement{elemDef: elemDef}, elemDef.Path, fhirPath, idx, ctx, result)",
     "\t\t\tv.validateElement(data, elemDef.Path, fhirPath, idx, ctx, result)"),
    ("m4 primitivos: sin cadenas (un salto)", PRIM,
     "if hops >= registry.MaxContentReferenceHops {", "if hops >= 1 {"),
    ("m5 primitivos: el índice de la definición propia para un destino de otra", PRIM,
     "target.def, target.idx = tnode.Def, v.getOrBuildIndex(tsd)", "target.def, target.idx, _ = tnode.Def, idx, tsd"),
    ("m6 primitivos: un solo destino en cache por índice", PRIM,
     "idx.targets.Load(elemDef)", "idx.targets.Load(idx.sd)",
     "idx.targets.LoadOrStore(elemDef, target)", "idx.targets.LoadOrStore(idx.sd, target)"),
    ("m7 registro: url#id resuelto en la definición propia", TREE,
     "\ttarget := sd\n\tif !isOwnCanonical(sd, url) {", "\ttarget := sd\n\tif false && !isOwnCanonical(sd, url) {"),
    ("m8 hl7diff: el error de tipo JSON sin familia", FAM,
     '        "TYPE_WRONG_JSON_TYPE"\n      ],\n      "hl7NoID": [\n        {\n          "code": "invalid",\n          "text": "^Error parsing JSON: the primitive value must be a "',
     '        "TYPE_WRONG_JSON_TYPE_X"\n      ],\n      "hl7NoID": [\n        {\n          "code": "invalid",\n          "text": "^Error parsing JSON: the primitive value must be a "'),
    ("m9 hl7diff: el texto del parser de HL7 no coincide", FAM,
     '"^Error parsing JSON: the primitive value must be a "', '"^Error parsing JSON: the value must be a "'),
    ("m10 primitivos: sigue el destino primero; los hijos propios solo si no resuelve", PRIM,
     "\t\tif idx.parents[elemDef.Path] {\n\t\t\tv.validateElement(data, elemDef.Path, fhirPath, idx, ctx, result)\n\t\t\treturn\n\t\t}\n\t\tif hops >= registry.MaxContentReferenceHops {\n\t\t\treturn\n\t\t}\n\t\ttarget := v.resolveContentTarget(idx, elemDef)\n\t\tif target.def == nil {\n\t\t\treturn\n\t\t}\n",
     "\t\tif hops >= registry.MaxContentReferenceHops {\n\t\t\treturn\n\t\t}\n\t\ttarget := v.resolveContentTarget(idx, elemDef)\n\t\tif target.def == nil {\n\t\t\tif idx.parents[elemDef.Path] {\n\t\t\t\tv.validateElement(data, elemDef.Path, fhirPath, idx, ctx, result)\n\t\t\t}\n\t\t\treturn\n\t\t}\n"),
    ("m11 primitivos: los hijos propios solo en el primer salto", PRIM,
     "\t\tif idx.parents[elemDef.Path] {\n\t\t\tv.validateElement(data, elemDef.Path, fhirPath, idx, ctx, result)\n\t\t\treturn",
     "\t\tif hops == 0 && idx.parents[elemDef.Path] {\n\t\t\tv.validateElement(data, elemDef.Path, fhirPath, idx, ctx, result)\n\t\t\treturn"),
]

MUTATIONS.append(
    ("m12 primitivos: la cota de saltos antes de los hijos propios", PRIM,
     "\t\tif idx.parents[elemDef.Path] {\n\t\t\tv.validateElement(data, elemDef.Path, fhirPath, idx, ctx, result)\n\t\t\treturn\n\t\t}\n\t\tif hops >= registry.MaxContentReferenceHops {\n\t\t\treturn\n\t\t}\n",
     "\t\tif hops >= registry.MaxContentReferenceHops {\n\t\t\treturn\n\t\t}\n\t\tif idx.parents[elemDef.Path] {\n\t\t\tv.validateElement(data, elemDef.Path, fhirPath, idx, ctx, result)\n\t\t\treturn\n\t\t}\n"))


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
for name, rel, *pairs in MUTATIONS:
    path = f"{ROOT}/{rel}"
    src = open(path).read()
    mutated, bad = src, None
    for old, new in zip(pairs[::2], pairs[1::2]):
        if mutated.count(old) != 1:
            bad = mutated.count(old)
            break
        mutated = mutated.replace(old, new)
    if bad is not None:
        print(f"{name}: NO APLICA (el texto aparece {bad} veces)")
        survivors += 1
        continue
    open(path, "w").write(mutated)
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
