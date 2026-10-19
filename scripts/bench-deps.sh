#!/usr/bin/env bash
set -euo pipefail

# Descarga los packages de scripts/bench-packages.txt a una caché propia del repo
# (.cache/bench-packages, con el formato de la caché FHIR: nombre#versión/package/...). No toca
# ~/.fhir/packages. En cada corrida:
# - el .tgz guardado (.tgz/) se verifica contra su SHA-256 fijado; si falta o no coincide, se
#   descarga de nuevo y se verifica;
# - el árbol extraído se verifica contra el hash de sus archivos registrado al extraerlo
#   (.bench-sha256, que nombra también el SHA del .tgz); si no coincide, se vuelve a extraer.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
DEST="${BENCH_PACKAGES:-${ROOT_DIR}/.cache/bench-packages}"
REGISTRY="${FHIR_REGISTRY:-https://packages.fhir.org}"
MARKER=".bench-sha256"

# tree_hash imprime un hash de los archivos del árbol (rutas y contenidos), sin el marcador.
tree_hash() {
    (cd "$1" && find . -type f ! -name "${MARKER}" -print0 | LC_ALL=C sort -z \
        | xargs -0 shasum -a 256 | shasum -a 256 | cut -d' ' -f1)
}

sha_of() {
    shasum -a 256 "$1" | cut -d' ' -f1
}

mkdir -p "${DEST}/.tgz"
while read -r name version sha; do
    [[ -z "${name}" || "${name}" == \#* ]] && continue
    tgz="${DEST}/.tgz/${name}-${version}.tgz"
    target="${DEST}/${name}#${version}"

    if [[ ! -f "${tgz}" || "$(sha_of "${tgz}")" != "${sha}" ]]; then
        echo "downloading ${name}#${version}"
        curl -fsSL "${REGISTRY}/${name}/${version}" -o "${tgz}.part"
        got=$(sha_of "${tgz}.part")
        if [[ "${got}" != "${sha}" ]]; then
            rm -f "${tgz}.part"
            echo "error: ${name}#${version}: sha256 ${got}, want ${sha}" >&2
            exit 1
        fi
        mv "${tgz}.part" "${tgz}"
    fi

    # Un enlace u otra entrada que no sea archivo ni directorio no la cubre el hash: invalida el árbol.
    if [[ -f "${target}/${MARKER}" ]] && [[ -z "$(find "${target}" ! -type f ! -type d -print -quit)" ]] \
        && [[ "$(cat "${target}/${MARKER}")" == "${sha} $(tree_hash "${target}")" ]]; then
        continue
    fi
    echo "extracting ${name}#${version}"
    tmp="${DEST}/.extract-${name}-${version}"
    rm -rf "${tmp}"
    mkdir -p "${tmp}"
    tar -xzf "${tgz}" -C "${tmp}"
    echo "${sha} $(tree_hash "${tmp}")" > "${tmp}/${MARKER}"
    rm -rf "${target}"
    mv "${tmp}" "${target}"
done < "${SCRIPT_DIR}/bench-packages.txt"
echo "bench packages in ${DEST}"
