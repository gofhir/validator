#!/bin/bash
# uso: run.sh <dir> <version> [hl7]
d=$1; v=$2; cd "$(dirname "$0")"; mkdir -p $d; cd $d; python3 -I ../gen.py $v; python3 -I ../pk.py ig ../$d.tgz $v
if [ -n "$3" ]; then while [ "$(ps -ax -o command | grep -c '[j]ava ')" -ge 2 ]; do sleep 5; done
 java -Duser.timezone=UTC -jar ~/tools/validator_cli.jar inst/*.json -version $v -ig ig -tx n/a -output hl7.json > console.txt 2>&1; python3 -I ../h.py hl7.json inst > hl7.txt; fi
g(){ env $2 ../../$3 -package-file ../$d.tgz -no-download -version $v -output json inst/*.json > $1.raw 2>/dev/null; python3 -I ../gs.py $1.raw > $1.txt; }
g base X=1 gv-base; g proto RV3_PROTO=1 gv-proto; g filt "RV3_PROTO=1 RV3_FILTER=1" gv-proto
