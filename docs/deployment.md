# Deployment

## Options

Deploy with one of these paths:

- Docker Compose with Traefik and HTTPS: `docker/docker-compose.yaml`.
- Docker Compose without HTTPS: `docker/docker-compose.simple.yaml`.
- Release bundle from `make build`.

The compose files use the GHCR image `ghcr.io/royalcat/factorio-server-manager:latest`.

## Docker Compose

Copy the compose file to the host:

```sh
cp docker/docker-compose.yaml /path/to/server/
```

For local or private-network use without HTTPS:

```sh
cp docker/docker-compose.simple.yaml /path/to/server/
```

Set environment values:

- `RCON_PASS`: optional RCON password. If empty, one is generated.
- `FSM_ADMIN_USERNAME`: initial web admin username. Defaults to `admin`.
- `FSM_ADMIN_PASSWORD`: initial web admin password. If empty, one is generated.
- `DOMAIN_NAME`: required by the Traefik compose file.
- `EMAIL_ADDRESS`: required by Let's Encrypt in the Traefik compose file.

Start:

```sh
docker compose up -d
```

Simple mode:

```sh
docker compose -f docker-compose.simple.yaml up -d
```

## Persistent Data

The compose files mount:

- `./fsm-data` to `/opt/fsm-data`
- `./factorio-data` to `/opt/factorio`
- `./factorio-data/mod_packs` to `/opt/fsm/mod_packs`

Mounting the whole Factorio directory persists the downloaded Factorio binary, `data/`, saves, mods, and config across container rebuilds and restarts.

Back up these directories before upgrades.

## Ports

- Manager UI: TCP `80`, or TCP `443` with Traefik.
- Factorio game server: UDP `34197`.

## First Start

The container starts Factorio Server Manager without downloading Factorio. Log in and install the target Factorio headless server version from the Server Status panel.

If `RCON_PASS` is empty, check the generated value in:

```text
fsm-data/conf.json
```

`FSM_ADMIN_USERNAME` and `FSM_ADMIN_PASSWORD` are used only when the user database is empty. They do not reset existing users after `fsm-data/sqlite.db` exists.

If no admin password was configured, check container logs:

```sh
docker logs factorio-server-manager
```

## Updating Factorio

1. Save the game in the UI.
2. Stop the Factorio server in the UI.
3. Select the target Factorio version in the Server Status panel.
4. Click Install.

## Release Bundle

Build a release bundle:

```sh
make build
```

The output zip is written under `build/`. It contains the backend binary, generated frontend assets, and a starter `conf.json`.

## Release Automation

Pushing to `develop` or tagging a release triggers `.github/workflows/build-docker.yaml`, which builds and pushes the multi-arch GHCR Docker images (`ghcr.io/royalcat/factorio-server-manager`) with `GITHUB_TOKEN`.
