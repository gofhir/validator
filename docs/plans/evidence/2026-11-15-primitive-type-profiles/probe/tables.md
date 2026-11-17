
## Caso 1: un solo perfil

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c1-len-ok-obs` | one profile ps-len on Observation.valueString='ab' | nada | igual |
| `c1-len-ok-giv` | one profile ps-len on Patient.name.given[0]='ab' | nada | igual |
| `c1-len-ok-ext` | one profile ps-len on Extension.valueString='ab' | nada | igual |
| `c1-len-bad-obs` | one profile ps-len on Observation.valueString='abcd' | err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c1-len-bad-giv` | one profile ps-len on Patient.name.given[0]='abcd' | err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c1-len-bad-ext` | one profile ps-len on Extension.valueString='abcd' | err `ps-len#len-1` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c1-max-ok-obs` | one profile ps-max on Observation.valueString='ab' | nada | igual |
| `c1-max-ok-giv` | one profile ps-max on Patient.name.given[0]='ab' | nada | igual |
| `c1-max-ok-ext` | one profile ps-max on Extension.valueString='ab' | nada | igual |
| `c1-max-bad-obs` | one profile ps-max on Observation.valueString='abcd' | nada | igual |
| `c1-max-bad-giv` | one profile ps-max on Patient.name.given[0]='abcd' | nada | igual |
| `c1-max-bad-ext` | one profile ps-max on Extension.valueString='abcd' | nada | igual |
| `c1-fix-ok-obs` | one profile ps-fix on Observation.valueString='abc' | nada | igual |
| `c1-fix-ok-giv` | one profile ps-fix on Patient.name.given[0]='abc' | nada | igual |
| `c1-fix-ok-ext` | one profile ps-fix on Extension.valueString='abc' | nada | igual |
| `c1-fix-bad-obs` | one profile ps-fix on Observation.valueString='abd' | nada | igual |
| `c1-fix-bad-giv` | one profile ps-fix on Patient.name.given[0]='abd' | nada | igual |
| `c1-fix-bad-ext` | one profile ps-fix on Extension.valueString='abd' | nada | igual |
| `c1-pat-ok-obs` | one profile ps-pat on Observation.valueString='abc' | nada | igual |
| `c1-pat-ok-giv` | one profile ps-pat on Patient.name.given[0]='abc' | nada | igual |
| `c1-pat-ok-ext` | one profile ps-pat on Extension.valueString='abc' | nada | igual |
| `c1-pat-bad-obs` | one profile ps-pat on Observation.valueString='abd' | nada | igual |
| `c1-pat-bad-giv` | one profile ps-pat on Patient.name.given[0]='abd' | nada | igual |
| `c1-pat-bad-ext` | one profile ps-pat on Extension.valueString='abd' | nada | igual |
| `c1-valc-ok-obs` | one profile ps-valc on Observation.valueString='ab' | nada | igual |
| `c1-valc-ok-giv` | one profile ps-valc on Patient.name.given[0]='ab' | nada | igual |
| `c1-valc-ok-ext` | one profile ps-valc on Extension.valueString='ab' | nada | igual |
| `c1-valc-bad-obs` | one profile ps-valc on Observation.valueString='abcd' | nada | igual |
| `c1-valc-bad-giv` | one profile ps-valc on Patient.name.given[0]='abcd' | nada | igual |
| `c1-valc-bad-ext` | one profile ps-valc on Extension.valueString='abcd' | nada | igual |
| `c1-up-ok-obs` | one profile ps-up on Observation.valueString='AB' | nada | igual |
| `c1-up-ok-giv` | one profile ps-up on Patient.name.given[0]='AB' | nada | igual |
| `c1-up-ok-ext` | one profile ps-up on Extension.valueString='AB' | nada | igual |
| `c1-up-bad-obs` | one profile ps-up on Observation.valueString='ab' | err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c1-up-bad-giv` | one profile ps-up on Patient.name.given[0]='ab' | err `ps-up#up-1` @ `Patient.name[0].given[0]` | igual |
| `c1-up-bad-ext` | one profile ps-up on Extension.valueString='ab' | err `ps-up#up-1` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c1-imin-ok-obs` | one profile pi-min (minValue 5) on Observation.valueInteger=7 | nada | igual |
| `c1-imin-ok-ext` | pi-min on Extension.valueInteger=7 | nada | igual |
| `c1-ifx-ok-obs` | one profile pi-fx (fixed 7) on valueInteger=7 | nada | igual |
| `c1-icon-ok-obs` | one profile pi-fix (root constraint $this=7) on valueInteger=7 | nada | igual |
| `c1-imin-bad-obs` | one profile pi-min (minValue 5) on Observation.valueInteger=3 | nada | igual |
| `c1-imin-bad-ext` | pi-min on Extension.valueInteger=3 | nada | igual |
| `c1-ifx-bad-obs` | one profile pi-fx (fixed 7) on valueInteger=8 | nada | igual |
| `c1-icon-bad-obs` | one profile pi-fix (root constraint $this=7) on valueInteger=8 | err `pi-fix#fi-1` @ `Obs.value.ofType(integer)` | igual |
| `c1-dmin-ok-bd` | one profile pd-min (minValue 2000-01-01) on Patient.birthDate=2001-05-05 | nada | igual |
| `c1-dcon-ok-bd` | one profile pd-con (root constraint >= 2000) on Patient.birthDate=2001-05-05 | nada | igual |
| `c1-dmin-bad-bd` | one profile pd-min (minValue 2000-01-01) on Patient.birthDate=1999-01-01 | nada | igual |
| `c1-dcon-bad-bd` | one profile pd-con (root constraint >= 2000) on Patient.birthDate=1999-01-01 | err `pd-con#dc-1` @ `Patient.birthDate` | igual |
| `c1-bind-ok-gen` | pc-bind (required VS male/female) on Patient.gender=male | nada | igual |
| `c1-bind-ok-ext` | pc-bind on Extension.valueCode=male | nada | igual |
| `c1-ccon-ok-gen` | pc-ext (root constraint $this=male) on Patient.gender=male | nada | igual |
| `c1-bind-bad-gen` | pc-bind (required VS male/female) on Patient.gender=other | nada | igual |
| `c1-bind-bad-ext` | pc-bind on Extension.valueCode=other | nada | igual |
| `c1-ccon-bad-gen` | pc-ext (root constraint $this=male) on Patient.gender=other | err `pc-ext#cx-1` @ `Patient.gender` | igual |
| `c1-ext1-ok-obs` | ps-ext1 (string.extension min 1): _valueString=1 ext | nada | igual |
| `c1-ext1-bad-obs` | ps-ext1 (string.extension min 1): _valueString=absent | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c1-sub-ok-obs` | ps-sub (required sub-extension ext-req): _valueString ext=ok | nada | igual |
| `c1-sub-bad-obs` | ps-sub (required sub-extension ext-req): _valueString ext=bad | info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)` | igual |
| `c1-sub-none-obs` | ps-sub (required sub-extension ext-req): _valueString ext=none | err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)` | igual |
| `c1-idc-ok-obs` | ps-idc (string.id constraint length<=3): _valueString={"id": "a1"} | nada | igual |
| `c1-idc-bad-obs` | ps-idc (string.id constraint length<=3): _valueString={"id": "abcdef"} | err `ps-idc#idc-1` @ `Obs.value.ofType(string).id` | igual |
| `c1-idc-none-obs` | ps-idc (string.id constraint length<=3): _valueString=null | nada | igual |
| `c1-id1-ok-obs` | ps-id1 (string.id min 1): _valueString={"id": "a1"} | nada | igual |
| `c1-id1-bad-obs` | ps-id1 (string.id min 1): _valueString=null | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |

