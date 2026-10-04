package registry

import (
	"encoding/json"
	"slices"
)

// implementsExtension is the extension a definition declares the interfaces its type implements
// with (R5: CanonicalResource, MetadataResource).
const implementsExtension = "http://hl7.org/fhir/StructureDefinition/structuredefinition-implements"

// The interface every canonical resource implements, by name.
const canonicalResourceInterface = "CanonicalResource"

// canonicalResourceTypes are, for a FHIR version whose definitions declare no interfaces, the
// resources its specification lists as having canonical URLs, in references.html#canonical. The
// list is prose there, and no rule over that version's definitions reproduces it (a root url
// element names Device and Contract and misses NamingSystem; the targets of canonical elements
// miss ten), so it is kept as data, with its source, for that version only.
var canonicalResourceTypes = map[string][]string{
	// hl7.org/fhir/R4/references.html#canonical: the resources "allowed as targets of canonical
	// references".
	fhirR4: {
		"ActivityDefinition", "CapabilityStatement", "ChargeItemDefinition", "CodeSystem",
		"CompartmentDefinition", "ConceptMap", "EffectEvidenceSynthesis", "EventDefinition", "Evidence",
		"EvidenceVariable", "ExampleScenario", "GraphDefinition", "ImplementationGuide", "Library",
		"Measure", "MessageDefinition", "NamingSystem", "OperationDefinition", "PlanDefinition",
		"Questionnaire", "ResearchDefinition", "ResearchElementDefinition", "RiskEvidenceSynthesis",
		"SearchParameter", "StructureDefinition", "StructureMap", "TerminologyCapabilities", "TestScript",
		"ValueSet",
	},
	// hl7.org/fhir/R4B/references.html#canonical: "The following resources have canonical URLs and
	// are allowed to be the target of a references to a canonical URLs".
	fhirR4B: {
		"ActivityDefinition", "CapabilityStatement", "ChargeItemDefinition", "Citation", "CodeSystem",
		"CompartmentDefinition", "ConceptMap", "EventDefinition", "Evidence", "EvidenceReport",
		"EvidenceVariable", "ExampleScenario", "GraphDefinition", "ImplementationGuide", "Library",
		"Measure", "MessageDefinition", "NamingSystem", "OperationDefinition", "PlanDefinition",
		"Questionnaire", "ResearchDefinition", "ResearchElementDefinition", "SearchParameter",
		"StructureDefinition", "StructureMap", "SubscriptionTopic", "TerminologyCapabilities",
		"TestScript", "ValueSet",
	},
}

// Interfaces returns the interfaces the type sd defines implements: those its definition declares
// (structuredefinition-implements), and those each of them declares in turn (R5: MetadataResource
// implements CanonicalResource); or, for a version whose definitions declare none,
// CanonicalResource for the resources its specification lists as canonical. R4's and R4B's
// MetadataResource is a logical model no resource derives from or declares, and names none, as in
// the HL7 validator.
func (r *Registry) Interfaces(sd *StructureDefinition) []string {
	if sd == nil {
		return nil
	}
	r.mu.RLock()
	out, ok := r.interfaces[sd]
	r.mu.RUnlock()
	if ok {
		return out
	}
	out = r.interfacesOf(sd)
	r.mu.Lock()
	r.interfaces[sd] = out
	r.mu.Unlock()
	return out
}

// interfacesOf works out Interfaces.
func (r *Registry) interfacesOf(sd *StructureDefinition) []string {
	var out []string
	seen := map[string]bool{}
	pending := declaredInterfaces(sd)
	for len(pending) > 0 {
		url := pending[0]
		pending = pending[1:]
		if seen[url] {
			continue
		}
		seen[url] = true
		if iface, _ := r.ResolveCanonical(url); iface != nil {
			out = append(out, iface.Type)
			pending = append(pending, declaredInterfaces(iface)...)
		}
	}
	if len(out) == 0 && slices.Contains(canonicalResourceTypes[sd.FHIRVersion], sd.Type) {
		out = append(out, canonicalResourceInterface)
	}
	return out
}

// declaredInterfaces are the canonical URLs of the interfaces sd declares its type implements.
func declaredInterfaces(sd *StructureDefinition) []string {
	if len(sd.raw) == 0 {
		return nil
	}
	var doc struct {
		Extension []struct {
			URL            string `json:"url"`
			ValueURI       string `json:"valueUri"`
			ValueCanonical string `json:"valueCanonical"`
		} `json:"extension"`
	}
	if json.Unmarshal(sd.raw, &doc) != nil {
		return nil
	}
	var out []string
	for _, x := range doc.Extension {
		if x.URL != implementsExtension {
			continue
		}
		if x.ValueCanonical != "" {
			out = append(out, x.ValueCanonical)
		} else if x.ValueURI != "" {
			out = append(out, x.ValueURI)
		}
	}
	return out
}
