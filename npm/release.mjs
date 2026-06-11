#!/usr/bin/env node
// Build per-platform mkdown binaries, generate the platform npm packages,
// sync every package version, and optionally publish.
//
//   node npm/release.mjs 0.2.0            # build + scaffold + version sync (dry)
//   node npm/release.mjs 0.2.0 --publish  # also npm publish (after `npm login`)
//
// The main package (@mkdown/cli) ships only a JS launcher; the actual binaries
// live in optional @mkdown/cli-<platform> packages gated by npm os/cpu so only
// the matching one installs. This mirrors how esbuild distributes its Go binary.
import { execFileSync } from "node:child_process";
import {
  mkdirSync,
  writeFileSync,
  readFileSync,
  chmodSync,
} from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const NPM_DIR = dirname(fileURLToPath(import.meta.url));
const ROOT = dirname(NPM_DIR);
const SCOPE = "@mkdown";
const MAIN = `${SCOPE}/cli`;

// npm os/cpu values vs Go GOOS/GOARCH. npm 'x64' == Go 'amd64'.
const PLATFORMS = [
  { key: "darwin-arm64", os: "darwin", cpu: "arm64", goos: "darwin", goarch: "arm64", exe: "mkdown" },
  { key: "darwin-x64", os: "darwin", cpu: "x64", goos: "darwin", goarch: "amd64", exe: "mkdown" },
  { key: "linux-arm64", os: "linux", cpu: "arm64", goos: "linux", goarch: "arm64", exe: "mkdown" },
  { key: "linux-x64", os: "linux", cpu: "x64", goos: "linux", goarch: "amd64", exe: "mkdown" },
  { key: "win32-arm64", os: "win32", cpu: "arm64", goos: "windows", goarch: "arm64", exe: "mkdown.exe" },
  { key: "win32-x64", os: "win32", cpu: "x64", goos: "windows", goarch: "amd64", exe: "mkdown.exe" },
];

const args = process.argv.slice(2);
const publish = args.includes("--publish");
const version = (args.find((a) => /^v?\d+\.\d+\.\d+/.test(a)) || process.env.VERSION || "0.0.0").replace(/^v/, "");

function run(cmd, cmdArgs, opts = {}) {
  return execFileSync(cmd, cmdArgs, { stdio: "inherit", ...opts });
}

const optionalDeps = {};
for (const p of PLATFORMS) {
  const pkgDir = join(NPM_DIR, p.key);
  const binDir = join(pkgDir, "bin");
  mkdirSync(binDir, { recursive: true });
  const out = join(binDir, p.exe);
  console.log(`building ${p.key}`);
  run(
    "go",
    [
      "build",
      "-trimpath",
      "-ldflags",
      `-s -w -X main.version=${version} -X main.date=${new Date().toISOString().slice(0, 10)}`,
      "-o",
      out,
      "./cmd/mkdown",
    ],
    { cwd: ROOT, env: { ...process.env, GOOS: p.goos, GOARCH: p.goarch, CGO_ENABLED: "0" } }
  );
  if (p.exe === "mkdown") chmodSync(out, 0o755);
  writeFileSync(
    join(pkgDir, "package.json"),
    JSON.stringify(
      {
        name: `${MAIN}-${p.key}`,
        version,
        description: `mkdown binary for ${p.os}-${p.cpu}`,
        license: "MIT",
        repository: { type: "git", url: "git+https://github.com/ekinertac/mkdown.git" },
        os: [p.os],
        cpu: [p.cpu],
        files: ["bin"],
      },
      null,
      2
    ) + "\n"
  );
  optionalDeps[`${MAIN}-${p.key}`] = version;
}

// Sync the main package version + optionalDependencies to match.
const mainPkgPath = join(NPM_DIR, "cli", "package.json");
const mainPkg = JSON.parse(readFileSync(mainPkgPath, "utf8"));
mainPkg.version = version;
mainPkg.optionalDependencies = optionalDeps;
writeFileSync(mainPkgPath, JSON.stringify(mainPkg, null, 2) + "\n");

if (publish) {
  // Platform packages first so the main package's optional deps resolve.
  for (const p of PLATFORMS) {
    run("npm", ["publish", "--access", "public"], { cwd: join(NPM_DIR, p.key) });
  }
  run("npm", ["publish", "--access", "public"], { cwd: join(NPM_DIR, "cli") });
  console.log(`\npublished ${MAIN}@${version}`);
} else {
  console.log(`\nBuilt 6 binaries + synced versions to ${version} (dry run).`);
  console.log(`Publish with:  npm login  &&  node npm/release.mjs ${version} --publish`);
}