## Caso 2: perfil que no resuelve

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c2-obs` | unresolved profile on Observation.valueString | err `Validation_VAL_Unknown_Profile` @ `Obs.value.ofType(string)` | igual |
| `c2-giv` | unresolved profile on Patient.name.given | err `Validation_VAL_Unknown_Profile` @ `Patient.name[0].given[0]` | igual |
| `c2-bd` | unresolved profile on Patient.birthDate | err `Validation_VAL_Unknown_Profile` @ `Patient.birthDate` | igual |
| `c2-int` | unresolved profile on Observation.valueInteger | err `Validation_VAL_Unknown_Profile` @ `Obs.value.ofType(integer)` | igual |
| `c2-ext` | unresolved profile on Extension.valueString | err `Validation_VAL_Unknown_Profile` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c2-obs2` | ps-len + unresolved on valueString (value conforms ps-len) | err `Validation_VAL_Unknown_Profile` @ `Obs.value.ofType(string)` | igual |
| `c2-obs3` | ps-len + unresolved on valueString (value fails ps-len) | err `Validation_VAL_Unknown_Profile` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c2-obs4` | two unresolved (same url) on valueString | err `Validation_VAL_Unknown_Profile` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)` | igual |

## Caso 3: dos perfiles, arrays, valueless

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c3-both-obs` | ps-len+ps-up on valueString='AB' | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c3-both-giv` | ps-len+ps-up on given[0]='AB' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c3-both-ext` | ps-len+ps-up on Extension.valueString='AB' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c3-len-obs` | ps-len+ps-up on valueString='ab' | nada | igual |
| `c3-len-giv` | ps-len+ps-up on given[0]='ab' | nada | igual |
| `c3-len-ext` | ps-len+ps-up on Extension.valueString='ab' | nada | igual |
| `c3-up-obs` | ps-len+ps-up on valueString='ABCDE' | nada | igual |
| `c3-up-giv` | ps-len+ps-up on given[0]='ABCDE' | nada | igual |
| `c3-up-ext` | ps-len+ps-up on Extension.valueString='ABCDE' | nada | igual |
| `c3-none-obs` | ps-len+ps-up on valueString='abcde' | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c3-none-giv` | ps-len+ps-up on given[0]='abcde' | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `ps-up#up-1` @ `Patient.name[0].given[0]` | igual |
| `c3-none-ext` | ps-len+ps-up on Extension.valueString='abcde' | err `Validation_VAL_Profile_NoMatch` @ `Patient.extension[0].value.ofType(string)`<br>err `ps-len#len-1` @ `Patient.extension[0].value.ofType(string)`<br>err `ps-up#up-1` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c3-idx1` | two profiles, given[1] only fails both | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]`<br>err `ps-up#up-1` @ `Patient.name[0].given[1]` | igual |
| `c3-idx1b` | two profiles, given[1] matches one | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c3-idx1c` | two profiles, given[1] matches both | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c3-idx1one` | ONE profile ps-len, given[1] only fails | err `ps-len#len-1` @ `Patient.name[0].given[1]` | igual |
| `c3-al` | two profiles, _given aligned: [null,{ext}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]`<br>err `ps-up#up-1` @ `Patient.name[0].given[1]` | igual |
| `c3-al2` | ps-ext1+ps-len: _given [ {ext}, null ] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]` | igual |
| `c3-vl` | two profiles, given[0] null valueless with _given[0] ext | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `ps-up#up-1` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c3-vl1` | ONE profile ps-len, valueless given[0] with ext | err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c3-vl2` | ps-ext1+ps-len, valueless given[0] with ext | info `(none)` @ `` | igual |
| `c3-vl3` | ONE profile ps-ext1, valueless given[0] with ext, given[1] has no ext | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]` | igual |
| `c3-vl4` | ps-ext1+ps-up, valueless given[0] WITHOUT ext (invalid anyway) | err `Element#ele-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-up#up-1` @ `Patient.name[0].given[0]` | R4B: err `Element#ele-1` @ `Patient.name[0].given[0]`<br>err `Element#ele-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-up#up-1` @ `Patient.name[0].given[0]` / R5: err `Element#ele-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-up#up-1` @ `Patient.name[0].given[0]` |
| `c3-lon` | two profiles, _given longer than given: given[1] valueless | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]`<br>err `ps-up#up-1` @ `Patient.name[0].given[1]` | igual |
| `c3-lon1` | ONE profile ps-ext1, _given longer than given: given[1] valueless | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c3-lon2` | ps-ext1+ps-len, _given longer: given[1] valueless with ext | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c3-vlo` | two profiles, only _valueString (no valueString): ext only (valueless) | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c3-vlo1` | ONE profile ps-ext1, only _valueString (no valueString): ext only | nada | igual |
| `c3-bd-both` | two date profiles, both conform | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.birthDate` | igual |
| `c3-bd-one` | two date profiles, none conform | nada | igual |
| `c3-i-both` | two integer profiles, both conform (7) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(integer)` | igual |
| `c3-i-one` | two integer profiles, only pi-min conforms (6) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(integer)` | igual |
| `c3-i-none` | two integer profiles, none conform (3) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(integer)` | igual |
| `c3-max-both` | maxLength + pattern both conform | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c3-max-one` | maxLength conforms, pattern not | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c3-max-none` | neither conforms | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c3-3p` | three profiles, all conform | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |

## Caso 4: misma clave en ambos perfiles

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c4-both-ok` | same key len-1, different message: conforms both | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c4-both-bad` | same key len-1, different message: fails both | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c4-giv-bad` | same key len-1, different message: fails both (given) | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c4-expr-one` | same key len-1, different expression (<=3, <=2): "abc" fits only ps-len | nada | igual |
| `c4-expr-bad` | same key len-1, different expression: fails both | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `ps-len3#len-1` @ `Obs.value.ofType(string)` | igual |
| `c4-same` | the same profile twice, fails | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c4-same-ok` | the same profile twice, conforms | nada | igual |

## Caso 5: choice y slice por tipo

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c5-xc2-both` | Extension.value[x] string/integer, profiles ps-len+ps-up, value 'AB' | nada | igual |
| `c5-xc2-none` | Extension.value[x] string/integer, profiles ps-len+ps-up, value 'abcde' | err `ps-len#len-1` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c5-xc2-len` | Extension.value[x] string/integer, profiles ps-len+ps-up, value 'ab' | nada | igual |
| `c5-xc1-ok` | Extension.value[x] string/integer, ONE profile ps-len, value 'ab' | nada | igual |
| `c5-xc1-bad` | Extension.value[x] string/integer, ONE profile ps-len, value 'abcd' | err `ps-len#len-1` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c5-xc2-int` | Extension.value[x] string/integer: an integer value (no profile for integer) | nada | igual |
| `c5-os2-both` | slice by type valueString with profiles ps-len+ps-up, value 'AB' | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c5-os2-none` | slice by type valueString with profiles ps-len+ps-up, value 'abcde' | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c5-os2-len` | slice by type valueString with profiles ps-len+ps-up, value 'ab' | nada | igual |
| `c5-os1-ok` | slice by type valueString with ONE profile ps-len, value 'ab' | nada | igual |
| `c5-os1-bad` | slice by type valueString with ONE profile ps-len, value 'abcd' | err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c5-oc2-both` | Observation.value[x] string(ps-len+ps-up)/integer, value 'AB' | nada | igual |
| `c5-oc2-none` | Observation.value[x] string(ps-len+ps-up)/integer, value 'abcde' | err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c5-oc2-len` | Observation.value[x] string(ps-len+ps-up)/integer, value 'ab' | nada | igual |
| `c5-oc2r-both` | Observation.value[x] string(ps-up+ps-len)/integer, value 'AB' | nada | igual |
| `c5-oc2r-none` | Observation.value[x] string(ps-up+ps-len)/integer, value 'abcde' | err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c5-oc2r-len` | Observation.value[x] string(ps-up+ps-len)/integer, value 'ab' | err `ps-up#up-1` @ `Obs.value.ofType(string)` | igual |
| `c5-oc1-ok` | Observation.value[x] string(ps-len)/integer, value 'ab' | nada | igual |
| `c5-oc1-bad` | Observation.value[x] string(ps-len)/integer, value 'abcd' | err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |

