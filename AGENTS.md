# Agent Instructions

## Project Overview
- Factorio Server Manager is a Go backend with a React/Vite frontend.
- Backend code lives in `src/`; `go.mod` is at the repository root (there is no `src/go.mod`).
- Frontend source lives in `ui/`; built assets are emitted into `app/`.
- Docker deployment config lives in `docker/`; the root `Dockerfile` is a multistage build that produces the container image, and CI publishes it as a multi-arch image.

## Common Commands
- Install project tools: `mise install`
- Install frontend dependencies: `CI=true mise exec -- pnpm install`
- Build frontend assets: `mise exec -- pnpm run build` (Vite emits `app/bundle.js`, `app/style.css`, and `app/index.html`)
- Start the Vite dev server: `mise exec -- pnpm run dev`
- Run focused backend tests in cached Linux Docker: `./scripts/go-test-docker.sh ./factorio -run TestName -v`
- Run the full backend suite in short mode: `./scripts/go-test-docker.sh ./... -v -test.short`
- Build the container image: `make docker-build` (same as `docker build -t factorio-server-manager:develop .`)
- Clean generated artifacts: `make clean`
- The root `Makefile` only defines `docker-build` and `clean`; there is no `make build`, `make app/bundle`, or `make test-go-docker` target.

## Devcontainer
- The `.devcontainer/` setup installs mise and then runs `mise install` plus `pnpm install`.
- Tool versions are pinned in `.mise.toml`.
- The devcontainer sets `FSM_MODPACK_DIR`, `FSM_DIR`, and `FSM_CONF` to match the defaults used by `scripts/go-test-docker.sh`.
- If Go tests mutate `conf.json.example`, restore that fixture before committing.

## Working Rules
- Keep changes focused and avoid unrelated refactors.
- Use `gofmt` for Go files before committing.
- **Do NOT run `go build` or `go test` natively on macOS.** The backend deploys to Linux; some Go files use `_linux` build tags (e.g. `server_linux.go`) and will not compile on macOS. Always use `scripts/go-test-docker.sh` for Go compilation and testing — it runs inside a Linux Docker container with cached Go module/build volumes.
- Do not edit generated frontend assets in `app/`; edit `ui/` and rebuild.
- Do not commit local runtime files such as `conf.json`, `.env`, `dev/`, `dev_packs/`, `build/`, `node_modules/`, or generated bundles.
- Preserve existing API routes and authentication behavior unless the task explicitly changes them.
- Prefer short Go tests near the changed package; run them through `scripts/go-test-docker.sh` because the backend targets Linux deployments and the script preserves Go module/build caches.
- UI has no configured test runner.

## Project Conventions
- Backend packages:
  - `api`: HTTP routes, handlers, auth, websocket setup.
  - `factorio`: Factorio server, saves, mods, config, RCON, and portal logic.
  - `bootstrap`: startup configuration and initial users.
  - `lockfile`: lockfile helper.
- Frontend conventions:
  - React components use `.jsx` under `ui/App/`.
  - Frontend entrypoint is `ui/index.jsx`.
  - API clients live under `ui/api/`.
  - Styling starts at `ui/index.scss` with Tailwind configured in `tailwind.config.js`.
  - Vite configuration lives in `vite.config.js`.
- CI (`.github/workflows/build-docker.yaml`) builds and pushes the multi-arch Docker image; the Dockerfile runs `pnpm run build` and `go build ./src/main.go`, and CI does not run Go tests. Run `./scripts/go-test-docker.sh ./... -v -test.short` locally before committing backend changes.

## Mod Dependency Validation
- Factorio ships bundled ("built-in") mods as directories under `<factorio_dir>/data/<name>`; they have no zip in `mods/`, so dependency checks must treat dependencies on them as satisfied.
- Factorio 2.0 bundled mods: `base`, `elevated-rails`, `quality`, `space-age`. Factorio 2.1 split the recycler out of `quality` into the bundled `recycler` mod, and `quality` now depends on it.
- `factorio.IsBuiltInMod` (`src/factorio/mod_Mods.go`) combines a static allowlist with discovery of `<dirname(config.FactorioBaseModDir)>/<name>/info.json`, so bundled mods added in future Factorio versions are recognized without code changes. The static allowlist is the fallback when Factorio is not installed (and for unit tests).
- `requiredDependencyNames` filters bundled mods before validation. It feeds `ValidateEnabledDependencies` (blocks server start in `api/handlers.go`, mod updates, and mod-pack validation), `ValidateModCanEnable` (toggling a mod on), and `ValidateModCanDisable` (toggling off or deleting).
- Bundled mods are intentionally absent from the mod UI. The frontend also keeps a copy of the list in `ui/App/views/Mods/Mods.jsx` and `ui/App/views/Mods/components/LoadMods.jsx`; keep both in sync when a new bundled mod appears, otherwise `LoadMods` tries to install it from the mod portal and fails.

## Test Gotchas
- `bootstrap.Config` is a process-global (`bootstrap.instantiated`) mutated by `bootstrap.NewConfig`. Non-flag fields such as `FactorioBaseModDir` are not reset by `mapFlags`, so tests that rely on config paths must write them into their temp `conf.json` (for example `"basemod_dir"`); otherwise the test passes in isolation but inherits a stale value in the full suite.
- `bootstrap.NewConfig` can resave the config and generates a random rcon password/port; always point tests at throwaway config files.
- `scripts/go-test-docker.sh` runs `go test` in `golang:1.26.4` with `/workspace/src` as the workdir, `FSM_DIR=../`, `FSM_CONF=../../conf.json.example`, and `FSM_MODPACK_DIR=../dev_pack`; Go resolves the module from the repository-root `go.mod`. Its default arguments are `./... -v -test.short`, which skips the long mod-pack handler tests; omit `-test.short` to run them.

## Commit Messages
- Prefer conventional commits, for example `fix: handle missing save name` or `docs: add agent instructions`.
