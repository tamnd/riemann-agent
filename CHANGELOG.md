# Changelog

All changes that a user can see are in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The versions follow [Semantic Versioning](https://semver.org/).

## v0.2.0 (2026-11-21)

Milestone M1, the map.

### Added

- `rh index` reads the Lean source without Lean and writes `modules.jsonl.gz`, `decls/<xx>.jsonl.gz` and `summary.json`. The package `lean/syntax` finds the imports, the namespaces, the open commands, the declarations and the identifiers. The package `index` resolves the identifiers with the rules of spec 04.2 and makes the syntactic declaration graph. On the release it reads 175,187 modules and 1,061,531 declarations in about 6 minutes, and the longest import chain from `Main` has 203 modules.
- `rh graph` answers four questions on the declaration graph: what a declaration uses, what uses it, a shortest chain of uses from one declaration to another, and the closure. `rh spine` finds the declarations that the root uses, stops at the named results of spec 04.6, and gives the depth, fan-in and dominator size of each node, the trunk and the chapter candidates. It writes `spine.json` and `spine.jsonl.gz`. On the release the spine of `riemannHypothesis` has 156,339 declarations with the cut points and 769,260 without them.
- The code pages. With `-index` and `-spine`, `rh site build` adds the numbers of the release, the trunk, the large branches and the libraries to `/code/`, a page for each declaration at `/code/decl/?n=Name` and a page for each module at `/code/module/?m=Name`. The pages read the records from shards in `api/decl/`, `api/module/` and `api/names/`, so the site has about 5,300 files for a million declarations. The search page and the code page search the Lean names.
- Each declaration page now has a plain reading of the statement. The package `lean/read` reads the Lean signature without Lean and gives what the statement takes, what it assumes and what it gives, in words. The page also shows the chain of declarations from `riemannHypothesis` down to it, the list of its parts in the dominator tree with the largest part first, and links to the previous and the next part of the same parent. A long list of parts shows 100 rows first and a button shows the rest.

- The proof pages have a list of all chapters in reading order in the side panel, and links to the chapter before and the chapter after at the bottom of each chapter.

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
