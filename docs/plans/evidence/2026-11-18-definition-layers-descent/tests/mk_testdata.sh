#!/bin/sh
# Copia los perfiles y las instancias de R4 que generó run_hl7.sh en <work> a
# pkg/validator/testdata/definition_layers/<fuente>/ (el test ajusta fhirVersion por versión: los archivos de cada
# fuente solo difieren en ese campo entre versiones) y genera expected.json con gen_expected.py.
# Uso: mk_testdata.sh <work>
set -e
here=$(cd "$(dirname "$0")" && pwd)
work=${1:?carpeta de trabajo de run_hl7.sh}
out="$here/../../../../../pkg/validator/testdata/definition_layers"
rm -rf "${out:?}"
for s in matrix x2 p g probe m1; do
  mkdir -p "$out/$s"
  cp -R "$work/$s/4.0.1/ig" "$work/$s/4.0.1/inst" "$out/$s/"
done
python3 -I "$here/gen_expected.py" "$work" "$out/expected.json"
