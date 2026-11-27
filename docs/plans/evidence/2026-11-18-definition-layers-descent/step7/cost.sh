#!/bin/bash
# Paso 7, criterio 4: tiempo y RSS del CLI de main (base) y de la rama (head) sobre los 5.306 ejemplos del core R4,
# intercalados, N corridas cada uno. Uso: cost.sh <dir con bin/gv-base, bin/gv-head y core-r4.files> [N]. Correr con
# la máquina libre.
D=$1; N=${2:-3}; files=(); while IFS= read -r l; do files+=("$l"); done < "$D/core-r4.files"
args=(-no-download -tx n/a -version 4.0.1 -output json)
for i in $(seq "$N"); do for m in base head; do
 /usr/bin/time -l "$D/bin/gv-$m" "${args[@]}" "${files[@]}" > /dev/null 2> "$D/t.$m.$i"
 printf '%s %s ' "$m" "$i"; awk '/ real /{printf "%ss ", $1} /maximum resident/{printf "%d MB\n", $1/1048576}' "$D/t.$m.$i"
done; done
