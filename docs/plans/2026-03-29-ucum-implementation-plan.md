# gofhir/ucum Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a standalone Go UCUM library with full validation, conversion, canonical form, comparability, analyse, and multiply — improving on the Java reference by correctly handling all special units.

**Architecture:** Lexer -> Parser -> AST -> Converter pipeline. Definitions loaded from embedded ucum-essence.xml. Internal arithmetic uses math/big.Rat for exact rational operations. Special units use function-pair handlers instead of Java's broken multiplicative-only approach.

**Tech Stack:** Go stdlib only (math/big, encoding/xml, embed, sync)

**Repo:** github.com/gofhir/ucum (new standalone repository)

---

## Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `ucum.go`
- Create: `errors.go`

**Step 1: Initialize Go module**

```bash
mkdir -p ~/projects/personal/opensource/ucum && cd ~/projects/personal/opensource/ucum
go mod init github.com/gofhir/ucum
```

**Step 2: Create ucum.go with public API types**

```go
// Package ucum provides UCUM (Unified Code for Units of Measure) services
// including validation, conversion, and canonical form computation.
package ucum

import "io"

// Service is the main interface for UCUM operations.
type Service interface {
	// Validate checks whether a UCUM code is syntactically valid.
	Validate(code string) error

	// ValidateInProperty validates a code and checks it measures the given property.
	ValidateInProperty(code, property string) error

	// Canonical returns the canonical form of a value+code pair.
	Canonical(value float64, code string) (Pair, error)

	// Convert converts a value from one unit to another.
	Convert(value float64, from, to string) (float64, error)

	// IsComparable reports whether two units measure the same property.
	IsComparable(code1, code2 string) (bool, error)

	// Analyse returns a human-readable description of a UCUM code.
	Analyse(code string) (string, error)

	// Multiply multiplies two value+unit pairs.
	Multiply(v1, v2 Pair) (Pair, error)
}

// Pair represents a numeric value with its UCUM unit code.
type Pair struct {
	Value float64
	Code  string
}

// New creates a Service using the embedded ucum-essence.xml definitions.
func New() (Service, error) {
	return newService(nil)
}

// NewFromReader creates a Service loading definitions from a custom source.
func NewFromReader(r io.Reader) (Service, error) {
	return newService(r)
}
```

**Step 3: Create errors.go**

```go
package ucum

import "fmt"

// ValidationError indicates an invalid UCUM code.
type ValidationError struct {
	Code    string
	Message string
	Offset  int
}

func (e *ValidationError) Error() string {
	if e.Offset >= 0 {
		return fmt.Sprintf("invalid UCUM code %q at position %d: %s", e.Code, e.Offset, e.Message)
	}
	return fmt.Sprintf("invalid UCUM code %q: %s", e.Code, e.Message)
}

// ConversionError indicates a failed unit conversion.
type ConversionError struct {
	From    string
	To      string
	Message string
}

func (e *ConversionError) Error() string {
	return fmt.Sprintf("cannot convert %q to %q: %s", e.From, e.To, e.Message)
}
```

**Step 4: Verify it compiles**

```bash
go build ./...
```

**Step 5: Commit**

```bash
git init && git add -A
git commit -m "feat: project scaffolding with Service interface and error types"
```

---

## Task 2: Decimal (math/big.Rat wrapper)

**Files:**
- Create: `decimal.go`
- Create: `decimal_test.go`

**Step 1: Write failing tests**

```go
package ucum

import "testing"

func TestDecimalFromString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1", "1"},
		{"0.001", "1/1000"},
		{"1e3", "1000"},
		{"1e-24", "1/1000000000000000000000000"},
		{"2.54", "127/50"},
	}
	for _, tt := range tests {
		d, err := decimalFromString(tt.input)
		if err != nil {
			t.Errorf("decimalFromString(%q) error: %v", tt.input, err)
			continue
		}
		if d.val.RatString() != tt.want {
			t.Errorf("decimalFromString(%q) = %s, want %s", tt.input, d.val.RatString(), tt.want)
		}
	}
}

func TestDecimalExactDivision(t *testing.T) {
	one := decimalFromInt(1)
	three := decimalFromInt(3)
	result := one.div(three).mul(three)
	if !result.equal(one) {
		t.Errorf("1/3*3 = %v, want 1", result.float64())
	}
}

func TestDecimalPow(t *testing.T) {
	two := decimalFromInt(2)
	result := two.pow(10)
	if result.float64() != 1024 {
		t.Errorf("2^10 = %v, want 1024", result.float64())
	}
	result = two.pow(-3)
	if result.float64() != 0.125 {
		t.Errorf("2^-3 = %v, want 0.125", result.float64())
	}
}
```

**Step 2: Run tests, verify they fail**

```bash
go test -run TestDecimal -v
```