## Caso 6: %resource / %rootResource en un contenido

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c6-res-a-f` | contained Observation (final) with ps-res on valueString; top status amended | nada | igual |
| `c6-res-a-a` | contained Observation (amended) with ps-res on valueString; top status amended | err `ps-res#res-1` @ `Observation.contained[0]/*Observation/c1*/.value.ofType(string)` | igual |
| `c6-res-f-f` | contained Observation (final) with ps-res on valueString; top status final | nada | igual |
| `c6-res-f-a` | contained Observation (amended) with ps-res on valueString; top status final | err `ps-res#res-1` @ `Observation.contained[0]/*Observation/c1*/.value.ofType(string)` | igual |
| `c6-rroot-a-f` | contained Observation (final) with ps-rroot on valueString; top status amended | err `ps-rroot#rroot-1` @ `Observation.contained[0]/*Observation/c1*/.value.ofType(string)` | igual |
| `c6-rroot-a-a` | contained Observation (amended) with ps-rroot on valueString; top status amended | err `ps-rroot#rroot-1` @ `Observation.contained[0]/*Observation/c1*/.value.ofType(string)` | igual |
| `c6-rroot-f-f` | contained Observation (final) with ps-rroot on valueString; top status final | nada | igual |
| `c6-rroot-f-a` | contained Observation (amended) with ps-rroot on valueString; top status final | nada | igual |
| `c6-pair-a-f` | contained Observation (final) with ps-res+ps-rroot on valueString; top status amended | nada | igual |
| `c6-pair-f-a` | contained Observation (amended) with ps-res+ps-rroot on valueString; top status final | nada | igual |
| `c6-top-f` | top-level Observation final, ps-res | nada | igual |
| `c6-top-a` | top-level Observation amended, ps-res | err `ps-res#res-1` @ `Obs.value.ofType(string)` | igual |

## Caso 7: constraints de la raíz con extension, id, hasValue() y %context

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c7-hx-sc-obs` | root-constraint profile ps-hx on Observation.valueString='ab', _valueString=null | nada | igual |
| `c7-hx-sc-giv` | root-constraint profile ps-hx on given[0]='ab', _given[0]=null | nada | igual |
| `c7-hx-scx-obs` | root-constraint profile ps-hx on Observation.valueString='ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-hx-scx-giv` | root-constraint profile ps-hx on given[0]='ab', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-hx-vl-obs` | root-constraint profile ps-hx on Observation.valueString=absent, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-hx#hx-1` @ `Obs.value.ofType(string)` | igual |
| `c7-hx-vl-giv` | root-constraint profile ps-hx on given[0]=null, _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-hx#hx-1` @ `Patient.name[0].given[0]` | igual |
| `c7-hx-long-obs` | root-constraint profile ps-hx on Observation.valueString='abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-hx-long-giv` | root-constraint profile ps-hx on given[0]='abcd', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rx-sc-obs` | root-constraint profile ps-rx on Observation.valueString='ab', _valueString=null | err `ps-rx#rx-1` @ `Obs.value.ofType(string)` | igual |
| `c7-rx-sc-giv` | root-constraint profile ps-rx on given[0]='ab', _given[0]=null | err `ps-rx#rx-1` @ `Patient.name[0].given[0]` | igual |
| `c7-rx-scx-obs` | root-constraint profile ps-rx on Observation.valueString='ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rx-scx-giv` | root-constraint profile ps-rx on given[0]='ab', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rx-vl-obs` | root-constraint profile ps-rx on Observation.valueString=absent, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rx-vl-giv` | root-constraint profile ps-rx on given[0]=null, _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `(none)` @ `` | igual |
| `c7-rx-long-obs` | root-constraint profile ps-rx on Observation.valueString='abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rx-long-giv` | root-constraint profile ps-rx on given[0]='abcd', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-ri-sc-obs` | root-constraint profile ps-ri on Observation.valueString='ab', _valueString=null | err `ps-ri#ri-1` @ `Obs.value.ofType(string)` | igual |
| `c7-ri-sc-giv` | root-constraint profile ps-ri on given[0]='ab', _given[0]=null | err `ps-ri#ri-1` @ `Patient.name[0].given[0]` | igual |
| `c7-ri-scx-obs` | root-constraint profile ps-ri on Observation.valueString='ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-ri-scx-giv` | root-constraint profile ps-ri on given[0]='ab', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-ri-vl-obs` | root-constraint profile ps-ri on Observation.valueString=absent, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-ri-vl-giv` | root-constraint profile ps-ri on given[0]=null, _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `(none)` @ `` | igual |
| `c7-ri-long-obs` | root-constraint profile ps-ri on Observation.valueString='abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-ri-long-giv` | root-constraint profile ps-ri on given[0]='abcd', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rc-sc-obs` | root-constraint profile ps-rc on Observation.valueString='ab', _valueString=null | nada | igual |
| `c7-rc-sc-giv` | root-constraint profile ps-rc on given[0]='ab', _given[0]=null | nada | igual |
| `c7-rc-scx-obs` | root-constraint profile ps-rc on Observation.valueString='ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rc-scx-giv` | root-constraint profile ps-rc on given[0]='ab', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rc-vl-obs` | root-constraint profile ps-rc on Observation.valueString=absent, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-rc#rc-1` @ `Obs.value.ofType(string)` | igual |
| `c7-rc-vl-giv` | root-constraint profile ps-rc on given[0]=null, _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-rc#rc-1` @ `Patient.name[0].given[0]` | igual |
| `c7-rc-long-obs` | root-constraint profile ps-rc on Observation.valueString='abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-rc#rc-1` @ `Obs.value.ofType(string)` | igual |
| `c7-rc-long-giv` | root-constraint profile ps-rc on given[0]='abcd', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `ps-rc#rc-1` @ `Patient.name[0].given[0]` | igual |
| `c7-rcx-sc-obs` | root-constraint profile ps-rcx on Observation.valueString='ab', _valueString=null | err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` | igual |
| `c7-rcx-sc-giv` | root-constraint profile ps-rcx on given[0]='ab', _given[0]=null | err `ps-rcx#rcx-1` @ `Patient.name[0].given[0]` | igual |
| `c7-rcx-scx-obs` | root-constraint profile ps-rcx on Observation.valueString='ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcx-scx-giv` | root-constraint profile ps-rcx on given[0]='ab', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcx-vl-obs` | root-constraint profile ps-rcx on Observation.valueString=absent, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcx-vl-giv` | root-constraint profile ps-rcx on given[0]=null, _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `(none)` @ `` | igual |
| `c7-rcx-long-obs` | root-constraint profile ps-rcx on Observation.valueString='abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcx-long-giv` | root-constraint profile ps-rcx on given[0]='abcd', _given[0]={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rxri-x-obs` | ps-rx+ps-ri on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rxri-x-vl` | ps-rx+ps-ri on a valueless valueString, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rxri-i-obs` | ps-rx+ps-ri on valueString=ab, _valueString={"id": "a1"} | nada | igual |
| `c7-rxri-i-vl` | ps-rx+ps-ri on a valueless valueString, _valueString={"id": "a1"} | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` |
| `c7-rxri-xi-obs` | ps-rx+ps-ri on valueString=ab, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-rxri-xi-vl` | ps-rx+ps-ri on a valueless valueString, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-rxri-none-obs` | ps-rx+ps-ri on valueString=ab, _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)`<br>err `ps-ri#ri-1` @ `Obs.value.ofType(string)` | igual |
| `c7-rxri-none-vl` | ps-rx+ps-ri on a valueless valueString, _valueString=null | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` |
| `c7-hxrx-x-obs` | ps-hx+ps-rx on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-hxrx-x-vl` | ps-hx+ps-rx on a valueless valueString, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-hxrx-i-obs` | ps-hx+ps-rx on valueString=ab, _valueString={"id": "a1"} | nada | igual |
| `c7-hxrx-i-vl` | ps-hx+ps-rx on a valueless valueString, _valueString={"id": "a1"} | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` |
| `c7-hxrx-xi-obs` | ps-hx+ps-rx on valueString=ab, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-hxrx-xi-vl` | ps-hx+ps-rx on a valueless valueString, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-hxrx-none-obs` | ps-hx+ps-rx on valueString=ab, _valueString=null | nada | igual |
| `c7-hxrx-none-vl` | ps-hx+ps-rx on a valueless valueString, _valueString=null | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-hx#hx-1` @ `Obs.value.ofType(string)`<br>err `ps-rx#rx-1` @ `Obs.value.ofType(string)` |
| `c7-rcrx-x-obs` | ps-rc+ps-rcx on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-rcrx-x-vl` | ps-rc+ps-rcx on a valueless valueString, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcrx-i-obs` | ps-rc+ps-rcx on valueString=ab, _valueString={"id": "a1"} | nada | igual |
| `c7-rcrx-i-vl` | ps-rc+ps-rcx on a valueless valueString, _valueString={"id": "a1"} | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` |
| `c7-rcrx-xi-obs` | ps-rc+ps-rcx on valueString=ab, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c7-rcrx-xi-vl` | ps-rc+ps-rcx on a valueless valueString, _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c7-rcrx-none-obs` | ps-rc+ps-rcx on valueString=ab, _valueString=null | nada | igual |
| `c7-rcrx-none-vl` | ps-rc+ps-rcx on a valueless valueString, _valueString=null | err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-rc#rc-1` @ `Obs.value.ofType(string)`<br>err `ps-rcx#rcx-1` @ `Obs.value.ofType(string)` |

