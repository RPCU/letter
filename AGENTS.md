# AGENTS.md

Guidance for AI agents and contributors working in this repository.

## Golden rules

- **NEVER commit without explicit user confirmation.** Do not run `git commit`,
  `git push`, `git tag`, or any history-rewriting command unless the user has
  clearly asked for it in that turn.
- Match the commit style of the sibling repos (gitmoji): `feat ✨:`,
  `fix 🐛 (scope):`, `refactor 🎨:`.

## Project overview

Letter is a one-page Go web app: a sealed envelope, a letter (`LETTER_TITLE`,
`LETTER_MESSAGE`, signed `LETTER_FROM`) and a question (`LETTER_QUESTION`)
with a yes and a no. After each answer, Claude (through Claude Code) writes a
short note, streamed into the page. Apology by default; every word is
configurable (a good trip, an invitation…). There is no auth (an optional code, `LETTER_PIN`) and no prompt
box. The only thing written is the answers log (`answers.go`).

Module path: `github.com/Banh-Canh/letter`.

- Entry point: `main.go` → `cmd/` (`serve` default, `version`; stdlib only).
- Env config + validation: `internal/config/`.
- HTTP routes, the answer stream, Claude Code, security headers: `internal/server/`.
- Template + static assets, embedded with `embed`: `web/`.
- Nix build (binary, OCI image): `nix/`.

## Build / test / verify

```sh
go build ./... && go vet ./... && go test -race ./...
gofumpt -d .                         # must print nothing
golines --max-len=160 . --dry-run    # must print nothing
nix-build nix/oci.nix --argstr doCheck false   # after go.mod / nix/ changes
```

Use `devenv shell -- <cmd>`, which provides Go, gofumpt, golines and air.

## Conventions

- Standard library only (the notes shell out to the `claude` CLI, which the
  images ship). Adding a dependency means `vendorHash` in
  `nix/binaries.nix` stops being `null` and has to be computed.
- Logging is `log/slog` (JSON) only. Hestia's `default-error-rate` alert fires
  on **any `ERROR` log line**, so log at ERROR only for real failures; Claude
  failures (not signed in, limits, missing) are Warn.
- New settings are `LETTER_*` env vars, validated in `config.Load`, which
  reports every invalid setting at once. Every setting is optional with a
  default; keep it that way. Document them in README.md and wire them into
  hestia's ReleaseBindings.
- The page has a strict CSP: no inline scripts or styles, and only Google
  Fonts is allowed off-origin. Pass data to JS through `data-*` attributes;
  JS styles through classes and CSS custom properties (CSSOM).
- `LETTER_GIFT` is never in the page, meta tags, logs or Claude's prompt: only
  the `done` event of a yes carries it. `LETTER_CONTEXT` is for Claude only,
  never rendered. The link preview says only "A letter for {name}". Tests
  assert this.
- `LETTER_PIN` (`pin.go`) locks the page: without the key cookie, `/` renders
  only `lockedView()` (seal, theme, no name or words) and `/api/answer`
  answers 403. The cookie is an HMAC derived from the code, never the code;
  guesses go through their own global limiter. Tests assert the locked page
  leaks nothing.
- The tone is the product: cute but mature, never cheesy. The system prompt
  forbids guilt-tripping, pressure and invented memories; keep it. The page
  steers toward yes gently (No shrinks, Yes grows, No leaves when
  `LETTER_NO` runs out, and the server answers 409 to a No past it), but
  every No still gets a gracious note.
- Keep Claude Code isolated (`claudeArgs`/`claudeEnv`, same as birthday):
  no tools, MCP servers, connectors, plugins, settings, memory or saved
  sessions, an empty working directory, and never `LETTER_*` in its
  environment. Runs take turns (`turn.go`: in-process slot plus a flock on
  `$CLAUDE_CONFIG_DIR/letter-claude.lock`), since concurrent OAuth renewals
  revoke each other. When Claude fails, canned notes (`LETTER_FALLBACK_*`)
  stand in for `awayFor` (a minute).
- Each no's style, label and fallback are indexed by the count of earlier
  noes, capped at the last one. Keep the default lists the same length.
- CSS/JS URLs carry `?v=<content hash>` (`fingerprint` in the server). Keep
  any new asset linked from the template in that fingerprint.
- The image's default command must start the server (`serve`): OpenChoreo
  cannot set command/args. Both images (Dockerfile, `nix/oci.nix`) ship Claude
  Code, a shell for `claude auth login`, and a writable `/home/letter` with
  `CLAUDE_CONFIG_DIR=/home/letter/.claude`; keep them in sync.
- `workload.yaml` (repo root) is the OpenChoreo workload descriptor:
  `endpoint-1`, HTTP on 8080, which hestia's `http-route` trait and the
  web-application ComponentType rely on. Keep its port in sync with
  `LETTER_PORT`, and don't delete it.
- Respect `prefers-reduced-motion` and keep both color schemes and all five
  themes working when touching the CSS.

## CI / CD

- **ci.yaml**: vet, race tests, build.
- **linting.yaml**: gofumpt + golines (160).
- **image.yaml**: pushes `zot.rpcu.io/public/letter:<8-char SHA>` (plus
  `:<tag>`). Uses the org's `REGISTRY_USER` / `REGISTRY_PASSWORD` secrets.
  OpenChoreo's `prebuilt-image` workflow (hestia) waits for that exact tag.
- **nix-build.yaml**: builds `nix/oci.nix` when go.mod / nix change.
