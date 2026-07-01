# Releasing console.store

How `console` ships, updates itself, and how to push to each channel. This is the
agent-facing playbook **and** the human runbook. If you are an agent and the user
says "push this to alpha / beta / main", do exactly what **§3** says.

---

## 1. The model in one paragraph

Users install one binary, always named **`console`**, via
`curl -fsSL consolestore.in/install | sh`. On every launch it checks its channel's
signed manifest and, if a newer signed build exists, downloads + verifies + swaps
itself and re-execs — so the running session is already on the latest. The binary
name never changes; only its **channel** differs. The channel is pinned in
`~/.config/console-store/channel.json` and chosen at install time (or via
`console update --channel …`). Local dev builds are a *different* binary name
(`localconsole` / `localsafeconsole`, see `scripts/build.sh`) and never auto-update.

## 2. Channels

| Channel | Who gets it | Install | Tag shape |
| --- | --- | --- | --- |
| **stable** | everyone (default) | `curl -fsSL consolestore.in/install \| sh` | `vX.Y.Z` |
| **beta** | opt-in testers | `… \| sh -s -- --beta` | `vX.Y.Z-beta.N` |
| **alpha** | invite-only (per-person code) | `… \| sh -s -- --alpha --code <CODE>` | `vX.Y.Z-alpha.N` |

Windows: `irm consolestore.in/install.ps1 | iex` (set `$env:CONSOLE_CHANNEL` /
`$env:CONSOLE_ALPHA_CODE` for non-stable).

A user switches channels without reinstalling:
`console update --channel beta` (alpha needs `--code`). The marker file is rewritten
and the next launch tracks the new channel.

## 3. Pushing a release — what the agent does

The user speaks in channel names; you translate to a **git tag** and push it. CI
(`.github/workflows/release.yml`) does the build, sign, and publish. **Promotion is
re-tagging the same commit — never rebuild to move a build up a channel.**

| User says | You run |
| --- | --- |
| "push to alpha" / "ship alpha" / "cut an alpha" | `git tag vX.Y.Z-alpha.N && git push origin vX.Y.Z-alpha.N` |
| "push to beta" / "promote to beta" | `git tag vX.Y.Z-beta.N <same-commit> && git push origin vX.Y.Z-beta.N` |
| "push to main" / "production" / "stable" / "release it" | `git tag vX.Y.Z <same-commit> && git push origin vX.Y.Z` |

**Enforced promotion path (local → alpha → beta → production).** Every change is
first built + tested locally as `localconsole`/`localsafeconsole` (`scripts/build.sh`,
which gates on vet+test), then shipped to **alpha**, then promoted to **beta**,
then **stable** — never skipping. CI **enforces** this: the "Enforce promotion
path" step refuses a `-beta` tag unless an `-alpha` release exists on the **same
commit**, and refuses a stable tag unless a `-beta` does. So you cannot tag beta
or production directly — the release fails before it builds. alpha is the entry
point (its gate is the local `build.sh` run). Promotion = re-tag the same commit.

**Choosing the version number:**
- New work in flight → start an alpha cycle: bump the base version, start at `-alpha.1`
  (e.g. last stable `v0.3.0` → first alpha of the next release is `v0.4.0-alpha.1`).
- More alpha iterations → bump the prerelease counter: `-alpha.2`, `-alpha.3`, …
- Promote alpha → beta: keep the base version, switch suffix, restart counter:
  `v0.4.0-alpha.3` → `v0.4.0-beta.1` (tag the **same commit** the alpha pointed at).
- Promote beta → stable: drop the suffix: `v0.4.0-beta.2` → `v0.4.0`.
- Patch a shipped stable → `vX.Y.(Z+1)`.

**Always, before tagging:**
1. Confirm you're on the intended commit (`git log --oneline -3`). For a promotion,
   tag the exact commit the lower channel already validated:
   `git tag v0.4.0-beta.1 <sha-of-v0.4.0-alpha.3>`.
2. Confirm tests are green (`go test ./...`) — CI gates on this too, but don't push a
   tag you expect to fail.
3. Push the tag (`git push origin <tag>`). Pushing the tag is what triggers the
   release; pushing the branch does not.
4. Watch it: `gh run watch` (or `gh run list --workflow=release.yml`).
5. Verify it went out (§6).

**Do NOT** delete/move a published tag to "redo" a release — cut a new counter
(`-alpha.4`) instead. A tag that's already been downloaded must stay immutable.

## 3b. Writing release notes (the in-app "what's new")

After a user updates, the TUI shows a one-time **"what's new"** modal with the
release's notes. The notes are the **GitHub Release body** for that tag — the landing
proxies it at `consolestore.in/<channel>/notes/<tag>`, and `console` fetches it on the
first launch after an update. GoReleaser's auto-changelog is disabled
(`.goreleaser.yaml` → `changelog.disable`), so the body is **only what you write**.

- **Write user-facing notes in the GitHub Release body** for the tag (a few bullet
  lines — what changed, in plain language; light markdown: `#` headers, `- ` bullets).
  Do it after CI publishes the release (the body starts empty), or pre-stage them.
- **No notes?** Leave the body empty → the endpoint 404s → the app shows nothing and
  silently records the version. Notes are optional per release.
- Notes show **once per version** (the app records the last-seen version; updates never
  re-show, fresh installs get the onboarding manual instead, dev builds skip). They
  degrade gracefully — offline/slow fetch shows nothing and never blocks launch.
