# Changelog

## [2.2.1](https://github.com/gofhir/validator/compare/v2.2.0...v2.2.1) (2026-10-30)


### Bug Fixes

* **narrative:** read the narrative as strict XHTML and derive its HTML rules from HTML 4.0 ([#185](https://github.com/gofhir/validator/issues/185)) ([970e2c1](https://github.com/gofhir/validator/commit/970e2c1b1e1a9657b55de9bbab56c7889b643650))

## [2.2.0](https://github.com/gofhir/validator/compare/v2.1.0...v2.2.0) (2026-10-29)


### Features

* **binding:** bind each value by the definition that governs it, over the definitions ([#157](https://github.com/gofhir/validator/issues/157)) ([3818a4d](https://github.com/gofhir/validator/commit/3818a4d02254c92e1550cfe2a5dbaa46a5bd9151))
* **binding:** check a Quantity's system, a Coding's system URI, and a Coding with no code as HL7 does ([#166](https://github.com/gofhir/validator/issues/166)) ([17a6dee](https://github.com/gofhir/validator/commit/17a6dee4b535755a147a0df172489fc1f4185a93))
* **binding:** check the base element's binding where a profile's value set is not found ([#160](https://github.com/gofhir/validator/issues/160)) ([9642050](https://github.com/gofhir/validator/commit/9642050461003e65f04298b61c06c677476e23e8))
* **binding:** report a code its code system cannot check with the severity HL7 gives it ([#164](https://github.com/gofhir/validator/issues/164)) ([c582c55](https://github.com/gofhir/validator/commit/c582c555032c47a9961783e9674d5c9dacf6bc84))
* **binding:** report a Coding's version its code system is not loaded in, and check its code in no other ([#167](https://github.com/gofhir/validator/issues/167)) ([c7b2205](https://github.com/gofhir/validator/commit/c7b2205536ee6cc880a6f2cb1f992e87f47d3705))
* **binding:** walk extensions under their definitions in the binding phase ([#159](https://github.com/gofhir/validator/issues/159)) ([9de43fd](https://github.com/gofhir/validator/commit/9de43fd6514524f42e5c873393ee9a3f5c10c78d))
* check a held resource against its holder element's type.profile ([#161](https://github.com/gofhir/validator/issues/161)) ([f7b9b6e](https://github.com/gofhir/validator/commit/f7b9b6ed858811a922cb453d5fb57c9635198620))
* **loader:** load a package's example folder as the HL7 validator loads it ([#165](https://github.com/gofhir/validator/issues/165)) ([5cb0071](https://github.com/gofhir/validator/commit/5cb007124b4867465a2a03ef97f6741ac07574b3))
* **reference:** check references over the definitions, by the element that governs each ([#162](https://github.com/gofhir/validator/issues/162)) ([2f947af](https://github.com/gofhir/validator/commit/2f947af59fab2a60fe30ef0598970551f8a1bd88))
* **reference:** report entries without a fullUrl and relative references they leave without a base ([#163](https://github.com/gofhir/validator/issues/163)) ([74afca6](https://github.com/gofhir/validator/commit/74afca6243c131b2b6e3125b6c45c01067d11cfa))


### Bug Fixes

* **deps:** update fhirpath to v1.16.6 and ucum to v4.3.4, and stop registering htmlChecks() globally ([#184](https://github.com/gofhir/validator/issues/184)) ([0a0f891](https://github.com/gofhir/validator/commit/0a0f8917adf373ff323e95d84f2850daea7ff823)), closes [#176](https://github.com/gofhir/validator/issues/176)
* **extension:** validate what an extension's value holds, and compare its type exactly ([#181](https://github.com/gofhir/validator/issues/181)) ([5c0001c](https://github.com/gofhir/validator/commit/5c0001ce50d9c3b807c8c27f3ee285387bbc64d0))
* match a choice key to its type exactly, from the types of the element's base ([#182](https://github.com/gofhir/validator/issues/182)) ([548ad1f](https://github.com/gofhir/validator/commit/548ad1f61d1d3923ab6ccdf67833bcfb3a82346f)), closes [#180](https://github.com/gofhir/validator/issues/180)
* **registry:** merge a choice element of the differential as one element ([#177](https://github.com/gofhir/validator/issues/177)) ([a127801](https://github.com/gofhir/validator/commit/a12780150511f7eba877018e140e14a2ffb4f8a9))
* report issues in a deterministic order, and check every value[x] of an extension ([#179](https://github.com/gofhir/validator/issues/179)) ([6c0db79](https://github.com/gofhir/validator/commit/6c0db79d9d4819f3fbf2ab4350d41fc88c70340b))
* **terminology:** decide nested ValueSets as a least fixed point, and excludes by the evidence of a code's existence ([#170](https://github.com/gofhir/validator/issues/170)) ([a2ac6ea](https://github.com/gofhir/validator/commit/a2ac6ea6050e636e771245820ac420e6adee7e33))
* **terminology:** decide ValueSet membership by the ValueSet's definition ([#156](https://github.com/gofhir/validator/issues/156)) ([b622f78](https://github.com/gofhir/validator/commit/b622f786a189a63e83c9057229acad1626f3bd53))


### Performance Improvements

* **registry:** extract fixed[x]/pattern[x] once, and read types without a lock ([#173](https://github.com/gofhir/validator/issues/173)) ([7292a80](https://github.com/gofhir/validator/commit/7292a809d6c3353863b2c2d0ed0dc7cdceca65a4))

## [2.1.0](https://github.com/gofhir/validator/compare/v2.0.1...v2.1.0) (2026-10-13)


### Features

* check the extensions of every element, following the definitions ([#152](https://github.com/gofhir/validator/issues/152)) ([27ade0a](https://github.com/gofhir/validator/commit/27ade0acc5f5e0170f1585375a351e2f4ea261f4))
* check the resources other elements hold for cardinality, slicing and references ([#150](https://github.com/gofhir/validator/issues/150)) ([48cd6dc](https://github.com/gofhir/validator/commit/48cd6dcb16da91f936c3e4cf46422145c87ac5f3))
* check the values of every resource another holds, once ([#151](https://github.com/gofhir/validator/issues/151)) ([2e547a7](https://github.com/gofhir/validator/commit/2e547a7ffe59f797784d7117bc4916eec072008b))
* **constraint:** an invariant whose result is empty fails, with the published corrections (plan C, C-2) ([#142](https://github.com/gofhir/validator/issues/142)) ([7c65e72](https://github.com/gofhir/validator/commit/7c65e723bdc4470bea83353916e30d6558e62d63))
* **fixedpattern:** check fixed and pattern values on every definition that governs a value (plan B, B3) ([#138](https://github.com/gofhir/validator/issues/138)) ([5a94c1f](https://github.com/gofhir/validator/commit/5a94c1fd8777b642cf50b36e3760881e061a4178))
* **reference:** check a reference's target against the profiles its element allows (plan B, B5b) ([#144](https://github.com/gofhir/validator/issues/144)) ([e21ed47](https://github.com/gofhir/validator/commit/e21ed4740b138348064b42b90be59b4da1909b7a))
* **structural:** report a value written otherwise than its element says ([#153](https://github.com/gofhir/validator/issues/153)) ([4702665](https://github.com/gofhir/validator/commit/470266597a8bc7f035afd576fb41e3783ef55885))
* **walker:** discover nested resources from the definitions ([#149](https://github.com/gofhir/validator/issues/149)) ([d2b1be5](https://github.com/gofhir/validator/commit/d2b1be5b79283bf5b4b8584f6e3d31af5a0b9b67))


### Bug Fixes

* **binding:** a CodeableConcept with no code provides none to its binding ([#155](https://github.com/gofhir/validator/issues/155)) ([1f5572d](https://github.com/gofhir/validator/commit/1f5572dfee1503f3218c9878e6f1a0d80badbf49))
* **binding:** check a Coding's display in the resource's language, with the concept's designations ([#143](https://github.com/gofhir/validator/issues/143)) ([4491d71](https://github.com/gofhir/validator/commit/4491d719929989637768118e529eeb5e22fd11b4))
* **canonical:** resolve the versioned profiles and targetProfiles of nested resources (plan B, B5) ([#140](https://github.com/gofhir/validator/issues/140)) ([1d510ec](https://github.com/gofhir/validator/commit/1d510ecf35be6d15ec4c96dc6dcbeb76e6ddbec7))
* **constraint:** resolve() a relative reference from the referring entry's base ([#146](https://github.com/gofhir/validator/issues/146)) ([5c6b907](https://github.com/gofhir/validator/commit/5c6b907e5a5e0fba3387f19debeeee7388317b1d))
* **slicing:** slice the resources a resource holds against the profiles they declare ([#145](https://github.com/gofhir/validator/issues/145)) ([edc263d](https://github.com/gofhir/validator/commit/edc263df1a09d08358203138625af5f368b3e42b))
* tell an element that holds a resource by its definition, not by its value ([#147](https://github.com/gofhir/validator/issues/147)) ([0da5c88](https://github.com/gofhir/validator/commit/0da5c8858ecbe0b9eb1e2446a5684fd6a820c5d0))
* **ucum:** check the Quantities in backbone elements ([#148](https://github.com/gofhir/validator/issues/148)) ([27ecc94](https://github.com/gofhir/validator/commit/27ecc94e5a231ed33e412cfa3a403a351a0bdcfa))
* validate the resource as HL7's parser leaves it, without its misshapen values ([#154](https://github.com/gofhir/validator/issues/154)) ([bb8a16f](https://github.com/gofhir/validator/commit/bb8a16fecf1d5bd325b2b3bc9ffb5498c3664932))

## [2.0.1](https://github.com/gofhir/validator/compare/v2.0.0...v2.0.1) (2026-10-06)


### Bug Fixes

* **constraint:** read a profile's invariant's value as the JSON writes it, and resolve() in the innermost Bundle (plan C, C-1) ([#134](https://github.com/gofhir/validator/issues/134)) ([b32b9a2](https://github.com/gofhir/validator/commit/b32b9a2c3f20885681ea13f68082d8a37da626e3))
* **module:** the module path ends in /v2, so v2 can be installed ([#137](https://github.com/gofhir/validator/issues/137)) ([534d154](https://github.com/gofhir/validator/commit/534d15484f4cde96a11eb88258970b606584d255))

## [2.0.0](https://github.com/gofhir/validator/compare/v1.28.0...v2.0.0) (2026-10-06)


### ⚠ BREAKING CHANGES

* **extension:** registry.Registry.IsCanonicalResource and IsMetadataResource follow Interfaces: the interfaces a definition declares (R5), or in R4 and R4B the canonical resources references.html lists, and no MetadataResource, as in the HL7 validator. In R4, IsMetadataResource was true for ValueSet and other knowledge resources and IsCanonicalResource for Device and Contract; both are false now.

### Features

* **extension:** check an extension's context of use from the definitions, fhirpath contexts by place (plan B, B4b) ([#133](https://github.com/gofhir/validator/issues/133)) ([492a1a7](https://github.com/gofhir/validator/commit/492a1a749b45a158e2816c607b30a1bf61501303))


### Bug Fixes

* **extension:** a sub-extension the definition does not declare is an error; one defined separately is validated (plan B, B4) ([#129](https://github.com/gofhir/validator/issues/129)) ([9001b81](https://github.com/gofhir/validator/commit/9001b8125a0c660969badae10b334dda61665ff9))
* **registry:** generate snapshots by element id, with slices and unrolled children as HL7 does (plan B, B7) ([#131](https://github.com/gofhir/validator/issues/131)) ([e10a53e](https://github.com/gofhir/validator/commit/e10a53e25a1e227ccf4be6bbc994f0001d98cb92))

## [1.28.0](https://github.com/gofhir/validator/compare/v1.27.0...v1.28.0) (2026-10-04)


### Features

* **cardinality,slicing:** check an extension's structure against the definition its url names (plan B, PR B4c) ([#125](https://github.com/gofhir/validator/issues/125)) ([ebd808d](https://github.com/gofhir/validator/commit/ebd808d9676f869a463f926a2553b3aaef4326ef))
* **constraint:** check an extension against the invariants of the definition its url names (plan B, PR B4a) ([#123](https://github.com/gofhir/validator/issues/123)) ([7eed079](https://github.com/gofhir/validator/commit/7eed0790b1c81725417150187e44baaf3d23cf49))
* **validator,registry:** load a guide's dependencies and resolve canonicals across the versions loaded (plan B, B-D6) ([#126](https://github.com/gofhir/validator/issues/126)) ([bbf0d4f](https://github.com/gofhir/validator/commit/bbf0d4f32f914aebfe4bd8b6399fe9f9c011db8b))


### Performance Improvements

* **loader,terminology:** read a package's value sets and code systems when needed ([#127](https://github.com/gofhir/validator/issues/127)) ([b8c3ba1](https://github.com/gofhir/validator/commit/b8c3ba1994791230f1d033b3d1cea7a6d2f756dd))

## [1.27.0](https://github.com/gofhir/validator/compare/v1.26.2...v1.27.0) (2026-10-04)


### Features

* **cardinality,slicing:** follow the profile a type declares (plan B, PR B1a) ([#117](https://github.com/gofhir/validator/issues/117)) ([27fef75](https://github.com/gofhir/validator/commit/27fef750f76dcc1aa161f5157101115ca73c9635))
* **constraint:** check a value against the slice it belongs to (plan B, PR B2b) ([#122](https://github.com/gofhir/validator/issues/122)) ([65db7ec](https://github.com/gofhir/validator/commit/65db7ec0b3f033029c7589ed2462eaa1c9e11696))
* **constraint:** walk the element tree, nested resources included (plan B, PR B2a) ([#121](https://github.com/gofhir/validator/issues/121)) ([8b6f2e9](https://github.com/gofhir/validator/commit/8b6f2e9809a7d49ff78343ca59a83ab5ede633db))


### Bug Fixes

* **location:** locate every issue in one reading of the source, and locate it right ([#120](https://github.com/gofhir/validator/issues/120)) ([3bb8544](https://github.com/gofhir/validator/commit/3bb8544e775a9e28aefb1ea8e6358271d25f4a82))

## [1.26.2](https://github.com/gofhir/validator/compare/v1.26.1...v1.26.2) (2026-10-02)


### Bug Fixes

* **registry:** correct the id types the published definitions get wrong ([#114](https://github.com/gofhir/validator/issues/114)) ([c80ad0c](https://github.com/gofhir/validator/commit/c80ad0ca891ebcd26e66bff6f1de45a01b07c349))


### Performance Improvements

* **constraint:** fhirpath v1.9.8, and cache the resource constraints share ([#116](https://github.com/gofhir/validator/issues/116)) ([ac63ab1](https://github.com/gofhir/validator/commit/ac63ab156866b92b6e438d93f078ddccc9b7d1ea))

## [1.26.1](https://github.com/gofhir/validator/compare/v1.26.0...v1.26.1) (2026-10-02)


### Bug Fixes

* **cli:** -tx takes its value, as in the HL7 validator ([#111](https://github.com/gofhir/validator/issues/111)) ([150a428](https://github.com/gofhir/validator/commit/150a4289ecb02fd5d617b15f4a2a4ff1d003f6aa))
* **constraint:** an expression that does not compile is an error ([#112](https://github.com/gofhir/validator/issues/112)) ([3ad7975](https://github.com/gofhir/validator/commit/3ad7975e49d7db12f869fbcaadac403d6aa2eaff))

## [1.26.0](https://github.com/gofhir/validator/compare/v1.25.1...v1.26.0) (2026-10-02)


### Features

* **constraint:** evaluate FHIRPath with a model from the registry (plan B, PR B8) ([#107](https://github.com/gofhir/validator/issues/107)) ([dbc6bff](https://github.com/gofhir/validator/commit/dbc6bff1da54e4a8bff29c6897a1b45489edbb89))


### Bug Fixes

* **constraint:** a constraint on a primitive is evaluated on its value ([#106](https://github.com/gofhir/validator/issues/106)) ([3e9e207](https://github.com/gofhir/validator/commit/3e9e20735403b8c5cd298fdc411727b2981d10c4))
* **constraint:** a primitive focus is read as its FHIR type ([#109](https://github.com/gofhir/validator/issues/109)) ([2020228](https://github.com/gofhir/validator/commit/20202289af8234be8cdcd88c830c8588866a4066))
* **constraint:** an invariant that cannot be evaluated fails ([#110](https://github.com/gofhir/validator/issues/110)) ([ecd0bdf](https://github.com/gofhir/validator/commit/ecd0bdf64ea8f7f71bd40c1eb978571032b890b8))

## [1.25.1](https://github.com/gofhir/validator/compare/v1.25.0...v1.25.1) (2026-10-01)


### Bug Fixes

* **deps:** fhirpath v1.9.5, plan A release notes and the note to the server (A5) ([#97](https://github.com/gofhir/validator/issues/97)) ([4eac0ca](https://github.com/gofhir/validator/commit/4eac0cafe48078b5fb03aba204b3b4180431c32b))

## [1.25.0](https://github.com/gofhir/validator/compare/v1.24.0...v1.25.0) (2026-10-01)


### Features

* **slicing:** slicing on the element tree, per parent instance (A4) + fhirpath v1.9.2 ([#96](https://github.com/gofhir/validator/issues/96)) ([b2e9cc6](https://github.com/gofhir/validator/commit/b2e9cc639969bc43897d6fe3e56a57f19d7d249a))

## [1.24.0](https://github.com/gofhir/validator/compare/v1.23.0...v1.24.0) (2026-10-01)


### Features

* **cardinality:** cardinality on the element tree (A3) + A2 review fixes ([#95](https://github.com/gofhir/validator/issues/95)) ([764a456](https://github.com/gofhir/validator/commit/764a4569156cb84875e14e6627fdea14b0f7b65c))

## [1.23.0](https://github.com/gofhir/validator/compare/v1.22.0...v1.23.0) (2026-10-01)


### Features

* **slicematch:** conformant slice matcher on the element tree (A2) ([#94](https://github.com/gofhir/validator/issues/94)) ([d85fc3d](https://github.com/gofhir/validator/commit/d85fc3df2e1c93b11e2562618213c4e4e0707157))

## [1.22.0](https://github.com/gofhir/validator/compare/v1.21.1...v1.22.0) (2026-09-30)


### Features

* **registry:** element tree by id, exact canonical resolution, jsoncompare (A1) ([#93](https://github.com/gofhir/validator/issues/93)) ([9ac6e84](https://github.com/gofhir/validator/commit/9ac6e846fd5cbfeb434ec88ea324a95978e5db73))

## [1.21.1](https://github.com/gofhir/validator/compare/v1.21.0...v1.21.1) (2026-09-23)


### Bug Fixes

* **extension:** una URL de extensión `urn:` pasaba el control de formato, que la spec prohíbe ([#89](https://github.com/gofhir/validator/issues/89)) ([300d1c6](https://github.com/gofhir/validator/commit/300d1c6068b164d0d7513c04b3816e6c7854db0e))

## [1.21.0](https://github.com/gofhir/validator/compare/v1.20.0...v1.21.0) (2026-08-03)


### Features

* **deps:** fhirpath v1.6.0 — substring panic fixed, R5 auditable for the first time ([#86](https://github.com/gofhir/validator/issues/86)) ([6738ef5](https://github.com/gofhir/validator/commit/6738ef531f9464a6aaed476dd462fefce745c281))

## [1.20.0](https://github.com/gofhir/validator/compare/v1.19.0...v1.20.0) (2026-08-03)


### Features

* **deps:** bump gofhir/fhirpath to v1.4.0, activating ref-1 ([#82](https://github.com/gofhir/validator/issues/82)) ([f5321ad](https://github.com/gofhir/validator/commit/f5321ad39b44195d381e51da5bacbe68149a055c))
* **deps:** fhirpath v1.5.1 and ucum v4, closing three of four engine gaps ([#84](https://github.com/gofhir/validator/issues/84)) ([5bfe542](https://github.com/gofhir/validator/commit/5bfe542e423e0309c896df93b7575d207a21d503))

## [1.19.0](https://github.com/gofhir/validator/compare/v1.18.0...v1.19.0) (2026-07-31)


### Features

* **ucum:** report an invalid UCUM code as an error, not a warning ([#78](https://github.com/gofhir/validator/issues/78)) ([caea909](https://github.com/gofhir/validator/commit/caea9095425c68c48dbe6d2cd90e02c6dd9599cd))


### Bug Fixes

* **contained:** resolve a fragment deterministically and report a duplicate id ([#79](https://github.com/gofhir/validator/issues/79)) ([72770cf](https://github.com/gofhir/validator/commit/72770cf0360b49b1796810d01f31662b93b15b2f))

## [1.18.0](https://github.com/gofhir/validator/compare/v1.17.0...v1.18.0) (2026-07-31)


### Features

* **constraint:** evaluate type constraints on every type a choice element declares ([#74](https://github.com/gofhir/validator/issues/74)) ([346d8e7](https://github.com/gofhir/validator/commit/346d8e7db35245b8b8b603ffa552aa1c9b8780f9))

## [1.17.0](https://github.com/gofhir/validator/compare/v1.16.1...v1.17.0) (2026-07-31)


### Features

* **binding:** check a Coding against its CodeSystem regardless of binding strength ([#70](https://github.com/gofhir/validator/issues/70)) ([ccdc996](https://github.com/gofhir/validator/commit/ccdc996b65ebe2386501486e116ddfc82a45934b))
* **binding:** report a Coding that is missing its system or its code ([#72](https://github.com/gofhir/validator/issues/72)) ([f5731aa](https://github.com/gofhir/validator/commit/f5731aa0d28b392b1ca1756b1bd5c6eed729ecd5))

## [1.16.1](https://github.com/gofhir/validator/compare/v1.16.0...v1.16.1) (2026-07-30)


### Bug Fixes

* **terminology:** include abstract concepts in an is-a expansion ([#67](https://github.com/gofhir/validator/issues/67)) ([c4a871a](https://github.com/gofhir/validator/commit/c4a871a8ade91544f73f85d0c1cb5aaa800f5744))

## [1.16.0](https://github.com/gofhir/validator/compare/v1.15.0...v1.16.0) (2026-07-30)


### Features

* unify extension binding validation, consume Supports and CodeResult.Message ([#64](https://github.com/gofhir/validator/issues/64)) ([a823061](https://github.com/gofhir/validator/commit/a823061f8cdeb4240dd4e654b905e33e8db29114))

## [1.15.0](https://github.com/gofhir/validator/compare/v1.14.1...v1.15.0) (2026-07-30)


### Features

* **terminology:** Authority port, provider-first delegation, and conformance fixes ([#62](https://github.com/gofhir/validator/issues/62)) ([62472a8](https://github.com/gofhir/validator/commit/62472a8809c1bf687f6ff0240652fc7b260ab71a))

## [1.14.1](https://github.com/gofhir/validator/compare/v1.14.0...v1.14.1) (2026-06-29)


### Bug Fixes

* **structural:** deterministic choice-type exclusivity ([#60](https://github.com/gofhir/validator/issues/60)) ([53bd0b0](https://github.com/gofhir/validator/commit/53bd0b09f87e6e3b885427e13548bd678b638928))

## [1.14.0](https://github.com/gofhir/validator/compare/v1.13.3...v1.14.0) (2026-04-26)


### Features

* **validator:** WithConformancePackage preserves IG package metadata ([c91b51c](https://github.com/gofhir/validator/commit/c91b51cfa8452700687d372dfa0fa6c5c921a165))


### Bug Fixes

* **registry:** serialize EnsureSnapshot to eliminate data race ([f5d2bbc](https://github.com/gofhir/validator/commit/f5d2bbc24469e0a65d8729526f6a2f736f580200))
* **slicing:** value discriminator follows type.profile chain to referenced SD ([2dd72ed](https://github.com/gofhir/validator/commit/2dd72edf8ba5258d4305ca578edd4d2ee44f4ca6))

## [1.13.3](https://github.com/gofhir/validator/compare/v1.13.2...v1.13.3) (2026-04-19)


### Bug Fixes

* extension validator uses ResolveByCanonical for ProfileResolver fallback ([a1f9c1f](https://github.com/gofhir/validator/commit/a1f9c1f7c582e70c893e3cb36ca672b452381a1b)), closes [#55](https://github.com/gofhir/validator/issues/55)

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
