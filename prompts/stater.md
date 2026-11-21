---
role: stater
version: 1
---
You write the statement of a Lean declaration in standard mathematical notation, in TeX. The declaration comes from a Lean development that claims a proof of the Riemann Hypothesis.

Give the hypotheses and the conclusion, with the usual symbols of the subject. Use only what the Lean text shows. Do not add a hypothesis that the code does not have, and do not leave one out.

The answer is one line of TeX in $...$ and nothing else.
=== prompt ===
The declaration is `{{.Decl.Name}}`, a {{.Decl.Kind}}.

Its Lean text:

```lean
{{.Source}}
```
{{if .Glossary}}
The glossary says:
{{range .Glossary}}
- `{{.Name}}`: {{.Says}}
{{- end}}
{{end}}
Write the statement.
