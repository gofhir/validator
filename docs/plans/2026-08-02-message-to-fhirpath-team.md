# Message to the gofhir/fhirpath team

Ready to send as a GitHub issue. Evidence and reproductions are all verified against v1.5.1.

---

**Title:** ReDoS guard rejects valid patterns: `prevWasQuant` not cleared across `(` and `)`

---

Thanks for v1.4.0 and v1.5.1 — three of the four gaps we reported are closed, and we have
confirmed each one end to end against HL7 `validator_cli` 6.9.12:

| Fixed in | What | Effect for us |
| --- | --- | --- |
| v1.4.0 | type-name shadowing | `ref-1` stopped being a silent false pass — it could never fail before, on every `Reference` in every resource |
| v1.5.1 | `%ucum` undefined | `age-1`, `cnt-3`, `dis-1`, `drt-1`, `ras-1` now produce verdicts |
| v1.5.1 | `Quantity` comparison | `rng-2` now produces a verdict |

We also noticed the `trace()` output no longer reaches stdout, which had been leaking engine
diagnostics into our CLI's own output. And we have migrated to `ucum/v4` after seeing v1.5.1
pick it up.

One gap remains, and on closer reading it looks like a bug rather than a policy decision.

## The guard rejects patterns it was not meant to catch

```go
case '*', '+', '?':
    quantifierRun++
    if prevWasQuant {
        // Consecutive quantifiers like ** or *+ are dangerous
        return eval.NewEvalError(eval.ErrInvalidExpression,
            "potentially dangerous regex: consecutive quantifiers")
    }
    prevWasQuant = true
```

`funcs/regex.go:255`

`prevWasQuant` is cleared only in the `default:` branch. `case '('` and `case ')'` fall through
without clearing it, so a quantifier, a group close, and another quantifier are read as
adjacent:

```
(a+)?     +  sets the flag  →  )  leaves it set  →  ?  sees it  →  rejected
```

## Reproduction

```go
for _, expr := range []string{
    `'aaa'.matches('(a+)?')`,
    `'aaa'.matches('(a*)?')`,
    `'aaa'.matches('(a+)*')`,
    `'aaa'.matches('a+?')`,
    `'aaa'.matches('a**')`,
    `'aaa'.matches('a{1,3}(b)?')`,
} {
    e, _ := fhirpath.Compile(expr)
    _, err := e.EvaluateWithContext(eval.NewContext([]byte(`{}`)))
    fmt.Printf("%-32s %v\n", expr, err)
}
```

Observed on v1.5.1:

| Pattern | Result | Expected |
| --- | --- | --- |
| `(a+)?` | rejected | valid — quantified group, then optional |
| `(a*)?` | rejected | valid |
| `(a+)*` | rejected | valid |
| `a+?` | rejected | valid — this `?` is the lazy modifier, not a second quantifier |
| `a**` | rejected | correct, and RE2 rejects it unaided |
| `a{1,3}(b)?` | accepted | correct — `}` reaches `default:` and clears the flag |

Two distinct false positives fall out: any quantified group followed by a quantifier, and
standard non-greedy syntax (`a+?`, `a*?`).

## What it blocks

`eld-19` and `eld-20` on `ElementDefinition`, whose patterns HL7 publishes in the R4
specification. `eld-19` is `SHALL`-level:

```
eld-19: path.matches('[^\s\.,:;\'"\/|?!@#$%&*()\[\]{}]{1,64}(\.[^\s\.,:;\'"\/|?!@#$%&*()\[\]{}]{1,64}(\[x\])?(\:[^\s\.]+)?)*')
```

Its `(\:[^\s\.]+)?)*` tail is exactly the `+)?` shape above, so `ElementDefinition.path` cannot
be validated at all and a malformed path passes. HL7's validator evaluates the same pattern
without complaint, reporting `eld-19` as an error and `eld-20` as a warning on a path containing
spaces and special characters.

Scope is narrow but concentrated: `ElementDefinition` appears in only two places in all of R4 —
`StructureDefinition.differential.element` and `.snapshot.element` — so no clinical resource is
affected, only profiles and IG content. Because the failure is a property of the expression
rather than of the data, it repeats per element: a nine-element StructureDefinition yields 21
issues of which 18 are this one message, and a full 200-element snapshot yields roughly 400.

## On the ReDoS concern

Worth noting in case it affects how you want to fix it: the engine compiles with
`regexp.Compile` and matches with `re.MatchString` (`funcs/regex.go:79`, `:158`), which is RE2 —
linear time, no backtracking — so catastrophic backtracking is not reachable here. RE2 also
rejects genuinely malformed patterns on its own; `a**` fails to compile without the guard's
help. And `MatchString` already runs under a timeout (100ms in `DefaultRegexCache`), which
bounds any pathological input regardless of shape.

## Suggested fix

Clear `prevWasQuant` when crossing `(` and `)`, and treat `?` immediately following another
quantifier as the lazy modifier rather than a second quantifier. That keeps `a**` and `a*+`
caught while letting `(a+)?` and `a+?` through.

Happy to send a PR if that helps.