**Step 3: Implement decimal.go**

```go
package ucum

import (
	"fmt"
	"math/big"
	"strings"
)

type decimal struct{ val *big.Rat }

func decimalFromInt(n int64) decimal {
	return decimal{new(big.Rat).SetInt64(n)}
}

func decimalFromString(s string) (decimal, error) {
	// Handle scientific notation: split on 'e' or 'E'
	s = strings.TrimSpace(s)
	if idx := strings.IndexAny(s, "eE"); idx >= 0 {
		base, exp := s[:idx], s[idx+1:]
		r := new(big.Rat)
		if _, ok := r.SetString(base); !ok {
			return decimal{}, fmt.Errorf("invalid decimal %q", s)
		}
		e := new(big.Int)
		if _, ok := e.SetString(exp, 10); !ok {
			return decimal{}, fmt.Errorf("invalid exponent in %q", s)
		}
		ten := big.NewInt(10)
		if e.Sign() >= 0 {
			factor := new(big.Int).Exp(ten, e, nil)
			r.Mul(r, new(big.Rat).SetInt(factor))
		} else {
			e.Neg(e)
			factor := new(big.Int).Exp(ten, e, nil)
			r.Quo(r, new(big.Rat).SetInt(factor))
		}
		return decimal{r}, nil
	}
	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok {
		return decimal{}, fmt.Errorf("invalid decimal %q", s)
	}
	return decimal{r}, nil
}

func (d decimal) add(o decimal) decimal { return decimal{new(big.Rat).Add(d.val, o.val)} }
func (d decimal) sub(o decimal) decimal { return decimal{new(big.Rat).Sub(d.val, o.val)} }
func (d decimal) mul(o decimal) decimal { return decimal{new(big.Rat).Mul(d.val, o.val)} }
func (d decimal) div(o decimal) decimal { return decimal{new(big.Rat).Quo(d.val, o.val)} }

func (d decimal) pow(n int) decimal {
	if n == 0 {
		return decimalFromInt(1)
	}
	base := d
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	result := decimalFromInt(1)
	for i := 0; i < n; i++ {
		result = result.mul(base)
	}
	if neg {
		result = decimalFromInt(1).div(result)
	}
	return result
}

func (d decimal) float64() float64 {
	f, _ := d.val.Float64()
	return f
}

func (d decimal) cmp(o decimal) int   { return d.val.Cmp(o.val) }
func (d decimal) equal(o decimal) bool { return d.val.Cmp(o.val) == 0 }
func (d decimal) isZero() bool        { return d.val.Sign() == 0 }

func (d decimal) String() string {
	if d.val.IsInt() {
		return d.val.Num().String()
	}
	return d.val.FloatString(10)
}
```

**Step 4: Run tests, verify they pass**

```bash
go test -run TestDecimal -v
```

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: add decimal type wrapping math/big.Rat for exact arithmetic"
```

---

## Task 3: AST Types

**Files:**
- Create: `ast.go`

**Step 1: Create ast.go**

```go
package ucum

// operator represents a binary operator in a UCUM expression.
type operator int

const (
	opMultiplication operator = iota
	opDivision
)

func (o operator) String() string {
	if o == opDivision {
		return "/"
	}
	return "."
}

// component is the interface for AST nodes.
type component interface {
	isComponent()
}

// term represents a binary operation: comp op term.
// This is a right-recursive linked list.
type term struct {
	comp component
	op   operator
	term *term // nil if no continuation
}

func (term) isComponent() {}

// symbol represents a unit reference with optional prefix and exponent.
type symbol struct {
	unit     *Unit   // resolved unit from model
	prefix   *Prefix // nil if no prefix
	exponent int     // default 1
}

func (symbol) isComponent() {}

// factor represents a numeric literal.
type factor struct {
	value int
}

func (factor) isComponent() {}
```

**Step 2: Verify it compiles**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add -A && git commit -m "feat: add AST types (term, symbol, factor, operator)"
```

---

## Task 4: Model and XML Definitions

**Files:**
- Create: `model.go`
- Create: `definitions.go`
- Embed: `ucum-essence.xml`
- Create: `definitions_test.go`

**Step 1: Download ucum-essence.xml**

```bash
curl -L https://raw.githubusercontent.com/FHIR/Ucum-java/master/src/main/resources/ucum-essence.xml -o ucum-essence.xml
```

**Step 2: Create model.go**

