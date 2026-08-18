[![docker](https://github.com/royalcat/factorio-server-manager/actions/workflows/build-docker.yaml/badge.svg)](https://github.com/royalcat/factorio-server-manager/actions/workflows/build-docker.yaml)
[![Discord](https://img.shields.io/discord/779512040934342687?label=Discord)](https://discord.gg/SB647WmSbU)

# Factorio Server Manager

### A tool for managing Factorio servers.
This tool runs on a Factorio server and allows management of the Factorio server, saves, mods and many other features.

> [!NOTE]
> This is a fork of [OpenFactorioServerManager/factorio-server-manager](https://github.com/OpenFactorioServerManager/factorio-server-manager) based on another fork [dnaroma/factorio-server-manager](https://github.com/dnaroma/factorio-server-manager).
>
> **Added by this fork**
> - ARM64 & RISC-V support via box64 emulation
> - Proper docker image build pipeline
>
> **Added by the dnaroma fork**
> - Mod portal integration with metadata, updates, and dependency checks
> - Modpacks with diff view and dry-run validation
> - Save backup/restore, scheduled backups, and fresh restart
> - Grouped server settings editor with validation and change preview
> - Factorio version management with controlled upgrades
> - Server lifecycle controls and event history

## Features
* Allows control of the Factorio Server, starting and stopping the Factorio binary.
* Allows the management of save files, upload, download, backup, restore, duplicate, rename and delete saves.
* Manage installed mods, upload new ones and more
* Manage modpacks, so it is easier to play with different configurations
* Allow viewing of the server logs and current configuration.
* Authentication for protecting against unauthorized users
* Available as a Docker container

#### Manage Factorio Server
![Factorio Server Manager Screenshot](screenshots/Screenshot_Controls.png)

#### Manage save files
![Factorio Server Manager Screenshot](screenshots/Screenshot_Saves.png)

#### Manage mods
![Factorio Server Manager Screenshot](screenshots/Screenshot_Mods.png)

## Docker

```sh
docker run -d \
  --name factorio-server-manager \
  -p 80:80 \
  -p 34197:34197/udp \
  -v ./fsm-data:/opt/fsm-data \
  -v ./factorio-data:/opt/factorio \
  -v ./factorio-data/mod_packs:/opt/fsm/mod_packs \
  -e FSM_ADMIN_USERNAME=admin \
  -e FSM_ADMIN_PASSWORD=changeme \
  -e RCON_PASS=changeme \
  ghcr.io/royalcat/factorio-server-manager:develop
```

Then open `http://localhost` in your browser. On first start, install the Factorio headless server from the Server Status panel. See [docker/README.md](docker/README.md) for the full Docker guide.

## Documentation

- [Development](docs/development.md)
- [Maintenance](docs/maintenance.md)
- [Deployment](docs/deployment.md)
- [Roadmap](docs/roadmap.md)
- [Docker usage](docker/README.md)

## Contributing
1. Fork it!
2. Checkout the develop branch, only use that as a base: `git checkout develop`
2. Create your feature branch: `git checkout -b my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Add your changes a in human readable way into CHANGELOG.md
4. Push to the branch: `git push origin my-new-feature`
5. Submit a pull request, with `develop` as base :D

## Authors

* **Mitch Roote** - [roote.ca](https://roote.ca)
* **[knoxfighter](https://github.com/knoxfighter)**
* **[Jannaahs](https://github.com/jannaahs)**

## Special Thanks
- **[dnaroma/factorio-server-manager](https://github.com/dnaroma/factorio-server-manager)** for the fork this project is based on
- **[All Contributions](https://github.com/dnaroma/factorio-server-manager/graphs/contributors)**
- **mickael9** for reverseengineering the factorio-save-file: https://forums.factorio.com/viewtopic.php?f=5&t=8568#

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details
