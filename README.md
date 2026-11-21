# riemann-agent

`rh` is the toolchain of [riemann](https://github.com/tamnd/riemann). It reads the Lean source of the claimed proof of the Riemann Hypothesis in [riemann-lean](https://github.com/tamnd/riemann-lean), checks it, maps the proof and builds the site [riemann.tamnd.com](https://riemann.tamnd.com).

The project has three repositories:

| Repository | Content |
|---|---|
| [riemann-lean](https://github.com/tamnd/riemann-lean) | the Lean source of the release of 2026-11-20, not changed, with citation and hashes |
| [riemann](https://github.com/tamnd/riemann) | the written proof, the learning paths, the glossary and the records |
| riemann-agent (this repository) | the toolchain `rh` and the site builder |

## Status

The work goes in milestones. Each milestone has a tracking issue with the label `milestone`.

| Milestone | Goal | Release |
|---|---|---|
| M0 | the source and the site | v0.1.0 |
| M1 | the map of the Lean proof | v0.2.0 |
| M2 | the trust report | v0.3.0 |
| M3 | the top of the proof in words | v0.4.0 |
| M4 | learning paths and links to Math 2.0 | v0.5.0 |
| M5 | drafting at scale | v0.6.0 |
| M6 | the chapters | v0.7.0 and later |

A command that is not written yet exits with code 3 and names its milestone.

## Install

You need Go 1.27 or later.

```sh
go install github.com/tamnd/riemann-agent/cmd/rh@latest
```

## Use

```sh
rh help
rh version
rh site build -content ../riemann -out public
rh site serve -dir public
```

Index the Lean source of riemann-lean. The command needs about 4 GB of memory:

```sh
rh index -src ../riemann-lean/lean/src -out work/index
```

Download the release and check it against the manifest of riemann-lean:

```sh
rh fetch -dir work/release -compare ../riemann-lean/manifests/files.sha256
```

The command downloads about 240 MB. A second run downloads only the files with a wrong hash.

Check the axioms of the five theorems of `Main` in a built copy of riemann-lean, and write the result into the content repository:

```sh
rh trust -lean ../riemann-lean/lean -content ../riemann -lean4checker ~/lean4checker/.lake/build/bin/lean4checker
```

The command runs `#print axioms` and a walk of its own over the constants that each theorem uses. The two lists must agree, and each axiom must be one of `propext`, `Classical.choice` and `Quot.sound`. With `-lean4checker`, it also runs `lean4checker --fresh` on `Main`. It writes `records/axioms.json`, `records/replay.json` and the rows Q2 and Q3 of `records/trust.json`. `rh trust -print` prints the Lean file of the check.

Replace the syntactic declaration graph with the exact graph from the built environment:

```sh
rh extract -index work/index -lean ../riemann-lean/lean -out work/index-exact -keep work/constants.jsonl.gz
```

The command imports `Main` in Lean and writes each constant of the release with the constants that its type and value use. Lean adds auxiliary constants such as `foo.match_1` and the projections of a structure, so the command follows the edges through them to the declarations of the source. A declaration with no constant keeps its syntactic uses. `extract.json` in the output tells how many declarations matched and how many edges changed. Use `-exact FILE` to read the output of an earlier run.

`rh site build` reads the index of Math 2.0 from https://math2.tamnd.com/api/nodes.json. Use `-math2 FILE` to read a copy, or `-math2 ""` to build with no network.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | a check failed, or the run failed |
| 2 | usage error |
| 3 | the command is not written yet |

## Licence

MIT. See [LICENSE](LICENSE).
