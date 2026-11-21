# Changelog

All changes that a user can see are in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The versions follow [Semantic Versioning](https://semver.org/).

## Unreleased

## v0.6.0 (2026-11-21)

Milestone M5, drafting at scale. This release has no new code since v0.5.1. With this release, the content repo has drafted L2 steps for the main branch of each chapter: 50 steps in all, none checked by a person yet.

## v0.5.1 (2026-11-21)

The first part of milestone M5: the model client, `rh draft` and `rh report`.

### Added

- `rh draft` drafts steps of the written proof with a model. It takes Lean names or step ids, or `-queue N` for the next N open names in the order of the spine. For each name it sends the Lean text, the direct dependencies with their glossary text and signatures, and the matching glossary names. It writes the step with the status `drafted`, the fields `drafted_by` and `prompt`, and the fields `uses`, `routine` and `open` from the dependencies. Then it moves the name from `open` to `uses` in the parent steps. The flag `-n` prints the prompts and the cost estimate and sends nothing. The flag `-pr` opens a pull request with the label `agent-draft`.
- The package `llm` is the model client. It counts the cost of each call from the token usage and the price list. A fake client makes the tests run with no network.
- The package `prompts` holds the prompts of the roles prover, namer, stater, linker, chapterer and pathfinder. Each prompt has a version, and each draft records the prompt that made it.
- The file `agent.yaml` in the content repo sets the models and the budget per day and per run. With no file, the budget is zero and `rh draft` sends no request. Each call goes to `agent/ledger.jsonl` with its tokens and its cost.
- `rh` can write a step file. The fields keep the same order in each file.
- `rh report` writes the agent report of spec 09.4 as Markdown. It gives the calls, the failed calls and the cost of the last 7 days from `agent/ledger.jsonl`, the cost of each role, the steps that a model drafted, the steps that people accepted, and the lines that people added and deleted in each drafted file since the commit that added it. The flag `-days` sets the window, and `-until` sets the last day.
- The trust report and the home page have a line for the written steps. It tells how many steps are checked or reviewed. The list of the steps gives the same share for all the steps and for each chapter.

## v0.5.0 (2026-11-21)

Milestone M4, the paths and the links.

### Added

- `rh link` writes `links/math2.yaml`. It reads the `formal.decl` fields of the nodes of a math2 checkout and the `decl` fields of the seed list and the skeletons, keeps the tags that the Math 2.0 index has, and keeps each entry that a person added. It also matches each concept of a path that has no tag against the names and the aliases of the nodes. The flag `-n` prints the counts and writes nothing.
- `rh check` checks the learning paths in `paths/*.yaml` with the rules P1 to P5 and the glossary in `glossary/names.yaml` with the rules G1 to G4. A path must name chapters and paths that exist, with no cycle, and each concept must have a Math 2.0 node or a proposal for one. A glossary name must be a declaration of the records with the same kind, or a name space.
- The site has a page for each learning path at `/learn/<id>/`, a concept map at `/learn/map/` and a glossary of Lean names at `/glossary/`. Each path page lists its concepts in order, with the text, the steps of the proof that use the concept and the link to Math 2.0 or to the proposal. A reader can mark a concept as known. The browser keeps the marks. Each chapter page lists the paths to read before it. The search finds the glossary names and the concepts, and the declaration panel shows the glossary text of a name.

### Changed

- On a declaration page, the section "In the literature" shows the math of the statement as MathML, and it also shows the note that tells how the Lean statement differs from the published one.
- A glossary name can have the status `open`. It means that the meaning of the name is not known yet. The glossary page shows this next to the name.
- The field `text` of a path item is a standard text to learn the concept from, as spec 07.1 says. It is not required. The new field `says` gives the concept in one sentence, for a concept with no Math 2.0 node. The rule P3 now asks for the field `why` instead of `text`.
- `rh link` copies the first sentence of the summary of each Math 2.0 node that a path or the glossary uses into the table `summaries` of `links/math2.yaml`. A path page shows this sentence under each concept, so a reader sees what the concept is before the link.
- The timeline names the chapter by its title, and a new column Learn links each step of the note to the learning path with its background.
- `rh link` expands the TeX macros of Math 2.0, such as `\C` and `\abs`, when it copies a summary. It reads them from `macros/m2.katex.json` of the checkout. So the site renders each summary with no macros of its own.
- When a Math 2.0 summary starts with a setup sentence, such as "Let V be a vector space over C.", `rh link` keeps the setup with the next sentence. It skips a summary that only says which topic the node is a part of.
- A lead with code or math shows as rendered Markdown, not as raw text. A path page names the chapter that it prepares for as a chapter, so the link does not look like a link to the path itself.
- The link to Math 2.0 on a glossary name and on a cited result shows the name of the node. Before, it showed the raw text `[[TAG]]`.

## v0.2.2 (2026-11-21)

### Added

- `rh check` checks the written proof against the records. It reads the step files in `proof/steps/` of the content repository and the named results in `links/literature.yaml`, and it applies the rules C1 to C6 of spec 05 and the rule L1. It prints each problem and a summary with the number of steps at each status and the number of dependencies that have no step yet.
- The step pages. `rh site build` makes a page at `/step/<id>/` for each step file, with the statement, the Lean statement, the proof, the steps that it uses, the steps that use it, the concepts that it needs and its status with the reviews. A chapter lists its steps in the side panel in the order of its `steps` field, and each step links to the step before and the step after in the chapter.
- The page `/step/` lists all steps by chapter, and the page `/proof/named-results/` lists the published results that the proof uses, with the citations.
- A row of links "Level of detail" on the proof pages goes from the overview to the chapter, the step and the Lean declaration.
- A step can have the field `drafted_by: <role>@<model>` of spec 09. The step page then tells that a model wrote the draft.
- A declaration page that has a step links to it, and a declaration that is a named result shows what it says and where it is published.

### Fixed

- Long Lean names, file paths and long lines of Lean code wrap inside the text column. Before, they went out of the column on the step pages.

## v0.2.1 (2026-11-21)

### Changed

- The names in the side panel "In Lean" link to the declaration pages of the site.

## v0.2.0 (2026-11-21)

Milestone M1, the map.

### Added

- `rh index` reads the Lean source without Lean and writes `modules.jsonl.gz`, `decls/<xx>.jsonl.gz` and `summary.json`. The package `lean/syntax` finds the imports, the namespaces, the open commands, the declarations and the identifiers. The package `index` resolves the identifiers with the rules of spec 04.2 and makes the syntactic declaration graph. On the release it reads 175,187 modules and 1,061,531 declarations in about 6 minutes, and the longest import chain from `Main` has 203 modules.
- `rh graph` answers four questions on the declaration graph: what a declaration uses, what uses it, a shortest chain of uses from one declaration to another, and the closure. `rh spine` finds the declarations that the root uses, stops at the named results of spec 04.6, and gives the depth, fan-in and dominator size of each node, the trunk and the chapter candidates. It writes `spine.json` and `spine.jsonl.gz`. On the release the spine of `riemannHypothesis` has 156,339 declarations with the cut points and 769,260 without them.
- The code pages. With `-index` and `-spine`, `rh site build` adds the numbers of the release, the trunk, the large branches and the libraries to `/code/`, a page for each declaration at `/code/decl/?n=Name` and a page for each module at `/code/module/?m=Name`. The pages read the records from shards in `api/decl/`, `api/module/` and `api/names/`, so the site has about 5,300 files for a million declarations. The search page and the code page search the Lean names.
- Each declaration page now has a plain reading of the statement. The package `lean/read` reads the Lean signature without Lean and gives what the statement takes, what it assumes and what it gives, in words. The page also shows the chain of declarations from `riemannHypothesis` down to it, the list of its parts in the dominator tree with the largest part first, and links to the previous and the next part of the same parent. A long list of parts shows 100 rows first and a button shows the rest.

- The proof pages have a list of all chapters in reading order in the side panel, and links to the chapter before and the chapter after at the bottom of each chapter.
- The names in the side panel "In Lean" link to the declaration pages of the site when the build has the records. Those pages link to the source line.
### Changed

- The shards in `api/` are gzip files. The browser decompresses them. The site is now 416 MB and 5,340 files, below the limits of the Cloudflare static assets.
- The plain reading drops the prefix `_root_.` from the Lean names.

## v0.1.0 (2026-11-21)

Milestone M0, the source and the site.

### Added

- The workflow `release` builds `rh` for Linux, macOS and Windows on amd64 and arm64 when a tag `v*` is pushed. It adds `checksums.txt` and a build provenance attestation to the release.
- `rh site build`, `rh site check` and `rh site serve`. The build reads the content repository and writes riemann.tamnd.com: the main page, the proof pages with a trail and a side panel, the timeline, the learning, code, trust and about pages, search, a Markdown copy of each page, the JSON in `api/`, `llms.txt` and the sitemap. The style is the style of Math 2.0. The math renders to MathML. A link `[[text|TAG]]` goes to a node of Math 2.0, and a link `lean:File.lean#L4` goes to the source line in riemann-lean. The check finds broken links, formulas that do not render and files that are over the limits of the Cloudflare static assets.
- `rh version` and `rh help`. The other commands exit with code 3 and name their milestone.
- `rh fetch` downloads the 14 parts of the release of 2026-11-20 and the note, checks the size and SHA-256 of each file, and joins the parts into `lean.tar.gz`. The flags `-unpack`, `-compare` and `-write-manifest` unpack the archive, compare it with the manifest of riemann-lean and write a new manifest.
