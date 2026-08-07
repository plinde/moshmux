---
name: x-tag-and-release-from-main
version: 1.0.0
description: "Tag the current main branch and push the tag to trigger the GHA goreleaser release workflow."
---

# /x-tag-and-release-from-main

Tag `main` with the next semantic version and push it, triggering the GitHub Actions release workflow (goreleaser builds linux/darwin binaries and publishes a GitHub Release).

## Workflow

1. **Sync main**: ensure we're on `main` and up-to-date.
   ```bash
   git checkout main && git pull
   ```

2. **Find the latest tag** to determine the next version:
   ```bash
   git tag --sort=-v:refname | head -5
   gh release list --repo plinde/moshmux
   ```

3. **Determine next version**: ask the user if unclear, otherwise:
   - Breaking/major changes → bump major (e.g. v2.0.0 → v3.0.0)
   - New features (backward-compatible) → bump minor (e.g. v2.0.0 → v2.1.0)
   - Bug fixes only → bump patch (e.g. v2.0.0 → v2.0.1)
   - Summarize commits since last tag to help decide:
     ```bash
     git log $(git describe --tags --abbrev=0)..HEAD --oneline
     ```

4. **Confirm** with the user before tagging: "Ready to tag vX.Y.Z and push — proceed?"

5. **Create and push the tag**:
   ```bash
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```

6. **Verify** the GHA workflow started:
   ```bash
   gh run list --repo plinde/moshmux --workflow=release.yml --limit=3
   ```

7. **Report** the run URL so the user can watch progress.

## Rules

- Never tag from a branch other than `main`.
- Never push a tag if `main` is behind `origin/main` — pull first.
- Never force-push or move an existing tag.
- Always confirm the version with the user before pushing.
- Do NOT bump the version in any source file (version is injected via ldflags at build time from the git tag).

## Context

- GHA workflow: `.github/workflows/release.yml` — triggers on `v*` tag push
- Goreleaser config: `.goreleaser.yml` — builds `linux/amd64`, `darwin/amd64`, `darwin/arm64`
- Version injection: `ldflags: -X main.version={{ .Version }}`
- Releases: https://github.com/plinde/moshmux/releases
