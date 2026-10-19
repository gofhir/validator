.PHONY: download-specs test lint build check compare-baseline bench-deps bench bench-compare

# Línea base de las comparaciones (compare-baseline, bench-compare): un ref de git.
BASE ?=

# Las mismas versiones que .github/workflows/ci.yml: con otras, el lint da otro resultado (con Go
# 1.27 y golangci-lint 2.15 aparecen 27 avisos goconst que CI no ve; v2.11.3 no analiza Go 1.27).
LINT_GO ?= go1.25.0
LINT_VERSION ?= v2.11.3
LINT = GOTOOLCHAIN=$(LINT_GO) go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)

download-specs:
	./scripts/download-specs.sh

test:
	go test ./...

lint:
	$(LINT) run

build:
	go build ./...

# check es la verificación de cierre de un incremento (NUCLEO.md §10, paso 9): formato, vet, lint y
# tests con el detector de carreras (-short deja fuera las regeneraciones de guías completas).
check:
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
	go vet ./...
	$(LINT) run
	go test -race -short ./...

# compare-baseline exige que ningún resultado cambie respecto de BASE: corpusrun de la base y del
# árbol de trabajo sobre todo el manifiesto de hl7diff, las mismas líneas (hl7diff same).
compare-baseline:
	@test -n "$(BASE)" || { echo "usage: make compare-baseline BASE=<git ref>"; exit 2; }
	go run ./internal/tools/hl7diff same -baseline $(BASE) -heavy

# bench-deps descarga los packages de los benchmarks con guías, verificados con SHA-256.
bench-deps:
	./scripts/bench-deps.sh

bench: bench-deps
	go test -run '^$$' -bench . -benchmem ./pkg/validator

# bench-compare compara los benchmarks de BASE y del árbol de trabajo, intercalados (benchstat).
bench-compare:
	@test -n "$(BASE)" || { echo "usage: make bench-compare BASE=<git ref>"; exit 2; }
	BASE=$(BASE) ./scripts/bench-compare.sh
