# Changelog

## [1.7.0](https://github.com/gofhir/validator/compare/v1.6.2...v1.7.0) (2026-02-15)


### Features

* **slicing:** implement all discriminator types (value/pattern/exists/type/profile) ([1e5b238](https://github.com/gofhir/validator/commit/1e5b238c73358f6ea4121f52b5570d925883d4e0))

## [1.6.2](https://github.com/gofhir/validator/compare/v1.6.1...v1.6.2) (2026-02-15)


### Bug Fixes

* **slicing:** enforce cardinality on child elements within matched slices ([3f9270b](https://github.com/gofhir/validator/commit/3f9270b421a66b62e45a9110d772e9d76553279d)), closes [#17](https://github.com/gofhir/validator/issues/17)

## [1.6.1](https://github.com/gofhir/validator/compare/v1.6.0...v1.6.1) (2026-02-14)


### Bug Fixes

* **location:** correct off-by-one line number in JSON position tracker ([967d661](https://github.com/gofhir/validator/commit/967d6616e88af09248b7efbefd1898a1df5b97e0)), closes [#15](https://github.com/gofhir/validator/issues/15)

## [1.6.0](https://github.com/gofhir/validator/compare/v1.5.0...v1.6.0) (2026-02-12)


### Features

* **loader:** add WithPackageData and WithConformanceResources options ([48ba6b3](https://github.com/gofhir/validator/commit/48ba6b3f8f6ed2b7ae5e6ee7106a1d3109d3121f)), closes [#12](https://github.com/gofhir/validator/issues/12) [#13](https://github.com/gofhir/validator/issues/13)

## [1.5.0](https://github.com/gofhir/validator/compare/v1.4.1...v1.5.0) (2026-02-12)


### Features

* **validator:** support per-call profile parameter in Validate() ([0e7e5b2](https://github.com/gofhir/validator/commit/0e7e5b2f2a2634c0123952a88dd0ae08a5a78ce6)), closes [#10](https://github.com/gofhir/validator/issues/10)

## [1.4.1](https://github.com/gofhir/validator/compare/v1.4.0...v1.4.1) (2026-02-04)


### Bug Fixes

* **validator:** disable FHIRPath trace output by default ([c5105dc](https://github.com/gofhir/validator/commit/c5105dce98f4749c2f443a57c049707fff0732fe))

## [1.4.0](https://github.com/gofhir/validator/compare/v1.3.0...v1.4.0) (2026-02-02)


### Features

* **bundle:** validate fullUrl consistency with resource.id ([ff81065](https://github.com/gofhir/validator/commit/ff81065b3f626998e637966451e65f3d7cce4a3c))
* **location:** add line/column information to validation issues ([1b2dc71](https://github.com/gofhir/validator/commit/1b2dc71ab5e4b987eee6c6995b9ca01b1710d897))

## [1.3.0](https://github.com/gofhir/validator/compare/v1.2.0...v1.3.0) (2026-02-01)


### Features

* **reference:** implement targetProfile validation from StructureDefinition ([b705841](https://github.com/gofhir/validator/commit/b7058419cb6ebb9f0538fd37063938af32765df2))

## [1.2.0](https://github.com/gofhir/validator/compare/v1.1.0...v1.2.0) (2026-01-31)


### Features

* **terminology:** implement ValueSet filter expansion from CodeSystem hierarchy ([e231f8f](https://github.com/gofhir/validator/commit/e231f8f2d894c1fe7e212e87f2def50cf5983a40))

## [1.1.0](https://github.com/gofhir/validator/compare/v1.0.0...v1.1.0) (2026-01-30)


### Features

* Initial release of GoFHIR Validator ([d21da33](https://github.com/gofhir/validator/commit/d21da33b0676943b695f411a057adf7b6d8793db))


### Bug Fixes

* add CLI source and remove unused examples ([2e9d264](https://github.com/gofhir/validator/commit/2e9d264c035ec842feb17c45c5f9a89085487027))
* rename error variable to follow errXxx convention ([955b651](https://github.com/gofhir/validator/commit/955b651d7cffe5b27e5d4d9c80f2b9467e0a842f))


### Performance Improvements

* optimize tests with shared validator instance ([764e71b](https://github.com/gofhir/validator/commit/764e71b872f921fd85731b4acf070f0376b8b6ca))

## 1.0.0 (2026-01-30)


### Features

* Initial release of GoFHIR Validator ([d21da33](https://github.com/gofhir/validator/commit/d21da33b0676943b695f411a057adf7b6d8793db))


### Bug Fixes

* add CLI source and remove unused examples ([2e9d264](https://github.com/gofhir/validator/commit/2e9d264c035ec842feb17c45c5f9a89085487027))
* rename error variable to follow errXxx convention ([955b651](https://github.com/gofhir/validator/commit/955b651d7cffe5b27e5d4d9c80f2b9467e0a842f))


### Performance Improvements

* optimize tests with shared validator instance ([764e71b](https://github.com/gofhir/validator/commit/764e71b872f921fd85731b4acf070f0376b8b6ca))
