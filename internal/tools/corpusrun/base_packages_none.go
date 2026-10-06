//go:build !basepackages

package main

import (
	"errors"

	"github.com/gofhir/validator/v2/pkg/validator"
)

// baseOption fails: this build is against a library with no way to replace the base packages it
// embeds (before validator.WithBasePackages), and its runs use those.
func baseOption([]validator.PackageSpec) (validator.Option, error) {
	return nil, errors.New("this build cannot replace the base packages: its library predates validator.WithBasePackages")
}
