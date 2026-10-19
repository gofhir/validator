#!/usr/bin/env python3
"""Mutaciones del incremento A (paso 7): cada una por separado en la copia, con el test que debe fallar."""
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(sys.argv[1])
ONLY = sys.argv[2:]  # nombres de mutaciones a correr (vacío: todas)

MUTATIONS = [
    ("M1 GetFixed devuelve pattern", "pkg/registry/registry.go",
     "return ed.prefixedValuesOf().fixed.answer()", "return ed.prefixedValuesOf().pattern.answer()",
     "TestPrefixedValues(PublishedSnapshots|Cases)"),
    ("M2 sin hermano _ de fixed", "pkg/registry/prefixed_values.go",
     'p.fixedElement = siblingOf(obj, "fixed", p.fixed)', "p.fixedElement = prefixedValue{}",
     "TestPrefixedValues(PublishedSnapshots|Cases)"),
    ("M3 sin extracción en Snapshot.UnmarshalJSON", "pkg/registry/registry.go",
     "\t\ts.Element[i].keepPrefixedValues()\n", "",
     "TestPrefixedValuesPublishedSnapshots"),
    ("M4 sin extracción al final de generateSnapshot", "pkg/registry/snapshot.go",
     "snapshot.Element[i].keepPrefixedValues()", "_ = snapshot.Element[i]",
     "TestPrefixedValuesGeneratedSnapshots"),
    ("M5 valor guardado aunque raw cambió", "pkg/registry/prefixed_values.go",
     "if p == nil || !sameSlice(ed.prefixedSrc, ed.raw) {", "if p == nil {",
     "TestPrefixedValuesWithoutExtraction"),
    ("M5b sameSlice compara solo el largo", "pkg/registry/prefixed_values.go",
     "return len(a) == 0 || &a[0] == &b[0]", "return true",
     "TestPrefixedValuesWithoutExtraction"),
    ("M8 prefiltro sin el caso de escapes", "pkg/registry/prefixed_values.go",
     "bytes.Contains(raw, []byte(\"pattern\")) ||\n\t\tbytes.Contains(raw, []byte(`\\u`))",
     "bytes.Contains(raw, []byte(\"pattern\"))",
     "TestPrefixedValuesCases"),
    ("M6 devuelve el slice guardado sin copiar", "pkg/registry/prefixed_values.go",
     "return bytes.Clone(v.value), v.suffix, true", "_ = bytes.Clone\n\treturn v.value, v.suffix, true",
     "TestPrefixedValuesAreNotShared"),
    ("M7 SetRaw sin copiar la entrada", "pkg/registry/registry.go",
     "ed.raw = bytes.Clone(data)", "_ = bytes.Clone\n\ted.raw = data",
     "TestPrefixedValuesAreNotShared"),
]

results = []
for name, rel, old, new, tests in MUTATIONS:
    if ONLY and name.split()[0] not in ONLY:
        continue
    path = ROOT / rel
    original = path.read_text()
    if original.count(old) != 1:
        results.append((name, "NO APLICA: el texto no aparece una sola vez"))
        continue
    path.write_text(original.replace(old, new))
    try:
        proc = subprocess.run(
            ["go", "test", "-short", "-count=1", "-run", tests, "./pkg/registry"],
            cwd=ROOT, capture_output=True, text=True, timeout=900)
        failed = proc.returncode != 0 and "FAIL" in proc.stdout + proc.stderr
        build = "build failed" in proc.stdout + proc.stderr
        verdict = "detectada" if failed and not build else ("NO COMPILA" if build else "NO DETECTADA")
        results.append((name, f"{verdict} por {tests}"))
    finally:
        path.write_text(original)

for name, verdict in results:
    print(f"{name}: {verdict}")
