# npm distribution

mkdown is published to npm so it can be run with `npx` — no Go toolchain
needed:

```bash
npx @mkdown/cli README.md
npx @mkdown/cli *.md --no-highlight
```

## How it works

Same approach esbuild uses for its Go binary:

- **`@mkdown/cli`** (`cli/`) — the package users install. Contains only a tiny
  Node launcher (`cli/bin/mkdown`) and lists the platform packages as
  `optionalDependencies`.
- **`@mkdown/cli-<platform>`** (`darwin-arm64/`, `linux-x64/`, …) — one package
  per OS/arch, each carrying the matching prebuilt binary and gated by npm
  `os`/`cpu` fields, so npm downloads **only** the one for the host.

The launcher resolves the installed platform package and execs its binary. No
postinstall script and no network download — it works offline and behind
proxies, which the `curl`-in-postinstall approach does not.

## Cutting a release

```bash
npm login
node npm/release.mjs 0.2.0 --publish
```

`release.mjs` cross-compiles all six binaries with `go build`, regenerates the
platform `package.json` files, syncs every version (main + platforms +
`optionalDependencies`) to the given number, then publishes the platform
packages first and the main package last. Drop `--publish` for a dry run that
only builds and version-syncs.

Run it on the same tag you push for the goreleaser GitHub release, so the npm
and binary releases stay in lockstep.

## Renaming the package

The name is set in two places: `MAIN` in `release.mjs` and `cli/package.json`.
If the `@mkdown` org can't be claimed on npm, switch to an unscoped name
(`mkdown-cli`, with platform packages `mkdown-cli-<platform>`) or your own
scope (`@ekinertac/mkdown`). Update `MAIN`, re-run `release.mjs`, and the
launcher's `require.resolve` target updates automatically since it derives the
platform package name from `MAIN`'s scope — adjust the prefix in
`cli/bin/mkdown` to match.
