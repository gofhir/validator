# Mutaciones de #196 (paso 7). Uso: python3 -I mutations.py <copia del repo>
# Aplica cada defecto a mano en la copia, corre los tests de los paquetes tocados, y revierte.
# Una mutación sobrevive si los tests siguen pasando: el oráculo que debía atraparla se endurece.
import subprocess
import sys
import time

ROOT = sys.argv[1]
PKGS = ["./internal/elementvalues", "./internal/tools/hl7diff", "./pkg/structural", "./pkg/cardinality",
        "./pkg/slicing", "./pkg/constraint", "./pkg/validator"]

EV = "internal/elementvalues/elementvalues.go"
STRUCT = "pkg/structural/structural.go"
TREE = "pkg/constraint/tree.go"
FAM = "internal/tools/hl7diff/families.json"

MUTATIONS = [
    ("m1 un hermano _x solo no es variante", EV,
     "\tif underscored {\n\t\tif _, hasValue := inst[key]; hasValue {",
     "\tif underscored {\n\t\treturn \"\", \"\", false\n\t\tif _, hasValue := inst[key]; hasValue {"),
    ("m2 un tipo que solo declara el perfil es variante (#180)", EV,
     "if code == \"\" || (underscored", "if (underscored"),
    ("m3 variantes en el orden del mapa", EV, "\tsort.Strings(out)\n", "\n"),
    ("m4 cardinalidad: una aparición por variante", "pkg/cardinality/cardinality.go",
     "count := elementvalues.Occurrences(child, values)", "count := len(values)"),
    ("m5 slicing: una aparición por variante", "pkg/slicing/slicing.go",
     "count := elementvalues.Occurrences(child, values)", "count := len(values)"),
    ("m6 estructural: sin exclusión mutua", STRUCT,
     "\tv.reportCompetingVariants(data, sdPath, fhirPath, idx, result)\n", "\n"),
    ("m7 estructural: reconoce una clave por los tipos del perfil (#180)", STRUCT,
     "matchedType := v.baseChoiceType(idx, choiceElemDef, elementName)",
     "matchedType := choiceElemDef.ChoiceTypeOf(elementName[len(choiceBaseName):])"),
    ("m8 constraint: sin comparar el tipo de una variante con el perfil", TREE,
     "func (v *Validator) walk(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, value any, raw, element json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {\n\tseen := map[string]bool{}",
     "func (v *Validator) walk(sd *registry.StructureDefinition, node *registry.ElementNode, typeCode string, value any, raw, element json.RawMessage, fhirPath string, opts *constraintEvalOpts, result *issue.Result) {\n\tseen := map[string]bool{typeNotAllowedKey(fhirPath): true}"),
    ("m9 hl7diff: la exclusión mutua sin familia", FAM,
     "\"STRUCTURE_UNKNOWN_ELEMENT\",\n        \"STRUCTURE_CHOICE_MUTUAL_EXCLUSION\"", "\"STRUCTURE_UNKNOWN_ELEMENT\""),
    ("m11 hl7diff: por la lista de variantes, también el padre o el hijo", "internal/tools/hl7diff/match.go",
     "return Located(ruleEqual, ComparableGoLocation(goLoc), h.Location)",
     "return Located(ruleParent, ComparableGoLocation(goLoc), h.Location)"),
    ("m10 hl7diff: el MUTUAL nunca empareja con la propiedad que HL7 nombra", "internal/tools/hl7diff/match.go",
     "\tprop = strings.TrimPrefix(prop, \"_\")\n", "\tprop = \"\\x00\" + prop\n"),
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