```go
package ucum

// UcumModel holds the complete set of UCUM definitions.
type UcumModel struct {
	Version      string
	Revision     string
	RevisionDate string
	Prefixes     []*Prefix
	BaseUnits    []*BaseUnit
	DefinedUnits []*DefinedUnit

	// O(1) lookup indexes (built after loading)
	prefixByCode map[string]*Prefix
	unitByCode   map[string]*Unit
}

// Unit is the common representation for base and defined units.
type Unit struct {
	Code       string
	Name       string
	Property   string
	IsMetric   bool
	IsSpecial  bool
	IsBase     bool
	IsArbitrary bool
	Dim        string // dimension symbol, base units only
	Value      *UnitValue
	Class      string
}

// UnitValue holds the conversion definition for a defined unit.
type UnitValue struct {
	Unit  string  // UCUM expression
	Text  string
	Value decimal // numeric multiplier
}

// Prefix represents an SI prefix (kilo, milli, etc.).
type Prefix struct {
	Code  string
	Name  string
	Value decimal
}

// BaseUnit represents one of the 7 fundamental SI base units.
type BaseUnit struct {
	Code     string
	Name     string
	Property string
	Dim      string // single character dimension symbol
}

// DefinedUnit represents a non-base UCUM unit.
type DefinedUnit struct {
	Code        string
	Name        string
	Property    string
	IsMetric    bool
	IsSpecial   bool
	IsArbitrary bool
	Class       string
	Value       *UnitValue
}

// getUnit looks up a unit by code (searches base and defined).
func (m *UcumModel) getUnit(code string) *Unit {
	return m.unitByCode[code]
}

// getPrefix looks up a prefix by code.
func (m *UcumModel) getPrefix(code string) *Prefix {
	return m.prefixByCode[code]
}

// buildIndexes populates the lookup maps from the loaded lists.
func (m *UcumModel) buildIndexes() {
	m.prefixByCode = make(map[string]*Prefix, len(m.Prefixes))
	for _, p := range m.Prefixes {
		m.prefixByCode[p.Code] = p
	}

	m.unitByCode = make(map[string]*Unit, len(m.BaseUnits)+len(m.DefinedUnits))
	for _, bu := range m.BaseUnits {
		m.unitByCode[bu.Code] = &Unit{
			Code: bu.Code, Name: bu.Name, Property: bu.Property,
			IsBase: true, Dim: bu.Dim,
		}
	}
	for _, du := range m.DefinedUnits {
		m.unitByCode[du.Code] = &Unit{
			Code: du.Code, Name: du.Name, Property: du.Property,
			IsMetric: du.IsMetric, IsSpecial: du.IsSpecial,
			IsArbitrary: du.IsArbitrary, Class: du.Class,
			Value: du.Value,
		}
	}
}
```

**Step 3: Create definitions.go (XML parser + embed)**

```go
package ucum

import (
	"embed"
	"encoding/xml"
	"fmt"
	"io"
)

//go:embed ucum-essence.xml
var embeddedDefinitions embed.FS

// loadDefinitions parses ucum-essence.xml from the given reader, or from embedded if nil.
func loadDefinitions(r io.Reader) (*UcumModel, error) {
	if r == nil {
		f, err := embeddedDefinitions.Open("ucum-essence.xml")
		if err != nil {
			return nil, fmt.Errorf("open embedded definitions: %w", err)
		}
		defer f.Close()
		r = f
	}
	return parseDefinitions(r)
}

// XML structures for unmarshaling ucum-essence.xml
type xmlRoot struct {
	XMLName      xml.Name        `xml:"root"`
	Version      string          `xml:"version,attr"`
	Revision     string          `xml:"revision,attr"`
	RevisionDate string          `xml:"revision-date,attr"`
	Prefixes     []xmlPrefix     `xml:"prefix"`
	BaseUnits    []xmlBaseUnit   `xml:"base-unit"`
	Units        []xmlDefinedUnit `xml:"unit"`
}

type xmlPrefix struct {
	Code        string   `xml:"Code,attr"`
	CodeUC      string   `xml:"CODE,attr"`
	Name        string   `xml:"name"`
	PrintSymbol string   `xml:"printSymbol"`
	Value       xmlValue `xml:"value"`
}

type xmlBaseUnit struct {
	Code        string `xml:"Code,attr"`
	CodeUC      string `xml:"CODE,attr"`
	Dim         string `xml:"dim,attr"`
	Name        string `xml:"name"`
	PrintSymbol string `xml:"printSymbol"`
	Property    string `xml:"property"`
}

type xmlDefinedUnit struct {
	Code        string   `xml:"Code,attr"`
	CodeUC      string   `xml:"CODE,attr"`
	IsMetric    string   `xml:"isMetric,attr"`
	IsSpecial   string   `xml:"isSpecial,attr"`
	IsArbitrary string   `xml:"isArbitrary,attr"`
	Class       string   `xml:"class,attr"`
	Name        string   `xml:"name"`
	PrintSymbol string   `xml:"printSymbol"`
	Property    string   `xml:"property"`
	Value       xmlValue `xml:"value"`
}

type xmlValue struct {
	Unit    string `xml:"Unit,attr"`
	UNIT    string `xml:"UNIT,attr"`
	Value   string `xml:"value,attr"`
	Text    string `xml:",chardata"`
}

func parseDefinitions(r io.Reader) (*UcumModel, error) {
	var root xmlRoot
	dec := xml.NewDecoder(r)
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("decode ucum-essence.xml: %w", err)
	}

	model := &UcumModel{
		Version:      root.Version,
		Revision:     root.Revision,
		RevisionDate: root.RevisionDate,
	}

	// Parse prefixes
	for _, xp := range root.Prefixes {
		val, err := decimalFromString(xp.Value.Value)
		if err != nil {
			return nil, fmt.Errorf("prefix %s value: %w", xp.Code, err)
		}
		model.Prefixes = append(model.Prefixes, &Prefix{
			Code: xp.Code, Name: xp.Name, Value: val,
		})
	}

	// Parse base units
	for _, xb := range root.BaseUnits {
		model.BaseUnits = append(model.BaseUnits, &BaseUnit{
			Code: xb.Code, Name: xb.Name, Property: xb.Property, Dim: xb.Dim,
		})
	}

	// Parse defined units
	for _, xu := range root.Units {
		var unitVal *UnitValue
		if xu.Value.Value != "" || xu.Value.Unit != "" {
			v, err := decimalFromString(xu.Value.Value)
			if err != nil {
				// Some special units have empty value; default to 1
				v = decimalFromInt(1)
			}
			unitVal = &UnitValue{Unit: xu.Value.Unit, Text: xu.Value.Text, Value: v}
		}
		model.DefinedUnits = append(model.DefinedUnits, &DefinedUnit{
			Code: xu.Code, Name: xu.Name, Property: xu.Property,
			IsMetric: xu.IsMetric == "yes", IsSpecial: xu.IsSpecial == "yes",
			IsArbitrary: xu.IsArbitrary == "yes", Class: xu.Class,
			Value: unitVal,
		})
	}

	model.buildIndexes()
	return model, nil
}
```