## Caso 8: escalares iguales bajo padres y recursos distintos

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c8-par-one-a` | ps-ext1, name[0].given[0]=name[1].given[0]='ab'; only name[0] has an extension | err `Validation_VAL_Profile_Minimum` @ `Patient.name[1].given[0]` | igual |
| `c8-par-one-b` | ps-ext1, name[0].given[0]=name[1].given[0]='ab'; only name[1] has an extension | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c8-par-one-c` | ps-ext1, both names equal with an extension | nada | igual |
| `c8-par-one-d` | ps-ext1, both names equal without extension | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[1].given[0]` | igual |
| `c8-idx-one` | ps-ext1, given[0] and given[1] equal, only given[0] has an extension | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]` | igual |
| `c8-idx-one-r` | ps-ext1, given[0] and given[1] equal, only given[1] has an extension | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c8-res-one-ab` | Bundle of two Patients, ps-ext1, equal given 'ab'; first has extension=True, second has extension=False | err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-one-ba` | Bundle of two Patients, ps-ext1, equal given 'ab'; first has extension=False, second has extension=True | err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]` | igual |
| `c8-res-one-aa` | Bundle of two Patients, ps-ext1, equal given 'ab'; first has extension=True, second has extension=True | nada | igual |
| `c8-res-one-bb` | Bundle of two Patients, ps-ext1, equal given 'ab'; first has extension=False, second has extension=False | err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-par-two-a` | ps-ext1+ps-len, name[0].given[0]=name[1].given[0]='abcd'; only name[0] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-par-two-b` | ps-ext1+ps-len, name[0].given[0]=name[1].given[0]='abcd'; only name[1] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c8-par-two-c` | ps-ext1+ps-len, both names equal with an extension | nada | igual |
| `c8-par-two-d` | ps-ext1+ps-len, both names equal without extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-idx-two` | ps-ext1+ps-len, given[0] and given[1] equal, only given[0] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]` | igual |
| `c8-idx-two-r` | ps-ext1+ps-len, given[0] and given[1] equal, only given[1] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c8-res-two-ab` | Bundle of two Patients, ps-ext1+ps-len, equal given 'abcd'; first has extension=True, second has extension=False | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-two-ba` | Bundle of two Patients, ps-ext1+ps-len, equal given 'abcd'; first has extension=False, second has extension=True | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]` | igual |
| `c8-res-two-aa` | Bundle of two Patients, ps-ext1+ps-len, equal given 'abcd'; first has extension=True, second has extension=True | nada | igual |
| `c8-res-two-bb` | Bundle of two Patients, ps-ext1+ps-len, equal given 'abcd'; first has extension=False, second has extension=False | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-par-twoup-a` | ps-ext1+ps-up, name[0].given[0]=name[1].given[0]='AB'; only name[0] has an extension | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c8-par-twoup-b` | ps-ext1+ps-up, name[0].given[0]=name[1].given[0]='AB'; only name[1] has an extension | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[1].given[0]` | igual |
| `c8-par-twoup-c` | ps-ext1+ps-up, both names equal with an extension | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[1].given[0]` | igual |
| `c8-par-twoup-d` | ps-ext1+ps-up, both names equal without extension | nada | igual |
| `c8-idx-twoup` | ps-ext1+ps-up, given[0] and given[1] equal, only given[0] has an extension | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c8-idx-twoup-r` | ps-ext1+ps-up, given[0] and given[1] equal, only given[1] has an extension | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c8-res-twoup-ab` | Bundle of two Patients, ps-ext1+ps-up, equal given 'AB'; first has extension=True, second has extension=False | warn `Validation_VAL_Profile_MultipleMatches` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]` | igual |
| `c8-res-twoup-ba` | Bundle of two Patients, ps-ext1+ps-up, equal given 'AB'; first has extension=False, second has extension=True | warn `Validation_VAL_Profile_MultipleMatches` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-twoup-aa` | Bundle of two Patients, ps-ext1+ps-up, equal given 'AB'; first has extension=True, second has extension=True | warn `Validation_VAL_Profile_MultipleMatches` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-twoup-bb` | Bundle of two Patients, ps-ext1+ps-up, equal given 'AB'; first has extension=False, second has extension=False | nada | igual |
| `c8-par-sub-a` | ps-sub+ps-len, name[0].given[0]=name[1].given[0]='abcd'; only name[0] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-par-sub-b` | ps-sub+ps-len, name[0].given[0]=name[1].given[0]='abcd'; only name[1] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[1].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-par-sub-c` | ps-sub+ps-len, both names equal with an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[1].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-par-sub-d` | ps-sub+ps-len, both names equal without extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[1].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1].given[0]`<br>err `ps-len#len-1` @ `Patient.name[1].given[0]` | igual |
| `c8-idx-sub` | ps-sub+ps-len, given[0] and given[1] equal, only given[0] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]` | igual |
| `c8-idx-sub-r` | ps-sub+ps-len, given[0] and given[1] equal, only given[1] has an extension | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[1]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[1].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[1]`<br>err `ps-len#len-1` @ `Patient.name[0].given[1]` | igual |
| `c8-res-sub-ab` | Bundle of two Patients, ps-sub+ps-len, equal given 'abcd'; first has extension=True, second has extension=False | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-sub-ba` | Bundle of two Patients, ps-sub+ps-len, equal given 'abcd'; first has extension=False, second has extension=True | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-sub-aa` | Bundle of two Patients, ps-sub+ps-len, equal given 'abcd'; first has extension=True, second has extension=True | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |
| `c8-res-sub-bb` | Bundle of two Patients, ps-sub+ps-len, equal given 'abcd'; first has extension=False, second has extension=False | err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[0].resource/*Patient/pa*/.name[0].given[0]`<br>err `Validation_VAL_Profile_NoMatch` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]`<br>err `ps-len#len-1` @ `Bundle.entry[1].resource/*Patient/pb*/.name[0].given[0]` | igual |

