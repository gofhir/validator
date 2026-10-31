package registry

import (
	"context"
	"testing"
	"time"
)

// Definiciones de TestChoiceTypeBaseElement (gofhir/validator#183). La base de un elemento es el hijo
// de la base de su padre, según lo que esa base define: una contentReference «bring[s] across all the
// rules» del elemento que nombra (ElementDefinition.contentReference), y un slice deriva del elemento
// que corta (profiling.html#slicing). Los modelos lógicos derivan de Base, que no existe en R4 ni en
// R4B: ahí la cadena de definedIn termina en el modelo, que ya define todo lo que se busca.
const (
	choiceBaseURL = "http://example.org/base/"

	// Caso 1: en el modelo LT, Timing.repeat es de tipo Timing (el del núcleo, cuyo repeat.bounds[x] es
	// Duration|Range|Period) y Timing.repeat.item es una contentReference a #Timing.repeat, cuyo
	// bounds[x] es Coding|date. LTDiff restringe Timing.repeat.item.bounds[x] solo con un diferencial (el
	// generador reemplaza la referencia por el tipo del destino); LTKept publica un snapshot que la
	// conserva.
	ltModel = `{"resourceType":"StructureDefinition","id":"lt","url":"http://example.org/base/LT","name":"LT","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/LT","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"Timing","path":"Timing","base":{"path":"Timing","min":0,"max":"*"}},
 {"id":"Timing.repeat","path":"Timing.repeat","min":0,"max":"1","base":{"path":"Timing.repeat","min":0,"max":"1"},"type":[{"code":"Timing"}]},
 {"id":"Timing.repeat.bounds[x]","path":"Timing.repeat.bounds[x]","min":0,"max":"1","base":{"path":"Timing.repeat.bounds[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"date"}]},
 {"id":"Timing.repeat.item","path":"Timing.repeat.item","min":0,"max":"*","base":{"path":"Timing.repeat.item","min":0,"max":"*"},"contentReference":"#Timing.repeat"}]}}`
	ltDiff = `{"resourceType":"StructureDefinition","id":"lt-diff","url":"http://example.org/base/LTDiff","name":"LTDiff","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/LT","baseDefinition":"http://example.org/base/LT","derivation":"constraint","differential":{"element":[
 {"id":"Timing","path":"Timing"},
 {"id":"Timing.repeat.item.bounds[x]","path":"Timing.repeat.item.bounds[x]","type":[{"code":"Coding"}]}]}}`
	ltKept = `{"resourceType":"StructureDefinition","id":"lt-kept","url":"http://example.org/base/LTKept","name":"LTKept","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/LT","baseDefinition":"http://example.org/base/LT","derivation":"constraint","snapshot":{"element":[
 {"id":"Timing","path":"Timing","base":{"path":"Timing","min":0,"max":"*"}},
 {"id":"Timing.repeat","path":"Timing.repeat","min":0,"max":"1","base":{"path":"Timing.repeat","min":0,"max":"1"},"type":[{"code":"Timing"}]},
 {"id":"Timing.repeat.bounds[x]","path":"Timing.repeat.bounds[x]","min":0,"max":"1","base":{"path":"Timing.repeat.bounds[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"date"}]},
 {"id":"Timing.repeat.item","path":"Timing.repeat.item","min":0,"max":"*","base":{"path":"Timing.repeat.item","min":0,"max":"*"},"contentReference":"#Timing.repeat"},
 {"id":"Timing.repeat.item.bounds[x]","path":"Timing.repeat.item.bounds[x]","min":0,"max":"1","base":{"path":"Timing.repeat.bounds[x]","min":0,"max":"1"},"type":[{"code":"Coding"}]}]}}`

	// Caso 2 y test 6: un snapshot publicado cuyo slice de extensión no repite type, y un reslice que
	// tampoco. La base de Patient.extension:e.value[x] es Extension.value[x], que declara Coding.
	// SliceDiff es el mismo perfil solo con diferencial.
	sliceNoType = `{"resourceType":"StructureDefinition","id":"slice-notype","url":"http://example.org/base/SliceNoType","name":"SliceNoType","status":"active","kind":"resource","abstract":false,"type":"Patient","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","derivation":"constraint","snapshot":{"element":[
 {"id":"Patient","path":"Patient","base":{"path":"Patient","min":0,"max":"*"}},
 {"id":"Patient.extension","path":"Patient.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"base":{"path":"DomainResource.extension","min":0,"max":"*"},"type":[{"code":"Extension"}]},
 {"id":"Patient.extension:e","path":"Patient.extension","sliceName":"e","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"},"min":0,"max":"*","base":{"path":"DomainResource.extension","min":0,"max":"*"}},
 {"id":"Patient.extension:e.value[x]","path":"Patient.extension.value[x]","min":1,"max":"1","base":{"path":"Extension.value[x]","min":0,"max":"1"},"type":[{"code":"string"}]},
 {"id":"Patient.extension:e/r","path":"Patient.extension","sliceName":"e/r","min":0,"max":"1","base":{"path":"DomainResource.extension","min":0,"max":"*"}},
 {"id":"Patient.extension:e/r.value[x]","path":"Patient.extension.value[x]","min":1,"max":"1","type":[{"code":"string"}]}]}}`
	sliceDiff = `{"resourceType":"StructureDefinition","id":"slice-diff","url":"http://example.org/base/SliceDiff","name":"SliceDiff","status":"active","kind":"resource","abstract":false,"type":"Patient","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Patient","derivation":"constraint","differential":{"element":[
 {"id":"Patient","path":"Patient"},
 {"id":"Patient.extension","path":"Patient.extension","slicing":{"discriminator":[{"type":"value","path":"url"}],"rules":"open"}},
 {"id":"Patient.extension:e","path":"Patient.extension","sliceName":"e","min":0,"max":"1"},
 {"id":"Patient.extension:e.value[x]","path":"Patient.extension.value[x]","min":1,"max":"1","type":[{"code":"string"}]}]}}`

	// Caso 3: A.item es una contentReference a otra definición (U#U.grp), cuyo value[x] es Coding|date.
	// El perfil AP conserva la referencia y restringe A.item.value[x] a Coding.
	uModel = `{"resourceType":"StructureDefinition","id":"u","url":"http://example.org/base/U","name":"U","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/U","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"U","path":"U","base":{"path":"U","min":0,"max":"*"}},
 {"id":"U.grp","path":"U.grp","min":0,"max":"*","base":{"path":"U.grp","min":0,"max":"*"},"type":[{"code":"BackboneElement"}]},
 {"id":"U.grp.value[x]","path":"U.grp.value[x]","min":0,"max":"1","base":{"path":"U.grp.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"date"}]}]}}`
	aModel = `{"resourceType":"StructureDefinition","id":"a","url":"http://example.org/base/A","name":"A","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/A","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"A","path":"A","base":{"path":"A","min":0,"max":"*"}},
 {"id":"A.item","path":"A.item","min":0,"max":"*","base":{"path":"A.item","min":0,"max":"*"},"contentReference":"http://example.org/base/U#U.grp"}]}}`
	aProfile = `{"resourceType":"StructureDefinition","id":"ap","url":"http://example.org/base/AP","name":"AP","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/A","baseDefinition":"http://example.org/base/A","derivation":"constraint","snapshot":{"element":[
 {"id":"A","path":"A","base":{"path":"A","min":0,"max":"*"}},
 {"id":"A.item","path":"A.item","min":0,"max":"*","base":{"path":"A.item","min":0,"max":"*"},"contentReference":"http://example.org/base/U#U.grp"},
 {"id":"A.item.value[x]","path":"A.item.value[x]","min":0,"max":"1","base":{"path":"U.grp.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"}]}]}}`

	// Test 2: un perfil de Questionnaire con snapshot propio, sin base, que conserva la contentReference
	// de item.item (por URL). La base de answerOption.value[x] es la del núcleo, que declara date y no
	// dateTime en R4, R4B y R5. Un snapshot sin base viola sdf-8b; se deja a propósito porque solo sin base
	// el test distingue la búsqueda estructural de base.path.
	qNoBase = `{"resourceType":"StructureDefinition","id":"q-nobase","url":"http://example.org/base/QNoBase","name":"QNoBase","status":"active","kind":"resource","abstract":false,"type":"Questionnaire","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Questionnaire","derivation":"constraint","snapshot":{"element":[
 {"id":"Questionnaire","path":"Questionnaire"},
 {"id":"Questionnaire.item","path":"Questionnaire.item","min":0,"max":"*","type":[{"code":"BackboneElement"}]},
 {"id":"Questionnaire.item.item","path":"Questionnaire.item.item","min":0,"max":"*","contentReference":"http://hl7.org/fhir/StructureDefinition/Questionnaire#Questionnaire.item"},
 {"id":"Questionnaire.item.item.answerOption","path":"Questionnaire.item.item.answerOption","min":0,"max":"*","type":[{"code":"BackboneElement"}]},
 {"id":"Questionnaire.item.item.answerOption.value[x]","path":"Questionnaire.item.item.answerOption.value[x]","min":1,"max":"1","type":[{"code":"Coding"}]}]}}`

	// Test 3: en el modelo M, M.grp es de tipo Extension y M.item una contentReference a #M.grp. El hijo
	// M.item.value[x] no lo declara el destino: llega por su tipo (Extension.value[x], que declara
	// Coding). MKept conserva la referencia, sin base. Solo con diferencial el generador no despliega
	// M.item.value[x] (no despliega el tipo del destino de una referencia): queda para el generador.
	mModel = `{"resourceType":"StructureDefinition","id":"m","url":"http://example.org/base/M","name":"M","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/M","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"M","path":"M","base":{"path":"M","min":0,"max":"*"}},
 {"id":"M.grp","path":"M.grp","min":0,"max":"*","base":{"path":"M.grp","min":0,"max":"*"},"type":[{"code":"Extension"}]},
 {"id":"M.item","path":"M.item","min":0,"max":"*","base":{"path":"M.item","min":0,"max":"*"},"contentReference":"#M.grp"}]}}`
	mKept = `{"resourceType":"StructureDefinition","id":"m-kept","url":"http://example.org/base/MKept","name":"MKept","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/M","baseDefinition":"http://example.org/base/M","derivation":"constraint","snapshot":{"element":[
 {"id":"M","path":"M"},
 {"id":"M.grp","path":"M.grp","min":0,"max":"*","type":[{"code":"Extension"}]},
 {"id":"M.item","path":"M.item","min":0,"max":"*","contentReference":"#M.grp"},
 {"id":"M.item.value[x]","path":"M.item.value[x]","min":1,"max":"1","type":[{"code":"string"}]}]}}`

	// Test 4: en el modelo S (versión 1.0), S.other y S.pinned son contentReferences al slice
	// S.agent:a, por id y por URL con versión. El value[x] del slice es Coding|date; el del elemento
	// cortado, S.agent.value[x], también admite string: el destino es el slice, no su ruta.
	sModel = `{"resourceType":"StructureDefinition","id":"s","url":"http://example.org/base/S","version":"1.0","name":"S","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/S","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"S","path":"S","base":{"path":"S","min":0,"max":"*"}},
 {"id":"S.agent","path":"S.agent","min":0,"max":"*","slicing":{"discriminator":[{"type":"type","path":"value"}],"rules":"open"},"base":{"path":"S.agent","min":0,"max":"*"},"type":[{"code":"BackboneElement"}]},
 {"id":"S.agent.value[x]","path":"S.agent.value[x]","min":0,"max":"1","base":{"path":"S.agent.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"date"},{"code":"string"}]},
 {"id":"S.agent:a","path":"S.agent","sliceName":"a","min":0,"max":"1","base":{"path":"S.agent","min":0,"max":"*"},"type":[{"code":"BackboneElement"}]},
 {"id":"S.agent:a.value[x]","path":"S.agent.value[x]","min":0,"max":"1","base":{"path":"S.agent.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"date"}]},
 {"id":"S.other","path":"S.other","min":0,"max":"*","base":{"path":"S.other","min":0,"max":"*"},"contentReference":"#S.agent:a"},
 {"id":"S.pinned","path":"S.pinned","min":0,"max":"*","base":{"path":"S.pinned","min":0,"max":"*"},"contentReference":"http://example.org/base/S|1.0#S.agent:a"}]}}`
	sKept = `{"resourceType":"StructureDefinition","id":"s-kept","url":"http://example.org/base/SKept","name":"SKept","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/S","baseDefinition":"http://example.org/base/S","derivation":"constraint","snapshot":{"element":[
 {"id":"S","path":"S"},
 {"id":"S.other","path":"S.other","min":0,"max":"*","contentReference":"#S.agent:a"},
 {"id":"S.other.value[x]","path":"S.other.value[x]","min":0,"max":"1","type":[{"code":"Coding"}]},
 {"id":"S.pinned","path":"S.pinned","min":0,"max":"*","contentReference":"http://example.org/base/S|1.0#S.agent:a"},
 {"id":"S.pinned.value[x]","path":"S.pinned.value[x]","min":0,"max":"1","type":[{"code":"Coding"}]}]}}`

	// Test 5, entrada incompleta: el slice de elección effective[x]:effectiveTiming no declara tipo y
	// effective[x] tiene varios. La especificación no ata el nombre del slice a un tipo (profiling.html,
	// slicing) ni dice qué hacer en este caso: no se adivina, y repeat.bounds[x] conserva solo sus tipos
	// propios (Period). El caso conforme, un slice que declara Timing, es el test 7 (EffTyped).
	effSlice = `{"resourceType":"StructureDefinition","id":"eff-slice","url":"http://example.org/base/EffSlice","name":"EffSlice","status":"active","kind":"resource","abstract":false,"type":"Observation","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation","derivation":"constraint","snapshot":{"element":[
 {"id":"Observation","path":"Observation","base":{"path":"Observation","min":0,"max":"*"}},
 {"id":"Observation.effective[x]","path":"Observation.effective[x]","min":0,"max":"1","slicing":{"discriminator":[{"type":"type","path":"$this"}],"rules":"open"},"base":{"path":"Observation.effective[x]","min":0,"max":"1"},"type":[{"code":"dateTime"},{"code":"Period"},{"code":"Timing"},{"code":"instant"}]},
 {"id":"Observation.effective[x]:effectiveTiming","path":"Observation.effective[x]","sliceName":"effectiveTiming","min":0,"max":"1","base":{"path":"Observation.effective[x]","min":0,"max":"1"}},
 {"id":"Observation.effective[x]:effectiveTiming.repeat","path":"Observation.effective[x].repeat","min":0,"max":"1","base":{"path":"Timing.repeat","min":0,"max":"1"},"type":[{"code":"Element"}]},
 {"id":"Observation.effective[x]:effectiveTiming.repeat.bounds[x]","path":"Observation.effective[x].repeat.bounds[x]","min":0,"max":"1","base":{"path":"Timing.repeat.bounds[x]","min":0,"max":"1"},"type":[{"code":"Period"}]}]}}`

	// Test 7: los tipos del elemento del perfil mandan sobre los de su base. EffTiming restringe
	// effective[x] a Timing sin cortarlo; EffTyped corta un slice que declara Timing. Sin base.
	effTiming = `{"resourceType":"StructureDefinition","id":"eff-timing","url":"http://example.org/base/EffTiming","name":"EffTiming","status":"active","kind":"resource","abstract":false,"type":"Observation","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation","derivation":"constraint","snapshot":{"element":[
 {"id":"Observation","path":"Observation"},
 {"id":"Observation.effective[x]","path":"Observation.effective[x]","min":0,"max":"1","type":[{"code":"Timing"}]},
 {"id":"Observation.effective[x].repeat","path":"Observation.effective[x].repeat","min":0,"max":"1","type":[{"code":"Element"}]},
 {"id":"Observation.effective[x].repeat.bounds[x]","path":"Observation.effective[x].repeat.bounds[x]","min":0,"max":"1","type":[{"code":"Period"}]}]}}`
	effTyped = `{"resourceType":"StructureDefinition","id":"eff-typed","url":"http://example.org/base/EffTyped","name":"EffTyped","status":"active","kind":"resource","abstract":false,"type":"Observation","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Observation","derivation":"constraint","snapshot":{"element":[
 {"id":"Observation","path":"Observation"},
 {"id":"Observation.effective[x]","path":"Observation.effective[x]","min":0,"max":"1","slicing":{"discriminator":[{"type":"type","path":"$this"}],"rules":"open"},"type":[{"code":"dateTime"},{"code":"Period"},{"code":"Timing"},{"code":"instant"}]},
 {"id":"Observation.effective[x]:effectiveTiming","path":"Observation.effective[x]","sliceName":"effectiveTiming","min":0,"max":"1","type":[{"code":"Timing"}]},
 {"id":"Observation.effective[x]:effectiveTiming.repeat","path":"Observation.effective[x].repeat","min":0,"max":"1","type":[{"code":"Element"}]},
 {"id":"Observation.effective[x]:effectiveTiming.repeat.bounds[x]","path":"Observation.effective[x].repeat.bounds[x]","min":0,"max":"1","type":[{"code":"Period"}]}]}}`

	// Test 8: el perfil DocNoBase despliega el tipo T (modelo lógico, por URL), cuyo T.item.item es una
	// contentReference interna a #T.item (value[x] Coding|date). Sin base: solo la estructura lleva a
	// T.item.value[x].
	tType = `{"resourceType":"StructureDefinition","id":"t","url":"http://example.org/base/T","name":"T","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/T","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"T","path":"T","base":{"path":"T","min":0,"max":"*"}},
 {"id":"T.item","path":"T.item","min":0,"max":"*","base":{"path":"T.item","min":0,"max":"*"},"type":[{"code":"BackboneElement"}]},
 {"id":"T.item.value[x]","path":"T.item.value[x]","min":0,"max":"1","base":{"path":"T.item.value[x]","min":0,"max":"1"},"type":[{"code":"Coding"},{"code":"date"}]},
 {"id":"T.item.item","path":"T.item.item","min":0,"max":"*","base":{"path":"T.item.item","min":0,"max":"*"},"contentReference":"#T.item"}]}}`
	docModel = `{"resourceType":"StructureDefinition","id":"doc","url":"http://example.org/base/Doc","name":"Doc","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/Doc","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"Doc","path":"Doc","base":{"path":"Doc","min":0,"max":"*"}},
 {"id":"Doc.t","path":"Doc.t","min":0,"max":"1","base":{"path":"Doc.t","min":0,"max":"1"},"type":[{"code":"http://example.org/base/T"}]}]}}`
	docNoBase = `{"resourceType":"StructureDefinition","id":"doc-nobase","url":"http://example.org/base/DocNoBase","name":"DocNoBase","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/Doc","baseDefinition":"http://example.org/base/Doc","derivation":"constraint","snapshot":{"element":[
 {"id":"Doc","path":"Doc"},
 {"id":"Doc.t","path":"Doc.t","min":0,"max":"1","type":[{"code":"http://example.org/base/T"}]},
 {"id":"Doc.t.item","path":"Doc.t.item","min":0,"max":"*","type":[{"code":"BackboneElement"}]},
 {"id":"Doc.t.item.item","path":"Doc.t.item.item","min":0,"max":"*"},
 {"id":"Doc.t.item.item.value[x]","path":"Doc.t.item.item.value[x]","min":0,"max":"1","type":[{"code":"Coding"}]}]}}`

	// Test 9: X.item apunta a un id que no existe y X.ref a una definición que no está cargada. Sin
	// base: el elemento no tiene más tipos que los suyos.
	xModel = `{"resourceType":"StructureDefinition","id":"x","url":"http://example.org/base/X","name":"X","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/X","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"X","path":"X","base":{"path":"X","min":0,"max":"*"}},
 {"id":"X.item","path":"X.item","min":0,"max":"*","base":{"path":"X.item","min":0,"max":"*"},"contentReference":"#X.nothing"},
 {"id":"X.ref","path":"X.ref","min":0,"max":"*","base":{"path":"X.ref","min":0,"max":"*"},"contentReference":"http://example.org/base/Missing#Missing.grp"}]}}`
	xKept = `{"resourceType":"StructureDefinition","id":"x-kept","url":"http://example.org/base/XKept","name":"XKept","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/X","baseDefinition":"http://example.org/base/X","derivation":"constraint","snapshot":{"element":[
 {"id":"X","path":"X"},
 {"id":"X.item","path":"X.item","min":0,"max":"*","contentReference":"#X.nothing"},
 {"id":"X.item.value[x]","path":"X.item.value[x]","min":0,"max":"1","type":[{"code":"string"}]},
 {"id":"X.ref","path":"X.ref","min":0,"max":"*","contentReference":"http://example.org/base/Missing#Missing.grp"},
 {"id":"X.ref.value[x]","path":"X.ref.value[x]","min":0,"max":"1","type":[{"code":"string"}]}]}}`

	// Test 10: la base de NotLoadedBase no está cargada; Patient.extension es de tipo Extension, así que
	// su value[x] deriva de Extension.value[x], que declara Coding.
	notLoaded = `{"resourceType":"StructureDefinition","id":"not-loaded-base","url":"http://example.org/base/NotLoadedBase","name":"NotLoadedBase","status":"active","kind":"resource","abstract":false,"type":"Patient","baseDefinition":"http://example.org/base/NotLoaded","derivation":"constraint","snapshot":{"element":[
 {"id":"Patient","path":"Patient"},
 {"id":"Patient.extension","path":"Patient.extension","min":0,"max":"*","type":[{"code":"Extension"}]},
 {"id":"Patient.extension.value[x]","path":"Patient.extension.value[x]","min":0,"max":"1","type":[{"code":"string"}]}]}}`

	// Test 11: al snapshot de QOrphan le falta Questionnaire.item.item, así que su answerOption cuelga
	// de Questionnaire.item (ElementTree, orphan) y la estructura no llega a la base: la da base.path.
	qOrphan = `{"resourceType":"StructureDefinition","id":"q-orphan","url":"http://example.org/base/QOrphan","name":"QOrphan","status":"active","kind":"resource","abstract":false,"type":"Questionnaire","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Questionnaire","derivation":"constraint","snapshot":{"element":[
 {"id":"Questionnaire","path":"Questionnaire","base":{"path":"Questionnaire","min":0,"max":"*"}},
 {"id":"Questionnaire.item","path":"Questionnaire.item","min":0,"max":"*","base":{"path":"Questionnaire.item","min":0,"max":"*"},"type":[{"code":"BackboneElement"}]},
 {"id":"Questionnaire.item.item.answerOption","path":"Questionnaire.item.item.answerOption","min":0,"max":"*","base":{"path":"Questionnaire.item.answerOption","min":0,"max":"*"},"type":[{"code":"BackboneElement"}]},
 {"id":"Questionnaire.item.item.answerOption.value[x]","path":"Questionnaire.item.item.answerOption.value[x]","min":1,"max":"1","base":{"path":"Questionnaire.item.answerOption.value[x]","min":1,"max":"1"},"type":[{"code":"Coding"}]}]}}`

	// Una contentReference en ciclo (C.a → #C.b → #C.a) no tiene destino que defina a C.a.value[x].
	cModel = `{"resourceType":"StructureDefinition","id":"c","url":"http://example.org/base/C","name":"C","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/C","baseDefinition":"http://hl7.org/fhir/StructureDefinition/Base","derivation":"specialization","snapshot":{"element":[
 {"id":"C","path":"C","base":{"path":"C","min":0,"max":"*"}},
 {"id":"C.a","path":"C.a","min":0,"max":"*","base":{"path":"C.a","min":0,"max":"*"},"contentReference":"#C.b"},
 {"id":"C.b","path":"C.b","min":0,"max":"*","base":{"path":"C.b","min":0,"max":"*"},"contentReference":"#C.a"}]}}`
	cKept = `{"resourceType":"StructureDefinition","id":"c-kept","url":"http://example.org/base/CKept","name":"CKept","status":"active","kind":"logical","abstract":false,"type":"http://example.org/base/C","baseDefinition":"http://example.org/base/C","derivation":"constraint","snapshot":{"element":[
 {"id":"C","path":"C"},
 {"id":"C.a","path":"C.a","min":0,"max":"*","contentReference":"#C.b"},
 {"id":"C.a.value[x]","path":"C.a.value[x]","min":0,"max":"1","type":[{"code":"string"}]}]}}`
)

