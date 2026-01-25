# Changelog

## [1.1.0](https://github.com/gofhir/validator/compare/v1.0.1...v1.1.0) (2026-01-25)


### Features

* **cardinality:** add choice type occurrence counting ([d15a979](https://github.com/gofhir/validator/commit/d15a979be2c3de7e12018f31e36fd8291592055a))
* **constraints:** add dom-6 well-known constraint support ([219c8d1](https://github.com/gofhir/validator/commit/219c8d13d0d254f59830248a85b370448e576060))
* **engine:** add recursive Bundle entry validation ([3d271c0](https://github.com/gofhir/validator/commit/3d271c016d62c1322cab7b5d4d949992d8b65ceb))
* enhance service interfaces and terminology support ([afc69e4](https://github.com/gofhir/validator/commit/afc69e4bc8b1baa357479dcbec51706a2c979995))
* **extensions:** recursive type resolution through nested DataTypes ([513c0fb](https://github.com/gofhir/validator/commit/513c0fb8501e6a2d2cfb5cae41ec8d1377d47104))
* **loader:** enhance profile loading and conversion ([0460237](https://github.com/gofhir/validator/commit/04602374226b12f442c299a634c980d63505ba95))
* **terminology:** validate RFC 2606 example URLs in CodeSystem URIs ([2ab7e1b](https://github.com/gofhir/validator/commit/2ab7e1b4923752fd6cf80ff5a61dce5030000cce))
* **walker:** add ToFHIRPath function for FHIRPath expression generation ([0c2677a](https://github.com/gofhir/validator/commit/0c2677ac8cec3b7f00c81697f073193e073ce20e))
* **walker:** improve type-aware tree walking and element indexing ([4bcb033](https://github.com/gofhir/validator/commit/4bcb0333802a06954573bca9d367beb9dc50af1a))


### Bug Fixes

* **terminology:** align CodeSystem severity with HL7 validator ([41efc3f](https://github.com/gofhir/validator/commit/41efc3f24fdd09c51cc87447c9e8e0140c473b9a))

## [1.0.1](https://github.com/gofhir/validator/compare/v1.0.0...v1.0.1) (2026-01-24)


### Bug Fixes

* resolve all golangci-lint errors ([b86288b](https://github.com/gofhir/validator/commit/b86288bf5d31a6048bcc2c46468fcf0c189f8585))
