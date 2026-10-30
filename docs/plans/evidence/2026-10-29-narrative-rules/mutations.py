"""Mutaciones del incremento de la narrativa (plan §2). Uso: python3 -I mutations.py <copia del repo> <dir>.

Cada una se aplica sola sobre una copia limpia, corre los tests del incremento y tiene que hacerlos
fallar: KILLED. SURVIVED es un oráculo que no la ve. Mutan el comportamiento efectivo de htmlChecks().
"""
import pathlib, shutil, subprocess, sys

ROOT = pathlib.Path(sys.argv[1]).resolve()
TESTS = [
    ["go", "test", "./pkg/constraint", "-run", "TestNarrativeRulesFollowHTML40|TestNarrativeXML$|TestHTMLChe", "-count=1"],
    ["go", "test", "./pkg/validator", "-run", "TestNarrativeXHTMLProbes|TestNarrativeAndUCUMAnnotation", "-count=1"],
]
H = "pkg/constraint/htmlchecks.go"
DECODER = "\td := xml.NewDecoder(strings.NewReader(s))\n"
MUTATIONS = {
    # Un parser que repara la entrada, como el de HTML de antes.
    "n1-lenient-parser": [(H, DECODER, DECODER + "\td.Strict = false\n\td.AutoClose = xml.HTMLAutoClose\n\td.Entity = xml.HTMLEntity\n")],
    "n2-html-entities": [(H, DECODER, DECODER + "\td.Entity = xml.HTMLEntity\n")],
    "n3-any-namespace": [(H, "namespaceOf(t.Name.Space, e, open) != xhtmlNamespace || ", "")],
    "n4-accept-center": [(H, "\"bdo\": true,\n", "\"bdo\": true, \"center\": true,\n")],
    "n5-reject-bdo": [(H, "\"bdo\": true,\n", "\n")],
    "n6-accept-target": [(H, "\"shape\", \"tabindex\", \"type\"),", "\"shape\", \"tabindex\", \"target\", \"type\"),")],
    "n7-reject-td-char": [(H, "\"td\":    set(\"abbr\", \"align\", \"axis\", \"bgcolor\", \"char\", ", "\"td\":    set(\"abbr\", \"align\", \"axis\", \"bgcolor\", ")],
    "n8-text-outside-div": [(H, "if len(open) == 0 && strings.Trim(s[start:d.InputOffset()], xmlSpace) != \"\" {", "if false && strings.Trim(s[start:d.InputOffset()], xmlSpace) != \"\" {")],
    "n9-element-after-div": [(H, "len(open) == 0 && (closed || t.Name.Local != \"div\")", "len(open) == 0 && t.Name.Local != \"div\"")],
    "n10-declarations-ok": [(H, "\t\tdefault:\n", "\t\tcase xml.ProcInst, xml.Directive:\n\t\t\tcontinue\n\t\tdefault:\n")],
    "n11-img-not-content": [(H, "\t\t\tif t.Name.Local == \"img\" {\n\t\t\t\tcontent = true\n", "\t\t\tif t.Name.Local == \"img\" {\n")],
    "n12-lang-on-br": [(H, "\t\treturn tag != \"br\" && tag != \"hr\"\n", "\t\treturn true\n")],
    "n13-foreign-attribute": [(H, "\t\treturn \"xml:\" + n.Local, true\n\t}\n\treturn \"\", false\n", "\t\treturn \"xml:\" + n.Local, true\n\t}\n\treturn n.Local, true\n")],
    "n14-xml-space-anywhere": [(H, "\"lang\": true, \"dir\": true, \"xml:lang\": true}", "\"lang\": true, \"dir\": true, \"xml:lang\": true, \"xml:space\": true}")],
    "n15-events": [(H, "\tcase attr == xmlnsAttribute, coreAttrs[attr]:\n", "\tcase attr == xmlnsAttribute, coreAttrs[attr], len(attr) > 2 && attr[:2] == \"on\":\n")],
    "n16-blank-ok": [(H, "\t\t\treturn len(open) == 0 && closed && content\n", "\t\t\treturn len(open) == 0 && closed\n")],
    # Ronda 1 del code review (plan §8).
    "n17-unicode-space": [(H, "len(bytes.Trim(t, xmlSpace)) > 0", "len(bytes.TrimSpace(t)) > 0")],
    "n18-decoded-outside": [(H, "strings.Trim(s[start:d.InputOffset()], xmlSpace) != \"\"", "strings.TrimSpace(string(t)+s[start:start]) != \"\"")],
    "n19-any-declaration": [(H, "\t\tif !validDeclaration(prefix, a.Value) {\n\t\t\treturn e, false\n\t\t}\n", "")],
    "n20-xml-prefix-anything": [(H, "\t\treturn ns == xmlNamespace\n", "\t\treturn true\n")],
    "n21-empty-prefix-ok": [(H, "case prefix != \"\" && ns == \"\":", "case false:")],
    "n22-comment-outside-ok": [(H, "\t\tcase xml.Comment:\n\t\t\tif len(open) == 0 {\n\t\t\t\treturn false\n\t\t\t}\n", "")],
    "n23-reject-prefix-declarations": [(H, "\t\tif !validDeclaration(prefix, a.Value) {", "\t\tif prefix != \"\" || !validDeclaration(prefix, a.Value) {")],
    # Ronda 2 (plan §8).
    "n24-no-end-match": [(H, "if len(open) == 0 || open[len(open)-1].name != t.Name {", "if len(open) == 0 {")],
    "n26-close-without-open": [(H, "if len(open) == 0 || open[len(open)-1].name != t.Name {", "if len(open) > 0 && open[len(open)-1].name != t.Name {")],
    "n25-unbound-is-xhtml": [(H, "\t\t}\n\t}\n\treturn \"\"\n}", "\t\t}\n\t}\n\treturn xhtmlNamespace\n}")],
}

def run(name, edits, work):
    if work.exists():
        shutil.rmtree(work)
    shutil.copytree(ROOT, work, ignore=shutil.ignore_patterns(".git", "hl7diff", "corpusrun"))
    for path, old, new in edits:
        f = work / path
        text = f.read_text()
        if text.count(old) != 1:
            return "NOT-APPLIED"
        f.write_text(text.replace(old, new))
    for cmd in TESTS:
        r = subprocess.run(cmd, cwd=work, capture_output=True, text=True)
        if "build failed" in r.stdout + r.stderr or "[setup failed]" in r.stdout:
            return "BUILD-FAILED"
        if r.returncode != 0:
            return "KILLED"
    return "SURVIVED"

work = pathlib.Path(sys.argv[2]).resolve()
for name, edits in MUTATIONS.items():
    print(name, run(name, edits, work / name), flush=True)
