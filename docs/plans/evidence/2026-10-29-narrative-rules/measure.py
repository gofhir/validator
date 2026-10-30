import json, os, sys, glob, collections, xml.etree.ElementTree as ET
OURS_E={"div","p","br","hr","h1","h2","h3","h4","h5","h6","blockquote","pre","address","ul","ol","li","dl","dt","dd","table","caption","thead","tbody","tfoot","tr","th","td","colgroup","col","span","b","i","em","strong","small","big","tt","code","samp","kbd","var","cite","dfn","abbr","acronym","q","sub","sup","center","a","img"}
OURS_COMMON={"id","class","style","title","lang","dir","xml:lang","xmlns"}
OURS_EA={"a":{"href","name","rel","target"},"img":{"src","alt","width","height"},"table":{"border","cellpadding","cellspacing","width","summary"},"td":{"colspan","rowspan","width","valign","align","headers","scope","abbr"},"th":{"colspan","rowspan","width","valign","align","headers","scope","abbr"},"col":{"span","width","align","valign"},"colgroup":{"span","width","align","valign"},"ol":{"start","type"},"blockquote":{"cite"},"q":{"cite"},"tr":{"align","valign"},"caption":{"align"},"thead":{"align","valign"},"tbody":{"align","valign"},"tfoot":{"align","valign"}}
X_E=set("a abbr acronym b big blockquote br caption cite code col colgroup dd dfn div dl dt em h1 h2 h3 h4 h5 h6 hr i img li ol p pre q samp small span strong sub sup table tbody td tfoot th thead tr tt ul var".split())
X_A=set("abbr accesskey align alt axis bgcolor border cellhalign cellpadding cellspacing cellvalign char charoff charset cite class colspan compact coords dir frame headers height href hreflang hspace id lang longdesc name nowrap rel rev rowspan rules scope shape span src start style summary tabindex title type valign value vspace width".split())
J_E=set("p br div h1 h2 h3 h4 h5 h6 a span b em i strong small big tt dfn q var abbr acronym cite blockquote hr address bdo kbd sub sup ul ol li dl dt dd pre table caption colgroup col thead tr tfoot tbody th td code samp img map area".split())
J_A=set("title style class id lang xml:lang xml:space dir accesskey tabindex span width align valign char charoff abbr axis headers scope rowspan colspan".split())
J_C=set("a.href a.name img.src img.border div.xmlns blockquote.cite q.cite a.charset a.type a.hreflang a.rel a.rev a.shape a.coords img.alt img.longdesc img.height img.width img.usemap img.ismap map.name area.shape area.coords area.href area.nohref area.alt table.summary table.width table.border table.frame table.rules table.cellspacing table.cellpadding pre.space td.nowrap".split())
NS='{http://www.w3.org/1999/xhtml}'
def name(t): return t.replace(NS,'').replace('{http://www.w3.org/XML/1998/namespace}','xml:')
def divs(o):
    if isinstance(o,dict):
        for k,v in o.items():
            if k=='div' and isinstance(v,str): yield v
            else: yield from divs(v)
    elif isinstance(o,list):
        for v in o: yield from divs(v)
for pkg in sys.argv[1:]:
    stats=collections.Counter(); bad={'ours':collections.Counter(),'xpath':collections.Counter(),'java':collections.Counter()}
    nd=0; verdict=collections.Counter()
    for f in glob.glob(os.path.join(pkg,'*.json')):
        try: d=json.load(open(f))
        except Exception: continue
        for div in divs(d):
            nd+=1
            try: root=ET.fromstring(div)
            except Exception: verdict['unparseable']+=1; continue
            okO=okX=okJ=True
            for el in root.iter():
                if not isinstance(el.tag,str): continue
                n=name(el.tag)
                if n not in OURS_E: bad['ours']['<'+n+'>']+=1; okO=False
                if n not in X_E: bad['xpath']['<'+n+'>']+=1; okX=False
                if n not in J_E: bad['java']['<'+n+'>']+=1; okJ=False
                for a in el.attrib:
                    a=name(a)
                    if a.startswith('on') or not (a in OURS_COMMON or a in OURS_EA.get(n,())): bad['ours'][n+'@'+a]+=1; okO=False
                    if a not in X_A: bad['xpath'][n+'@'+a]+=1; okX=False
                    if not (a.startswith('xmlns') or a in J_A or n+'.'+a in J_C): bad['java'][n+'@'+a]+=1; okJ=False
            verdict[(okO,okX,okJ)]+=1
    print('==',os.path.basename(os.path.dirname(pkg)),'divs',nd)
    print('  (ours,xpath,java) ok ->',dict(verdict))
    for k in bad: print('  not in',k,':',bad[k].most_common(15))
