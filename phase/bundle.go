package phase

import (
	"context"
	"fmt"
	"strings"

	fv "github.com/gofhir/validator"
	"github.com/gofhir/validator/pipeline"
)

// BundleType represents the type of Bundle.
type BundleType string

const (
	BundleTypeDocument     BundleType = "document"
	BundleTypeMessage      BundleType = "message"
	BundleTypeTransaction  BundleType = "transaction"
	BundleTypeBatch        BundleType = "batch"
	BundleTypeHistory      BundleType = "history"
	BundleTypeSearchset    BundleType = "searchset"
	BundleTypeCollection   BundleType = "collection"
	BundleTypeSubscription BundleType = "subscription-notification"
)

// BundlePhase validates Bundle-specific constraints.
// This includes:
// - Bundle type-specific rules
// - Entry fullUrl uniqueness
// - Entry request/response requirements
// - Document bundle structure (Composition first)
// - Message bundle structure (MessageHeader first)
// - Transaction/batch request validation
type BundlePhase struct{}

// NewBundlePhase creates a new Bundle validation phase.
func NewBundlePhase() *BundlePhase {
	return &BundlePhase{}
}

// Name returns the phase name.
func (p *BundlePhase) Name() string {
	return "bundle"
}

// Validate performs Bundle-specific validation.
func (p *BundlePhase) Validate(ctx context.Context, pctx *pipeline.Context) []fv.Issue {
	var issues []fv.Issue

	select {
	case <-ctx.Done():
		return issues
	default:
	}

	if pctx.ResourceMap == nil || pctx.ResourceType != "Bundle" {
		return issues
	}

	// Get bundle type
	bundleType, _ := pctx.ResourceMap["type"].(string)
	if bundleType == "" {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeRequired,
			"Bundle must have a 'type' element",
			"Bundle.type",
			"bundle",
		))
		return issues
	}

	// Get entries
	entries, _ := pctx.ResourceMap["entry"].([]any)

	// Validate based on bundle type
	switch BundleType(bundleType) {
	case BundleTypeDocument:
		issues = append(issues, p.validateDocumentBundle(ctx, entries)...)
	case BundleTypeMessage:
		issues = append(issues, p.validateMessageBundle(ctx, entries)...)
	case BundleTypeTransaction:
		issues = append(issues, p.validateTransactionBundle(ctx, entries)...)
	case BundleTypeBatch:
		issues = append(issues, p.validateBatchBundle(ctx, entries)...)
	case BundleTypeHistory:
		issues = append(issues, p.validateHistoryBundle(ctx, entries)...)
	case BundleTypeSearchset:
		issues = append(issues, p.validateSearchsetBundle(ctx, entries)...)
	case BundleTypeCollection:
		issues = append(issues, p.validateCollectionBundle(ctx, entries)...)
	default:
		issues = append(issues, WarningIssue(
			fv.IssueTypeValue,
			fmt.Sprintf("Unknown bundle type: '%s'", bundleType),
			"Bundle.type",
			"bundle",
		))
	}

	// Validate fullUrl uniqueness
	issues = append(issues, p.validateFullURLUniqueness(entries)...)

	return issues
}

// validateDocumentBundle validates a document bundle.
func (p *BundlePhase) validateDocumentBundle(_ context.Context, entries []any) []fv.Issue {
	var issues []fv.Issue

	if len(entries) == 0 {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeRequired,
			"Document Bundle must have at least one entry",
			"Bundle.entry",
			"bundle",
		))
		return issues
	}

	// First entry must be a Composition
	firstEntry, ok := entries[0].(map[string]any)
	if !ok {
		return issues
	}

	resource, _ := firstEntry["resource"].(map[string]any)
	if resource == nil {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeRequired,
			"First entry in document Bundle must contain a resource",
			"Bundle.entry[0].resource",
			"bundle",
		))
		return issues
	}

	resourceType, _ := resource["resourceType"].(string)
	if resourceType != "Composition" {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeStructure,
			fmt.Sprintf("First entry in document Bundle must be a Composition, found '%s'", resourceType),
			"Bundle.entry[0].resource",
			"bundle",
		))
	}

	// All entries should have fullUrl
	for i, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if _, hasFullURL := entryMap["fullUrl"]; !hasFullURL {
			issues = append(issues, WarningIssue(
				fv.IssueTypeRequired,
				"Document Bundle entries should have fullUrl",
				fmt.Sprintf("Bundle.entry[%d].fullUrl", i),
				"bundle",
			))
		}
	}

	return issues
}

