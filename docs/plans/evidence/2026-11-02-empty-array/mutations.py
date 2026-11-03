# Mutaciones de #190 (paso 7). Uso: python3 -I mutations.py <copia del repo>
# Aplica cada defecto a mano en la copia, corre solo los tests que deben atraparlo (con -failfast),
# y revierte. Una mutación sobrevive si esos tests siguen pasando: el oráculo se endurece.
import subprocess
import sys
import time

ROOT = sys.argv[1]
VALIDATOR = ["./pkg/validator", "-run", "TestEmptyArray", "-failfast"]
HL7DIFF = ["./internal/tools/hl7diff"]
BOTH = ["./pkg/structural", "./pkg/validator", "-run", "EmptyArray|EmptyPrimitive", "-failfast"]

SHAPE = "pkg/structural/shape.go"
NORM = "pkg/structural/normalize.go"
FAM = "internal/tools/hl7diff/families.json"

REPORT = '\tresult.AddErrorWithID(issue.DiagStructureArrayEmpty, map[string]any{"element": key}, path)\n'

# (nombre, archivo, tests, viejo, nuevo[, viejo2, nuevo2...])
MUTATIONS = [
    ("m1 el chequeo apagado", SHAPE, VALIDATOR, REPORT, "\n"),
    ("m2 solo complejos (sin primitivos en R4/R4B)", SHAPE, VALIDATOR,
     "if primitive && !emptyPrimitiveArraysForbidden(v.registry.FHIRVersion()) {", "if primitive {"),
    ("m3 primitivos también en R5", SHAPE, VALIDATOR,
     "if primitive && !emptyPrimitiveArraysForbidden(v.registry.FHIRVersion()) {", "if false && primitive && !emptyPrimitiveArraysForbidden(v.registry.FHIRVersion()) {"),
    ("m4 R4B fuera de la lista de releases", SHAPE, VALIDATOR,
     '[]string{"1.0", "3.0", "4.0", "4.3"}', '[]string{"1.0", "3.0", "4.0"}'),
    ("m5 también cuando el elemento no se repite", SHAPE, VALIDATOR,
     "if isArray && len(items) == 0 && repeats(resolved) {", "if isArray && len(items) == 0 {"),
    ("m6 sin registrar el objeto que lo contiene (ele-1)", SHAPE, VALIDATOR,
     "\tctx.edits.holders.add(data)\n", "\n"),
    ("m7 el path con índice", SHAPE, VALIDATOR,
     REPORT, '\tresult.AddErrorWithID(issue.DiagStructureArrayEmpty, map[string]any{"element": key}, path+"[0]")\n'),
    ("m8 también sin edits (duplicado en la fase estructural)", SHAPE, BOTH,
     "\tif ctx.edits == nil {\n\t\treturn\n\t}\n\tctx.edits.holders.add(data)\n",
     "\tif ctx.edits != nil {\n\t\tctx.edits.holders.add(data)\n\t}\n"),
    ("m9 el ID nuevo fuera del switch de Normalize", NORM, VALIDATOR,
     "case issue.DiagStructureEmptyObject, issue.DiagStructureArrayEmpty:", "case issue.DiagStructureEmptyObject:"),
    ("m10 el chequeo detrás de la compuerta primitiveKind (D8)", SHAPE, BOTH,
     "\tprimitive, known := v.primitiveKind(resolved)\n\titems, isArray := value.([]any)\n",
     "\tprimitive, known := v.primitiveKind(resolved)\n\tif !known {\n\t\treturn value, true\n\t}\n\titems, isArray := value.([]any)\n"),
    ("m11 el _x vacío no se revisa", SHAPE, VALIDATOR,
     "\tif len(items) == 0 {\n\t\tv.checkEmptyArray(data, sibling, path, true, ctx, result)\n\t\treturn true\n\t}\n", "\n"),
    ("m12 hl7diff: el ID de gofhir mal escrito en la familia", FAM, HL7DIFF,
     '        "STRUCTURE_ARRAY_EMPTY"\n', '        "STRUCTURE_ARRAY_EMPTY_X"\n'),
    ("m13 hl7diff: el texto de HL7 no coincide", FAM, HL7DIFF,
     '"^Array cannot be empty"', '"^Array must not be empty"'),
    ("m14 DSTU2 fuera de la lista de releases", SHAPE, BOTH,
     '[]string{"1.0", "3.0", "4.0", "4.3"}', '[]string{"3.0", "4.0", "4.3"}'),
    ("m15 una versión desconocida sigue la regla de R4", SHAPE, BOTH,
     'return version != "" && slices.ContainsFunc', 'return version == "" || slices.ContainsFunc'),
]


def run(args):
    t = time.time()
    p = subprocess.run(["go", "test", "-count=1", *args], cwd=ROOT, capture_output=True, text=True)
    fails = [l for l in p.stdout.splitlines() if l.startswith("--- FAIL")]
    return p.returncode, fails, time.time() - t, p.stdout + p.stderr


for args in (VALIDATOR, HL7DIFF, BOTH):
    code, _, secs, out = run(args)
    print(f"control {' '.join(args)}: exit={code} ({secs:.0f}s)")
    if code != 0:
        print(out[-3000:])
        sys.exit(1)

died = 0
for name, path, args, *pairs in MUTATIONS:
    full = f"{ROOT}/{path}"
    orig = open(full).read()
    src = orig
    for old, new in zip(pairs[::2], pairs[1::2]):
        if src.count(old) != 1:
            print(f"{name}: SIN APLICAR (el texto aparece {src.count(old)} veces)")
            break
        src = src.replace(old, new)
    else:
        open(full, "w").write(src)
        try:
            code, fails, secs, out = run(args)
        finally:
            open(full, "w").write(orig)
        if code != 0:
            died += 1
            build = "" if fails else " (no compila o falla fuera de un test)"
            print(f"{name}: MUERE ({secs:.0f}s){build} {fails[:2]}")
        else:
            print(f"{name}: SOBREVIVE ({secs:.0f}s)")
print(f"{died}/{len(MUTATIONS)} mueren")
