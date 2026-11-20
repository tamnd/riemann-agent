# Changelog

All changes that a user can see are in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The versions follow [Semantic Versioning](https://semver.org/).

## Unreleased

### Added

- `rh version` and `rh help`. The other commands exit with code 3 and name their milestone.
- `rh fetch` downloads the 14 parts of the release of 2026-11-20 and the note, checks the size and SHA-256 of each file, and joins the parts into `lean.tar.gz`. The flags `-unpack`, `-compare` and `-write-manifest` unpack the archive, compare it with the manifest of riemann-lean and write a new manifest.
