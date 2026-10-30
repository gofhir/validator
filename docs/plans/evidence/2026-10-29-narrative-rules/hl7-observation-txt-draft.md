**Summary:** Narrative invariants txt-1 and txt-2 share one expression, so a FHIRPath engine cannot tell them apart.

**Specification:** FHIR R4 4.0.1, R4B 4.3.0 and R5 5.0.0, StructureDefinition-Narrative, `Narrative.div`.

**Problem.** Both invariants publish the same FHIRPath expression, `htmlChecks()`:

- txt-1: "The narrative SHALL contain only the basic html formatting elements and attributes described in chapters 7-11 (except section 4 of chapter 9) and 15 of the HTML 4.0 standard, <a> elements (either name or href), images and internally contained style attributes"
- txt-2: "The narrative SHALL have some non-whitespace content"

`htmlChecks()` returns false whenever any of the narrative rules fails (fhirpath.html#functions). So a narrative that breaks only one rule fails both invariants. For example, a div with only whitespace fails txt-1 too, and a div with `<script>` and some text fails txt-2 too.

R4 and R4B still carry XPath expressions that tell the two apart. R5 removed them, so the FHIRPath expression is the only executable definition.

The HL7 Java validator works around this by rewriting the expression by key: `FHIRPathExpressionFixer` turns txt-2 into a non-standard `htmlChecks2()`. Implementations that follow the published expressions report both invariants for each failure, and those that copy the workaround depend on a function the specification does not define.

**Proposal.** Give each invariant its own expression:

- either two functions defined in fhirpath.html#functions (one for elements and attributes, one for content);
- or keep `htmlChecks()` for txt-1, and write txt-2 in plain FHIRPath. In R4 the XPath was `descendant::text()[normalize-space(.)!=''] or descendant::h:img[@src]`, so the FHIRPath could be something like `descendants().where(...)`. This depends on how xhtml children are exposed.

**Also unclear:** narrative.html lists the allowed content only by reference to HTML 4.0 chapters. The R4 XPath enumerates a stricter list: it rejects `xml:lang`, which the language section allows. An enumerated list, or a schema in the package, would let implementations agree.