// validateMessageBundle validates a message bundle.
func (p *BundlePhase) validateMessageBundle(_ context.Context, entries []any) []fv.Issue {
	var issues []fv.Issue

	if len(entries) == 0 {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeRequired,
			"Message Bundle must have at least one entry",
			"Bundle.entry",
			"bundle",
		))
		return issues
	}

	// First entry must be a MessageHeader
	firstEntry, ok := entries[0].(map[string]any)
	if !ok {
		return issues
	}

	resource, _ := firstEntry["resource"].(map[string]any)
	if resource == nil {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeRequired,
			"First entry in message Bundle must contain a resource",
			"Bundle.entry[0].resource",
			"bundle",
		))
		return issues
	}

	resourceType, _ := resource["resourceType"].(string)
	if resourceType != "MessageHeader" {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeStructure,
			fmt.Sprintf("First entry in message Bundle must be a MessageHeader, found '%s'", resourceType),
			"Bundle.entry[0].resource",
			"bundle",
		))
	}

	return issues
}

// validateTransactionBundle validates a transaction bundle.
func (p *BundlePhase) validateTransactionBundle(_ context.Context, entries []any) []fv.Issue {
	var issues []fv.Issue

	for i, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		path := fmt.Sprintf("Bundle.entry[%d]", i)

		// Transaction entries must have request
		request, hasRequest := entryMap["request"].(map[string]any)
		if !hasRequest {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeRequired,
				"Transaction Bundle entries must have 'request' element",
				path+".request",
				"bundle",
			))
			continue
		}

		// Request must have method
		method, _ := request["method"].(string)
		if method == "" {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeRequired,
				"Transaction request must have 'method' element",
				path+".request.method",
				"bundle",
			))
		}

		// Request must have url
		url, _ := request["url"].(string)
		if url == "" {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeRequired,
				"Transaction request must have 'url' element",
				path+".request.url",
				"bundle",
			))
		}

		// Validate method-specific requirements
		issues = append(issues, p.validateRequestMethod(entryMap, method, path)...)
	}

	return issues
}

// validateBatchBundle validates a batch bundle.
func (p *BundlePhase) validateBatchBundle(_ context.Context, entries []any) []fv.Issue {
	// Batch has same structure as transaction
	return p.validateTransactionBundle(context.Background(), entries)
}

// validateHistoryBundle validates a history bundle.
func (p *BundlePhase) validateHistoryBundle(_ context.Context, entries []any) []fv.Issue {
	var issues []fv.Issue

	for i, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		path := fmt.Sprintf("Bundle.entry[%d]", i)

		// History entries should have request or response
		_, hasRequest := entryMap["request"]
		_, hasResponse := entryMap["response"]

		if !hasRequest && !hasResponse {
			issues = append(issues, WarningIssue(
				fv.IssueTypeStructure,
				"History Bundle entries should have 'request' or 'response' element",
				path,
				"bundle",
			))
		}
	}

	return issues
}

// validateSearchsetBundle validates a searchset bundle.
func (p *BundlePhase) validateSearchsetBundle(_ context.Context, entries []any) []fv.Issue {
	var issues []fv.Issue

	for i, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		path := fmt.Sprintf("Bundle.entry[%d]", i)

		// Search entries should have search element
		search, hasSearch := entryMap["search"].(map[string]any)
		if hasSearch {
			mode, _ := search["mode"].(string)
			if mode != "" && mode != "match" && mode != "include" && mode != "outcome" {
				issues = append(issues, ErrorIssue(
					fv.IssueTypeValue,
					fmt.Sprintf("Invalid search mode: '%s'. Must be 'match', 'include', or 'outcome'", mode),
					path+".search.mode",
					"bundle",
				))
			}
		}
	}

	return issues
}

// validateCollectionBundle validates a collection bundle.
func (p *BundlePhase) validateCollectionBundle(_ context.Context, entries []any) []fv.Issue {
	var issues []fv.Issue

	// Collection bundles have minimal requirements
	// Each entry should have a resource
	for i, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		path := fmt.Sprintf("Bundle.entry[%d]", i)

		if _, hasResource := entryMap["resource"]; !hasResource {
			issues = append(issues, WarningIssue(
				fv.IssueTypeStructure,
				"Collection Bundle entries should have a 'resource' element",
				path+".resource",
				"bundle",
			))
		}
	}

	return issues
}

// validateRequestMethod validates method-specific requirements.
func (p *BundlePhase) validateRequestMethod(entry map[string]any, method, path string) []fv.Issue {
	var issues []fv.Issue

	resource, hasResource := entry["resource"]

	switch strings.ToUpper(method) {
	case "POST", "PUT":
		// POST and PUT should have a resource
		if !hasResource {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeRequired,
				fmt.Sprintf("%s request must have a 'resource' element", method),
				path+".resource",
				"bundle",
			))
		}

	case "DELETE", "GET", "HEAD":
		// DELETE/GET/HEAD should not have a resource
		if hasResource && resource != nil {
			issues = append(issues, WarningIssue(
				fv.IssueTypeStructure,
				fmt.Sprintf("%s request should not have a 'resource' element", method),
				path+".resource",
				"bundle",
			))
		}

	case "PATCH":
		// PATCH should have a resource (Parameters)
		if !hasResource {
			issues = append(issues, ErrorIssue(
				fv.IssueTypeRequired,
				"PATCH request must have a 'resource' element (Parameters)",
				path+".resource",
				"bundle",
			))
		}
	}

	return issues
}