// La base de un elemento de elección sigue las contentReferences hasta su destino, los slices hasta el
// elemento que cortan y los tipos hasta su definición, en R4, R4B y R5 (gofhir/validator#183). Una
// propiedad nombra un valor si su tipo lo declara la base (formats.html#choice): un tipo que el perfil
// no permite da el tipo; uno que la base no declara, "".
func TestChoiceTypeBaseElement(t *testing.T) {
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		t.Run(version, func(t *testing.T) {
			r := sharedVersion(t, version).Clone()
			loadProfiles(t, r, ltModel, ltDiff, ltKept, sliceNoType, sliceDiff, uModel, aModel, aProfile, qNoBase,
				mModel, mKept, sModel, sKept, effSlice, effTiming, effTyped, tType, docModel, docNoBase,
				xModel, xKept, notLoaded, qOrphan)
			node := func(name, id string) *ElementNode {
				t.Helper()
				sd := r.GetByURL(choiceBaseURL + name)
				if sd == nil {
					t.Fatalf("%s not loaded", name)
				}
				if err := r.EnsureSnapshot(context.Background(), sd); err != nil {
					t.Fatalf("EnsureSnapshot(%s): %v", name, err)
				}
				n := sd.Tree().ByID(id)
				if n == nil {
					t.Fatalf("%s has no %s", name, id)
				}
				return n
			}
			ltD := node("LTDiff", "Timing.repeat.item.bounds[x]")
			ltK := node("LTKept", "Timing.repeat.item.bounds[x]")
			slice := node("SliceNoType", "Patient.extension:e.value[x]")
			reslice := node("SliceNoType", "Patient.extension:e/r.value[x]")
			sliceD := node("SliceDiff", "Patient.extension:e.value[x]")
			cross := node("AP", "A.item.value[x]")
			answer := node("QNoBase", "Questionnaire.item.item.answerOption.value[x]")
			mK := node("MKept", "M.item.value[x]")
			other := node("SKept", "S.other.value[x]")
			pinned := node("SKept", "S.pinned.value[x]")
			bounds := node("EffSlice", "Observation.effective[x]:effectiveTiming.repeat.bounds[x]")
			restricted := node("EffTiming", "Observation.effective[x].repeat.bounds[x]")
			typed := node("EffTyped", "Observation.effective[x]:effectiveTiming.repeat.bounds[x]")
			unrolled := node("DocNoBase", "Doc.t.item.item.value[x]")
			missing := node("XKept", "X.item.value[x]")
			unloaded := node("XKept", "X.ref.value[x]")
			noChain := node("NotLoadedBase", "Patient.extension.value[x]")
			orphan := node("QOrphan", "Questionnaire.item.item.answerOption.value[x]")
			for _, tt := range []struct {
				name      string
				node      *ElementNode
				key, want string
			}{
				// Caso 1: bounds[x] del destino de la contentReference, no del Timing del núcleo.
				{"case 1, differential", ltD, "boundsDate", "date"},
				{"case 1, differential", ltD, "boundsCoding", "Coding"},
				{"case 1, differential", ltD, "boundsPeriod", ""},
				{"case 1, kept reference", ltK, "boundsDate", "date"},
				{"case 1, kept reference", ltK, "boundsPeriod", ""},
				// Caso 2 y test 6: el slice sin type deriva de Patient.extension, de tipo Extension.
				{"case 2, slice without type", slice, "valueCoding", "Coding"},
				{"case 2, slice without type", slice, "valueFoo", ""},
				{"case 2, reslice without type", reslice, "valueCoding", "Coding"},
				{"case 2, differential", sliceD, "valueCoding", "Coding"},
				// Caso 3: el destino está en otra definición.
				{"case 3, reference to another definition", cross, "valueDate", "date"},
				{"case 3, reference to another definition", cross, "valueString", ""},
				// Test 2: el answerOption.value[x] del núcleo.
				{"Questionnaire without base", answer, "valueDate", "date"},
				{"Questionnaire without base", answer, "valueDateTime", ""},
				// Test 3: el destino M.grp no declara value[x]; su tipo Extension sí.
				{"child by the target's type", mK, "valueCoding", "Coding"},
				{"child by the target's type", mK, "valueFoo", ""},
				// Test 4: el destino es el slice S.agent:a, que no admite string.
				{"reference to a slice", other, "valueDate", "date"},
				{"reference to a slice", other, "valueString", ""},
				{"versioned reference to a slice", pinned, "valueDate", "date"},
				{"versioned reference to a slice", pinned, "valueString", ""},
				// Test 5: sin tipo en el slice no hay base; solo los tipos propios.
				{"choice slice without type", bounds, "boundsPeriod", "Period"},
				{"choice slice without type", bounds, "boundsDuration", ""},
				// Test 7: el tipo que declara el elemento del perfil.
				{"restricted choice", restricted, "boundsDuration", "Duration"},
				{"restricted choice", restricted, "boundsDate", ""},
				{"typed choice slice", typed, "boundsRange", "Range"},
				{"typed choice slice", typed, "boundsDate", ""},
				// Test 8: la contentReference interna del tipo desplegado.
				{"type's own reference without base", unrolled, "valueDate", "date"},
				{"type's own reference without base", unrolled, "valueString", ""},
				// Test 9: sin destino, solo los tipos propios.
				{"missing target", missing, "valueString", "string"},
				{"missing target", missing, "valueCoding", ""},
				{"unloaded target", unloaded, "valueString", "string"},
				{"unloaded target", unloaded, "valueCoding", ""},
				// Test 10: sin cadena, el tipo del elemento del perfil.
				{"base definition not loaded", noChain, "valueCoding", "Coding"},
				{"base definition not loaded", noChain, "valueFoo", ""},
				// Test 11: base.path cuando falta un elemento intermedio.
				{"orphan element", orphan, "valueDate", "date"},
				{"orphan element", orphan, "valueDateTime", ""},
			} {
				if got := r.ChoiceType(tt.node, tt.key); got != tt.want {
					t.Errorf("%s: ChoiceType(%s, %q) = %q, want %q", tt.name, tt.node.Def.ID, tt.key, got, tt.want)
				}
			}
		})
	}
}

// Una contentReference en ciclo termina: sin destino que lo defina, el elemento solo tiene sus tipos.
func TestChoiceTypeBaseElementCycle(t *testing.T) {
	for _, version := range []string{"4.0.1", "4.3.0", "5.0.0"} {
		r := sharedVersion(t, version).Clone()
		loadProfiles(t, r, cModel, cKept)
		node := r.GetByURL(choiceBaseURL + "CKept").Tree().ByID("C.a.value[x]")
		if node == nil {
			t.Fatalf("%s: CKept has no C.a.value[x]", version)
		}
		done := make(chan [2]string, 1)
		go func() {
			done <- [2]string{r.ChoiceType(node, "valueString"), r.ChoiceType(node, "valueCoding")}
		}()
		select {
		case got := <-done:
			if got != [2]string{"string", ""} {
				t.Errorf("%s: ChoiceType(valueString, valueCoding) = %q, want [string \"\"]", version, got)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: ChoiceType does not return on a reference cycle", version)
		}
	}
}
