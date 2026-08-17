# Development

## Repository Layout

- `src/`: Go backend module.
- `ui/`: React/Vite frontend source.
- `app/`: generated frontend bundle served by the backend.
- `docker/`: Compose files, entrypoint, and deployment helpers. The root `Dockerfile` is a multistage build that produces the container image from source.
- `conf.json.example`: default local configuration fixture.

## Tooling

Install pinned tools with:

```sh
mise install
```

The pinned tool versions are in `.mise.toml`. Use `mise exec -- <command>` when running project commands.

Install frontend dependencies:

```sh
CI=true mise exec -- pnpm install
```

## Frontend Workflow

Run Vite:

```sh
mise exec -- pnpm run dev
```

Build frontend assets:

```sh
mise exec -- make app/bundle
```

or:

```sh
mise exec -- pnpm run build
```

Edit files under `ui/`. Do not edit generated files under `app/` directly.

## Backend Workflow

Backend code is in the Go module under `src/`.

Run short backend tests in a Linux environment:

```sh
make test-go-docker
```

Full tests use Factorio portal credentials when available:

```sh
./scripts/go-test-docker.sh ./... -v
```

The Docker runner reuses named volumes for `/go/pkg/mod` and `/root/.cache/go-build`, so repeated runs do not re-download modules or rebuild unchanged packages.

Run `gofmt` before committing Go changes:

```sh
cd src
mise exec -- gofmt -w .
```

## Local Runtime

Build the frontend first, then build the backend and run it from the repository root so it can serve `app/`:

```sh
mise exec -- make app/bundle
cd src
mise exec -- go build -o ../factorio-server-manager-dev .
cd ..
./factorio-server-manager-dev --conf conf.json.example --dir .
```

Useful flags and matching environment variables are defined in `src/bootstrap/config.go`. Common ones:

- `--conf` / `FSM_CONF`
- `--dir` / `FSM_DIR`
- `--host` / `FSM_SERVER_IP`
- `--port` / `FSM_PORT`
- `--mod-pack-dir` / `FSM_MODPACK_DIR`

## Before Opening a Pull Request

Run:

```sh
mise exec -- make app/bundle
make test-go-docker
```

If tests mutate `conf.json.example`, restore it before committing.
