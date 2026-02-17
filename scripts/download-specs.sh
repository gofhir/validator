#!/usr/bin/env bash
set -euo pipefail

# Downloads FHIR NPM packages and repackages them with only the resource types
# needed by the validator: StructureDefinition, ValueSet, CodeSystem.
# This reduces the embedded binary size significantly.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SPECS_DIR="${ROOT_DIR}/pkg/specs"

download_and_filter() {
    local name=$1 version=$2 output=$3
    local tmp
    tmp=$(mktemp -d)

    echo "Downloading ${name}#${version}..."
    curl -fsSL "https://packages.fhir.org/${name}/${version}" -o "${tmp}/original.tgz"

    # Extract
    mkdir -p "${tmp}/extracted"
    tar -xzf "${tmp}/original.tgz" -C "${tmp}/extracted"

    # Filter: keep only package.json and files whose resourceType is SD, VS, or CS
    mkdir -p "${tmp}/filtered/package"
    cp "${tmp}/extracted/package/package.json" "${tmp}/filtered/package/"

    local total=0
    local kept=0
    for f in "${tmp}/extracted/package/"*.json; do
        [ "$(basename "$f")" = "package.json" ] && continue
        [ "$(basename "$f")" = ".index.json" ] && continue
        total=$((total + 1))
        rt=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1])).get('resourceType',''))" "$f" 2>/dev/null || echo "")
        case "$rt" in
            StructureDefinition|ValueSet|CodeSystem)
                cp "$f" "${tmp}/filtered/package/"
                kept=$((kept + 1))
                ;;
        esac
    done

    # Repackage
    mkdir -p "$(dirname "$output")"
    tar -czf "$output" -C "${tmp}/filtered" package

    local orig_size new_size
    orig_size=$(wc -c < "${tmp}/original.tgz" | tr -d ' ')
    new_size=$(wc -c < "$output" | tr -d ' ')
    echo "  ${name}: ${kept}/${total} resources, $((orig_size/1024))KB -> $((new_size/1024))KB"

    rm -rf "$tmp"
}

echo "Downloading and filtering FHIR packages (SD+VS+CS only)..."
echo ""

# R4
download_and_filter "hl7.fhir.r4.core"           "4.0.1" "${SPECS_DIR}/r4/hl7.fhir.r4.core-4.0.1.tgz"
download_and_filter "hl7.terminology.r4"          "7.0.1" "${SPECS_DIR}/r4/hl7.terminology.r4-7.0.1.tgz"
download_and_filter "hl7.fhir.uv.extensions.r4"   "5.2.0" "${SPECS_DIR}/r4/hl7.fhir.uv.extensions.r4-5.2.0.tgz"

# R4B (only core; shares terminology and extensions with R4)
download_and_filter "hl7.fhir.r4b.core"           "4.3.0" "${SPECS_DIR}/r4b/hl7.fhir.r4b.core-4.3.0.tgz"

# R5
download_and_filter "hl7.fhir.r5.core"            "5.0.0" "${SPECS_DIR}/r5/hl7.fhir.r5.core-5.0.0.tgz"
download_and_filter "hl7.terminology.r5"           "7.0.1" "${SPECS_DIR}/r5/hl7.terminology.r5-7.0.1.tgz"
download_and_filter "hl7.fhir.uv.extensions.r5"    "5.2.0" "${SPECS_DIR}/r5/hl7.fhir.uv.extensions.r5-5.2.0.tgz"

echo ""
echo "Done. Filtered packages saved to pkg/specs/"
echo ""
du -sh "${SPECS_DIR}/r4/" "${SPECS_DIR}/r4b/" "${SPECS_DIR}/r5/"
