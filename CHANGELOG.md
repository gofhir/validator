# Changelog

## [1.13.2](https://github.com/gofhir/validator/compare/v1.13.1...v1.13.2) (2026-04-10)


### Bug Fixes

* enforce positiveInt/unsignedInt constraints via SD-derived regex ([50e5b51](https://github.com/gofhir/validator/commit/50e5b51ddeb7d3924759bf170f9da535ff3ac8de)), closes [#53](https://github.com/gofhir/validator/issues/53)

## [1.13.1](https://github.com/gofhir/validator/compare/v1.13.0...v1.13.1) (2026-04-09)


### Bug Fixes

* close 4 FHIR R4 compliance gaps on write-time validation ([c828de2](https://github.com/gofhir/validator/commit/c828de22e30c3019808be4750047da03bcd4f88d)), closes [#51](https://github.com/gofhir/validator/issues/51)

## [1.13.0](https://github.com/gofhir/validator/compare/v1.12.1...v1.13.0) (2026-03-30)


### Features

* add UCUM syntax validation for Quantity elements ([30f184d](https://github.com/gofhir/validator/commit/30f184dc79be18cd98a578eb214753c49b30eb79)), closes [#50](https://github.com/gofhir/validator/issues/50)
* add ValidateWithIG per-call option for implementation guide context ([f0cc05a](https://github.com/gofhir/validator/commit/f0cc05a47cc014b3cccc581f9345d35bc3ddf24e)), closes [#45](https://github.com/gofhir/validator/issues/45)
* add ValidateWithMode per-call option for $validate mode parameter ([9f0715a](https://github.com/gofhir/validator/commit/9f0715aaec5614a734ff4be5214a5b38d56a13ed)), closes [#44](https://github.com/gofhir/validator/issues/44)
* **bundle:** validate fullUrl consistency with resource.id ([ff81065](https://github.com/gofhir/validator/commit/ff81065b3f626998e637966451e65f3d7cce4a3c))
* **constraint:** wire FHIRPath with resolve(), memberOf(), context, and timeout ([#31](https://github.com/gofhir/validator/issues/31), [#34](https://github.com/gofhir/validator/issues/34)) ([2668a9f](https://github.com/gofhir/validator/commit/2668a9f17cb0a015eaf9b1c4fd8dcf59467a3360))
* Initial release of GoFHIR Validator ([d21da33](https://github.com/gofhir/validator/commit/d21da33b0676943b695f411a057adf7b6d8793db))
* **loader:** add WithPackageData and WithConformanceResources options ([48ba6b3](https://github.com/gofhir/validator/commit/48ba6b3f8f6ed2b7ae5e6ee7106a1d3109d3121f)), closes [#12](https://github.com/gofhir/validator/issues/12) [#13](https://github.com/gofhir/validator/issues/13)
* **location:** add line/column information to validation issues ([1b2dc71](https://github.com/gofhir/validator/commit/1b2dc71ab5e4b987eee6c6995b9ca01b1710d897))
* nested constraint evaluation, BackboneElement binding traversal, and lint fixes ([#32](https://github.com/gofhir/validator/issues/32), [#39](https://github.com/gofhir/validator/issues/39), [#40](https://github.com/gofhir/validator/issues/40), [#41](https://github.com/gofhir/validator/issues/41), [#42](https://github.com/gofhir/validator/issues/42)) ([4dfe98f](https://github.com/gofhir/validator/commit/4dfe98f2b254de7ea5eda22a8df291246a9758d7))
* **reference:** implement targetProfile validation from StructureDefinition ([b705841](https://github.com/gofhir/validator/commit/b7058419cb6ebb9f0538fd37063938af32765df2))
* **reference:** validate ElementDefinition.type.aggregation modes ([#36](https://github.com/gofhir/validator/issues/36)) ([2ecd3a3](https://github.com/gofhir/validator/commit/2ecd3a3d92923f88dd2c43a00718e3df91f912a9))
* **registry:** generate snapshot from differential + baseDefinition ([#38](https://github.com/gofhir/validator/issues/38)) ([5a1c806](https://github.com/gofhir/validator/commit/5a1c806492ec18f9b76dc361e16709b97b981923))
* **registry:** version-aware profile resolution with ProfileResolver interface ([#29](https://github.com/gofhir/validator/issues/29)) ([e167955](https://github.com/gofhir/validator/commit/e16795518cc1f6338f8e0e26a2971ac7ec9ece5a))
* **slicing:** implement all discriminator types (value/pattern/exists/type/profile) ([1e5b238](https://github.com/gofhir/validator/commit/1e5b238c73358f6ea4121f52b5570d925883d4e0))
* **specs:** embed filtered FHIR specs for R4, R4B, and R5 ([ef4bb7b](https://github.com/gofhir/validator/commit/ef4bb7bd51ded0996da92e0c7761aaa0b43b7a3f))
* **terminology:** add Provider interface for external terminology validation ([ea9a89e](https://github.com/gofhir/validator/commit/ea9a89e5bdf99248e06fba8aa7cb93e8bffdc8f3)), closes [#22](https://github.com/gofhir/validator/issues/22)
* **terminology:** implement ValueSet filter expansion from CodeSystem hierarchy ([e231f8f](https://github.com/gofhir/validator/commit/e231f8f2d894c1fe7e212e87f2def50cf5983a40))
* **validator:** support per-call profile parameter in Validate() ([0e7e5b2](https://github.com/gofhir/validator/commit/0e7e5b2f2a2634c0123952a88dd0ae08a5a78ce6)), closes [#10](https://github.com/gofhir/validator/issues/10)


### Bug Fixes

* add CLI source and remove unused examples ([2e9d264](https://github.com/gofhir/validator/commit/2e9d264c035ec842feb17c45c5f9a89085487027))
* **deps:** update fhirpath to v1.0.3, remove transitive gofhir/fhir dependency ([dce251d](https://github.com/gofhir/validator/commit/dce251d2151ed85d0586cffe559491700d03b1df))
* **location:** correct off-by-one line number in JSON position tracker ([967d661](https://github.com/gofhir/validator/commit/967d6616e88af09248b7efbefd1898a1df5b97e0)), closes [#15](https://github.com/gofhir/validator/issues/15)
* **reference:** traverse BackboneElement children using parent resource SD ([1263ede](https://github.com/gofhir/validator/commit/1263edebe0d96b93b512bce384fed33434e0801d))
* **reference:** validate fragment references against contained resources ([#26](https://github.com/gofhir/validator/issues/26)) ([8ecfbc4](https://github.com/gofhir/validator/commit/8ecfbc440e2c7d98d27e6bfd4ba858620512fe7b))
* rename error variable to follow errXxx convention ([955b651](https://github.com/gofhir/validator/commit/955b651d7cffe5b27e5d4d9c80f2b9467e0a842f))
* resolve all golangci-lint issues across codebase ([3b93403](https://github.com/gofhir/validator/commit/3b93403bb03a0a2eef8d6eff604a3a9fe19908ac))
* **slicing:** add MessageID to all slicing validation issues ([e1e7571](https://github.com/gofhir/validator/commit/e1e7571dfa9c7e4d63b688beeb37822808f0ff8c)), closes [#20](https://github.com/gofhir/validator/issues/20)
* **slicing:** enforce cardinality on child elements within matched slices ([3f9270b](https://github.com/gofhir/validator/commit/3f9270b421a66b62e45a9110d772e9d76553279d)), closes [#17](https://github.com/gofhir/validator/issues/17)
* **validator:** disable FHIRPath trace output by default ([c5105dc](https://github.com/gofhir/validator/commit/c5105dce98f4749c2f443a57c049707fff0732fe))
* wire -tx n/a flag, error on unknown modifier extensions, add constraint source ([11ea30f](https://github.com/gofhir/validator/commit/11ea30fab2687ab542847d72550399cec1820943))


### Performance Improvements

* optimize tests with shared validator instance ([764e71b](https://github.com/gofhir/validator/commit/764e71b872f921fd85731b4acf070f0376b8b6ca))
* **tests:** use sync.Once shared setup to avoid redundant FHIR package loading ([6c73530](https://github.com/gofhir/validator/commit/6c735304b3d99e79424b90c61b31480af7d88b98))

## [1.12.1](https://github.com/gofhir/validator/compare/v1.12.0...v1.12.1) (2026-03-29)


### Performance Improvements

* **tests:** use sync.Once shared setup to avoid redundant FHIR package loading ([6c73530](https://github.com/gofhir/validator/commit/6c735304b3d99e79424b90c61b31480af7d88b98))

## [1.12.0](https://github.com/gofhir/validator/compare/v1.11.0...v1.12.0) (2026-03-28)


### Features

* add ValidateWithIG per-call option for implementation guide context ([f0cc05a](https://github.com/gofhir/validator/commit/f0cc05a47cc014b3cccc581f9345d35bc3ddf24e)), closes [#45](https://github.com/gofhir/validator/issues/45)
* add ValidateWithMode per-call option for $validate mode parameter ([9f0715a](https://github.com/gofhir/validator/commit/9f0715aaec5614a734ff4be5214a5b38d56a13ed)), closes [#44](https://github.com/gofhir/validator/issues/44)


### Bug Fixes

* resolve all golangci-lint issues across codebase ([3b93403](https://github.com/gofhir/validator/commit/3b93403bb03a0a2eef8d6eff604a3a9fe19908ac))

## [1.11.0](https://github.com/gofhir/validator/compare/v1.10.0...v1.11.0) (2026-03-01)


### Features

* **constraint:** wire FHIRPath with resolve(), memberOf(), context, and timeout ([#31](https://github.com/gofhir/validator/issues/31), [#34](https://github.com/gofhir/validator/issues/34)) ([2668a9f](https://github.com/gofhir/validator/commit/2668a9f17cb0a015eaf9b1c4fd8dcf59467a3360))
* nested constraint evaluation, BackboneElement binding traversal, and lint fixes ([#32](https://github.com/gofhir/validator/issues/32), [#39](https://github.com/gofhir/validator/issues/39), [#40](https://github.com/gofhir/validator/issues/40), [#41](https://github.com/gofhir/validator/issues/41), [#42](https://github.com/gofhir/validator/issues/42)) ([4dfe98f](https://github.com/gofhir/validator/commit/4dfe98f2b254de7ea5eda22a8df291246a9758d7))
* **reference:** validate ElementDefinition.type.aggregation modes ([#36](https://github.com/gofhir/validator/issues/36)) ([2ecd3a3](https://github.com/gofhir/validator/commit/2ecd3a3d92923f88dd2c43a00718e3df91f912a9))
* **registry:** generate snapshot from differential + baseDefinition ([#38](https://github.com/gofhir/validator/issues/38)) ([5a1c806](https://github.com/gofhir/validator/commit/5a1c806492ec18f9b76dc361e16709b97b981923))


### Bug Fixes

* wire -tx n/a flag, error on unknown modifier extensions, add constraint source ([11ea30f](https://github.com/gofhir/validator/commit/11ea30fab2687ab542847d72550399cec1820943))

## [1.10.0](https://github.com/gofhir/validator/compare/v1.9.2...v1.10.0) (2026-03-01)


### Features

* **registry:** version-aware profile resolution with ProfileResolver interface ([#29](https://github.com/gofhir/validator/issues/29)) ([e167955](https://github.com/gofhir/validator/commit/e16795518cc1f6338f8e0e26a2971ac7ec9ece5a))

## [1.9.2](https://github.com/gofhir/validator/compare/v1.9.1...v1.9.2) (2026-03-01)


### Bug Fixes

* **reference:** traverse BackboneElement children using parent resource SD ([1263ede](https://github.com/gofhir/validator/commit/1263edebe0d96b93b512bce384fed33434e0801d))

## [1.9.1](https://github.com/gofhir/validator/compare/v1.9.0...v1.9.1) (2026-03-01)


### Bug Fixes

* **reference:** validate fragment references against contained resources ([#26](https://github.com/gofhir/validator/issues/26)) ([8ecfbc4](https://github.com/gofhir/validator/commit/8ecfbc440e2c7d98d27e6bfd4ba858620512fe7b))

## [1.9.0](https://github.com/gofhir/validator/compare/v1.8.1...v1.9.0) (2026-02-17)


### Features

* **specs:** embed filtered FHIR specs for R4, R4B, and R5 ([ef4bb7b](https://github.com/gofhir/validator/commit/ef4bb7bd51ded0996da92e0c7761aaa0b43b7a3f))

## [1.8.1](https://github.com/gofhir/validator/compare/v1.8.0...v1.8.1) (2026-02-17)


### Bug Fixes

* **deps:** update fhirpath to v1.0.3, remove transitive gofhir/fhir dependency ([dce251d](https://github.com/gofhir/validator/commit/dce251d2151ed85d0586cffe559491700d03b1df))

## [1.8.0](https://github.com/gofhir/validator/compare/v1.7.1...v1.8.0) (2026-02-16)


### Features

* **terminology:** add Provider interface for external terminology validation ([ea9a89e](https://github.com/gofhir/validator/commit/ea9a89e5bdf99248e06fba8aa7cb93e8bffdc8f3)), closes [#22](https://github.com/gofhir/validator/issues/22)

## [1.7.1](https://github.com/gofhir/validator/compare/v1.7.0...v1.7.1) (2026-02-15)


### Bug Fixes

* **slicing:** add MessageID to all slicing validation issues ([e1e7571](https://github.com/gofhir/validator/commit/e1e7571dfa9c7e4d63b688beeb37822808f0ff8c)), closes [#20](https://github.com/gofhir/validator/issues/20)

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
