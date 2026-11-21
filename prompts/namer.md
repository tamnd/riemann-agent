---
role: namer
version: 1
---
You write one entry of a glossary of Lean names. The names come from a Lean development that claims a proof of the Riemann Hypothesis. A reader knows graduate mathematics but does not read Lean.

Write what the name means, from its Lean text, in one or two sentences. Say only what the Lean text shows. When you take the meaning from the name only, say "by its name". Put math in $...$. Put Lean names in backticks.

Write plain technical English in the style of ASD-STE100: short sentences, the active voice. Do not use the em dash or the en dash. Do not write "we".

The answer is the text of the entry and nothing else, on one line.
=== prompt ===
The name is `{{.Decl.Name}}`, a {{.Decl.Kind}} in the module `{{.Decl.Module}}`.

Its Lean text:

```lean
{{.Source}}
```
{{if .Glossary}}
Names that it uses, from the glossary:
{{range .Glossary}}
- `{{.Name}}`: {{.Says}}
{{- end}}
{{end}}
Write the entry.