// validateFullURLUniqueness checks that fullUrls are unique.
func (p *BundlePhase) validateFullURLUniqueness(entries []any) []fv.Issue {
	var issues []fv.Issue

	seen := make(map[string]int)

	for i, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}

		fullURL, ok := entryMap["fullUrl"].(string)
		if !ok || fullURL == "" {
			continue
		}

		// Skip urn:uuid: and urn:oid: as they might be intentionally duplicated
		// in some bundle types
		if strings.HasPrefix(fullURL, "urn:uuid:") || strings.HasPrefix(fullURL, "urn:oid:") {
			if prevIdx, exists := seen[fullURL]; exists {
				issues = append(issues, WarningIssue(
					fv.IssueTypeBusinessRule,
					fmt.Sprintf("fullUrl '%s' is duplicated (first seen at entry[%d])", fullURL, prevIdx),
					fmt.Sprintf("Bundle.entry[%d].fullUrl", i),
					"bundle",
				))
			}
		} else {
			if prevIdx, exists := seen[fullURL]; exists {
				issues = append(issues, ErrorIssue(
					fv.IssueTypeBusinessRule,
					fmt.Sprintf("fullUrl '%s' must be unique (first seen at entry[%d])", fullURL, prevIdx),
					fmt.Sprintf("Bundle.entry[%d].fullUrl", i),
					"bundle",
				))
			}
		}

		seen[fullURL] = i
	}

	return issues
}

// BundlePhaseConfig returns the standard configuration for the bundle phase.
func BundlePhaseConfig() *pipeline.PhaseConfig {
	return &pipeline.PhaseConfig{
		Phase:    NewBundlePhase(),
		Priority: pipeline.PriorityEarly, // Run early to catch structural issues
		Parallel: true,
		Required: false, // Only relevant for Bundle resources
		Enabled:  true,
	}
}

// BundleEntryValidator is a helper for validating individual bundle entries.
type BundleEntryValidator struct {
	entryIndex int
	entry      map[string]any
	bundleType BundleType
}

// NewBundleEntryValidator creates a validator for a single bundle entry.
func NewBundleEntryValidator(entry map[string]any, index int, bundleType BundleType) *BundleEntryValidator {
	return &BundleEntryValidator{
		entryIndex: index,
		entry:      entry,
		bundleType: bundleType,
	}
}

// Validate validates the bundle entry.
func (v *BundleEntryValidator) Validate() []fv.Issue {
	var issues []fv.Issue
	path := fmt.Sprintf("Bundle.entry[%d]", v.entryIndex)

	// Check for resource
	resource, _ := v.entry["resource"].(map[string]any)
	fullURL, _ := v.entry["fullUrl"].(string)

	// Entry should have resource or fullUrl
	if resource == nil && fullURL == "" {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeStructure,
			"Bundle entry must have either 'resource' or 'fullUrl'",
			path,
			"bundle",
		))
	}

	// Validate request if present
	if request, ok := v.entry["request"].(map[string]any); ok {
		issues = append(issues, v.validateRequest(request, path+".request")...)
	}

	// Validate response if present
	if response, ok := v.entry["response"].(map[string]any); ok {
		issues = append(issues, v.validateResponse(response, path+".response")...)
	}

	return issues
}

// validateRequest validates a bundle entry request.
func (v *BundleEntryValidator) validateRequest(request map[string]any, path string) []fv.Issue {
	var issues []fv.Issue

	method, _ := request["method"].(string)
	url, _ := request["url"].(string)

	if method == "" {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeRequired,
			"Request must have 'method'",
			path+".method",
			"bundle",
		))
	}

	if url == "" {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeRequired,
			"Request must have 'url'",
			path+".url",
			"bundle",
		))
	}

	// Validate HTTP method
	validMethods := map[string]bool{
		"GET": true, "HEAD": true, "POST": true,
		"PUT": true, "DELETE": true, "PATCH": true,
	}
	if method != "" && !validMethods[strings.ToUpper(method)] {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeValue,
			fmt.Sprintf("Invalid HTTP method: '%s'", method),
			path+".method",
			"bundle",
		))
	}

	return issues
}

// validateResponse validates a bundle entry response.
func (v *BundleEntryValidator) validateResponse(response map[string]any, path string) []fv.Issue {
	var issues []fv.Issue

	status, _ := response["status"].(string)
	if status == "" {
		issues = append(issues, ErrorIssue(
			fv.IssueTypeRequired,
			"Response must have 'status'",
			path+".status",
			"bundle",
		))
	}

	return issues
}
