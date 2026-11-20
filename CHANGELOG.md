# Changelog

All changes that a user can see are in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The versions follow [Semantic Versioning](https://semver.org/).

## Unreleased

## v0.1.0 (2026-11-21)

Milestone M0, the source and the site.

### Added

- The workflow `release` builds `rh` for Linux, macOS and Windows on amd64 and arm64 when a tag `v*` is pushed. It adds `checksums.txt` and a build provenance attestation to the release.
- `rh site build`, `rh site check` and `rh site serve`. The build reads the content repository and writes riemann.tamnd.com: the main page, the proof pages with a trail and a side panel, the timeline, the learning, code, trust and about pages, search, a Markdown copy of each page, the JSON in `api/`, `llms.txt` and the sitemap. The style is the style of Math 2.0. The math renders to MathML. A link `[[text|TAG]]` goes to a node of Math 2.0, and a link `lean:File.lean#L4` goes to the source line in riemann-lean. The check finds broken links, formulas that do not render and files that are over the limits of the Cloudflare static assets.
- `rh version` and `rh help`. The other commands exit with code 3 and name their milestone.
- `rh fetch` downloads the 14 parts of the release of 2026-11-20 and the note, checks the size and SHA-256 of each file, and joins the parts into `lean.tar.gz`. The flags `-unpack`, `-compare` and `-write-manifest` unpack the archive, compare it with the manifest of riemann-lean and write a new manifest.
