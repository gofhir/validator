#!/usr/bin/env bash
set -euo pipefail

# Compara los benchmarks de la validación entre una línea base (BASE, un ref de git) y el árbol de
# trabajo, intercalando las corridas (base, head, base, head...) para que la carga de la máquina
# afecte a ambos por igual. Los benchmarks con guías se copian al worktree de la base: solo usan la
# API pública. Los benchmarks en serie corren con un CPU; los paralelos, con cada valor de CPUS.
# El reporte de benchstat queda en docs/plans/evidence/<fecha>-<hora>-bench-compare-<head>/; una
# carpeta que ya existe no se sobrescribe.
#
#   BASE=74afca6 [COUNT=10] [CPUS=1,4,8] [BENCHTIME=1s] scripts/bench-compare.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
BASE="${BASE:?set BASE to the baseline git ref}"
COUNT="${COUNT:-10}"
CPUS="${CPUS:-1,4,8}"
BENCHTIME="${BENCHTIME:-1s}"
# Patrones de go test -bench. Uno con dos niveles no corre los benchmarks sin sub-benchmarks, así
# que los de Profiled (con sub-benchmarks) van aparte de los demás.
SERIAL_BENCH='^Benchmark(ValidateRotating|ValidatePatientWithData|ValidateObservation|ValidatorCreation|ValidatorLiveHeap)$'
SERIAL_SUB_BENCH='^BenchmarkValidateProfiled$/^(patient|blood-pressure|extension-values)$'
PARALLEL_BENCH='^BenchmarkValidateParallel$'
PARALLEL_SUB_BENCH='^BenchmarkValidateProfiled$/-parallel$'
# Benchmarks que deben aparecer en ambas salidas: los en serie con un CPU (nombre sin sufijo) y los
# paralelos con cada valor de CPUS (sufijo -N, salvo N = 1).
REQUIRED_SERIAL="BenchmarkValidateProfiled/patient BenchmarkValidateProfiled/blood-pressure
BenchmarkValidateProfiled/extension-values BenchmarkValidateRotating BenchmarkValidatePatientWithData BenchmarkValidateObservation
BenchmarkValidatorCreation BenchmarkValidatorLiveHeap"
REQUIRED_PARALLEL="BenchmarkValidateProfiled/patient-parallel BenchmarkValidateProfiled/blood-pressure-parallel
BenchmarkValidateProfiled/extension-values-parallel BenchmarkValidateParallel"
BENCHSTAT="golang.org/x/perf/cmd/benchstat@v0.0.0-20260929162123-406019bb8b68"
export BENCH_PACKAGES="${BENCH_PACKAGES:-${ROOT_DIR}/.cache/bench-packages}"

"${SCRIPT_DIR}/bench-deps.sh" >/dev/null

base_sha=$(git -C "${ROOT_DIR}" rev-parse --short=12 "${BASE}^{commit}")
head_sha=$(git -C "${ROOT_DIR}" rev-parse --short=12 HEAD)
dirty=""
if [[ -n "$(git -C "${ROOT_DIR}" status --porcelain --untracked-files=no)" ]]; then
    dirty=" (uncommitted changes)"
fi
out="${ROOT_DIR}/docs/plans/evidence/$(date +%Y-%m-%d-%H%M)-bench-compare-${head_sha}"
if [[ -e "${out}" ]]; then
    echo "error: ${out} exists; not overwriting evidence" >&2
    exit 1
fi
mkdir -p "${out}"

work=$(mktemp -d)
cleanup() {
    git -C "${ROOT_DIR}" worktree remove --force "${work}/base" >/dev/null 2>&1 || true
    rm -rf "${work}"
}
trap cleanup EXIT

git -C "${ROOT_DIR}" worktree add --detach "${work}/base" "${base_sha}" >/dev/null 2>&1
cp "${ROOT_DIR}/pkg/validator/profiled_bench_test.go" "${work}/base/pkg/validator/"
mkdir -p "${work}/base/testdata/bench"
cp "${ROOT_DIR}/testdata/bench/"*.json "${work}/base/testdata/bench/"

(cd "${work}/base" && go test -c -o "${work}/base.test" ./pkg/validator)
(cd "${ROOT_DIR}" && go test -c -o "${work}/head.test" ./pkg/validator)

# bench corre un binario de tests desde su directorio de paquete: los benchmarks en serie con un
# CPU y los paralelos con CPUS.
bench() {
    local dir=$1 bin=$2
    local pattern cpu
    for pattern in "${SERIAL_BENCH}:1" "${SERIAL_SUB_BENCH}:1" "${PARALLEL_BENCH}:${CPUS}" "${PARALLEL_SUB_BENCH}:${CPUS}"; do
        cpu=${pattern##*:}
        (cd "${dir}" && "${bin}" -test.run '^$' -test.bench "${pattern%:*}" -test.benchmem -test.cpu "${cpu}" \
            -test.benchtime "${BENCHTIME}" -test.count 1)
    done
}

: > "${out}/base.txt"
: > "${out}/head.txt"
for i in $(seq 1 "${COUNT}"); do
    echo "round ${i}/${COUNT}"
    bench "${work}/base/pkg/validator" "${work}/base.test" >> "${out}/base.txt"
    bench "${ROOT_DIR}/pkg/validator" "${work}/head.test" >> "${out}/head.txt"
done

# Los nombres exactos que esperamos, y los medidos (primera columna de cada resultado).
expected=$(
    for name in ${REQUIRED_SERIAL}; do echo "${name}"; done
    for name in ${REQUIRED_PARALLEL}; do
        for cpu in ${CPUS//,/ }; do
            if [[ "${cpu}" == 1 ]]; then echo "${name}"; else echo "${name}-${cpu}"; fi
        done
    done
)
for side in base head; do
    measured=$(awk '/^Benchmark/ && $2 ~ /^[0-9]+$/ {print $1}' "${out}/${side}.txt" | sort -u)
    for name in ${expected}; do
        if ! grep -Fxq "${name}" <<< "${measured}"; then
            echo "error: ${name} missing from ${side}.txt" >&2
            exit 1
        fi
    done
done

{
    echo "# Benchmark comparison"
    echo
    echo "- Date: $(date '+%Y-%m-%d %H:%M %Z')"
    echo "- Base: \`${base_sha}\` · Head: \`${head_sha}\`${dirty}"
    echo "- \`go version\`: $(go version)"
    echo "- Machine: $(sysctl -n machdep.cpu.brand_string 2>/dev/null || uname -m), $(sysctl -n hw.perflevel0.physicalcpu 2>/dev/null || echo '?')P+$(sysctl -n hw.perflevel1.physicalcpu 2>/dev/null || echo '?')E"
    echo "- Interleaved rounds: ${COUNT} · serial benchmarks: 1 CPU · parallel: ${CPUS} · benchtime: ${BENCHTIME}"
    echo "- Command: \`BASE=${BASE} COUNT=${COUNT} CPUS=${CPUS} BENCHTIME=${BENCHTIME} scripts/bench-compare.sh\`"
    echo
    echo '```'
    (cd "${out}" && go run "${BENCHSTAT}" base.txt head.txt)
    echo '```'
} > "${out}/report.md"
cat "${out}/report.md"
