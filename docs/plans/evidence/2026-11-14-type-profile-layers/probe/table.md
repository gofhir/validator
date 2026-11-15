| Probe | What it tests | HL7 R4 (4.0.1) | R4B (4.3.0) | R5 (5.0.0) |
|---|---|---|---|---|
| p1 | OR Simple/Money, meets Simple | nothing | same | same |
| p2 | OR, meets Money | nothing | same | same |
| p3 | OR, meets none (comparator <, UCUM mg) | NoMatch, Maximum comparator from SimpleQuantity, sqty-1, mqty-1 | same | NoMatch, Maximum comparator from SimpleQuantity, sqty-1, mtqy-1 |
| p4 | only SimpleQuantity, comparator < | sqty-1, Maximum comparator from SimpleQuantity | same | same |
| p5 | OR, unit without system, none | NoMatch, Maximum comparator from SimpleQuantity, sqty-1, mqty-1 | same | NoMatch, Maximum comparator from SimpleQuantity, sqty-1, mtqy-1 |
| p6 | unknown + Simple, fails Simple | Unknown_Profile, NoMatch, Maximum comparator from SimpleQuantity, sqty-1 | same | same |
| p7 | unknown + Simple, meets Simple | Unknown_Profile | same | same |
| p8 | two unknown profiles | Unknown_Profile, Unknown_Profile, NoMatch | same | same |
| p9 | component.value[x] repeated, one fails | NoMatch, Maximum comparator from SimpleQuantity, sqty-1, mqty-1 | same | NoMatch, Maximum comparator from SimpleQuantity, sqty-1, mtqy-1 |
| p10 | extension value[x] with OR | NoMatch, Maximum comparator from SimpleQuantity, sqty-1, mqty-1 | same | NoMatch, Maximum comparator from SimpleQuantity, sqty-1, mtqy-1 |
| p11 | choice type, valueString | nothing | same | same |
| p12 | choice type, valueQuantity meets none | Maximum comparator from SimpleQuantity, sqty-1 | same | same |
| p13 | OR, meets both | MultipleMatches | same | same |
| p14 | string profiles, meets both | MultipleMatches | same | same |
| p15 | string, none | NoMatch, short-1, upper-1 | same | same |
| p16 | string, only upper | nothing | same | same |
| p17 | versioned canonicals (|ver adapted), none | NoMatch, mqty-1, Maximum comparator from SimpleQuantity, sqty-1 | same | NoMatch, Maximum comparator from SimpleQuantity, sqty-1, mtqy-1 |
| p18 | Simple or sq-unit, only Simple | nothing | same | same |
| p19 | Simple or sq-unit, both | MultipleMatches | same | same |
| p20 | Simple or CodeableConcept profile (other type) | NoMatch, Maximum comparator from SimpleQuantity, sqty-1, NotAllowed, NotAllowed, NotAllowed, NotAllowed, NotAllowed, Minimum text from cc-prof | same | same |
| p21 | Simple or sq-unit, common violation | NoMatch, Maximum comparator from SimpleQuantity, sqty-1[x2], Maximum comparator from sq-unit, Minimum unit from sq-unit | same | same |
| a1n | no unit (fails qa), value 5 (fails inv2-1): neither; inv2-1 only in qi2 | NoMatch, Minimum unit from qa, inv2-1 | same | same |
| a1p | unit present, value 5: only qa conforms | nothing | same | same |
| a1q | no unit (fails qa), value 20: only qi2 conforms | nothing | same | same |
| a1rn | no unit (fails qa), value 5 (fails inv2-1): neither; inv2-1 only in qi2 | NoMatch, Minimum unit from qa, inv2-1 | same | same |
| a1rp | unit present, value 5: only qa conforms | nothing | same | same |
| a1rq | no unit (fails qa), value 20: only qi2 conforms | nothing | same | same |
| c1a | slice s1 min value matches 2nd layer (idb) only; value matches idb, not ida (period) | nothing | same | same |
| c1b | matches ida only; slice requires value (matches idb layer only) | Minimum value from obs-c1 | same | same |
| c1c | matches neither layer; slice requires value | Minimum value from obs-c1, NoMatch, Minimum value from idb, Minimum period from ida | same | same |
| c1da | slice s1 min value matches 2nd layer (idb) only; value matches idb, not ida (period) | nothing | same | same |
| c1db | matches ida only; slice requires value (matches idb layer only) | Minimum value from obs-c1d | same | same |
| c1dc | matches neither layer; slice requires value | NoMatch, Minimum value from idb, Minimum period from ida, Minimum value from obs-c1d | same | same |
| c1dn | outside the slice: node layers only (idb ok) | nothing | same | same |
| c1n | outside the slice: node layers only (idb ok) | nothing | same | same |
| ct1c | IG/contained canonical: code present, no unit (qb ok) | Unknown_Profile | same | same |
| ct1n | IG/contained canonical in type.profile: no unit, no code (fails qa/qb/cq) | Unknown_Profile | same | same |
| ct1u | IG/contained canonical: unit present, no code (qa/cq ok, qb fails) | Unknown_Profile | same | same |
| ct2c | IG/contained canonical: code present, no unit (qb ok) | Unknown_Profile | same | same |
| ct2n | IG/contained canonical in type.profile: no unit, no code (fails qa/qb/cq) | Unknown_Profile, NoMatch, Minimum code from qb | same | same |
| ct2u | IG/contained canonical: unit present, no code (qa/cq ok, qb fails) | Unknown_Profile, NoMatch, Minimum code from qb | same | same |
| d4a | two unresolved profiles, invalid base content (unknown element, wrong type) | Error parsing JSON: the primit, Unrecognized property 'bogus', Unknown_Profile, Unknown_Profile, NoMatch | same | same |
| d4b | two unresolved profiles, valid base content | Unknown_Profile, Unknown_Profile, NoMatch | same | same |
| d4c | one unresolved + qa, invalid base content | Error parsing JSON: the primit, Unrecognized property 'bogus', Unknown_Profile | same | same |
| dup | dup-1 and unit min fail in both layers: ['qdup-a', 'qdup-b'] | NoMatch, Minimum unit from qdup-b, dup-1[x2], Minimum unit from qdup-a | same | same |
| e2hnrempty | e2 resolve(): same, both Observations in a Bundle nested in a Bundle (ids-rempty) | NotSlice | same | same |
| e2hnrexists | e2 resolve(): same, both Observations in a Bundle nested in a Bundle (ids-rexists) | nothing | same | same |
| e2hnrid1 | e2 resolve(): same, both Observations in a Bundle nested in a Bundle (ids-rid1) | NotSlice | same | same |
| e2hnrid2 | e2 resolve(): same, both Observations in a Bundle nested in a Bundle (ids-rid2) | NotSlice | same | same |
| e2hnrrid0 | e2 resolve(): same, both Observations in a Bundle nested in a Bundle (ids-rrid0) | NotSlice | same | same |
| e2hnrrid1 | e2 resolve(): same, both Observations in a Bundle nested in a Bundle (ids-rrid1) | NotSlice | same | same |
| e2hnrro1 | e2 resolve(): same, both Observations in a Bundle nested in a Bundle (ids-rro1) | NotSlice | same | same |
| e2hnrrval | e2 resolve(): same, both Observations in a Bundle nested in a Bundle (ids-rrval) | nothing | same | same |
| e2hnrval | e2 resolve(): same, both Observations in a Bundle nested in a Bundle (ids-rval) | nothing | same | same |
| e2hrempty | e2 resolve(): hasMember discriminated by profile of resolve().identifier (ids-rempty); target in the same Bundle | NotSlice | same | same |
| e2hrexists | e2 resolve(): hasMember discriminated by profile of resolve().identifier (ids-rexists); target in the same Bundle | nothing | same | same |
| e2hrid1 | e2 resolve(): hasMember discriminated by profile of resolve().identifier (ids-rid1); target in the same Bundle | NotSlice | same | same |
| e2hrid2 | e2 resolve(): hasMember discriminated by profile of resolve().identifier (ids-rid2); target in the same Bundle | NotSlice | same | same |
| e2hrrid0 | e2 resolve(): hasMember discriminated by profile of resolve().identifier (ids-rrid0); target in the same Bundle | NotSlice | same | same |
| e2hrrid1 | e2 resolve(): hasMember discriminated by profile of resolve().identifier (ids-rrid1); target in the same Bundle | NotSlice | same | same |
| e2hrro1 | e2 resolve(): hasMember discriminated by profile of resolve().identifier (ids-rro1); target in the same Bundle | NotSlice | same | same |
| e2hrrval | e2 resolve(): hasMember discriminated by profile of resolve().identifier (ids-rrval); target in the same Bundle | nothing | same | same |
| e2hrval | e2 resolve(): hasMember discriminated by profile of resolve().identifier (ids-rval); target in the same Bundle | nothing | same | same |
| e2sremptyb | e2 profile-discriminator: Identifier profile ids-rempty, Observation in a Bundle | NotSlice | same | same |
| e2sremptyn | e2 profile-discriminator: Identifier profile ids-rempty, Observation in a Bundle in a Bundle | NotSlice | same | same |
| e2sremptyx | e2 profile-discriminator: Identifier profile ids-rempty, standalone Observation | NotSlice | same | same |
| e2srexistsb | e2 profile-discriminator: Identifier profile ids-rexists, Observation in a Bundle | nothing | same | same |
| e2srexistsn | e2 profile-discriminator: Identifier profile ids-rexists, Observation in a Bundle in a Bundle | nothing | same | same |
| e2srexistsx | e2 profile-discriminator: Identifier profile ids-rexists, standalone Observation | nothing | same | same |
| e2srid1b | e2 profile-discriminator: Identifier profile ids-rid1, Observation in a Bundle | NotSlice | same | same |
| e2srid1n | e2 profile-discriminator: Identifier profile ids-rid1, Observation in a Bundle in a Bundle | NotSlice | same | same |
| e2srid1x | e2 profile-discriminator: Identifier profile ids-rid1, standalone Observation | NotSlice | same | same |
| e2srid2b | e2 profile-discriminator: Identifier profile ids-rid2, Observation in a Bundle | NotSlice | same | same |
| e2srid2n | e2 profile-discriminator: Identifier profile ids-rid2, Observation in a Bundle in a Bundle | NotSlice | same | same |
| e2srid2x | e2 profile-discriminator: Identifier profile ids-rid2, standalone Observation | NotSlice | same | same |
| e2srrid0b | e2 profile-discriminator: Identifier profile ids-rrid0, Observation in a Bundle | NotSlice | same | same |
| e2srrid0n | e2 profile-discriminator: Identifier profile ids-rrid0, Observation in a Bundle in a Bundle | NotSlice | same | same |
| e2srrid0x | e2 profile-discriminator: Identifier profile ids-rrid0, standalone Observation | NotSlice | same | same |
| e2srrid1b | e2 profile-discriminator: Identifier profile ids-rrid1, Observation in a Bundle | NotSlice | same | same |
| e2srrid1n | e2 profile-discriminator: Identifier profile ids-rrid1, Observation in a Bundle in a Bundle | NotSlice | same | same |
| e2srrid1x | e2 profile-discriminator: Identifier profile ids-rrid1, standalone Observation | NotSlice | same | same |
| e2srro1b | e2 profile-discriminator: Identifier profile ids-rro1, Observation in a Bundle | NotSlice | same | same |
| e2srro1n | e2 profile-discriminator: Identifier profile ids-rro1, Observation in a Bundle in a Bundle | NotSlice | same | same |
| e2srro1x | e2 profile-discriminator: Identifier profile ids-rro1, standalone Observation | NotSlice | same | same |
| e2srrvalb | e2 profile-discriminator: Identifier profile ids-rrval, Observation in a Bundle | nothing | same | same |
| e2srrvaln | e2 profile-discriminator: Identifier profile ids-rrval, Observation in a Bundle in a Bundle | nothing | same | same |
| e2srrvalx | e2 profile-discriminator: Identifier profile ids-rrval, standalone Observation | nothing | same | same |
| e2srvalb | e2 profile-discriminator: Identifier profile ids-rval, Observation in a Bundle | nothing | same | same |
| e2srvaln | e2 profile-discriminator: Identifier profile ids-rval, Observation in a Bundle in a Bundle | nothing | same | same |
| e2srvalx | e2 profile-discriminator: Identifier profile ids-rval, standalone Observation | nothing | same | same |
| e2tremptyb | e2 plain type.profile: Identifier profile ids-rempty, Observation in a Bundle | e2-rempty | same | same |
| e2tremptyn | e2 plain type.profile: Identifier profile ids-rempty, Observation in a Bundle in a Bundle | e2-rempty | same | same |
| e2tremptyx | e2 plain type.profile: Identifier profile ids-rempty, standalone Observation | e2-rempty | same | same |
| e2trexistsb | e2 plain type.profile: Identifier profile ids-rexists, Observation in a Bundle | nothing | same | same |
| e2trexistsn | e2 plain type.profile: Identifier profile ids-rexists, Observation in a Bundle in a Bundle | nothing | same | same |
| e2trexistsx | e2 plain type.profile: Identifier profile ids-rexists, standalone Observation | nothing | same | same |
| e2trid1b | e2 plain type.profile: Identifier profile ids-rid1, Observation in a Bundle | nothing | same | same |
| e2trid1n | e2 plain type.profile: Identifier profile ids-rid1, Observation in a Bundle in a Bundle | nothing | same | same |
| e2trid1x | e2 plain type.profile: Identifier profile ids-rid1, standalone Observation | nothing | same | same |
| e2trid2b | e2 plain type.profile: Identifier profile ids-rid2, Observation in a Bundle | e2-rid2 | same | same |
| e2trid2n | e2 plain type.profile: Identifier profile ids-rid2, Observation in a Bundle in a Bundle | e2-rid2 | same | same |
| e2trid2x | e2 plain type.profile: Identifier profile ids-rid2, standalone Observation | e2-rid2 | same | same |
| e2trrid0b | e2 plain type.profile: Identifier profile ids-rrid0, Observation in a Bundle | e2-rrid0 | same | same |
| e2trrid0n | e2 plain type.profile: Identifier profile ids-rrid0, Observation in a Bundle in a Bundle | e2-rrid0 | same | same |
| e2trrid0x | e2 plain type.profile: Identifier profile ids-rrid0, standalone Observation | e2-rrid0 | same | same |
| e2trrid1b | e2 plain type.profile: Identifier profile ids-rrid1, Observation in a Bundle | e2-rrid1 | same | same |
| e2trrid1n | e2 plain type.profile: Identifier profile ids-rrid1, Observation in a Bundle in a Bundle | e2-rrid1 | same | same |
| e2trrid1x | e2 plain type.profile: Identifier profile ids-rrid1, standalone Observation | e2-rrid1 | same | same |
| e2trro1b | e2 plain type.profile: Identifier profile ids-rro1, Observation in a Bundle | nothing | same | same |
| e2trro1n | e2 plain type.profile: Identifier profile ids-rro1, Observation in a Bundle in a Bundle | nothing | same | same |
| e2trro1x | e2 plain type.profile: Identifier profile ids-rro1, standalone Observation | nothing | same | same |
| e2trrvalb | e2 plain type.profile: Identifier profile ids-rrval, Observation in a Bundle | e2-rrval | same | same |
| e2trrvaln | e2 plain type.profile: Identifier profile ids-rrval, Observation in a Bundle in a Bundle | e2-rrval | same | same |
| e2trrvalx | e2 plain type.profile: Identifier profile ids-rrval, standalone Observation | e2-rrval | same | same |
| e2trvalb | e2 plain type.profile: Identifier profile ids-rval, Observation in a Bundle | e2-rval | same | same |
| e2trvaln | e2 plain type.profile: Identifier profile ids-rval, Observation in a Bundle in a Bundle | e2-rval | same | same |
| e2trvalx | e2 plain type.profile: Identifier profile ids-rval, standalone Observation | e2-rval | same | same |
| ele1 | Quantity with only an id: ele-1 fails, inherited by both layers (qa, qb also fail by cardinality) | ele-1[x2], NoMatch, Minimum code from qb, Minimum unit from qa | ele-1, ele-1[x2], NoMatch, Minimum code from qb, Minimum unit from qa | same |
| ele2 | Quantity id+value, no unit/code: ele-1 ok; NoMatch only | NoMatch, Minimum code from qb, Minimum unit from qa | same | same |
| ig1c | IG/contained canonical: code present, no unit (qb ok) | Unknown_Profile | same | same |
| ig1n | IG/contained canonical in type.profile: no unit, no code (fails qa/qb/cq) | Unknown_Profile | same | same |
| ig1u | IG/contained canonical: unit present, no code (qa/cq ok, qb fails) | Unknown_Profile | same | same |
| ig2c | IG/contained canonical: code present, no unit (qb ok) | Unknown_Profile | same | same |
| ig2n | IG/contained canonical in type.profile: no unit, no code (fails qa/qb/cq) | Unknown_Profile, NoMatch, Minimum code from qb | same | same |
| ig2u | IG/contained canonical: unit present, no code (qa/cq ok, qb fails) | Unknown_Profile, NoMatch, Minimum code from qb | same | same |
| ig3c | IG/contained canonical: code present, no unit (qb ok) | Unknown_Profile | same | same |
| ig3n | IG/contained canonical in type.profile: no unit, no code (fails qa/qb/cq) | Unknown_Profile, NoMatch, Minimum code from qb | same | same |
| ig3u | IG/contained canonical: unit present, no code (qa/cq ok, qb fails) | Unknown_Profile, NoMatch, Minimum code from qb | same | same |
| leafa | slice leaf [ida,idb]: matches idb only | nothing | same | same |
| leafb | slice leaf: matches ida only | nothing | same | same |
| leafc | slice leaf: neither | NoMatch, Minimum value from idb, Minimum period from ida | same | same |
| pd1a | profile discriminator: ida only | nothing | same | same |
| pd1ab | profile discriminator: both | MultipleMatches | same | same |
| pd1b | profile discriminator: idb only | nothing | same | same |
| pd1n | profile discriminator: neither (closed) | NotSlice | same | same |
| pd2a | profile discriminator: ida only | nothing | same | same |
| pd2ab | profile discriminator: both | MultipleMatches | same | same |
| pd2b | profile discriminator: idb only | nothing | same | same |
| pd2n | profile discriminator: neither (closed) | NotSlice | same | same |
| rec1a | recursive: top and inner match idr-a | nothing | same | same |
| rec1b | recursive: top matches idr-b only (idr-b has no assigner rule) | nothing | same | same |
| rec1c | recursive: inner matches neither, so top fails idr-a; idr-b needs period: neither | NoMatch, Minimum period from idr-b, NoMatch, Minimum period from idr-b[x2], Minimum value from idr-a[x2] | same | same |
| rec1d | recursive: top matches idr-b; idr-a fails by inner | nothing | same | same |
| rec2a | recursive: top and inner match idr-a | nothing | same | same |
| rec2b | recursive: top matches idr-b only (idr-b has no assigner rule) | nothing | same | same |
| rec2c | recursive: inner matches neither, so top fails idr-a; idr-b needs period: neither | NoMatch, Minimum period from idr-b, NoMatch, Minimum period from idr-b[x2], Minimum value from idr-a[x2] | same | same |
| rec2d | recursive: top matches idr-b; idr-a fails by inner | nothing | same | same |
| ref0n | control, single profile idh-a: target does not satisfy org-x | Reference_REF_CantMatchChoice | same | same |
| ref0x | control, single profile idh-a: target satisfies org-x | nothing | same | same |
| ref1n | reference: target satisfies neither (D2: failure exclusive to each group) | NoMatch, Reference_REF_CantMatchChoice, Reference_REF_CantMatchChoice | same | same |
| ref1x | reference: contained Organization satisfies org-x only (active) | nothing | same | same |
| ref1y | reference: contained Organization satisfies org-y only (contact) | nothing | same | same |
| ref2n | reference: target satisfies neither (D2: failure exclusive to each group) | NoMatch, Reference_REF_CantMatchChoice, Reference_REF_CantMatchChoice | same | same |
| ref2x | reference: contained Organization satisfies org-x only (active) | nothing | same | same |
| ref2y | reference: contained Organization satisfies org-y only (contact) | nothing | same | same |
| refb1n | held in a Bundle entry: satisfies neither | NoMatch, Reference_REF_CantMatchChoice, Reference_REF_CantMatchChoice | same | same |
| refb1x | held in a Bundle entry: Organization satisfies org-x only | nothing | same | same |
| refb1y | held in a Bundle entry: satisfies org-y only | nothing | same | same |
| refb2n | held in a Bundle entry: satisfies neither | NoMatch, Reference_REF_CantMatchChoice, Reference_REF_CantMatchChoice | same | same |
| refb2x | held in a Bundle entry: Organization satisfies org-x only | nothing | same | same |
| refb2y | held in a Bundle entry: satisfies org-y only | nothing | same | same |
| res1 | top final + contained amended (contained has its own meta.profile); no unit: qres-a fails by unit, qres-b by status | NoMatch, rb-1, Minimum unit from qres-a | same | same |
| res2 | top amended: conforms qres-b only | nothing | same | same |
| res3 | top final with unit: conforms qres-a only | nothing | same | same |
| res4 | Bundle: entry final (no unit) and entry amended (no unit); same datatype value under two %resource | NoMatch, rb-1, Minimum unit from qres-a | same | same |
| res5 | top amended + contained final (no unit): top conforms qres-b; contained fails both unless HL7 uses the contained resource as %resource | NoMatch, rb-1, Minimum unit from qres-a | same | same |
| resla | reslice s3/ra [ida,idb]: matches idb only | nothing | same | same |
| reslb | reslice: matches ida only | nothing | same | same |
| reslc | reslice: neither | NoMatch, Minimum value from idb, Minimum period from ida | same | same |
| sev1 | dup-1 and unit min fail in both layers: ['qsev-a', 'qsev-b'] | NoMatch, Minimum unit from qsev-a, dup-1, Minimum unit from qsev-b, dup-1(w) | same | same |
| sev2 | dup-1 and unit min fail in both layers: ['qsev-b', 'qsev-a'] | NoMatch, Minimum unit from qsev-a, dup-1, Minimum unit from qsev-b, dup-1(w) | same | same |
| two | same two profiles on two nodes, both values fail both | NoMatch, Minimum code from qb, Minimum unit from qa, NoMatch, Minimum code from qb, Minimum unit from qa | same | same |
| twom | same unresolved canonical on two nodes (Unknown_Profile per node?) | Unknown_Profile, NoMatch, Minimum unit from qa, Unknown_Profile, NoMatch, Minimum unit from qa | same | same |
| twom1 | unresolved canonical on two nodes, only the first value fails | Unknown_Profile, NoMatch, Minimum unit from qa, Unknown_Profile | same | same |
| txt1 | dup-1 and unit min fail in both layers: ['qtxt-a', 'qtxt-b'] | NoMatch, Minimum unit from qtxt-a, dup-1[x2], Minimum unit from qtxt-b | same | same |
| txt2 | dup-1 and unit min fail in both layers: ['qtxt-b', 'qtxt-a'] | NoMatch, Minimum unit from qtxt-a, dup-1[x2], Minimum unit from qtxt-b | same | same |
