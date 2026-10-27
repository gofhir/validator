#!/bin/zsh
# Corre cada mutación de mutations.py sobre la copia $1 y escribe killed o SURVIVED por mutación. Los
# archivos mutados se restauran desde la copia intacta $2.
set -u
copy=$1; orig=$2; here=${0:A:h}
cd $copy
for m in $(python3 -c "import sys; sys.path.insert(0,'$here'); import mutations; print(' '.join(mutations.M))"); do
  f=$(python3 -c "import sys; sys.path.insert(0,'$here'); import mutations; print(mutations.M['$m'][0])")
  python3 -c "
import sys; sys.path.insert(0,'$here'); import mutations
f,a,b=mutations.M['$m']; s=open(f).read()
assert s.count(a)==1, ('$m', 'fragment not found once')
open(f,'w').write(s.replace(a,b))" || { echo "$m NOT-APPLIED"; continue; }
  if ! go build ./... 2>/dev/null; then
    echo "$m BUILD-FAILED"
  elif go test ./pkg/registry -run 'TestMerge|TestElementDefinitionURL|TestApplyDifferential' -count=1 >/dev/null 2>&1 &&
       go test ./pkg/validator -run TestFixedValueOfAProfileThatChangesItsType -count=1 >/dev/null 2>&1; then
    echo "$m SURVIVED"
  else
    echo "$m killed"
  fi
  cp $orig/$f $f
done