## Caso 9: slices de extensión obligatorios y prohibidos

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c9-sub-none-giv` | ps-sub on given[0]=ab, _given[0]=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c9-sub-none-ext` | ps-sub on Extension.valueString=ab, _valueString=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c9-sub-sub-giv` | ps-sub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | nada | igual |
| `c9-sub-sub-ext` | ps-sub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | nada | igual |
| `c9-sub-oth-giv` | ps-sub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c9-sub-oth-ext` | ps-sub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.extension[0].value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c9-sub-both-giv` | ps-sub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[1]` | igual |
| `c9-sub-both-ext` | ps-sub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.extension[0].value.ofType(string).extension[1]` | igual |
| `c9-sub-vl1` | ps-sub: given=[ab,null], _given=[null,{sub ext}]: valueless member with the required ext, the scalar without _x | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c9-sub-vl2` | ps-sub: given=[null,ab], _given=[{other ext},null]: valueless member with another ext | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[1]` | igual |
| `c9-sub-vl3` | ps-sub: given=[ab], _given=[null,{sub ext}]: _given longer than given | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c9-sub-vl4` | ps-sub: given=[ab], _given=[null,{other ext}]: _given longer than given | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[1].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[1]` | igual |
| `c9-sub-vlo` | ps-sub on a valueless valueString with another ext | info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)` | igual |
| `c9-sub-vlos` | ps-sub on a valueless valueString with the sub ext | nada | igual |
| `c9-nosub-none-giv` | ps-nosub on given[0]=ab, _given[0]=null | nada | igual |
| `c9-nosub-none-ext` | ps-nosub on Extension.valueString=ab, _valueString=null | nada | igual |
| `c9-nosub-sub-giv` | ps-nosub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c9-nosub-sub-ext` | ps-nosub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.extension[0].value.ofType(string).extension[0]` | igual |
| `c9-nosub-oth-giv` | ps-nosub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `Validation_VAL_Profile_Maximum` @ `Patient.name[0].given[0]` | igual |
| `c9-nosub-oth-ext` | ps-nosub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `Validation_VAL_Profile_Maximum` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c9-nosub-both-giv` | ps-nosub on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Maximum` @ `Patient.name[0].given[0]` | igual |
| `c9-nosub-both-ext` | ps-nosub on Extension.valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.extension[0].value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Maximum` @ `Patient.extension[0].value.ofType(string)` | igual |
| `c9-nosub-vl1` | ps-nosub: given=[ab,null], _given=[null,{sub ext}]: valueless member with the required ext, the scalar without _x | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[1].extension[0]` | igual |
| `c9-nosub-vl2` | ps-nosub: given=[null,ab], _given=[{other ext},null]: valueless member with another ext | err `Validation_VAL_Profile_Maximum` @ `Patient.name[0].given[0]` | igual |
| `c9-nosub-vl3` | ps-nosub: given=[ab], _given=[null,{sub ext}]: _given longer than given | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[1].extension[0]` | igual |
| `c9-nosub-vl4` | ps-nosub: given=[ab], _given=[null,{other ext}]: _given longer than given | err `Validation_VAL_Profile_Maximum` @ `Patient.name[0].given[1]` | igual |
| `c9-nosub-vlo` | ps-nosub on a valueless valueString with another ext | err `Validation_VAL_Profile_Maximum` @ `Obs.value.ofType(string)` | igual |
| `c9-nosub-vlos` | ps-nosub on a valueless valueString with the sub ext | info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]` | igual |
| `c9-pair-none-obs` | ps-sub+ps-nosub on valueString=ab, _valueString=null | nada | igual |
| `c9-sublen-none-obs` | ps-sub+ps-len on valueString=abcd, _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c9-sublen-none-giv` | ps-sub+ps-len on given=[abcd,ab], _given[0]=null | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c9-pair-sub-obs` | ps-sub+ps-nosub on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)`<br>info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]` | igual |
| `c9-sublen-sub-obs` | ps-sub+ps-len on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | nada | igual |
| `c9-sublen-sub-giv` | ps-sub+ps-len on given=[abcd,ab], _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | nada | igual |
| `c9-pair-oth-obs` | ps-sub+ps-nosub on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Maximum` @ `Obs.value.ofType(string)` | igual |
| `c9-sublen-oth-obs` | ps-sub+ps-len on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c9-sublen-oth-giv` | ps-sub+ps-len on given=[abcd,ab], _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c9-pair-both-obs` | ps-sub+ps-nosub on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[1]` | igual |
| `c9-sublen-both-obs` | ps-sub+ps-len on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Obs.value.ofType(string).extension[1]` | igual |
| `c9-sublen-both-giv` | ps-sub+ps-len on given=[abcd,ab], _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}, {"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[1]` | igual |
| `c9-sublen-vl` | ps-sub+ps-len: given=[ab,null], _given=[null,{sub ext}] | info `(none)` @ `` | igual |

