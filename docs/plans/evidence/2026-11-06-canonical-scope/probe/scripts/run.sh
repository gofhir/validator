#!/bin/bash
# run.sh <probe-dir> <pkgname> <names> [hl7=1]; GOFHIR=<binary>; summaries and logs go to $TMPDIR
D=$1; N=$2; NM=$3; H=${4:-1}; SC=$(cd $(dirname $0) && pwd); S=${TMPDIR:-/tmp}; G=${GOFHIR:-gofhir-validator}
cd $D
hl7(){ v=$1; if [ $v = 4.0.1 ]; then o=.; ig=ig; else o=v$v; ig=v$v/ig; fi
  java -jar ~/tools/validator_cli.jar inst/*.json -version $v -ig $ig -tx n/a -output $o/hl7-$NM-$v-7.0.4.json > $S/log-$N-$v.txt 2>&1; }
if [ $H = 1 ]; then (hl7 4.0.1; hl7 5.0.0) & hl7 4.3.0; wait; fi
for v in 4.0.1 4.3.0 5.0.0; do if [ $v = 4.0.1 ]; then o=.; else o=v$v; fi
  $G -version $v -package-file $o/probe-$N.tgz -tx n/a -output json inst/*.json 2>&1 | python3 -c "import sys;s=sys.stdin.read();sys.stdout.write(s[s.index(chr(10)+'['+chr(10))+1:])" > $o/gofhir-87c0b02-$NM-$v.json
  for k in hl7 gofhir; do f=$(ls $o/$k-*$NM-$v*.json 2>/dev/null|head -1); [ -n "$f" ] && python3 $SC/sum.py $k $f | sort > $S/sum-$N-$k-$v.txt; done
done
