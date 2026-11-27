#!/usr/bin/env python3
"""Sondas g1 a g4 de la ronda 2 (antes solo en el scratchpad). Reusa la cabecera de gen_x2.py.
Uso: python3 -I named_g.py <gen_x2.py> <fhirVersion> (desde la carpeta de salida)."""
import json, os, sys
exec(open(sys.argv[1]).read().split("# --- perfil derivado")[0].replace('V = sys.argv[1] if len(sys.argv) > 1 else "4.0.1"', "V=%r" % sys.argv[2]))
IDT=lambda p:{"type":[{"code":"Identifier","profile":[B+p]}]}
# g1: el path ignorado cambia el tipo del foco de la invariante
K="($this is FHIR.string).not()"
T("g1-t","Identifier",[el("Identifier.value",**inv("gk",K))])
P("g1",[el("Patient.identifier",**IDT("g1-t")),el("Patient.identifier.value",**inv("gk",K))])
I("bad-g1","g1",{"identifier":[{"value":"x"}]})
# g3: forma D con recurso sostenido: el perfil derivado pide un perfil al recurso de la entrada
T2=lambda sid,typ,diff,**k: put("ig",sid,sd(sid,typ,diff,**k))
T2("q-pat","Patient",[el("Patient.birthDate",fixedDate="2000-01-01")])
EN={"discriminator":[{"type":"value","path":"fullUrl"}],"rules":"open"}
P("b0",[el("Bundle.entry",slicing=EN),el("Bundle.entry:a",sliceName="a"),el("Bundle.entry:a.id",short="i"),el("Bundle.entry:a.fullUrl",fixedUri="urn:uuid:11111111-1111-4111-8111-111111111111")],typ="Bundle")
P("b1",[el("Bundle.entry.resource",type=[{"code":"Patient","profile":[B+"q-pat"]}])],base=B+"b0",typ="Bundle")
BD=lambda d:{"type":"collection","entry":[{"fullUrl":"urn:uuid:11111111-1111-4111-8111-111111111111","resource":{"resourceType":"Patient","id":"p1","birthDate":d}}]}
I("bad-g3","b1",BD("1999-01-01"),rt="Bundle"); I("ok-g3","b1",BD("2000-01-01"),rt="Bundle")
# Control (25-11-2026, plan §9): HL7 no genera el snapshot de b1 («Same id 'Bundle.entry:a.extension' on multiple
# elements»). El mismo diferencial en un solo perfil da el valor esperado de bad-g3 y ok-g3.
P("ctl-b1",[el("Bundle.entry",slicing=EN),el("Bundle.entry.resource",type=[{"code":"Patient","profile":[B+"q-pat"]}]),el("Bundle.entry:a",sliceName="a"),el("Bundle.entry:a.id",short="i"),
            el("Bundle.entry:a.fullUrl",fixedUri="urn:uuid:11111111-1111-4111-8111-111111111111")],typ="Bundle")
I("ctl-bad-g3","ctl-b1",BD("1999-01-01"),rt="Bundle"); I("ctl-ok-g3","ctl-b1",BD("2000-01-01"),rt="Bundle")
# g3b: contained
# c0 dividía por id (Patient.contained:a.id), y HL7 no despliega los hijos de Resource: el snapshot fallaba. Se divide por tipo.
P("c0",[el("Patient.contained",slicing={"discriminator":[{"type":"type","path":"$this"}],"rules":"open"}),el("Patient.contained:a",sliceName="a",type=[{"code":"Patient"}])])
P("c1",[el("Patient.contained",type=[{"code":"Patient","profile":[B+"q-pat"]}])],base=B+"c0")
I("bad-g3c","c1",{"contained":[{"resourceType":"Patient","id":"c1","birthDate":"1999-01-01"}],"link":[{"type":"seealso","other":{"reference":"#c1"}}]})
# g4 (extensiones con source distinto y sombra por perfil de tipo) se quitó: en R5 HL7 rechaza el snapshot de g4-t, porque
# la clave gs tiene otra expresión que la de la raíz de g4-s. Es el caso «misma clave y otra expresión» que el plan deja
# fuera del alcance (plan 2026-11-18, §2, «Fuera del alcance»), y así queda clasificado bad-g4.
