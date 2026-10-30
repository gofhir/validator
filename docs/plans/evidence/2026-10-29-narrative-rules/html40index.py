"""Extrae los índices de elementos y atributos de HTML 4.0 (W3C REC-html40-19980424) a TSV.

Uso: html40index.py elements|attributes <directorio con index/ y los capítulos>. Cada fila lleva la
sección de HTML 4.0 donde se define (el último encabezado h-N.N antes de su ancla).
"""
import html, os, re, sys

ROOT = sys.argv[2]
_sections = {}

def section(file, anchor):
    if file not in _sections:
        doc = open(os.path.join(ROOT, file), encoding='latin-1').read()
        _sections[file] = doc
    doc = _sections[file]
    m = re.search(r'name="' + re.escape(anchor) + '"', doc, flags=re.I)
    if not m:
        return '?'
    heads = re.findall(r'name="h-([0-9.]+)"', doc[:m.start()])
    return heads[-1] if heads else '?'

def cells(row):
    parts = re.split(r'<td[^>]*title="([^"]+)"[^>]*>', row, flags=re.I)
    return {parts[i]: parts[i + 1] for i in range(1, len(parts), 2)}

def text(s):
    return html.unescape(re.sub(r'<[^>]+>', '', s)).replace('\xa0', ' ').strip()

def rows(path):
    doc = open(path, encoding='latin-1').read().replace('\n', ' ')
    return [r for r in re.split(r'<tr>', doc, flags=re.I)[1:] if 'title="Name"' in r]

def elements():
    print('# element\tsection\tdeprecated\tdtd')
    for r in rows(os.path.join(ROOT, 'index/elements.html')):
        c = cells(r)
        m = re.search(r'href="\.\./([^"#]+)#(edef-[^"]+)"', c['Name'], flags=re.I)
        print(f"{text(c['Name']).lower()}\t{section(m.group(1), m.group(2))}\t{text(c['Depr.'])}\t{text(c['DTD'])}")

def attributes():
    print('# attribute\tsection\telements\tdeprecated\tdtd')
    for r in rows(os.path.join(ROOT, 'index/attributes.html')):
        c = cells(r)
        m = re.search(r'href="\.\./([^"#]+)#(adef-[^"]+)"', c['Name'], flags=re.I)
        rel = re.sub(r'\s*,\s*', ',', text(c['Related Elements'])).lower()
        print(f"{text(c['Name'])}\t{section(m.group(1), m.group(2))}\t{rel}\t{text(c['Depr.'])}\t{text(c['DTD'])}")

{'elements': elements, 'attributes': attributes}[sys.argv[1]]()
