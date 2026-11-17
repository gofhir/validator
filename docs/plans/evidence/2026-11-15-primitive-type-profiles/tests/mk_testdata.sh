#!/bin/sh
# Copia los perfiles (en R4, con fhirVersion que el test ajusta) y las instancias de las sondas de la pieza 3 a
# pkg/validator/testdata/primitive_type_profiles/, junto con lo de p14, p15, p16 (pieza 2) y s22 (children-precedence),
# y genera expected.json con gen_expected.py.
# Uso: mk_testdata.sh  (desde cualquier carpeta)
set -e
here=$(cd "$(dirname "$0")" && pwd)
ev="$here/../.."
out="$here/../../../../../pkg/validator/testdata/primitive_type_profiles"
rm -rf "$out/ig" "$out/inst"
mkdir -p "$out/ig" "$out/inst"
cp "$here"/../probe/ig-4.0.1/*.json "$out/ig/"
# p14, p15 y p16: dos perfiles de string en Observation.value[x]
cp "$ev"/2026-11-14-type-profile-layers/probe/ig-4.0.1/old-StructureDefinition-obs-str-or.json \
   "$ev"/2026-11-14-type-profile-layers/probe/ig-4.0.1/old-StructureDefinition-str-short.json \
   "$ev"/2026-11-14-type-profile-layers/probe/ig-4.0.1/old-StructureDefinition-str-upper.json "$out/ig/"
# s22: un slice de Patient.name.given con un perfil de string que no se resuelve
cp "$ev"/2026-11-05-children-precedence/probe/review2/ig/StructureDefinition-pat-given-miss.json "$out/ig/"
cp "$here"/../probe/inst/*.json "$out/inst/"
for n in p14 p15 p16; do cp "$ev/2026-11-14-type-profile-layers/probe/inst/$n.json" "$out/inst/"; done
cp "$ev/2026-11-05-children-precedence/probe/review2/inst/s22.json" "$out/inst/"
python3 -I "$here/gen_expected.py" "$out/expected.json"
