### fhirpath v1.14.0
## [1.14.0](https://github.com/gofhir/fhirpath/compare/v1.13.6...v1.14.0) (2026-10-13)


### Features

* **difftest:** compare a battery across this engine, fhirpath.js and HL7's Java engine ([#173](https://github.com/gofhir/fhirpath/issues/173)) ([de05b21](https://github.com/gofhir/fhirpath/commit/de05b214f3bbb0e2b217032380c8aee23f44ce87))


### Bug Fixes

* = between complex values finds each child by its key, as fhirpath.js and HL7's Java engine do ([#174](https://github.com/gofhir/fhirpath/issues/174)) ([81a2168](https://github.com/gofhir/fhirpath/commit/81a216869eb03df2ef4f1e0107c2fe6e2d216101))
### fhirpath v1.14.1
## [1.14.1](https://github.com/gofhir/fhirpath/compare/v1.14.0...v1.14.1) (2026-10-14)


### Bug Fixes

* type Resource.id, Element.id and Extension.url by the FHIR type their definitions name ([#178](https://github.com/gofhir/fhirpath/issues/178)) ([fab7ab1](https://github.com/gofhir/fhirpath/commit/fab7ab1ca28644606543e5586e695615d2c2ef88))
### fhirpath v1.14.2
## [1.14.2](https://github.com/gofhir/fhirpath/compare/v1.14.1...v1.14.2) (2026-10-14)


### Bug Fixes

* where fhirpath.js and HL7's Java engine agree, answer as they do on offsets, leap seconds, as() and comparable() ([#181](https://github.com/gofhir/fhirpath/issues/181)) ([4f6b85b](https://github.com/gofhir/fhirpath/commit/4f6b85b378b2b7dc0f06f058ac48d09337bd80e3))
### fhirpath v1.15.0
## [1.15.0](https://github.com/gofhir/fhirpath/compare/v1.14.7...v1.15.0) (2026-10-16)


### Features

* FHIR functions subsumes, subsumedBy, htmlChecks, checkModifiers, elementDefinition and slice, and each FHIR function as its version's page says ([#195](https://github.com/gofhir/fhirpath/issues/195)) ([5a723da](https://github.com/gofhir/fhirpath/commit/5a723dad1a8a2555dffe886ef58b75e9f2db635d))
### fhirpath v1.15.1
## [1.15.1](https://github.com/gofhir/fhirpath/compare/v1.15.0...v1.15.1) (2026-10-16)


### Bug Fixes

* the Math functions refuse what is no number, round quantities, and arithmetic takes the unit the text names ([#197](https://github.com/gofhir/fhirpath/issues/197)) ([74cbcda](https://github.com/gofhir/fhirpath/commit/74cbcdab746f9baf5307adee23b7fd34b18707ae)), closes [#167](https://github.com/gofhir/fhirpath/issues/167)
### fhirpath v1.16.0
## [1.16.0](https://github.com/gofhir/fhirpath/compare/v1.15.1...v1.16.0) (2026-10-17)


### Bug Fixes

* conversions as the text has them, toDate and toDateTime take 3.0.0's format, and toLong reads as toInteger ([#200](https://github.com/gofhir/fhirpath/issues/200)) ([ebf2b67](https://github.com/gofhir/fhirpath/commit/ebf2b67e2127f8eeb2536ba1710e47248e636e1d)), closes [#167](https://github.com/gofhir/fhirpath/issues/167)


### Miscellaneous Chores

* release 1.16.0 ([d232ea5](https://github.com/gofhir/fhirpath/commit/d232ea59700ef8ba53af58281543f7a418531714))
### fhirpath v1.16.1
## [1.16.1](https://github.com/gofhir/fhirpath/compare/v1.16.0...v1.16.1) (2026-10-17)


### Bug Fixes

* aggregate() keeps the whole $total and reads init on the outer context, and sum() and avg() take quantities ([#202](https://github.com/gofhir/fhirpath/issues/202)) ([6e2b8ac](https://github.com/gofhir/fhirpath/commit/6e2b8ac66a4d0f4aa88dc8049db57aeaa7f500c2)), closes [#167](https://github.com/gofhir/fhirpath/issues/167)
### fhirpath v1.16.2
## [1.16.2](https://github.com/gofhir/fhirpath/compare/v1.16.1...v1.16.2) (2026-10-17)


### Bug Fixes

* memberOf() of more than one item is empty ([#204](https://github.com/gofhir/fhirpath/issues/204)) ([19430bb](https://github.com/gofhir/fhirpath/commit/19430bb793f1ef9d2fa3826984b8fa73ef924f44))
### fhirpath v1.16.3
## [1.16.3](https://github.com/gofhir/fhirpath/compare/v1.16.2...v1.16.3) (2026-10-18)


### Bug Fixes

* ~ pairs a primitive's value with the element beside it ([#206](https://github.com/gofhir/fhirpath/issues/206)) ([22b7251](https://github.com/gofhir/fhirpath/commit/22b72516c40038704357fc36e510b0f7838d54e4)), closes [#175](https://github.com/gofhir/fhirpath/issues/175)
### fhirpath v1.16.4
## [1.16.4](https://github.com/gofhir/fhirpath/compare/v1.16.3...v1.16.4) (2026-10-18)


### Performance Improvements

* parse with SLL prediction first, falling back to LL ([#209](https://github.com/gofhir/fhirpath/issues/209)) ([461aaa1](https://github.com/gofhir/fhirpath/commit/461aaa190d8812e599a6dc789c10e1a87a339422)), closes [#176](https://github.com/gofhir/fhirpath/issues/176)
### fhirpath v1.16.5
## [1.16.5](https://github.com/gofhir/fhirpath/compare/v1.16.4...v1.16.5) (2026-10-18)


### Bug Fixes

* static analysis gaps and type operators that skip the specifier check ([#211](https://github.com/gofhir/fhirpath/issues/211)) ([97436d1](https://github.com/gofhir/fhirpath/commit/97436d19119d4e648240bcec01003e5b4188a886)), closes [#188](https://github.com/gofhir/fhirpath/issues/188)
### fhirpath v1.16.6
## [1.16.6](https://github.com/gofhir/fhirpath/compare/v1.16.5...v1.16.6) (2026-10-18)


### Bug Fixes

* answer now(), today() and timeOfDay() from one instant per evaluation ([#226](https://github.com/gofhir/fhirpath/issues/226)) ([0be3c80](https://github.com/gofhir/fhirpath/commit/0be3c80395efaceda8beac440f6f2a69a2c2a47b)), closes [#225](https://github.com/gofhir/fhirpath/issues/225)
* read a choice element only by the suffixes the model allows ([#227](https://github.com/gofhir/fhirpath/issues/227)) ([0cdf188](https://github.com/gofhir/fhirpath/commit/0cdf188c5d2f1f0d9d58d37d41ffab0dc7588c65)), closes [#219](https://github.com/gofhir/fhirpath/issues/219)
* read getExtensionValue(url) as extension(url).value ([#220](https://github.com/gofhir/fhirpath/issues/220)) ([42d50d4](https://github.com/gofhir/fhirpath/commit/42d50d41818e7953420a1db2834baae2d6fdab1f)), closes [#218](https://github.com/gofhir/fhirpath/issues/218)
* type extension(url) items as .extension.where(url = ...) does ([#217](https://github.com/gofhir/fhirpath/issues/217)) ([067c1de](https://github.com/gofhir/fhirpath/commit/067c1de0cfed1630572a7b365c3d2c12340e9fdc)), closes [#214](https://github.com/gofhir/fhirpath/issues/214)