- Verify a tag's notes endpoint: `curl -s consolestore.in/stable/notes/<tag>`
  (alpha needs `?code=<CODE>`).

## 4. What CI does on a tag (so you can debug it)

`.github/workflows/release.yml`, triggered by `push: tags: ['v*']`:
1. **Gate** — `go vet ./...` + `go test ./...` (arming defaults OFF under test).
2. **GoReleaser** (`.goreleaser.yaml`) cross-compiles the **armed** `console` for
   darwin/linux/windows × amd64/arm64, stamps version/channel/commit + the armed
   ldflag, and publishes the binaries + `SHA256SUMS` to a GitHub Release
   (`prerelease: auto` marks `-alpha`/`-beta` tags as prereleases).
3. **Channel derivation** — `-alpha*` → alpha, `-beta*` → beta, else stable.
4. **Sign** — `cmd/signtool sign` reads `dist/SHA256SUMS` (the authoritative
   published names+hashes), builds the URL-free manifest payload
   `{version, channel, assets{os_arch: sha256}}`, and signs it with
   `CONSOLE_SIGN_KEY` (ed25519) into `console-manifest.json`.
5. **Attach** — uploads `console-manifest.json` to the Release.

The landing site (`consolestore.in`, Railway) serves the branded endpoints:
`/install`, `/install.ps1`, `/:channel/manifest.json`, `/:channel/download/:asset`,
`/:channel/checksum/:asset`. stable/beta redirect to the public GitHub asset; alpha
is gated by code and streamed server-side (+ logged).

## 5. One-time / occasional human setup (NOT the agent's to do)

- **Signing key (once):** `go run ./cmd/signtool keygen` →
  `gh secret set CONSOLE_SIGN_KEY --repo sujan174/console.store` (paste the
  `PRIVATE=` value) and embed the `PUBLIC=` value in
  `internal/updater/pubkey.go` (`signingPubKeyB64`). Until the pubkey is embedded,
  shipped binaries can't verify updates and the updater no-ops by design.
- **Alpha testers:** set Railway env `CONSOLE_ALPHA_CODES="alice:code1,bob:code2"`
  on the landing service. Add/revoke a tester by editing this and redeploying.
  Each alpha manifest/download hit is logged `alpha-grant who=<label> …` (Railway
  logs).
- **Landing must be deployed** before the first tag, or
  `consolestore.in/<channel>/manifest.json` 404s and installs fail.
- **`install.sh`/`install.ps1`** (served by the landing) now call `console agents install --quiet` post-install; any change to the agent-wiring logic in these scripts must be deployed to the landing alongside the binary release.

## 6. Confirming a release went out

```bash
gh run watch                                   # the release workflow
curl -s -o /dev/null -w "%{http_code}\n" https://consolestore.in/stable/manifest.json   # 200
# on a machine tracking that channel, just launch:
console version                                  # should show the new vX.Y.Z
```

For a fresh stable install end-to-end: `curl -fsSL consolestore.in/install | sh && console version`.
For alpha logging, check Railway logs for an `alpha-grant who=…` line after a coded fetch.

## 7. Safety invariants (do not break)

- The published `console` is **armed** — it places real Swiggy orders on the user's
  own account after explicit Enter-to-confirm. Never auto-confirm in code/tests.
- The updater **never touches the OS keyring** — auth survives every update.
- The ed25519 signing **private key lives only in the GH secret**; the repo holds
  only the public key. A leaked alpha code grants download access only — it cannot
  forge a binary (signature still required).
- Re-tagging the same commit is how you promote; **rebuilding to promote is wrong**
  (it produces different bytes than the channel below validated).
- **Telemetry is anonymous + fire-and-forget and NEVER touches the keyring/token or
  blocks an order.** Each ping carries only a random install id (not linked to the
  Swiggy account), the channel, and the version — never the token, address, items,
  price, or restaurant. `CONSOLE_NO_TELEMETRY=1` disables it; dev builds never send.
  Counts: launch heartbeat → installs, `broker.Service.PlaceOrder` success →
  orders. Aggregate at `GET consolestore.in/stats`; raw rows in the Railway Postgres
  `installs`/`orders` tables. Reported channel = the build's `version.Channel`.

## 8. Recovery runbook (a bad release is never unrecoverable)

The installer is a **static script on the landing**, independent of the binary — a
bad *binary* release can never break new installs. And a bad binary self-rescues in
the common case. Escalating recovery paths:

1. **Fix-forward (the normal fix).** Push a higher version on the same channel
   (`vX.Y.Z-alpha.N+1`). On the next launch every install runs the update check
   first and auto-updates to it. This works whenever the bad binary crashes *after*
   its launch-time update check (the usual case for app-logic bugs).
2. **Force re-pull (`CONSOLE_FORCE_UPDATE=1`).** If a release got a **mis-stamped
   version** so the binary thinks it's already newest and won't update, the user
   runs `CONSOLE_FORCE_UPDATE=1 console` once — it bypasses the newer-than check and
   re-pulls the channel's current signed build, then unset the var.
3. **Reinstall (always works).** `curl -fsSL consolestore.in/install | sh` (add
   `--alpha --code <CODE>` for alpha) installs the channel's latest fresh, over the
   top. This is the ground-truth recovery for any stuck state.
4. **Windows mid-swap.** If `console.exe` ever goes missing (process killed during a
   swap), `console.exe.old` in the same folder is the previous binary — rename it
   back, or just reinstall.

Never delete/move a published tag to "undo" a bad release — cut a new counter and
fix-forward; the install script is the immutable escape hatch.
