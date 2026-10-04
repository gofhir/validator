//go:build basepackages

package main

import "github.com/gofhir/validator/pkg/validator"

// baseOption replaces the base packages gofhir embeds with those given. Built with the
// basepackages tag, against a library that has validator.WithBasePackages.
func baseOption(base []validator.PackageSpec) (validator.Option, error) {
	return validator.WithBasePackages(base...), nil
}