## Caso 10 (A4): perfiles pareados por .value, y .value con id/extension

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c10-lenmax-0` | ps-len+ps-max on valueString='ab' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-lenmax-0-giv` | ps-len+ps-max on given[0]='ab' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-lenmax-1` | ps-len+ps-max on valueString='abcd' (pair that differs only in .value) | nada | igual |
| `c10-lenmax-1-giv` | ps-len+ps-max on given[0]='abcd' | nada | igual |
| `c10-upfix-0` | ps-up+ps-fix on valueString='AB' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-upfix-0-giv` | ps-up+ps-fix on given[0]='AB' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-upfix-1` | ps-up+ps-fix on valueString='ab' (pair that differs only in .value) | nada | igual |
| `c10-upfix-1-giv` | ps-up+ps-fix on given[0]='ab' | nada | igual |
| `c10-maxfix-0` | ps-max+ps-fix on valueString='abc' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-maxfix-0-giv` | ps-max+ps-fix on given[0]='abc' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-maxfix-1` | ps-max+ps-fix on valueString='abcd' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-maxfix-1-giv` | ps-max+ps-fix on given[0]='abcd' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-valcpat-0` | ps-valc+ps-pat on valueString='abc' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-valcpat-0-giv` | ps-valc+ps-pat on given[0]='abc' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-valcpat-1` | ps-valc+ps-pat on valueString='abcd' (pair that differs only in .value) | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-valcpat-1-giv` | ps-valc+ps-pat on given[0]='abcd' | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c10-mixe-x-abcd` | ps-mixe (value maxLength 3 + extension min 1) on 'abcd', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixelen-x-abcd` | ps-mixe+ps-len on 'abcd', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixeup-x-abcd` | ps-mixe+ps-up on 'abcd', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixe-x-ab` | ps-mixe (value maxLength 3 + extension min 1) on 'ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixelen-x-ab` | ps-mixe+ps-len on 'ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-mixeup-x-ab` | ps-mixe+ps-up on 'ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixe-nox-abcd` | ps-mixe (value maxLength 3 + extension min 1) on 'abcd', _valueString=null | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixelen-nox-abcd` | ps-mixe+ps-len on 'abcd', _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixeup-nox-abcd` | ps-mixe+ps-up on 'abcd', _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixe-nox-ab` | ps-mixe (value maxLength 3 + extension min 1) on 'ab', _valueString=null | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixelen-nox-ab` | ps-mixe+ps-len on 'ab', _valueString=null | nada | igual |
| `c10-mixeup-nox-ab` | ps-mixe+ps-up on 'ab', _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixe-xi-abcd` | ps-mixe (value maxLength 3 + extension min 1) on 'abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixelen-xi-abcd` | ps-mixe+ps-len on 'abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixeup-xi-abcd` | ps-mixe+ps-up on 'abcd', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixe-xi-ab` | ps-mixe (value maxLength 3 + extension min 1) on 'ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixelen-xi-ab` | ps-mixe+ps-len on 'ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-mixeup-xi-ab` | ps-mixe+ps-up on 'ab', _valueString={"id": "a1", "extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c10-mixi-i-abd` | ps-mixi (value fixed abc + id min 1) on 'abd', _valueString={"id": "a1"} | nada | igual |
| `c10-mixiup-i-abd` | ps-mixi+ps-up on 'abd', _valueString={"id": "a1"} | nada | igual |
| `c10-mixilen-i-abd` | ps-mixi+ps-len on 'abd', _valueString={"id": "a1"} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-mixi-i-abc` | ps-mixi (value fixed abc + id min 1) on 'abc', _valueString={"id": "a1"} | nada | igual |
| `c10-mixiup-i-abc` | ps-mixi+ps-up on 'abc', _valueString={"id": "a1"} | nada | igual |
| `c10-mixilen-i-abc` | ps-mixi+ps-len on 'abc', _valueString={"id": "a1"} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c10-mixi-noi-abd` | ps-mixi (value fixed abc + id min 1) on 'abd', _valueString=null | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixiup-noi-abd` | ps-mixi+ps-up on 'abd', _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixilen-noi-abd` | ps-mixi+ps-len on 'abd', _valueString=null | nada | igual |
| `c10-mixi-noi-abc` | ps-mixi (value fixed abc + id min 1) on 'abc', _valueString=null | err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixiup-noi-abc` | ps-mixi+ps-up on 'abc', _valueString=null | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-up#up-1` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c10-mixilen-noi-abc` | ps-mixi+ps-len on 'abc', _valueString=null | nada | igual |

## Caso 11: guardas del camino sin perfiles

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c11-plain-obs` | no profile: Observation.valueString=abcd | nada | igual |
| `c11-plain-giv` | no profile: given=[abcd,ef] | nada | igual |
| `c11-ext-giv` | no profile: given[0] with id and extension | nada | igual |
| `c11-vl-giv` | no profile: valueless given[0] with only an id (ele-1; R4B repeats it) | err `Element#ele-1` @ `Patient.name[0].given[0]` | R4B: err `Element#ele-1` @ `Patient.name[0].given[0]`<br>err `Element#ele-1` @ `Patient.name[0].given[0]` / R5: err `Element#ele-1` @ `Patient.name[0].given[0]` |
| `c11-vlx-giv` | no profile: valueless given[0] with an extension (valid) | info `(none)` @ `` | igual |
| `c11-vl-obs` | no profile: valueless Observation.valueString with only an id | err `Element#ele-1` @ `Obs.value.ofType(string)` | R4B: err `Element#ele-1` @ `Obs.value.ofType(string)`<br>err `Element#ele-1` @ `Obs.value.ofType(string)` / R5: err `Element#ele-1` @ `Obs.value.ofType(string)` |
| `c11-vl-bd` | no profile: valueless Patient.birthDate with only an id | err `Element#ele-1` @ `Patient.birthDate` | R4B: err `Element#ele-1` @ `Patient.birthDate`<br>err `Element#ele-1` @ `Patient.birthDate` / R5: err `Element#ele-1` @ `Patient.birthDate` |
| `c11-lon-giv` | no profile: _given longer than given, given[1] valueless with only an id | err `Element#ele-1` @ `Patient.name[0].given[1]` | R4B: err `Element#ele-1` @ `Patient.name[0].given[1]`<br>err `Element#ele-1` @ `Patient.name[0].given[1]` / R5: err `Element#ele-1` @ `Patient.name[0].given[1]` |

## Caso 12: huecos de la corrida de mutaciones (plan §7)

| Instancia | Qué | HL7 7.0.4 (R4) | R4B / R5 |
|---|---|---|---|
| `c12-idc-up-bad-obs` | ps-idc+ps-up on valueString='AB', _valueString={"id": "abcdef"} | nada | igual |
| `c12-idc-up-bad-giv` | ps-idc+ps-up on given[0]='AB', _given[0]={"id": "abcdef"} | nada | igual |
| `c12-idc-up-ok-obs` | ps-idc+ps-up on valueString='AB', _valueString={"id": "a1"} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-idc-up-ok-giv` | ps-idc+ps-up on given[0]='AB', _given[0]={"id": "a1"} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-extc-up-bad-obs` | ps-extc+ps-up on valueString='AB', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c12-extc-up-bad-giv` | ps-extc+ps-up on given[0]='AB', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | nada | igual |
| `c12-extc-up-ok-obs` | ps-extc+ps-up on valueString='AB', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-extc-up-ok-giv` | ps-extc+ps-up on given[0]='AB', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-len-extw-bad-obs` | ps-len+ps-extw on valueString='ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)`<br>warn `ps-extw#xw-1` @ `Obs.value.ofType(string).extension[0]` | igual |
| `c12-len-extw-bad-giv` | ps-len+ps-extw on given[0]='ab', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>warn `ps-extw#xw-1` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-len-extw-ok-obs` | ps-len+ps-extw on valueString='ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-len-extw-ok-giv` | ps-len+ps-extw on given[0]='ab', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-extw-len-bad-obs` | ps-extw+ps-len on valueString='ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)`<br>warn `ps-extw#xw-1` @ `Obs.value.ofType(string).extension[0]` | igual |
| `c12-extw-len-bad-giv` | ps-extw+ps-len on given[0]='ab', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>warn `ps-extw#xw-1` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-extw-len-ok-obs` | ps-extw+ps-len on valueString='ab', _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-extw-len-ok-giv` | ps-extw+ps-len on given[0]='ab', _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-ext1x2-none-obs` | ps-ext1+ps-ext1b (both extension min 1), _valueString={"id": "a1"} | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)`<br>err `Validation_VAL_Profile_Minimum` @ `Obs.value.ofType(string)` | igual |
| `c12-ext1x2-none-giv` | ps-ext1+ps-ext1b (both extension min 1), _given[0]={"id": "a1"} | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c12-ext1x2-x-obs` | ps-ext1+ps-ext1b (both extension min 1), _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-ext1x2-x-giv` | ps-ext1+ps-ext1b (both extension min 1), _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-xs-bind-bad-one` | ps-xbind on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "other"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>err `Terminology_TX_NoValid_12` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-bad-pair` | ps-len+ps-xbind on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "other"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-bad-both` | ps-len+ps-xbind on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "other"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>err `Terminology_TX_NoValid_12` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-bad-giv` | ps-len+ps-xbind on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "other"}}]} | info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-ok-one` | ps-xbind on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-ok-pair` | ps-len+ps-xbind on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-ok-both` | ps-len+ps-xbind on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-bind-ok-giv` | ps-len+ps-xbind on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)`<br>info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)` | igual |
| `c12-xs-fix-bad-one` | ps-xfix on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "abc"}]} | err `ext-str-con#esc-1` @ `Obs.value.ofType(string).extension[0].value.ofType(string)` | igual |
| `c12-xs-fix-bad-pair` | ps-len+ps-xfix on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "abc"}]} | nada | igual |
| `c12-xs-fix-bad-both` | ps-len+ps-xfix on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "abc"}]} | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `ext-str-con#esc-1` @ `Obs.value.ofType(string).extension[0].value.ofType(string)` | igual |
| `c12-xs-fix-bad-giv` | ps-len+ps-xfix on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "abc"}]} | nada | igual |
| `c12-xs-fix-ok-one` | ps-xfix on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "ab"}]} | nada | igual |
| `c12-xs-fix-ok-pair` | ps-len+ps-xfix on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "ab"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-xs-fix-ok-both` | ps-len+ps-xfix on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "ab"}]} | nada | igual |
| `c12-xs-fix-ok-giv` | ps-len+ps-xfix on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-str", "valueString": "ab"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-xs-ctx-x-one` | ps-xctx on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-ctx-x-pair` | ps-len+ps-xctx on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)`<br>info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-ctx-x-both` | ps-len+ps-xctx on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Obs.value.ofType(string).extension[0].value.ofType(Coding)` | igual |
| `c12-xs-ctx-x-giv` | ps-len+ps-xctx on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-code", "valueCoding": {"system": "urn:probe:cs", "code": "male"}}]} | info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]`<br>info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)`<br>info `MSG_DRAFT` @ `Patient.name[0].given[0].extension[0].value.ofType(Coding)` | igual |
| `c12-xs-ref-bad-one` | ps-xref on valueString=ab, a reference to a contained Observation | err `Reference_REF_BadTargetType` @ `Obs.value.ofType(string).extension[0].value.ofType(Reference)` | igual |
| `c12-xs-ref-bad-pair` | ps-len+ps-xref on valueString=ab, a reference to a contained Observation | nada | igual |
| `c12-xs-ref-bad-both` | ps-len+ps-xref on valueString=abcd, a reference to a contained Observation | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)`<br>err `Reference_REF_BadTargetType` @ `Obs.value.ofType(string).extension[0].value.ofType(Reference)` | igual |
| `c12-xs-ref-ok-one` | ps-xref on valueString=ab, a reference to a contained Patient | nada | igual |
| `c12-xs-ref-ok-pair` | ps-len+ps-xref on valueString=ab, a reference to a contained Patient | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-xs-ref-ok-both` | ps-len+ps-xref on valueString=abcd, a reference to a contained Patient | nada | igual |
| `c12-pgp-open-x` | pgp-open on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgp-open-up` | pgp-open on given=['AB'], _given=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0]` | igual |
| `c12-pgp-open-none` | pgp-open on given=['ab'], _given=null | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0]` | igual |
| `c12-pgp-open-mix` | pgp-open on given=['AB', 'ab', 'cd'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[2]` | igual |
| `c12-pgp-open-names-ab` | pgp-open: name[0] and name[1] with given ab; extension only in name[0] | info `Details_for__matching_against_Profile_` @ `Patient.name[1].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1]` | igual |
| `c12-pgp-open-names-ba` | pgp-open: name[0] and name[1] with given ab; extension only in name[1] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0]` | igual |
| `c12-pgp-closed-x` | pgp-closed on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgp-closed-up` | pgp-closed on given=['AB'], _given=null | nada | igual |
| `c12-pgp-closed-none` | pgp-closed on given=['ab'], _given=null | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-closed-mix` | pgp-closed on given=['AB', 'ab', 'cd'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[2]` | igual |
| `c12-pgp-closed-names-ab` | pgp-closed: name[0] and name[1] with given ab; extension only in name[0] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[1].given[0]` | igual |
| `c12-pgp-closed-names-ba` | pgp-closed: name[0] and name[1] with given ab; extension only in name[1] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-one-x` | pgp-one on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgp-one-up` | pgp-one on given=['AB'], _given=null | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-one-none` | pgp-one on given=['ab'], _given=null | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-one-mix` | pgp-one on given=['AB', 'ab', 'cd'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[2]` | igual |
| `c12-pgp-one-names-ab` | pgp-one: name[0] and name[1] with given ab; extension only in name[0] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[1].given[0]` | igual |
| `c12-pgp-one-names-ba` | pgp-one: name[0] and name[1] with given ab; extension only in name[1] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-type-male` | pgp-type (slice a with a code profile on a string, b ps-up) on given=['male'] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-type-up` | pgp-type (slice a with a code profile on a string, b ps-up) on given=['AB'] | nada | igual |
| `c12-pgp-type-none` | pgp-type (slice a with a code profile on a string, b ps-up) on given=['xx'] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0]` | igual |
| `c12-pft-f` | pft: name.family and name.text both ab with ps-ext1+ps-len; _family={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, _text=null | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].family` | igual |
| `c12-pft-t` | pft: name.family and name.text both ab with ps-ext1+ps-len; _family=null, _text={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].text` | igual |
| `c12-pft-ft` | pft: name.family and name.text both ab with ps-ext1+ps-len; _family={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, _text={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].text`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].family` | igual |
| `c12-pft-none` | pft: name.family and name.text both ab with ps-ext1+ps-len; _family=null, _text=null | nada | igual |
| `c12-pgs1-none` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['ab'], _given=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs1-sub` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pgs1-oth` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs1-long` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['abcd'], _given=null | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs1-two` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['cd', 'ab'], _given=[null, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[1]` | igual |
| `c12-pgs1-twox` | pgs1 (slice by pattern ab, profiles ps-sub) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2-none` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['ab'], _given=null | nada | igual |
| `c12-pgs2-sub` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2-oth` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgs2-long` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['abcd'], _given=null | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2-two` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['cd', 'ab'], _given=[null, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2-twox` | pgs2 (slice by pattern ab, profiles ps-sub+ps-len) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c12-pgs2r-none` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['ab'], _given=null | nada | igual |
| `c12-pgs2r-sub` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2r-oth` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgs2r-long` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['abcd'], _given=null | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2r-two` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['cd', 'ab'], _given=[null, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgs2r-twox` | pgs2r (slice by pattern ab, profiles ps-len+ps-sub) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c12-pnh1-none` | pnh1 (Patient.name profile hn-sub) on given=['ab'], _given=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pnh1-sub` | pnh1 (Patient.name profile hn-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pnh1-oth` | pnh1 (Patient.name profile hn-sub) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pnh1-long` | pnh1 (Patient.name profile hn-sub) on given=['abcd'], _given=null | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pnh2-none` | pnh2 (Patient.name profile hn-sublen) on given=['ab'], _given=null | nada | igual |
| `c12-pnh2-sub` | pnh2 (Patient.name profile hn-sublen) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pnh2-oth` | pnh2 (Patient.name profile hn-sublen) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pnh2-long` | pnh2 (Patient.name profile hn-sublen) on given=['abcd'], _given=null | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhc1-none` | pnhc1 (Patient.name profile hn-subc) on given=['ab'], _given=null | nada | igual |
| `c12-pnhc1-sub` | pnhc1 (Patient.name profile hn-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pnhc1-oth` | pnhc1 (Patient.name profile hn-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-pnhc1-long` | pnhc1 (Patient.name profile hn-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-pnhc2-none` | pnhc2 (Patient.name profile hn-subclen) on given=['ab'], _given=null | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhc2-sub` | pnhc2 (Patient.name profile hn-subclen) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhc2-oth` | pnhc2 (Patient.name profile hn-subclen) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pnhc2-long` | pnhc2 (Patient.name profile hn-subclen) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc1-none` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['ab'], _given=null | nada | igual |
| `c12-pgsc1-sub` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pgsc1-oth` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-pgsc1-long` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc1-two` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['cd', 'ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc1-twox` | pgsc1 (slice by pattern ab, profiles ps-subc) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[1].extension[0]` | igual |
| `c12-pgsc2-none` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['ab'], _given=null | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2-sub` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2-oth` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgsc2-long` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2-two` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['cd', 'ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c12-pgsc2-twox` | pgsc2 (slice by pattern ab, profiles ps-subc+ps-len) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2r-none` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['ab'], _given=null | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2r-sub` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2r-oth` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgsc2r-long` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsc2r-two` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['cd', 'ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[1]` | igual |
| `c12-pgsc2r-twox` | pgsc2r (slice by pattern ab, profiles ps-len+ps-subc) on given=['cd', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-con-x` | pgp-con (slice a ps-ext1 with a constraint) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `pgp-con#sc-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-con-up` | pgp-con (slice a ps-ext1 with a constraint) on given=['AB'], _given=null | nada | igual |
| `c12-pgp-con-none` | pgp-con (slice a ps-ext1 with a constraint) on given=['ab'], _given=null | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]` | igual |
| `c12-pgp-con-mix` | pgp-con (slice a ps-ext1 with a constraint) on given=['AB', 'ab'], _given=[null, {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `pgp-con#sc-1` @ `Patient.name[0].given[1]` | igual |
| `c12-pnf-closed-x` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | nada | igual |
| `c12-pnf-closed-up` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "AB"}] | nada | igual |
| `c12-pnf-closed-none` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "ab"}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0]` | igual |
| `c12-pnf-closed-ab` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}, {"family": "ab"}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[1]` | igual |
| `c12-pnf-closed-ba` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "ab"}, {"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0]` | igual |
| `c12-pnf-closed-upx` | pnf-closed (Patient.name sliced by the profile of family) on [{"family": "AB"}, {"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | nada | igual |
| `c12-pnf-open-x` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | nada | igual |
| `c12-pnf-open-up` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "AB"}] | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient` | igual |
| `c12-pnf-open-none` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "ab"}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient` | igual |
| `c12-pnf-open-ab` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}, {"family": "ab"}] | info `Details_for__matching_against_Profile_` @ `Patient.name[1]` | igual |
| `c12-pnf-open-ba` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "ab"}, {"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]` | igual |
| `c12-pnf-open-upx` | pnf-open (Patient.name sliced by the profile of family) on [{"family": "AB"}, {"family": "ab", "_family": {"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}}] | nada | igual |
| `c12-xstr-bad-one` | ps-xstr on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueInteger": 5}]} | err `Extension_PROF_Type` @ `Obs.value.ofType(string).extension[0].value.ofType(integer)` | igual |
| `c12-xstr-bad-pair` | ps-len+ps-xstr on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueInteger": 5}]} | nada | igual |
| `c12-xstr-bad-both` | ps-len+ps-xstr on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueInteger": 5}]} | err `Validation_VAL_Profile_NoMatch` @ `Obs.value.ofType(string)`<br>err `Extension_PROF_Type` @ `Obs.value.ofType(string).extension[0].value.ofType(integer)`<br>err `ps-len#len-1` @ `Obs.value.ofType(string)` | igual |
| `c12-xstr-bad-giv` | ps-len+ps-xstr on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueInteger": 5}]} | nada | igual |
| `c12-xstr-ok-one` | ps-xstr on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueString": "q"}]} | nada | igual |
| `c12-xstr-ok-pair` | ps-len+ps-xstr on valueString=ab, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Obs.value.ofType(string)` | igual |
| `c12-xstr-ok-both` | ps-len+ps-xstr on valueString=abcd, _valueString={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueString": "q"}]} | nada | igual |
| `c12-xstr-ok-giv` | ps-len+ps-xstr on given[0]=ab, _given[0]={"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-two", "valueString": "q"}]} | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsd1-none` | pgsd1 (slice by pattern abcd, profiles ps-subc) on given=['abcd'], _given=null | nada | igual |
| `c12-pgsd1-sub` | pgsd1 (slice by pattern abcd, profiles ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pgsd1-oth` | pgsd1 (slice by pattern abcd, profiles ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-pgsd2-none` | pgsd2 (slice by pattern abcd, profiles ps-subc+ps-len) on given=['abcd'], _given=null | nada | igual |
| `c12-pgsd2-sub` | pgsd2 (slice by pattern abcd, profiles ps-subc+ps-len) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pgsd2-oth` | pgsd2 (slice by pattern abcd, profiles ps-subc+ps-len) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsd2r-none` | pgsd2r (slice by pattern abcd, profiles ps-len+ps-subc) on given=['abcd'], _given=null | nada | igual |
| `c12-pgsd2r-sub` | pgsd2r (slice by pattern abcd, profiles ps-len+ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}] | nada | igual |
| `c12-pgsd2r-oth` | pgsd2r (slice by pattern abcd, profiles ps-len+ps-subc) on given=['abcd'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsx1-none` | pgsx1 (slice s by pattern ab with its own extension min 1, profiles ps-len) on given=['ab'], _given=null | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsx1-x` | pgsx1 (slice s by pattern ab with its own extension min 1, profiles ps-len) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgsx1-two` | pgsx1 (slice s by pattern ab with its own extension min 1, profiles ps-len) on given=['cd', 'ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]` | igual |
| `c12-pgsx2-none` | pgsx2 (slice s by pattern ab with its own extension min 1, profiles ps-len+ps-up) on given=['ab'], _given=null | err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[0]` | igual |
| `c12-pgsx2-x` | pgsx2 (slice s by pattern ab with its own extension min 1, profiles ps-len+ps-up) on given=['ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}] | nada | igual |
| `c12-pgsx2-two` | pgsx2 (slice s by pattern ab with its own extension min 1, profiles ps-len+ps-up) on given=['cd', 'ab'], _given=[{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}, null] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_Minimum` @ `Patient.name[0].given[1]` | igual |
| `c12-pnhs-off-none` | pnhs (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"]}] | err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhs-off-sub` | pnhs (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}]}] | nada | igual |
| `c12-pnhs-off-oth` | pnhs (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["abcd"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0].given[0].extension[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhs-usual` | pnhs (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]` | igual |
| `c12-pnhs-both` | pnhs (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}, {"use": "official", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]`<br>err `Validation_VAL_Profile_Minimum_SLICE` @ `Patient.name[1].given[0]` | igual |
| `c12-pnhs2-off-none` | pnhs2 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhs2-off-sub` | pnhs2 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}]}] | warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhs2-off-oth` | pnhs2 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["abcd"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}]}] | err `Validation_VAL_Profile_NoMatch` @ `Patient.name[0].given[0]`<br>err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]`<br>err `ps-len#len-1` @ `Patient.name[0].given[0]` | igual |
| `c12-pnhs2-usual` | pnhs2 (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]` | igual |
| `c12-pnhs2-both` | pnhs2 (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}, {"use": "official", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]`<br>warn `Validation_VAL_Profile_MultipleMatches` @ `Patient.name[1].given[0]` | igual |
| `c12-pnhs3-off-none` | pnhs3 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"]}] | nada | igual |
| `c12-pnhs3-off-sub` | pnhs3 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["ab"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-req", "valueString": "q"}]}]}] | nada | igual |
| `c12-pnhs3-off-oth` | pnhs3 (Patient.name sliced by use; the official one has a given profile) on [{"use": "official", "given": ["abcd"], "_given": [{"extension": [{"url": "http://example.org/fhir/StructureDefinition/ext-other", "valueString": "q"}]}]}] | err `Validation_VAL_Profile_NotSlice` @ `Patient.name[0].given[0].extension[0]` | igual |
| `c12-pnhs3-usual` | pnhs3 (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]` | igual |
| `c12-pnhs3-both` | pnhs3 (Patient.name sliced by use; the official one has a given profile) on [{"use": "usual", "given": ["ab"]}, {"use": "official", "given": ["ab"]}] | info `Details_for__matching_against_Profile_` @ `Patient.name[0]` | igual |