**Step 4: Write tests**

```go
package ucum

import "testing"

func TestLoadEmbeddedDefinitions(t *testing.T) {
	model, err := loadDefinitions(nil)
	if err != nil {
		t.Fatalf("loadDefinitions: %v", err)
	}
	if model.Version != "2.2" {
		t.Errorf("version = %q, want %q", model.Version, "2.2")
	}
	if len(model.Prefixes) < 20 {
		t.Errorf("prefixes = %d, want >= 20", len(model.Prefixes))
	}
	if len(model.BaseUnits) != 7 {
		t.Errorf("base units = %d, want 7", len(model.BaseUnits))
	}
	if len(model.DefinedUnits) < 200 {
		t.Errorf("defined units = %d, want >= 200", len(model.DefinedUnits))
	}

	// Check prefix lookup
	kilo := model.getPrefix("k")
	if kilo == nil || kilo.Value.float64() != 1e3 {
		t.Errorf("prefix k = %v, want 1e3", kilo)
	}

	// Check unit lookup
	meter := model.getUnit("m")
	if meter == nil || !meter.IsBase {
		t.Errorf("unit m = %v, want base unit", meter)
	}
	inch := model.getUnit("[in_i]")
	if inch == nil || inch.IsBase {
		t.Errorf("unit [in_i] = %v, want defined unit", inch)
	}
}
```

**Step 5: Run tests, verify they pass**

```bash
go test -run TestLoad -v
```

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: add model types and XML parser for ucum-essence.xml"
```

---

## Task 5: Lexer

**Files:**
- Create: `lexer.go`
- Create: `lexer_test.go`

**Step 1: Write failing tests**

```go
package ucum

import "testing"

