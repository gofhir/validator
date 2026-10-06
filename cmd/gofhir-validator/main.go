// Package main implements the gofhir-validator CLI tool.
// This CLI is designed to be comparable with the HL7 FHIR Validator.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofhir/validator/v2/pkg/issue"
	"github.com/gofhir/validator/v2/pkg/loader"
	"github.com/gofhir/validator/v2/pkg/validator"
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
  gofhir-validator -version 4.0.1 patient.json
  gofhir-validator -ig http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient patient.json
  gofhir-validator -output json patient.json
  gofhir-validator -tx n/a patient.json            (no terminology server, as in the HL7 validator)
  gofhir-validator -no-terminology patient.json    (skip terminology and bindings)
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
	BasePackages  []string
	Registry      string
	NoDownload    bool
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
	config, err := parseArgs(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0) // -h: the flag package has printed the usage
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	if config.ShowVersion {
		fmt.Printf("gofhir-validator v%s\n", version)
		os.Exit(0)
	}

	if config.Help || len(config.Files) == 0 {
		printUsage()
		os.Exit(0)
	}

	exitCode := run(config)
	os.Exit(exitCode)
}

// printUsage prints the usage text and the flags; parseArgs sets it.
var printUsage = func() {}

// parseArgs reads the command line (without the program name) into a Config.
func parseArgs(args []string) (*Config, error) {
	config := &Config{
		Version: "4.0.1",
		Output:  OutputText,
	}
	fs := flag.NewFlagSet("gofhir-validator", flag.ContinueOnError)

	// Define flags compatible with HL7 validator
	var profiles, packages, basePackages, packageFiles, packageURLs, tx string
	var output string

	fs.StringVar(&config.Version, "version", "4.0.1", "FHIR version (4.0.1, 4.3.0, 5.0.0)")
	fs.StringVar(&profiles, "ig", "", "Profile URL(s) to validate against (comma-separated)")
	fs.StringVar(&packages, "package", "", "Additional FHIR package(s) to load, with the packages they depend on (e.g., hl7.fhir.us.core#6.1.0)")
	fs.StringVar(&basePackages, "base-package", "", "Base package(s) to load from the package cache instead of the ones embedded for the\n"+
		"version, comma-separated (e.g., hl7.fhir.r4.core#4.0.1,hl7.terminology.r4#6.2.0,hl7.fhir.uv.extensions.r4#5.3.0)")
	fs.StringVar(&config.Registry, "package-registry", loader.DefaultRegistry, "Package registry that packages missing from the package cache are downloaded from:\n"+
		"the packages given with -package and the packages they depend on (not -base-package)")
	fs.BoolVar(&config.NoDownload, "no-download", false, "Download no package: a dependency missing from the package cache is reported and not loaded")
	fs.StringVar(&packageFiles, "package-file", "", "Local .tgz package file(s) to load (comma-separated)")
	fs.StringVar(&packageURLs, "package-url", "", "Remote .tgz package URL(s) to load (comma-separated)")
	fs.StringVar(&output, "output", "text", "Output format: text, json")
	fs.StringVar(&tx, "tx", "", "Terminology server, as in the HL7 validator: 'n/a' for none. Codes are then\n"+
		"checked against the ValueSets and CodeSystems loaded, which is what happens without -tx too.\n"+
		"A server URL is not supported")
	fs.BoolVar(&config.NoTerminology, "no-terminology", false, "Skip all terminology and binding validation")
	fs.BoolVar(&config.Strict, "strict", false, "Treat warnings as errors")
	fs.BoolVar(&config.Quiet, "quiet", false, "Only show errors and warnings")
	fs.BoolVar(&config.Verbose, "verbose", false, "Show detailed output")
	fs.BoolVar(&config.ShowVersion, "v", false, "Show version")
	fs.BoolVar(&config.Help, "help", false, "Show help")

	printUsage = func() {
		fmt.Fprint(os.Stderr, usage)
		fs.SetOutput(os.Stderr)
		fs.PrintDefaults()
		fs.SetOutput(io.Discard)
	}
	fs.Usage = printUsage
	fs.SetOutput(io.Discard) // a parse error is reported once, by main

	// Flags may come before or after the files, as in the HL7 validator, whose usage puts the file
	// first (validator_cli.jar resource.json -version 4.0.1). The flag package stops at the first
	// argument that is not a flag, so parsing resumes after each file; after "--", everything is a
	// file.
	var files []string
	for rest := args; ; {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		consumed := len(rest) - len(fs.Args())
		if consumed > 0 && rest[consumed-1] == "--" {
			files = append(files, fs.Args()...)
			break
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		files = append(files, rest[0])
		rest = rest[1:]
	}

	// -tx names a terminology server. This validator has no client for one: codes are always
	// checked against the definitions loaded, which is the HL7 validator's -tx n/a.
	if tx != "" && tx != "n/a" {
		return nil, fmt.Errorf("-tx %s: a terminology server is not supported; use -tx n/a, or leave -tx out", tx)
	}

	// Parse profiles
	if profiles != "" {
		config.Profiles = strings.Split(profiles, ",")
	}

	// Parse packages
	if packages != "" {
		config.Packages = strings.Split(packages, ",")
	}

	// Parse base packages
	if basePackages != "" {
		config.BasePackages = strings.Split(basePackages, ",")
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

	config.Files = files

	return config, nil
}

func run(config *Config) int {
	opts := buildOptions(config)

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

	outputs, hasErrors := processFiles(v, config)

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

// buildOptions builds validator options from the CLI config.
func buildOptions(config *Config) []validator.Option {
	opts := []validator.Option{
		validator.WithVersion(config.Version),
	}

	for _, profile := range config.Profiles {
		opts = append(opts, validator.WithProfile(strings.TrimSpace(profile)))
	}

	for _, pkg := range config.Packages {
		parts := strings.SplitN(pkg, "#", 2)
		if len(parts) == 2 {
			opts = append(opts, validator.WithPackage(parts[0], parts[1]))
		}
	}

	var base []validator.PackageSpec
	for _, pkg := range config.BasePackages {
		if name, version, ok := strings.Cut(strings.TrimSpace(pkg), "#"); ok {
			base = append(base, validator.PackageSpec{Name: name, Version: version})
		}
	}
	if len(base) > 0 {
		opts = append(opts, validator.WithBasePackages(base...))
	}

	if !config.NoDownload && config.Registry != "" {
		opts = append(opts, validator.WithPackageRegistry(config.Registry))
	}

	for _, tgzPath := range config.PackageFiles {
		opts = append(opts, validator.WithPackageTgz(strings.TrimSpace(tgzPath)))
	}

	for _, url := range config.PackageURLs {
		opts = append(opts, validator.WithPackageURL(strings.TrimSpace(url)))
	}

	if config.Strict {
		opts = append(opts, validator.WithStrictMode(true))
	}

	if config.NoTerminology {
		opts = append(opts, validator.WithNoTerminology())
	}

	return opts
}

// processFiles validates all files specified in the config.
func processFiles(v *validator.Validator, config *Config) ([]ValidationOutput, bool) {
	hasErrors := false
	outputs := make([]ValidationOutput, 0, len(config.Files))

	for _, file := range config.Files {
		if file == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
				hasErrors = true
				continue
			}
			output, fileHasErrors := validateData(v, data, "stdin", config)
			outputs = append(outputs, output)
			if fileHasErrors {
				hasErrors = true
			}
			continue
		}

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
	}

	return outputs, hasErrors
}

func validateFile(v *validator.Validator, path string, config *Config) (ValidationOutput, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // G703: the file the user named on the command line is the one to read
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
