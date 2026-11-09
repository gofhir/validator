#!/bin/bash
# mkver.sh <probe-dir> <name> : creates v4.3.0/ v5.0.0/ with ig/, package.json, probe-<name>.tgz; also R4 tgz if missing
set -e
D=$1; N=$2
for v in 4.0.1 4.3.0 5.0.0; do
  if [ $v = 4.0.1 ]; then O=$D; IG=$D/ig; else O=$D/v$v; IG=$O/ig; mkdir -p $IG; rm -f $IG/*.json
    for f in $D/ig/*.json; do python3 -c "
import json,sys;d=json.load(open(sys.argv[1]))
if 'fhirVersion' in d: d['fhirVersion']=sys.argv[3]
open(sys.argv[2],'w').write(json.dumps(d))" $f $IG/$(basename $f) $v; done; fi
  if [ $v = 4.0.1 ] && [ -f $O/probe-$N.tgz ]; then continue; fi
  T=$(mktemp -d); mkdir $T/package; cp $IG/*.json $T/package/
  echo "{\"name\":\"probe.scope\",\"version\":\"1.0.0\",\"fhirVersions\":[\"$v\"]}" > $T/package/package.json
  [ $v != 4.0.1 ] && cp $T/package/package.json $O/package.json
  tar czf $O/probe-$N.tgz -C $T package; rm -rf $T
done