func TestLexerSimple(t *testing.T) {
	tests := []struct {
		input  string
		tokens []string
		types  []tokenType
	}{
		{"m", []string{"m"}, []tokenType{tokenSymbol}},
		{"kg", []string{"kg"}, []tokenType{tokenSymbol}},
		{"m/s", []string{"m", "/", "s"}, []tokenType{tokenSymbol, tokenSolidus, tokenSymbol}},
		{"m.s", []string{"m", ".", "s"}, []tokenType{tokenSymbol, tokenPeriod, tokenSymbol}},
		{"10*3", []string{"10", "*", "3"}, []tokenType{tokenSymbol, tokenSymbol, tokenSymbol}},
		{"m2", []string{"m", "2"}, []tokenType{tokenSymbol, tokenNumber}},
		{"{score}", []string{"{score}"}, []tokenType{tokenAnnotation}},
		{"mg/dL", []string{"mg", "/", "dL"}, []tokenType{tokenSymbol, tokenSolidus, tokenSymbol}},
		{"(m)", []string{"(", "m", ")"}, []tokenType{tokenOpen, tokenSymbol, tokenClose}},
		{"[lb_av]", []string{"[lb_av]"}, []tokenType{tokenSymbol}},
		{"cm[H2O]", []string{"cm", "[H2O]"}, []tokenType{tokenSymbol, tokenSymbol}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := newLexer(tt.input)
			for i, want := range tt.tokens {
				if l.getType() == tokenNone {
					t.Fatalf("token %d: unexpected end, want %q", i, want)
				}
				if l.getToken() != want {
					t.Errorf("token %d = %q, want %q", i, l.getToken(), want)
				}
				if l.getType() != tt.types[i] {
					t.Errorf("token %d type = %v, want %v", i, l.getType(), tt.types[i])
				}
				l.consume()
			}
			if !l.finished() {
				t.Errorf("expected finished, got token %q", l.getToken())
			}
		})
	}
}
```

**Step 2: Implement lexer.go**

Port the Java Lexer.java recursive descent tokenizer. Key elements:
- `tokenType` enum: `tokenNone, tokenNumber, tokenSymbol, tokenSolidus, tokenPeriod, tokenOpen, tokenClose, tokenAnnotation`
- `lexer` struct with `source string`, `index int`, `token string`, `typ tokenType`
- `newLexer(source)` constructor that calls `consume()` for first token
- `consume()` dispatches through: single-char operators, annotations `{...}`, signed numbers, general symbol/number
- `isValidSymbolChar(ch, allowDigits, inBrackets)` matches `[a-zA-Z0-9%*^'"_]` and `.` inside brackets
- Bracket tracking for `[unit_code]` style units

**Step 3: Run tests, verify they pass**

```bash
go test -run TestLexer -v
```

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: add UCUM expression lexer"
```

---

## Task 6: Expression Parser

**Files:**
- Create: `parser.go`
- Create: `parser_test.go`

**Step 1: Write failing tests**

```go
package ucum

import "testing"

func TestParserValid(t *testing.T) {
	model, err := loadDefinitions(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := newParser(model)

	valid := []string{
		"m", "kg", "m/s", "mg/dL", "10*3/uL", "m.s-1", "m2",
		"kg.m/s2", "%", "[lb_av]", "cm[H2O]", "mol/L", "mm[Hg]",
		"/m", "m{annotation}", "{score}", "4.[pi].10*-7.N/A2",
	}
	for _, code := range valid {
		t.Run(code, func(t *testing.T) {
			_, err := p.parse(code)
			if err != nil {
				t.Errorf("parse(%q) error: %v", code, err)
			}
		})
	}
}

func TestParserInvalid(t *testing.T) {
	model, err := loadDefinitions(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := newParser(model)

	invalid := []string{
		"m/", "xyz", "",
	}
	for _, code := range invalid {
		t.Run(code, func(t *testing.T) {
			_, err := p.parse(code)
			if err == nil {
				t.Errorf("parse(%q) should fail", code)
			}
		})
	}
}
```

**Step 2: Implement parser.go**

Port ExpressionParser.java recursive descent:
- `parser` struct with `model *UcumModel`
- `parse(code string) (*term, error)` — entry point
- `parseTerm(l *lexer, first bool) (*term, error)` — handles `comp [op term]`
- `parseComp(l *lexer) (component, error)` — dispatches NUMBER/SYMBOL/OPEN
- `parseSymbol(l *lexer) (component, error)` — resolves prefix+unit against model

Key resolution logic: for each prefix in model, if symbol starts with prefix code AND remainder is a metric unit, use that prefix. Otherwise try full symbol as unit.

**Step 3: Run tests, verify they pass**

```bash
go test -run TestParser -v
```

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: add UCUM expression parser with symbol resolution"
```

---

## Task 7: Expression Composer

**Files:**
- Create: `composer.go`
- Create: `composer_test.go`

**Step 1: Write tests**

```go
package ucum

import "testing"

func TestComposerRoundTrip(t *testing.T) {
	model, err := loadDefinitions(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := newParser(model)

	codes := []string{"m", "m/s", "kg.m/s2", "10*3/uL", "mg/dL", "%"}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			ast, err := p.parse(code)
			if err != nil {
				t.Fatal(err)
			}
			result := composeTerm(ast)
			// Re-parse to verify round-trip is valid
			_, err = p.parse(result)
			if err != nil {
				t.Errorf("compose(%q) = %q, fails re-parse: %v", code, result, err)
			}
		})
	}
}
```

**Step 2: Implement composer.go**

Tree walker that serializes AST back to UCUM string. Also composes Canonical form.

**Step 3: Run tests, verify they pass**

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: add expression composer (AST to UCUM string)"
```

---

## Task 8: Special Unit Handlers

**Files:**
- Create: `special.go`
- Create: `special_test.go`

**Step 1: Write failing tests**

```go
package ucum

import (
	"math"
	"testing"
)

func TestCelsiusConversion(t *testing.T) {
	h := specialHandlers["Cel"]
	// 0°C = 273.15 K
	if got := h.ToCanonical(0); got != 273.15 {
		t.Errorf("Cel.ToCanonical(0) = %v, want 273.15", got)
	}
	if got := h.FromCanonical(273.15); got != 0 {
		t.Errorf("Cel.FromCanonical(273.15) = %v, want 0", got)
	}
}

func TestFahrenheitConversion(t *testing.T) {
	h := specialHandlers["[degF]"]
	// 32°F = 273.15 K (freezing point)
	got := h.ToCanonical(32)
	if math.Abs(got-273.15) > 0.001 {
		t.Errorf("degF.ToCanonical(32) = %v, want ~273.15", got)
	}
}

func TestPHConversion(t *testing.T) {
	h := specialHandlers["[pH]"]
	// pH 7 = 1e-7 mol/L
	got := h.ToCanonical(7)
	if math.Abs(got-1e-7) > 1e-12 {
		t.Errorf("pH.ToCanonical(7) = %v, want 1e-7", got)
	}
}

func TestBelConversion(t *testing.T) {
	h := specialHandlers["B"]
	// 3 B = 1000 (power ratio)
	got := h.ToCanonical(3)
	if math.Abs(got-1000) > 0.001 {
		t.Errorf("B.ToCanonical(3) = %v, want 1000", got)
	}
}
```

**Step 2: Implement special.go**

```go
package ucum

import "math"

// SpecialHandler converts between a special unit and its canonical base.
type SpecialHandler interface {
	Code() string
	Units() string
	ToCanonical(value float64) float64
	FromCanonical(value float64) float64
}

// specialHandlers maps special unit codes to their handlers.
var specialHandlers = map[string]SpecialHandler{
	// Temperature (offset units)
	"Cel":     offsetHandler{code: "Cel", units: "K", offset: 273.15},
	"[degF]":  affineHandler{code: "[degF]", units: "K", scale: 5.0 / 9.0, offset: 459.67},
	"[degRe]": affineHandler{code: "[degRe]", units: "K", scale: 5.0 / 4.0, offset: 273.15},

	// Logarithmic
	"[pH]":     logHandler{code: "[pH]", units: "mol/l", base: 10, negate: true},
	"Np":       logHandler{code: "Np", units: "1", base: math.E},
	"B":        logHandler{code: "B", units: "1", base: 10},
	"B[SPL]":   logHandler{code: "B[SPL]", units: "10*-5.Pa", base: 10, factor: 2},
	"B[V]":     logHandler{code: "B[V]", units: "V", base: 10, factor: 2},
	"B[mV]":    logHandler{code: "B[mV]", units: "mV", base: 10, factor: 2},
	"B[uV]":    logHandler{code: "B[uV]", units: "uV", base: 10, factor: 2},
	"B[10.nV]": logHandler{code: "B[10.nV]", units: "10*-9.V", base: 10, factor: 2},
	"B[W]":     logHandler{code: "B[W]", units: "W", base: 10},
	"B[kW]":    logHandler{code: "B[kW]", units: "kW", base: 10},
	"bit_s":    logHandler{code: "bit_s", units: "1", base: 2},

	// Trigonometric
	"[p'diop]":  tanHandler{code: "[p'diop]", units: "rad", factor: 100},
	"%[slope]":  tanHandler{code: "%[slope]", units: "deg", factor: 100},

	// Power
	"[m/s2/Hz^(1/2)]": sqrtHandler{code: "[m/s2/Hz^(1/2)]", units: "m2/s4/Hz"},

	// Homeopathic
	"[hp'_X]": logHandler{code: "[hp'_X]", units: "1", base: 10, negate: true},
	"[hp'_C]": logHandler{code: "[hp'_C]", units: "1", base: 100, negate: true},
	"[hp'_M]": logHandler{code: "[hp'_M]", units: "1", base: 1000, negate: true},
	"[hp'_Q]": logHandler{code: "[hp'_Q]", units: "1", base: 50000, negate: true},
}

// offsetHandler: canonical = value + offset (Celsius)
type offsetHandler struct {
	code, units string
	offset      float64
}

func (h offsetHandler) Code() string                    { return h.code }
func (h offsetHandler) Units() string                   { return h.units }
func (h offsetHandler) ToCanonical(v float64) float64   { return v + h.offset }
func (h offsetHandler) FromCanonical(v float64) float64 { return v - h.offset }

// affineHandler: canonical = (value + offset) * scale (Fahrenheit, Reaumur)
type affineHandler struct {
	code, units    string
	scale, offset  float64
}

func (h affineHandler) Code() string                    { return h.code }
func (h affineHandler) Units() string                   { return h.units }
func (h affineHandler) ToCanonical(v float64) float64   { return (v + h.offset) * h.scale }
func (h affineHandler) FromCanonical(v float64) float64 { return v/h.scale - h.offset }

// logHandler: canonical = base^(value*factor) or base^(-value) if negate
type logHandler struct {
	code, units string
	base        float64
	factor      float64 // multiplier for exponent (default 1)
	negate      bool
}

func (h logHandler) Code() string  { return h.code }
func (h logHandler) Units() string { return h.units }
func (h logHandler) ToCanonical(v float64) float64 {
	f := h.effectiveFactor()
	if h.negate {
		return math.Pow(h.base, -v*f)
	}
	return math.Pow(h.base, v*f)
}
func (h logHandler) FromCanonical(v float64) float64 {
	f := h.effectiveFactor()
	if h.negate {
		return -math.Log(v) / (math.Log(h.base) * f)
	}
	return math.Log(v) / (math.Log(h.base) * f)
}
func (h logHandler) effectiveFactor() float64 {
	if h.factor == 0 {
		return 1
	}
	return h.factor
}

// tanHandler: canonical = arctan(value/factor) (prism diopter, percent slope)
type tanHandler struct {
	code, units string
	factor      float64
}

func (h tanHandler) Code() string                    { return h.code }
func (h tanHandler) Units() string                   { return h.units }
func (h tanHandler) ToCanonical(v float64) float64   { return math.Atan(v/h.factor) }
func (h tanHandler) FromCanonical(v float64) float64 { return math.Tan(v) * h.factor }

// sqrtHandler: canonical = value^2
type sqrtHandler struct {
	code, units string
}

func (h sqrtHandler) Code() string                    { return h.code }
func (h sqrtHandler) Units() string                   { return h.units }
func (h sqrtHandler) ToCanonical(v float64) float64   { return v * v }
func (h sqrtHandler) FromCanonical(v float64) float64 { return math.Sqrt(v) }
```

**Step 3: Run tests, verify they pass**

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: add special unit handlers (temperature, logarithmic, trigonometric)"
```

---

## Task 9: Converter (Canonical Form)

**Files:**
- Create: `converter.go`
- Create: `canonical.go`
- Create: `converter_test.go`

**Step 1: Create canonical.go**

```go
package ucum

// canonical represents a unit expression in normalized form.
type canonical struct {
	value decimal
	units []canonicalUnit
}

// canonicalUnit is a base unit with an exponent.
type canonicalUnit struct {
	base     *BaseUnit
	exponent int
}

func newCanonical(value decimal) *canonical {
	return &canonical{value: value}
}

func (c *canonical) multiplyValue(d decimal) { c.value = c.value.mul(d) }
func (c *canonical) divideValue(d decimal)   { c.value = c.value.div(d) }
```

**Step 2: Implement converter.go**

Port Converter.java: walk the AST, expand defined units recursively, apply prefixes, collate and sort canonical units. For special units, use the handler to get units string and value (for the multiplicative component), and mark that the handler must be invoked at the service level for the offset/log component.

**Step 3: Write tests**

```go
package ucum

import "testing"

func TestConverterCanonicalUnits(t *testing.T) {
	model, err := loadDefinitions(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := newParser(model)
	c := newConverter(model, specialHandlers)

	tests := []struct {
		input string
		want  string // canonical unit string
	}{
		{"m", "m"},
		{"km", "m"},
		{"m/s", "m.s-1"},
		{"km/h", "m.s-1"},
		{"N", "g.m.s-2"},
		{"Pa", "g.m-1.s-2"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			ast, err := p.parse(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			can, err := c.convert(ast)
			if err != nil {
				t.Fatal(err)
			}
			got := composeCanonicalUnits(can)
			if got != tt.want {
				t.Errorf("canonical(%q) units = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
```

**Step 4: Run tests, verify they pass**

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: add converter for canonical form normalization"
```

---

## Task 10: Service Implementation

**Files:**
- Create: `service.go`
- Create: `service_test.go`

**Step 1: Implement service.go**

Wire together all components: model loading, parser, converter, composer, special handlers, and expression cache (sync.Map).

Key methods:
- `Validate`: parse the expression, return nil or ValidationError
- `Canonical`: parse, convert to canonical form, compose
- `Convert`: get canonical forms for both units, check comparability, compute `value * srcCanonical / dstCanonical` with special handler offsets applied at boundaries
- `IsComparable`: canonical units strings must match
- `Analyse`: compose human-readable from AST
- `Multiply`: parse both, merge canonical forms

**Step 2: Write comprehensive tests**

```go
package ucum

import "testing"

func TestServiceValidate(t *testing.T) {
	svc, err := New()
	if err != nil {
		t.Fatal(err)
	}
	valid := []string{"m", "kg", "mg/dL", "10*3/uL", "%", "[lb_av]", "{score}"}
	for _, code := range valid {
		if err := svc.Validate(code); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", code, err)
		}
	}
	invalid := []string{"xyz", "m/", ""}
	for _, code := range invalid {
		if err := svc.Validate(code); err == nil {
			t.Errorf("Validate(%q) = nil, want error", code)
		}
	}
}

func TestServiceConvert(t *testing.T) {
	svc, err := New()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		value    float64
		from, to string
		want     float64
		delta    float64
	}{
		{1, "m", "cm", 100, 0.001},
		{1, "km", "m", 1000, 0.001},
		{1, "[lb_av]", "g", 453.59237, 0.001},
		{37, "Cel", "[degF]", 98.6, 0.1},
		{100, "Cel", "K", 373.15, 0.001},
	}
	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			got, err := svc.Convert(tt.value, tt.from, tt.to)
			if err != nil {
				t.Fatal(err)
			}
			if diff := got - tt.want; diff > tt.delta || diff < -tt.delta {
				t.Errorf("Convert(%v, %q, %q) = %v, want %v (±%v)", tt.value, tt.from, tt.to, got, tt.want, tt.delta)
			}
		})
	}
}

