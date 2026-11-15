#!/bin/sh
# Copia los perfiles (en R4, con fhirVersion que el test ajusta) y las instancias de las sondas de la pieza 2 a
# pkg/validator/testdata/type_profile_layers/, y genera expected.json con gen_expected.py.
# Uso: mk_testdata.sh  (desde cualquier carpeta)
set -e
here=$(cd "$(dirname "$0")" && pwd)
out="$here/../../../../../pkg/validator/testdata/type_profile_layers"
rm -rf "$out/ig" "$out/inst"
mkdir -p "$out/ig" "$out/inst"
cp "$here"/../probe-callers/ig-4.0.1/*.json "$here"/../probe-gaps/ig-4.0.1/*.json "$out/ig/"
cp "$here"/../probe/inst/*.json "$here"/../probe-callers/inst/*.json "$here"/../probe-gaps/inst/*.json "$out/inst/"
python3 -I "$here/gen_expected.py" "$out/expected.json"
