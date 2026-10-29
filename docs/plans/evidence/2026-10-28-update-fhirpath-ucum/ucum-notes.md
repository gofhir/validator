### ucum v4.3.0
## [4.3.0](https://github.com/gofhir/ucum/compare/v4.2.0...v4.3.0) (2026-10-16)


### Features

* Multiply and Divide write the result in the operands' own units ([#50](https://github.com/gofhir/ucum/issues/50)) ([00124e9](https://github.com/gofhir/ucum/commit/00124e9e575c6493c17f6c05c7f2e419d0fab283))
### ucum v4.3.1
## [4.3.1](https://github.com/gofhir/ucum/compare/v4.3.0...v4.3.1) (2026-10-26)


### Bug Fixes

* CanonicalKey uses Quantity.Exact as given and rounds once ([#57](https://github.com/gofhir/ucum/issues/57)) ([9691ced](https://github.com/gofhir/ucum/commit/9691ced3751286d34a963aa8635af2dbc09d8df5)), closes [#54](https://github.com/gofhir/ucum/issues/54)
### ucum v4.3.2
## [4.3.2](https://github.com/gofhir/ucum/compare/v4.3.1...v4.3.2) (2026-10-28)


### Bug Fixes

* report results outside the range of float64 as errors ([#62](https://github.com/gofhir/ucum/issues/62)) ([959d6b1](https://github.com/gofhir/ucum/commit/959d6b1d82ae92ad91751ede23c1912262116454)), closes [#59](https://github.com/gofhir/ucum/issues/59)
### ucum v4.3.3
## [4.3.3](https://github.com/gofhir/ucum/compare/v4.3.2...v4.3.3) (2026-10-28)


### Bug Fixes

* reject a curly brace nested inside an annotation ([#67](https://github.com/gofhir/ucum/issues/67)) ([cd0d2bd](https://github.com/gofhir/ucum/commit/cd0d2bd8e2aee4966343ccb0ba37029a2c8b8ef5))
* reject spaces inside unit annotations ([bd014cb](https://github.com/gofhir/ucum/commit/bd014cb9e54553f77d3c48e7affe0bd4049ddaa8))
### ucum v4.3.4
## [4.3.4](https://github.com/gofhir/ucum/compare/v4.3.3...v4.3.4) (2026-10-29)


### Bug Fixes

* copy cache keys so a sliced code does not retain the caller's input ([#68](https://github.com/gofhir/ucum/issues/68)) ([672635f](https://github.com/gofhir/ucum/commit/672635f57fa8d9528bac34f077d61591fbb52c3a))