func TestServiceIsComparable(t *testing.T) {
	svc, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := svc.IsComparable("mg", "g"); !ok {
		t.Error("mg and g should be comparable")
	}
	if ok, _ := svc.IsComparable("mg", "mL"); ok {
		t.Error("mg and mL should not be comparable")
	}
}
```

**Step 3: Run tests, verify they pass**

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: add service implementation with validate, convert, canonical, comparable"
```

---

## Task 11: Expression Cache + Thread Safety

**Files:**
- Modify: `service.go` (add sync.Map cache)
- Create: `service_concurrent_test.go`

**Step 1: Add cache to service**

Add `cache sync.Map` to the service struct. In `parse()`, check cache first, store after parsing.

**Step 2: Write concurrent test**

```go
package ucum

import (
	"sync"
	"testing"
)

func TestServiceConcurrent(t *testing.T) {
	svc, err := New()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	codes := []string{"m", "kg", "mg/dL", "10*3/uL", "mm[Hg]", "%"}
	for i := 0; i < 100; i++ {
		for _, code := range codes {
			wg.Add(1)
			go func(c string) {
				defer wg.Done()
				_ = svc.Validate(c)
				_, _ = svc.Convert(1, "m", "cm")
				_, _ = svc.IsComparable("mg", "g")
			}(code)
		}
	}
	wg.Wait()
}
```

