package loader

import "testing"

func TestPublishers(t *testing.T) {
	core := &Package{Name: "hl7.fhir.r4.core", Version: "4.0.1", Type: "fhir.core", Canonical: "http://hl7.org/fhir"}
	r5 := &Package{Name: "hl7.fhir.r5.core", Version: "5.0.0", Type: "Core", Canonical: "http://hl7.org/fhir"}
	tho := &Package{Name: "hl7.terminology.r4", Version: "7.4.0", Type: "IG", Canonical: "http://terminology.hl7.org"}
	const url = "http://terminology.hl7.org/CodeSystem/consentpolicycodes"

	var p Publishers
	if !p.Add(core) || p.IsCopy("hl7.fhir.r4.core#4.0.1", url) {
		t.Error("without the publisher loaded, the core package's definition is no copy")
	}
	if !p.Add(tho) || !p.IsCopy("hl7.fhir.r4.core#4.0.1", url) {
		t.Error("with the publisher loaded, the core package's definition is a copy")
	}
	if p.Add(tho) {
		t.Error("a canonical recorded twice changes nothing")
	}
	if p.IsCopy("hl7.terminology.r4#7.4.0", url) {
		t.Error("the publisher's definition is no copy")
	}
	if p.IsCopy("hl7.fhir.r4.core#4.0.1", "http://hl7.org/fhir/StructureDefinition/Patient") {
		t.Error("a definition under no other package's canonical is no copy")
	}
	// The specification's examples package carries the same copies; its own definitions, and the
	// core package's, are no copies of each other.
	examples := &Package{Name: "hl7.fhir.r4.examples", Version: "4.0.1", Type: "fhir.examples", Canonical: "http://hl7.org/fhir"}
	if !p.Add(examples) || !p.IsCopy("hl7.fhir.r4.examples#4.0.1", url) {
		t.Error("the examples package's definition under THO's canonical is a copy")
	}
	if p.IsCopy("hl7.fhir.r4.core#4.0.1", "http://hl7.org/fhir/StructureDefinition/Patient") ||
		p.IsCopy("hl7.fhir.r4.examples#4.0.1", "http://hl7.org/fhir/StructureDefinition/Patient") {
		t.Error("a definition under the package's own canonical is no copy")
	}
	expansions := &Package{Name: "hl7.fhir.r4.expansions", Version: "4.0.1", Type: "fhir.expansions", Canonical: "http://hl7.org/fhir"}
	if !p.Add(expansions) || !p.IsCopy("hl7.fhir.r4.expansions#4.0.1", url) {
		t.Error("the expansions package's definition under THO's canonical is a copy")
	}
	// A guide whose canonical covers the core package's does not make its definitions copies.
	p.Add(&Package{Name: "acme", Version: "1.0.0", Canonical: "http://hl7.org/fhir"})
	if p.IsCopy("hl7.fhir.r4.core#4.0.1", "http://hl7.org/fhir/StructureDefinition/Patient") {
		t.Error("a definition under the package's own canonical is no copy")
	}
	if p.Add(r5) || p.IsCopy("hl7.fhir.r5.core#5.0.0", url) {
		t.Error("the errata does not list the R5 core package")
	}
}
