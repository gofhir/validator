#!/bin/bash
R=/Volumes/dev/tmp/claude/claude-501/-Users-robertoaraneda-projects-personal-opensource-go-validator/11d0cc65-98da-4af5-bcc8-65c4743d06bd/scratchpad/vlplan
cd $R/rv5/real
n=$1; v=$2; pk=$3; lst=$4
files=(); while IFS= read -r l; do files+=("$l"); done < $lst
args=(-no-download -tx n/a -version $v -output json); [ -n "$pk" ] && args+=(-package "$pk")
s=$(date +%s); $R/rv4/gv-now "${args[@]}" "${files[@]}" > $n.base.out 2> $n.base.err; m=$(date +%s)
env RV5_SNAP=strict RV5_OWNFIRST=1 RV5_TYPEKEY=1 RV5_LOG=1 $R/rv5/gv5 "${args[@]}" "${files[@]}" > $n.v.out 2> $n.v.err; e=$(date +%s)
echo "$n files=${#files[@]} baseS=$((m-s)) protoS=$((e-m))"; grep RV5 $n.v.err | cut -c1-110
python3 -I $R/rv5/rep.py $n
