package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// change es un cambio declarado: en las líneas de la salida base cuyo messageId es MessageID y
// cuyo campo Field vale From, el cambio que se evalúa debe dar To.
type change struct {
	MessageID string `json:"messageId"`
	Field     string `json:"field"`
	From      string `json:"from"`
	To        string `json:"to"`
}

// readExpect lee y valida el archivo de cambios declarados de -expect. Es estricto: un campo
// desconocido, un valor vacío, un cambio nulo o repetido, o texto después del arreglo son errores,
// porque un archivo mal escrito que no reescribe nada haría pasar `same` por la vía equivocada.
func readExpect(path string) ([]change, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: the path is a flag the operator passes
	if err != nil {
		return nil, err
	}
	return parseExpect(data)
}

func parseExpect(data []byte) ([]change, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var changes []change
	if err := dec.Decode(&changes); err != nil {
		return nil, fmt.Errorf("expect file: %w", err)
	}
	// dec.More no prueba el fin de la entrada (`]` o `}` sobrantes dan true o falso según el caso):
	// una segunda lectura debe dar EOF; cualquier otra cosa es dato después del arreglo.
	var extra json.RawMessage
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("expect file: data after the array")
	}
	if len(changes) == 0 {
		return nil, errors.New("expect file: no changes declared")
	}
	type key struct{ id, field string }
	seen := map[key]bool{}
	for i, c := range changes {
		switch {
		case c.MessageID == "":
			return nil, fmt.Errorf("expect file: entry %d has no messageId", i)
		case c.Field == "":
			return nil, fmt.Errorf("expect file: entry %d has no field", i)
		case c.From == "":
			return nil, fmt.Errorf("expect file: entry %d has no from", i)
		case c.To == "":
			return nil, fmt.Errorf("expect file: entry %d has no to", i)
		case c.From == c.To:
			return nil, fmt.Errorf("expect file: entry %d changes nothing (from equals to)", i)
		}
		k := key{c.MessageID, c.Field}
		if seen[k] {
			return nil, fmt.Errorf("expect file: entry %d repeats %s.%s", i, c.MessageID, c.Field)
		}
		seen[k] = true
	}
	return changes, nil
}

// expectResult es lo que dejó aplicar los cambios a una salida base.
type expectResult struct {
	rewritten int
	perChange []int // por cambio: cuántas líneas de la salida reescribió
}

// normalize reescribe cada línea JSONL con las claves en orden y sin espacios sobrantes, y descarta
// las líneas vacías. Si changes no es nulo, antes de escribirla reescribe en cada línea el campo de
// cada cambio cuyo messageId coincide y cuyo valor es From. Aplicada igual a la base y al árbol de
// trabajo, el formato o el orden de las claves no crea ni oculta diferencias.
func normalize(out []byte, changes []change) ([]byte, expectResult, error) {
	res := expectResult{perChange: make([]int, len(changes))}
	var b bytes.Buffer
	for n, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(line))
		dec.UseNumber() // los números salen tal como llegaron
		var rec map[string]any
		if err := dec.Decode(&rec); err != nil {
			return nil, res, fmt.Errorf("line %d: %w", n+1, err)
		}
		for i, c := range changes {
			v, ok := rec[c.Field]
			if !ok {
				continue
			}
			if id, _ := rec["messageId"].(string); id != c.MessageID {
				continue
			}
			if s, _ := v.(string); s == c.From {
				rec[c.Field] = c.To
				res.rewritten++
				res.perChange[i]++
			}
		}
		enc, err := json.Marshal(rec)
		if err != nil {
			return nil, res, fmt.Errorf("line %d: %w", n+1, err)
		}
		b.Write(enc)
		b.WriteByte('\n')
	}
	return b.Bytes(), res, nil
}
