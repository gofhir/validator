"""python3 -I grid.py summary-<ver>.txt : e2 grid, ok = no error/warning issue for the instance"""
import sys,re,collections
cur=None; res={}
for l in open(sys.argv[1]):
    if l.startswith('## '): cur=l[3:].strip(); res[cur]=0
    elif cur and not l.startswith('  (none)'): res[cur]+=1
ks=['rempty','rexists','rid1','rid2','rro1','rrid0','rrid1','rval','rrval']
print('%-8s'%'', *['%-8s'%k for k in ks])
for kind in 'st':
    for w in 'xbn':
        print('e2%s %s  '%(kind,w), *['%-8s'%('ok' if res.get('e2%s%s%s'%(kind,k,w))==0 else 'FAIL') for k in ks])
for w in ('h','hn'):
    print('e2%s    '%w, *['%-8s'%('ok' if res.get('e2%s%s'%(w,k))==0 else 'FAIL') for k in ks])
