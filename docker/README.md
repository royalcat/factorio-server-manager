# Factorio Server Manager Docker Image

## Prerequisites
You need to have [Docker](https://www.digitalocean.com/community/tutorials/how-to-install-and-use-docker-on-ubuntu-20-04)
and [Docker Compose](https://docs.docker.com/compose/install/) installed.

## Getting started?

Copy `docker-compose.yaml` and `.env` files from this repository to somewhere on your server.

Edit values in the `.env` file:
* `RCON_PASS` (default empty string): Password for Factorio RCON (FSM uses it to communicate with the Factorio server). \
  If left empty, a random password will be generated and saved on the first start of the server. You can see the password in `fsm-data/conf.json` file.
* `FSM_ADMIN_USERNAME` (default `admin`): Initial web admin username. Used only when the user database is empty.
* `FSM_ADMIN_PASSWORD` (default empty string): Initial web admin password. If left empty, a random password is generated and logged on first start. Used only when the user database is empty.
* `DOMAIN_NAME` (must be set manually): The domain name where your FSM UI will be available. Must be set,
  so [Let's Encrypt](https://letsencrypt.org/) service can issue a valid HTTPS certificate for this domain.
* `EMAIL_ADDRESS` (must be set manually): Your email address. Used only by Let's Encrypt service.

Alternatively you can ignore `.env` file and edit this values directly in `environment` section of `docker-compose.yaml`.
But remember that if `.env` file is present, values set there take precedence over values set in `docker-compose.yaml`.

Now you can start the container by running:

```
docker-compose up -d
```

### Simple configuration without HTTPS

If you don't care about HTTPS and want to run just the Factorio Server Manager, or want to run it on a local machine you can use `docker-compose.simple.yaml`.

Ignore `DOMAIN_NAME` and `EMAIL_ADDREESS` variables in `.env` file and run
```
docker-compose -f docker-compose.simple.yaml up -d
```

### Factorio version

The container starts Factorio Server Manager without downloading Factorio. After logging in, install the Factorio headless server from the Server Status panel. Choose `Stable`, `Latest experimental`, or enter a specific version such as `1.1.110`.

The example Compose files persist the whole Factorio directory by mounting `./factorio-data` to `/opt/factorio`. This keeps the downloaded Factorio binary and `data/` directory across container rebuilds and restarts, along with saves, mods, and config.

## Accessing the application

Go to the domain specified in your `.env` file in your web browser. If running on localhost access the application at http://localhost

### First start

When the container starts, Factorio Server Manager is available before Factorio is installed. Install the Factorio server from the web UI before creating or starting saves.

It may take some time for Let's Encrypt to issue the certificate, so for the first couple of minutes after starting the container you may see
"Your connection is not private" error when you open your Factorio Server Manager address in your browser. This error should disappear within
a couple of minutes, if configuration parameters are set correctly.

## Updating Credentials, adding and deleting users.

An admin user is created initially using the credentials defined in the factorio-server-manager config file. When you haven't specified one, a random password will be logged in the container output (`docker logs factorio-server-manager`).

Set `FSM_ADMIN_USERNAME` and `FSM_ADMIN_PASSWORD` before the first startup to predefine the initial web admin credentials. These values do not reset existing users after `fsm-data/sqlite.db` has been created.

Users can be added and deleted on the settings page.

## Updating Factorio

You can update or downgrade Factorio from the Server Status panel while the Factorio server is stopped.

If you want to update Factorio:
1. Save your game and stop Factorio server in FSM UI.
2. Select the target version in the Server Status panel.
3. Click Install.

## Save backups

Manual save backups created in the UI are stored below `/opt/factorio/saves/backups`. The example Compose files mount `/opt/factorio` to `./factorio-data`, so backups persist across container restarts with the save files.

Restore operations write a temporary file in the saves volume and then rename it into place. This avoids partially restored saves when the container is stopped during a restore.

These backups are on the same Docker volume as the saves. Back up `./factorio-data` separately if you need protection from host disk or volume loss.

## Security

Authentication is supported in the application, but it is recommended to ensure access to the Factorio manager UI is accessible via VPN or internal network.

## Development

Build the container image directly from source, no host toolchain required:

```sh
docker build -t factorio-server-manager:dev .
```

The multistage `Dockerfile` at the repository root compiles the frontend, cross-compiles the Go backend, and assembles a runtime image in one command.

To build and push for a release:

```sh
docker buildx build \
  --platform linux/amd64 \
  --tag ghcr.io/royalcat/factorio-server-manager:latest \
  --push \
  .
```

Release bundles (zip) for non-Docker installs are produced with `make build` from the repository root.

## For everyone who actually read this thing to the end

And now go and build some nice factories!
