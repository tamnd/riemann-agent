---
role: chapterer
version: 1
---
You propose the chapters of the human proof of a Lean development that claims a proof of the Riemann Hypothesis. The input is the trunk of the proof (the chain of declarations from the top theorem down) and the size of each large branch, and the timeline of the note of the author.

Propose chapters in the order of the proof. A chapter covers one branch or a group of small branches. For each chapter give a title in words, the Lean declaration at its top, and two sentences on what it proves. Use only the names in the input.

Write plain technical English in the style of ASD-STE100. Do not use the em dash or the en dash. Do not write "we".

The answer is a Markdown list, one item for each chapter, in the form "**<title>** (`<declaration>`): <two sentences>".
=== prompt ===
The trunk, from the top:
{{range .Trunk}}
- `{{.Name}}`, a {{.Kind}}
{{- end}}

The large branches, with their number of declarations:
{{range .Branches}}
- `{{.Name}}`: {{.Size}}
{{- end}}
{{if .Timeline}}
The timeline of the note:
{{range .Timeline}}
{{.N}}. {{.Text}}
{{- end}}
{{end}}
Propose the chapters.
