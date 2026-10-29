import sys,re,collections,json
Q=re.compile(r'"((?:[^"\\]|\\.)*)"')
def items(s): return [json.loads('"'+m+'"') for m in Q.findall(s)]
lines=open(sys.argv[1]).read().split('\n')
only=0;other=collections.Counter();ex={}
i=0
while i<len(lines):
    if lines[i].startswith('DIFF'):
        miss=items(lines[i+1].split('no unknown at ',1)[1])
        cd=collections.Counter(items(lines[i+2].split('deleted ',1)[1]));ca=collections.Counter(items(lines[i+3].split('altered ',1)[1]))
        if cd==ca: only+=1
        else:
            for sign,c in (('+',ca-cd),('-',cd-ca)):
                for x in c:
                    p=x.split(' | '); k=sign+p[1]+' '+p[2][:60]; other[k]+=1; ex.setdefault(k,lines[i].split('package/')[1][:140])
        i+=4
    else: i+=1
print('only missing unknown:',only)
for k,v in other.most_common(): print(v,k,'|',ex[k])
