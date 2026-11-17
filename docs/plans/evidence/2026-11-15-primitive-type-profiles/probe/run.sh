#!/bin/sh
# usage: run.sh <fhir version>   (HL7 validator_cli 7.0.4, -tx n/a)
v=$1; ig=ig-$v
java -jar ~/tools/validator_cli.jar inst/*.json -version $v -ig $ig -tx n/a -output hl7-$v-7.0.4.json > hl7-console-$v.txt 2>&1