**Step 3: Run with race detector**

```bash
go test -race -run TestServiceConcurrent -v
```

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: add expression cache with sync.Map for thread safety"
```

---

## Task 12: Functional Test Suite

**Files:**
- Download: `testdata/UcumFunctionalTests.xml`
- Create: `functional_test.go`

**Step 1: Download test suite from Java project**

```bash
mkdir -p testdata
curl -L https://raw.githubusercontent.com/FHIR/Ucum-java/master/src/test/resources/UcumFunctionalTests.xml -o testdata/UcumFunctionalTests.xml
```

**Step 2: Write test runner that parses the XML and runs all cases**

Parse `<validation>`, `<conversion>`, and `<multiplication>` sections. Run as table-driven subtests. Add extra cases for special units that Java cannot handle.

**Step 3: Run full suite**

```bash
go test -run TestFunctional -v
```

**Step 4: Commit**

```bash
git add -A && git commit -m "test: port Java functional test suite with special unit additions"
```

---

## Task 13: Benchmarks

**Files:**
- Create: `benchmark_test.go`

**Step 1: Write benchmarks for hot paths**

```go
package ucum

import "testing"

func BenchmarkValidate(b *testing.B) {
	svc, _ := New()
	codes := []string{"m", "mg/dL", "10*3/uL", "kg.m/s2", "[lb_av]"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.Validate(codes[i%len(codes)])
	}
}

func BenchmarkConvert(b *testing.B) {
	svc, _ := New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.Convert(1, "km", "m")
	}
}
```

**Step 2: Run benchmarks**

```bash
go test -bench=. -benchmem
```

**Step 3: Commit**

```bash
git add -A && git commit -m "test: add benchmarks for Validate and Convert"
```

---

## Task 14: Final Verification + README

**Step 1: Run full test suite with race detector**

```bash
go test -race -count=1 ./...
```

**Step 2: Run linter**

```bash
golangci-lint run ./...
```

**Step 3: Verify zero dependencies**

```bash
go list -m all  # should show only the module itself
```

**Step 4: Commit and tag**

```bash
git add -A && git commit -m "chore: final verification, all tests passing"
git tag v0.1.0
```
