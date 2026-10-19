#!/usr/bin/env python3
"""Mutaciones del incremento B (paso 7): cada una por separado en la copia, con el test que debe fallar (bajo -race)."""
import pathlib
import subprocess
import sys

ROOT = pathlib.Path(sys.argv[1])
ONLY = sys.argv[2:]
TESTS = "TestPublishedTypes|TestResolvingAProfileKeepsTypes|TestIntermediateStepsDoNotPublish|TestSetFHIRVersionPublishesThePreferredType|TestResolvingANewerVersionOfATypePublishesIt|TestOnePublicationPerOperation"

MUTATIONS = [
    ("B1a LoadFromPackages no publica", "pkg/registry/registry.go",
     "\t// Se publica una vez, al terminar: el reindexado del principio no debe verse.\n\tr.publishTypesUnlocked()\n", ""),
    ("B1b SetFHIRVersion no publica", "pkg/registry/registry.go",
     "\tr.reindexUnlocked()\n\tr.publishTypesUnlocked()\n}", "\tr.reindexUnlocked()\n}"),
    ("B1c ResolveByCanonical no publica un tipo nuevo", "pkg/registry/registry.go",
     "\t\t\tr.refreshDerivedUnlocked() // a type is defined now\n\t\t\tr.publishTypesUnlocked()\n",
     "\t\t\tr.refreshDerivedUnlocked() // a type is defined now\n"),
    ("B7 ResolveByCanonical publica solo si el tipo es nuevo", "pkg/registry/registry.go",
     "\t\tif r.byType[sd.Type] != typeDef {\n\t\t\tr.refreshDerivedUnlocked() // a type is defined now\n\t\t\tr.publishTypesUnlocked()\n\t\t}",
     "\t\tif r.byType[sd.Type] != typeDef {\n\t\t\tr.refreshDerivedUnlocked() // a type is defined now\n\t\t\tif typeDef == nil {\n\t\t\t\tr.publishTypesUnlocked()\n\t\t\t}\n\t\t}"),
    ("B8 LoadFromPackages publica tras el reindexado inicial", "pkg/registry/registry.go",
     "\tif changed && len(r.all) > 0 {\n\t\tr.reindexUnlocked()\n\t}",
     "\tif changed && len(r.all) > 0 {\n\t\tr.reindexUnlocked()\n\t\tr.publishTypesUnlocked()\n\t}"),
    ("B9 la publicación final se mueve al reindexado inicial", "pkg/registry/registry.go", [
        ("\tif changed && len(r.all) > 0 {\n\t\tr.reindexUnlocked()\n\t}",
         "\treindexed := changed && len(r.all) > 0\n\tif reindexed {\n\t\tr.reindexUnlocked()\n\t\tr.publishTypesUnlocked()\n\t}"),
        ("\t// Se publica una vez, al terminar: el reindexado del principio no debe verse.\n\tr.publishTypesUnlocked()\n",
         "\tif !reindexed {\n\t\tr.publishTypesUnlocked()\n\t}\n"),
    ]),
    ("B6 un paso intermedio publica", "pkg/registry/registry.go",
     "\tr.buildTypeClassificationCaches()\n\tr.model = &FHIRPathModel{reg: r}\n}",
     "\tr.buildTypeClassificationCaches()\n\tr.model = &FHIRPathModel{reg: r}\n\tr.publishTypesUnlocked()\n}"),
    ("B2 Clone sin publicar", "pkg/registry/clone.go",
     "\tc.publishTypesUnlocked() // el clon aún no es visible para nadie: no hace falta su lock\n", ""),
    ("B3 publica con cada perfil constraint", "pkg/registry/registry.go",
     "\t\ttypeDef := r.byType[sd.Type]\n\t\tr.indexUnlocked(&sd)\n",
     "\t\ttypeDef := r.byType[sd.Type]\n\t\tr.indexUnlocked(&sd)\n\t\tr.publishTypesUnlocked()\n"),
    ("B4 GetByType lee el mapa vivo sin lock", "pkg/registry/registry.go",
     "\treturn r.types.Load().byType[typeName]", "\treturn r.byType[typeName]"),
    ("B5 publica el mapa vivo sin copiar", "pkg/registry/registry.go",
     "r.types.Store(&typeIndex{byType: maps.Clone(r.byType), domainResources: maps.Clone(r.domainResources)})",
     "_ = maps.Clone[map[string]bool]\n\tr.types.Store(&typeIndex{byType: r.byType, domainResources: r.domainResources})"),
]

for name, rel, *edit in MUTATIONS:
    if ONLY and name.split()[0] not in ONLY:
        continue
    # Una mutación es un reemplazo (old, new) o una lista de reemplazos aplicados juntos.
    edits = edit[0] if len(edit) == 1 else [tuple(edit)]
    path = ROOT / rel
    original = path.read_text()
    mutated = original
    missing = [old for old, _ in edits if mutated.count(old) != 1]
    if missing:
        print(f"{name}: NO APLICA (un texto no aparece una sola vez)")
        continue
    for old, new in edits:
        mutated = mutated.replace(old, new)
    path.write_text(mutated)
    try:
        proc = subprocess.run(["go", "test", "-race", "-short", "-count=1", "-run", TESTS, "./pkg/registry"],
                              cwd=ROOT, capture_output=True, text=True, timeout=900)
        out = proc.stdout + proc.stderr
        if "build failed" in out or "[setup failed]" in out:
            verdict = "NO COMPILA"
        elif proc.returncode != 0 and "FAIL" in out:
            verdict = "detectada" + (" (race)" if "DATA RACE" in out else "")
        else:
            verdict = "NO DETECTADA"
        print(f"{name}: {verdict}")
    finally:
        path.write_text(original)
