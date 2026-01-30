// Package main implements the gofhir-validator CLI tool.
// This CLI is designed to be comparable with the HL7 FHIR Validator.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofhir/validator/pkg/issue"
	"github.com/gofhir/validator/pkg/validator"
)

const (
	version = "0.1.0"
	usage   = `gofhir-validator - FHIR Resource Validator

Usage:
  gofhir-validator [options] <file>...
  gofhir-validator [options] -           (read from stdin)
  cat resource.json | gofhir-validator - (pipe input)

Examples:
  gofhir-validator patient.json
  gofhir-validator -version r4 patient.json
  gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json
  gofhir-validator -output json patient.json
  gofhir-validator -tx n/a patient.json
  gofhir-validator *.json
  cat patient.json | gofhir-validator -

Options:
`
)

// OutputFormat specifies the output format.
type OutputFormat string

// Output format constants.
const (
	OutputText OutputFormat = "text"
	OutputJSON OutputFormat = "json"
)

// Config holds CLI configuration
type Config struct {
	Version       string
	Profiles      []string
	Packages      []string
	PackageFiles  []string
	PackageURLs   []string
	Output        OutputFormat
	Strict        bool
	NoTerminology bool
	Quiet         bool
	Verbose       bool
	ShowVersion   bool
	Help          bool
	Files         []string
}

// ValidationOutput represents the JSON output structure
type ValidationOutput struct {
	Resource string        `json:"resource"`
	Valid    bool          `json:"valid"`
	Errors   int           `json:"errors"`
	Warnings int           `json:"warnings"`
	Info     int           `json:"info"`
	Issues   []IssueOutput `json:"issues,omitempty"`
	Duration string        `json:"duration"`
}

// IssueOutput represents a single issue in JSON output
type IssueOutput struct {
	Severity    string   `json:"severity"`
	Code        string   `json:"code"`
	Diagnostics string   `json:"diagnostics"`
	Expression  []string `json:"expression,omitempty"`
}

func main() {
	config := parseFlags()

	if config.ShowVersion {
		fmt.Printf("gofhir-validator v%s\n", version)
		os.Exit(0)
	}

	if config.Help || len(config.Files) == 0 {
		flag.Usage()
		os.Exit(0)
	}

	exitCode := run(config)
	os.Exit(exitCode)
}

func parseFlags() *Config {
	config := &Config{
		Version: "4.0.1",
		Output:  OutputText,
	}

	// Define flags compatible with HL7 validator
	var profiles, packages, packageFiles, packageURLs string
	var output string

	flag.StringVar(&config.Version, "version", "4.0.1", "FHIR version (4.0.1, 4.3.0, 5.0.0)")
	flag.StringVar(&profiles, "ig", "", "Profile URL(s) to validate against (comma-separated)")
	flag.StringVar(&packages, "package", "", "Additional FHIR package(s) to load (e.g., hl7.fhir.us.core#6.1.0)")
	flag.StringVar(&packageFiles, "package-file", "", "Local .tgz package file(s) to load (comma-separated)")
	flag.StringVar(&packageURLs, "package-url", "", "Remote .tgz package URL(s) to load (comma-separated)")
	flag.StringVar(&output, "output", "text", "Output format: text, json")
	flag.BoolVar(&config.Strict, "strict", false, "Treat warnings as errors")
	flag.BoolVar(&config.NoTerminology, "tx", false, "Disable terminology validation (use '-tx n/a')")
	flag.BoolVar(&config.Quiet, "quiet", false, "Only show errors and warnings")
	flag.BoolVar(&config.Verbose, "verbose", false, "Show detailed output")
	flag.BoolVar(&config.ShowVersion, "v", false, "Show version")
	flag.BoolVar(&config.Help, "help", false, "Show help")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
		flag.PrintDefaults()
	}

	flag.Parse()

	// Parse profiles
	if profiles != "" {
		config.Profiles = strings.Split(profiles, ",")
	}

	// Parse packages
	if packages != "" {
		config.Packages = strings.Split(packages, ",")
	}

	// Parse package files (.tgz)
	if packageFiles != "" {
		config.PackageFiles = strings.Split(packageFiles, ",")
	}

	// Parse package URLs
	if packageURLs != "" {
		config.PackageURLs = strings.Split(packageURLs, ",")
	}

	// Parse output format
	switch strings.ToLower(output) {
	case "json":
		config.Output = OutputJSON
	default:
		config.Output = OutputText
	}

	// Handle -tx n/a style flag
	for _, arg := range os.Args {
		if arg == "n/a" {
			config.NoTerminology = true
		}
	}

	// Remaining arguments are files
	config.Files = flag.Args()

	return config
}

