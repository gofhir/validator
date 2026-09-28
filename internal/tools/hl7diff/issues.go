package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GoIssue is one gofhir error, as corpusrun writes it.
type GoIssue struct {
	File        string   `json:"file"`
	Severity    string   `json:"severity"`
	Code        string   `json:"code"`
	MessageID   string   `json:"messageId"`
	Expression  []string `json:"expression"`
	Diagnostics string   `json:"diagnostics"`
	Failure     string   `json:"failure"`
}

// Location is the issue's first expression, as gofhir wrote it.
func (g GoIssue) Location() string {
	if len(g.Expression) == 0 {
		return ""
	}
	return g.Expression[0]
}

// HL7Issue is one HL7 validator error.
type HL7Issue struct {
	File     string
	Severity string
	Key      string // message ID, or the issue code when HL7 gives none
	HasID    bool
	Code     string
	Location string // normalized to gofhir syntax
	Text     string
}

// GoRun is one corpusrun output: errors per file, and the files it covered.
type GoRun struct {
	Errors  map[string][]GoIssue
	Covered map[string]bool
}

func isError(sev string) bool { return sev == "error" || sev == "fatal" }

// ReadGo reads a corpusrun .jsonl file. A line with a failure is kept as a fatal error, so a
// resource that could not be validated can never look clean.
func ReadGo(path string) (GoRun, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return GoRun{}, err
	}
	run := GoRun{Errors: map[string][]GoIssue{}, Covered: map[string]bool{}}
	for n, raw := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var g GoIssue
		if err := json.Unmarshal([]byte(raw), &g); err != nil {
			return GoRun{}, fmt.Errorf("%s:%d: %w", path, n+1, err)
		}
		g.File = filepath.Clean(g.File)
		run.Covered[g.File] = true
		if g.Failure != "" {
			g.Severity, g.MessageID, g.Diagnostics = "fatal", "FAILURE", g.Failure
		}
		if isError(g.Severity) {
			run.Errors[g.File] = append(run.Errors[g.File], g)
		}
	}
	return run, nil
}

// HL7Run is one validator_cli -output file: errors per file, and the files it has an outcome for.
type HL7Run struct {
	Errors  map[string][]HL7Issue
	Covered map[string]bool
}

type hl7Doc struct {
	ResourceType string                          `json:"resourceType"`
	Entry        []struct{ Resource hl7Outcome } `json:"entry"`
	hl7Outcome
}

type hl7Outcome struct {
	ResourceType string   `json:"resourceType"`
	Extension    []hl7Ext `json:"extension"`
	Issue        []struct {
		Severity   string   `json:"severity"`
		Code       string   `json:"code"`
		Expression []string `json:"expression"`
		Location   []string `json:"location"`
		Extension  []hl7Ext `json:"extension"`
		Details    struct {
			Text string `json:"text"`
		} `json:"details"`
	} `json:"issue"`
}

type hl7Ext struct {
	URL         string `json:"url"`
	ValueString string `json:"valueString"`
	ValueCode   string `json:"valueCode"`
}

func extValue(exts []hl7Ext, suffix string) string {
	for _, e := range exts {
		if strings.HasSuffix(e.URL, suffix) {
			if e.ValueString != "" {
				return e.ValueString
			}
			return e.ValueCode
		}
	}
	return ""
}

// ReadHL7 reads a validator_cli -output file: a Bundle of OperationOutcomes, or one
// OperationOutcome when a single file was validated.
func ReadHL7(path string) (HL7Run, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return HL7Run{}, err
	}
	var doc hl7Doc
	if err := json.Unmarshal(data, &doc); err != nil {
		return HL7Run{}, fmt.Errorf("%s: %w", path, err)
	}
	var outcomes []hl7Outcome
	switch doc.ResourceType {
	case "Bundle":
		for _, e := range doc.Entry {
			if e.Resource.ResourceType != "OperationOutcome" {
				return HL7Run{}, fmt.Errorf("%s: Bundle entry is a %q, not an OperationOutcome", path, e.Resource.ResourceType)
			}
			outcomes = append(outcomes, e.Resource)
		}
	case "OperationOutcome":
		outcomes = append(outcomes, doc.hl7Outcome)
	default:
		return HL7Run{}, fmt.Errorf("%s: unexpected resourceType %q", path, doc.ResourceType)
	}

	run := HL7Run{Errors: map[string][]HL7Issue{}, Covered: map[string]bool{}}
	for _, oo := range outcomes {
		file := extValue(oo.Extension, "/operationoutcome-file")
		if file == "" {
			return HL7Run{}, fmt.Errorf("%s: an OperationOutcome has no operationoutcome-file extension", path)
		}
		file = filepath.Clean(file)
		run.Covered[file] = true
		for _, i := range oo.Issue {
			if !isError(i.Severity) {
				continue
			}
			loc := ""
			switch {
			case len(i.Expression) > 0:
				loc = i.Expression[0]
			case len(i.Location) > 0:
				loc = i.Location[0]
			}
			id := extValue(i.Extension, "/operationoutcome-message-id")
			key := id
			if key == "" {
				key = i.Code
			}
			run.Errors[file] = append(run.Errors[file], HL7Issue{
				File: file, Severity: i.Severity, Key: key, HasID: id != "", Code: i.Code,
				Location: NormalizeHL7Location(loc), Text: i.Details.Text,
			})
		}
	}
	return run, nil
}
