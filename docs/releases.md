# Releasing

Whoever pushes the tag publishes the release. There is no other route, and that is the
design: `.github/workflows/release.yml` publishes on `push` of a tag matching `v*`, and
nothing a merge can do produces an artifact a parent installs on the machine that locks
their children's screens.

```sh
git tag -a v0.1.0 -m "plock v0.1.0"
git push origin v0.1.0            # this is the release
```

Then watch it: `gh run watch <id>`, and the result is
`https://github.com/vicowa-botworks/parental-lockout/releases/tag/v0.1.0`.

## What a tag push runs

Three jobs, in order:

| Job | Runs on | Does |
|---|---|---|
| `artifacts (amd64)` | `ubuntu-latest` | builds and packages `amd64` on an x86-64 runner |
| `artifacts (arm64)` | `ubuntu-24.04-arm` | builds and packages `arm64` on an aarch64 runner |
| `release` | `ubuntu-latest` | downloads all four files, checks their count, writes `SHA256SUMS`, publishes |

One job per architecture is not a parallelism preference. `rpmbuild` refuses to build for a
foreign architecture at all — `error: No compatible architectures found for build` — and
neither `--target aarch64`, nor `--define _cross_platform 1`, nor redefining `_build_cpu` /
`_host_cpu` / `_target_cpu` lifts it (four attempts, all identical, on `fedora:43`). The
`.deb` cross-builds fine, so the alternative was one complete format on one architecture and
half a format on the other. Building each architecture natively also means the arm64 packages
are *installed* on real aarch64 Debian and Fedora by the same job that builds them, which a
cross-build could never have checked.

## Versions

`scripts/package-lib.sh` owns the version, and the leading `v` is a tag prefix, not part of
the version:

| | from tag `v0.1.0` | from an untagged tree |
|---|---|---|
| `.deb` version | `0.1.0` | `0.0.0+<describe>` |
| `.rpm` | `plock-0.1.0-1.<arch>.rpm` | `plock-0.0.0-0.untagged.<describe>.<arch>.rpm` |
| `plock version` | `plock v0.1.0 (commit …, built …)` | `plock <describe>` — the short sha, or `v0.1.0-3-g<sha>` between releases |

An untagged build sorts **below** `1` deliberately: a developer's throwaway must never look
like an upgrade to a machine that already has a release on it. `check_version` refuses a
string either tool would silently mangle — a tag with a space in it is taken by `dpkg` and
rejected by `rpm`, so the two packages would be built from two different strings — and a
dirty tree is refused rather than sanitised, because `plock version` would otherwise report a
commit that does not contain the code doing the locking. `ALLOW_DIRTY=1` exists for a build
you are about to delete.

## The guards in the workflow, and why each one is there

Every one of these replaced a failure that was actually observed, most of them in the two
dispatch runs that came before the first release.

- **`if: startsWith(github.ref, 'refs/tags/')` on the publish job.** Without it a
  `workflow_dispatch` satisfies `needs: artifacts`, downloads, and calls
  `gh release create "$GITHUB_REF_NAME" --verify-tag` with `GITHUB_REF_NAME` set to the
  *branch*. A step-level guard on the artifacts job does not cover a sibling job: that step
  skips, the job goes green, and the publish job runs anyway. A guard that has to fail in
  order to be true is not a guard, it is a surprise with a log.
- **`GH_REPO` on the publish job.** `gh` resolves a repository from `--repo`, then `GH_REPO`,
  then the git remote — it does *not* read the runner's `GITHUB_REPOSITORY`, and the job has
  no checkout step to find a remote in. The first real tag would have gone gate green,
  download green, checksums green, and failed to run git.
- **The artifact-name sanitiser (`[^A-Za-z0-9._-]` → `-`).** `github.ref_name` is the tag on
  a push and the *branch* on a dispatch, and `/` is not a legal artifact name — the runner
  rejects the upload outright. Both jobs derive the name from the same step output, because
  a mismatch downloads zero files.
- **`mkdir -p staged` before the count.** `download-artifact` does not create its `path` when
  the pattern matches nothing; without the directory the step died on `cd` and the
  "fewer than four artifacts" guard never ran. Now the guard is the thing that reports.
- **`--verify-tag`.** The tag must exist, so a release can never name a commit the tag does
  not point at.
- **Permissions.** `contents: read` at the workflow; only the publish job asks for
  `contents: write`. Specifying any scope on a job sets every unlisted scope to `none`, which
  was measured both directions: upload under workflow-level `contents: read` (run
  `41167592068`, two ~8 MB artifacts accepted) and download with the job holding no
  `actions:` scope at all (run `41184236321`, 16,495,047 bytes). `actions: read` is therefore
  *not* on the publish job — it buys nothing there, and a scope that buys nothing while a
  comment explains it wrongly is worse than no scope, because the comment is what gets
  copied.

## What is measured and what is not

The first release, `v0.1.0` (run `41341363697`), published five assets and closed every question this
file used to hedge. Checked against the downloaded artifacts, not the job's green tick:

- `sha256sum -c SHA256SUMS` — the command `README.md` tells an operator to run — passes on all four
  packages, so the file covers what was actually uploaded.
- The published `amd64` binary answers `plock v0.1.0 (commit d9d7077, built 2026-11-01T00:25:27Z)`.
  The `v` is in the version string and out of the package version (`dpkg-deb -f` reports
  `Version: 0.1.0`), which is the table above working as written; and the built date is the tagged
  commit's date, not the runner's clock, which is what makes a rebuild byte-identical.
- The `arm64` payload is an `ELF … ARM aarch64` binary. `rpm -i` refuses a foreign-architecture
  payload, so the passing arm64 install gate in the same job is also the architecture proof.
- `publish` ran with the scopes listed above and nothing else.

Before that run, the publish job had never executed. Run `41184236321` was the experiment that took it
as far as it could go without a tag: the download under `actions: none`, the four-artifact count
against a real four, and the publish step against a branch name, ending at the `--verify-tag` refusal
with nothing created.

`scripts/package-verify.sh` is what makes the artifacts themselves trustworthy: it installs
each `.deb` and `.rpm` in `debian:13` and `fedora:43`, cross-foots the two payloads
byte-for-byte (`rpm2cpio | cpio --to-stdout`, compared by sha256 — rpm's own `brp-strip` once
rewrote the ELF at byte 41 while every per-package check stayed green), and exercises every
packaging scriptlet branch including the upgrade path, with planted bytes as sentinels
because "exists" is not the same claim as "is the same file".