func run(config *Config) int {
	// Build validator options
	opts := []validator.Option{
		validator.WithVersion(config.Version),
	}

	for _, profile := range config.Profiles {
		opts = append(opts, validator.WithProfile(strings.TrimSpace(profile)))
	}

	for _, pkg := range config.Packages {
		// Parse package format: name#version
		parts := strings.SplitN(pkg, "#", 2)
		if len(parts) == 2 {
			opts = append(opts, validator.WithPackage(parts[0], parts[1]))
		}
	}

	// Load packages from local .tgz files
	for _, tgzPath := range config.PackageFiles {
		opts = append(opts, validator.WithPackageTgz(strings.TrimSpace(tgzPath)))
	}

	// Load packages from remote URLs
	for _, url := range config.PackageURLs {
		opts = append(opts, validator.WithPackageURL(strings.TrimSpace(url)))
	}

	if config.Strict {
		opts = append(opts, validator.WithStrictMode(true))
	}

	// Create validator
	if !config.Quiet {
		fmt.Fprintf(os.Stderr, "Initializing FHIR Validator (version %s)...\n", config.Version)
	}

	v, err := validator.New(opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to initialize validator: %v\n", err)
		return 1
	}

	if !config.Quiet {
		fmt.Fprintf(os.Stderr, "Validator ready. Processing %d file(s)...\n\n", len(config.Files))
	}

	// Process files
	hasErrors := false
	outputs := make([]ValidationOutput, 0, len(config.Files))

	for _, file := range config.Files {
		var data []byte
		var name string

		if file == "-" {
			// Read from stdin
			name = "stdin"
			data, err = io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				hasErrors = true
				continue
			}
		} else {
			// Handle glob patterns
			matches, globErr := filepath.Glob(file)
			if globErr != nil {
				fmt.Fprintf(os.Stderr, "Error with pattern '%s': %v\n", file, globErr)
				hasErrors = true
				continue
			}

			if len(matches) == 0 {
				fmt.Fprintf(os.Stderr, "No files match pattern: %s\n", file)
				hasErrors = true
				continue
			}

			for _, match := range matches {
				output, fileHasErrors := validateFile(v, match, config)
				outputs = append(outputs, output)
				if fileHasErrors {
					hasErrors = true
				}
			}
			continue
		}

		// Validate stdin data
		output, fileHasErrors := validateData(v, data, name, config)
		outputs = append(outputs, output)
		if fileHasErrors {
			hasErrors = true
		}
	}

	// Output JSON if requested
	if config.Output == OutputJSON {
		jsonOutput, _ := json.MarshalIndent(outputs, "", "  ")
		fmt.Println(string(jsonOutput))
	}

	if hasErrors {
		return 1
	}
	return 0
}

func validateFile(v *validator.Validator, path string, config *Config) (ValidationOutput, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		output := ValidationOutput{
			Resource: path,
			Valid:    false,
			Errors:   1,
			Issues: []IssueOutput{{
				Severity:    "error",
				Code:        "exception",
				Diagnostics: fmt.Sprintf("Failed to read file: %v", err),
			}},
		}
		if config.Output == OutputText {
			fmt.Printf("Error reading %s: %v\n", path, err)
		}
		return output, true
	}

	return validateData(v, data, path, config)
}

func validateData(v *validator.Validator, data []byte, name string, config *Config) (ValidationOutput, bool) {
	ctx := context.Background()
	startTime := time.Now()

	result, err := v.Validate(ctx, data)
	duration := time.Since(startTime)

	if err != nil {
		output := ValidationOutput{
			Resource: name,
			Valid:    false,
			Errors:   1,
			Duration: duration.String(),
			Issues: []IssueOutput{{
				Severity:    "error",
				Code:        "exception",
				Diagnostics: fmt.Sprintf("Validation failed: %v", err),
			}},
		}
		if config.Output == OutputText {
			fmt.Printf("Error validating %s: %v\n", name, err)
		}
		return output, true
	}

	// Build output
	output := ValidationOutput{
		Resource: name,
		Valid:    !result.HasErrors(),
		Errors:   result.ErrorCount(),
		Warnings: result.WarningCount(),
		Info:     result.InfoCount(),
		Duration: duration.Round(time.Microsecond).String(),
	}

	// Convert issues
	for _, iss := range result.Issues {
		output.Issues = append(output.Issues, IssueOutput{
			Severity:    string(iss.Severity),
			Code:        string(iss.Code),
			Diagnostics: iss.Diagnostics,
			Expression:  iss.Expression,
		})
	}

	// Text output
	if config.Output == OutputText {
		printTextResult(name, result, duration, config)
	}

	return output, result.HasErrors()
}

func printTextResult(name string, result *issue.Result, duration time.Duration, config *Config) {
	// Header
	status := "VALID"
	if result.HasErrors() {
		status = "INVALID"
	}

	fmt.Printf("== %s ==\n", name)
	fmt.Printf("Status: %s\n", status)
	fmt.Printf("Errors: %d, Warnings: %d, Info: %d\n", result.ErrorCount(), result.WarningCount(), result.InfoCount())

	if result.Stats != nil {
		fmt.Printf("Profile: %s\n", result.Stats.ProfileURL)
		fmt.Printf("Duration: %s\n", duration.Round(time.Microsecond))
	}

	// Issues
	if len(result.Issues) > 0 {
		fmt.Println("\nIssues:")
		for _, iss := range result.Issues {
			// Skip info in quiet mode
			if config.Quiet && iss.Severity == issue.SeverityInformation {
				continue
			}

			severityIcon := getSeverityIcon(iss.Severity)
			location := ""
			if len(iss.Expression) > 0 {
				location = fmt.Sprintf(" @ %s", strings.Join(iss.Expression, ", "))
			}

			fmt.Printf("  %s [%s] %s%s\n", severityIcon, iss.Code, iss.Diagnostics, location)
		}
	}

	fmt.Println()
}

func getSeverityIcon(severity issue.Severity) string {
	switch severity {
	case issue.SeverityError:
		return "ERROR"
	case issue.SeverityWarning:
		return "WARN "
	case issue.SeverityInformation:
		return "INFO "
	default:
		return "     "
	}
}
