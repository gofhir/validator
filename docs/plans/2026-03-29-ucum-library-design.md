# Design: github.com/gofhir/ucum

Go library for UCUM (Unified Code for Units of Measure). Port of the Java reference
implementation (FHIR/Ucum-java) with improvements to special unit handling.

## Scope

Full UCUM service: validate, convert, canonical form, comparability, analyse, multiply.

## API

```go
package ucum

type Service interface {
    Validate(code string) error
    ValidateInProperty(code, property string) error
    Canonical(value float64, code string) (Pair, error)
    Convert(value float64, from, to string) (float64, error)
    IsComparable(code1, code2 string) (bool, error)
    Analyse(code string) (string, error)
    Multiply(v1 Pair, v2 Pair) (Pair, error)
}

type Pair struct {
    Value float64
    Code  string
}

// New loads the embedded ucum-essence.xml (v2.2).
func New() (Service, error)

// NewFromReader loads definitions from a custom source.
func NewFromReader(r io.Reader) (Service, error)
```

### Error Types

```go
type ValidationError struct {
    Code    string // the UCUM code that failed
    Message string
    Offset  int    // position in the input string
}

type ConversionError struct {
    From    string
    To      string
    Message string
}
```

## Architecture

```
ucum/
  ucum.go              Service interface, Pair, New(), NewFromReader()
  service.go           Service implementation
  model.go             UcumModel, BaseUnit, DefinedUnit, Prefix, Unit, Value
  parser.go            Lexer + ExpressionParser (hand-written recursive descent)
  ast.go               Term, Symbol, Factor, Operator
  composer.go          AST to string (canonical and human-readable)
  converter.go         Normalization to canonical form (big.Rat internally)
  decimal.go           Wrapper over math/big.Rat (~50 lines)
  special.go           SpecialHandler interface + concrete handlers
  definitions.go       XML parser for ucum-essence.xml
  ucum-essence.xml     Embedded via //go:embed (v2.2, 2024-06-17)
  ucum_test.go         Main tests
  converter_test.go    Conversion tests
  parser_test.go       Parser tests
  testdata/
    UcumFunctionalTests.xml
```

### Data Flow

```
Input "mg/dL"
  -> Lexer (tokens)
  -> Parser (AST: Term/Symbol/Factor)
  -> Validate: resolve symbols against Model (prefix + unit)
  -> Canonical: Converter walks AST, multiply/divide with big.Rat
  -> Output: Pair{value, canonical_code}
```

## Arithmetic: math/big.Rat

Internal arithmetic uses `math/big.Rat` for exact rational operations.
The API accepts/returns `float64`. Conversion happens at boundaries only.

Why not shopspring/decimal (used elsewhere in gofhir):
- `big.Rat` gives exact division: `1/3 * 3 = 1` always
- shopspring truncates division to 16 digits: `1/3 * 3 = 0.999...`
- UCUM chains multiplications/divisions where exactness matters
- FHIRPath/CQL use shopspring correctly for their domain (decimal arithmetic on clinical values)

```go
type Decimal struct{ val *big.Rat }

func (d Decimal) Add(o Decimal) Decimal
func (d Decimal) Sub(o Decimal) Decimal
func (d Decimal) Mul(o Decimal) Decimal
func (d Decimal) Div(o Decimal) Decimal
func (d Decimal) Pow(n int) Decimal
func (d Decimal) Float64() float64
func DecimalFromString(s string) (Decimal, error)
```

Dependencies: zero (stdlib only).

## Model

```go
type UcumModel struct {
    Prefixes     []Prefix
    BaseUnits    []BaseUnit
    DefinedUnits []DefinedUnit

    // O(1) lookup indexes
    prefixByCode map[string]*Prefix
    unitByCode   map[string]*Unit
}

type Prefix struct {
    Code  string
    Name  string
    Value Decimal
}

type Unit struct {
    Code      string
    Name      string
    Property  string
    IsMetric  bool
    IsSpecial bool
    IsBase    bool
    Dim       string  // dimension symbol, base units only
    Value     *Value  // conversion factor, defined units only
}

type Value struct {
    Unit  string
    Text  string
    Value Decimal
}
```

Loaded from ucum-essence.xml via `encoding/xml` struct tags. Parsed once in `New()`,
indexed in maps, then read-only.

## Special Units (improvement over Java)

The Java library throws exceptions for offset units (Celsius, Fahrenheit) and silently
returns incorrect results for logarithmic units (pH, Bel, Neper). We implement all of them.

### Handler Interface

```go
type SpecialHandler interface {
    Code() string
    Units() string
    ToCanonical(value float64) float64
    FromCanonical(value float64) float64
}
```

### Conversion Pipeline

```
1. If source is special: value = handler.ToCanonical(value)
2. Normal multiplicative conversion between canonical bases
3. If dest is special: value = handler.FromCanonical(value)
```

### 12 Function Types, ~20 Units

| Function | Formula (special -> base) | Units |
|----------|--------------------------|-------|
| Celsius offset | x + 273.15 | Cel |
| Fahrenheit offset | (x + 459.67) * 5/9 | [degF] |
| Reaumur offset | (x + 273.15) * 5/4 | [degRe] |
| pH | 10^(-x) | [pH] |
| ln (neper) | e^x | Np |
| log10 (bel) | 10^x | B, B[W], B[kW] |
| 2*log10 | 10^(x/2) | B[SPL], B[V], B[mV], B[uV] |
| log2 | 2^x | bit_s |
| tan*100 | arctan(x/100) | [p'diop], %[slope] |
| sqrt | x^2 | [m/s2/Hz^(1/2)] |
| homeopathic X | 10^(-x) | [hp'_X] |
| homeopathic C/M/Q | base^(-x) | [hp'_C], [hp'_M], [hp'_Q] |

Arbitrary units (IU, CFU, etc.) validate as valid UCUM but return ConversionError
on Convert().

## Thread Safety

- `UcumModel` is read-only after construction (no mutex needed for reads)
- Parsed expression cache uses `sync.Map` for concurrent access
- `Service` is safe for concurrent use from multiple goroutines

## Expression Cache

Parsed ASTs are cached in a `sync.Map` keyed by the input code string.
No eviction needed since UCUM codes are a bounded set in practice.

## Annotations

The parser recognizes `{annotation}` syntax (e.g., `{score}`, `{copies}/mL`,
`mol{creat}`). Annotations are preserved in the AST but ignored during
conversion and canonical form computation.

## Testing

1. Port `UcumFunctionalTests.xml` from Java as table-driven Go tests
2. Parser unit tests: edge cases like `10*3/uL`, `{score}`, `[pH]`, `cm[H2O]`, `%`
3. Special unit tests: Celsius, Fahrenheit, pH, Bel conversions (these FAIL in Java)
4. Decimal exactness tests: verify `big.Rat` round-trips
5. Benchmarks for Validate (hot path for FHIR validator)
6. Thread safety: parallel Validate/Convert calls

Success criteria: pass all Java functional tests PLUS special unit conversions that Java cannot do.
