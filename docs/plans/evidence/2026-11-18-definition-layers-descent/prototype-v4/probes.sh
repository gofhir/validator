#!/bin/bash
# Corre gv6 sobre las sondas de las rondas y las compara con HL7.
R=$1; E="RV5_SNAP=strict RV5_OWNFIRST=1 RV5_TYPEKEY=1"
run(){ n=$1; d=$2; t=$3; v=$4; (cd "$d" && env $E "$R/rv6/gv6" -package-file "$t" -no-download -tx n/a -version $v -output json inst/*.json > "$R/rv6/o/$n.raw" 2>"$R/rv6/o/$n.err"); python3 -I "$R/rv3/x/gs.py" "$R/rv6/o/$n.raw" > "$R/rv6/o/$n.txt"; python3 -I "$R/rv5/cmp.py" "$d/hl7.txt" "$R/rv6/o/$n.txt" > "$R/rv6/o/$n.cmp"; echo "$n: $(tail -1 "$R/rv6/o/$n.cmp")"; }
run p $R/rv5/p/r4 $R/rv5/p/r4.tgz 4.0.1
run g $R/rv4/g/r4 $R/rv4/g/r4.tgz 4.0.1
run x2r4 $R/rv4/x2/r4 $R/rv5/x2-r4.tgz 4.0.1
run x2r4b $R/rv4/x2/r4b $R/rv5/x2-r4b.tgz 4.3.0
run x2r5 $R/rv4/x2/r5 $R/rv5/x2-r5.tgz 5.0.0
