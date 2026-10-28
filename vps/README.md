# GrubDrops on a VPS

This folder deploys a prebuilt miner image from GitHub Container Registry (GHCR).
The database and encryption key stay in `vps/data`. Caddy can provide HTTPS,
and Watchtower checks for a new miner image every six hours.

Watchtower needs `/var/run/docker.sock` to replace containers. Access to this
socket gives Watchtower broad control over Docker on the VPS. The miner itself
does not receive the socket for Twitch-only use.

## Requirements

- A Linux VPS with Docker Engine and the Docker Compose plugin.
- A domain whose DNS A/AAAA record points to the VPS, with inbound TCP 80 and
  443 allowed by the VPS firewall/provider.
- The miner image published to GHCR by `.github/workflows/publish-vps-image.yml`.

## One-time setup

Copy this `vps/` folder to the VPS. Copy `.env.example` to `.env`, then set:

- `GRUB_IMAGE=ghcr.io/YOUR_GITHUB_USERNAME/grubdrops:latest`
- `GRUB_DOMAIN=your-real-domain.example`
- `TZ=Europe/Moscow` (or your preferred IANA timezone)

After the first image is published, make the GHCR package public in GitHub's
package settings so the VPS can pull it without credentials. Alternatively,
authenticate Docker on the VPS with a GitHub token that has `read:packages`.

The `.env` values and GHCR visibility are set once. For initial deployment with
HTTPS, create the data folder and run:

```sh
mkdir -p data
sudo chown -R 65532:65532 data
docker compose -f compose.yaml -f compose.https.yaml up -d
```

Caddy obtains and renews the TLS certificate. Open `https://your-real-domain/`
and complete the app's first-run setup. The miner itself is not exposed on a
public port; Caddy forwards HTTPS traffic over the private Compose network.

## How code updates become automatic

1. Push a change to `main` or `master` in your GitHub repository. The VPS does
   not receive unpushed changes from your PC.
2. The existing CI workflow builds and tests the project.
3. After CI succeeds, `publish-vps-image.yml` builds a multi-architecture image
   and publishes `ghcr.io/YOUR_GITHUB_USERNAME/grubdrops:latest`.
4. Watchtower on the VPS checks GHCR every six hours and replaces the miner
   container when that image changes. The `data` folder remains in place.

To apply an already-published image immediately instead of waiting for
Watchtower, run this from the `vps/` directory:

```sh
sh update.sh
```

If Kick sidecars are enabled, run `GRUB_VPS_KICK=1 sh update.sh` so the miner
keeps its Docker socket mount. The script pulls the current `GRUB_IMAGE` and
recreates only the miner; the database, key, Caddy and certificates stay in
place.

Logs and shutdown:

```sh
docker compose -f compose.yaml -f compose.https.yaml logs -f miner
docker compose -f compose.yaml -f compose.https.yaml down
```

`down` does not delete `data` or Caddy's certificate volumes. Back up `data`
regularly; the database and `master.key` must be kept together.

## Local or SSH-tunnel check without a domain

This mode builds from the source tree and binds the web port to loopback only.
It is for a local check or an SSH tunnel, not public HTTP:

```sh
docker compose -f compose.yaml -f compose.local.yaml up -d --build
```

From your computer, open a tunnel:

```sh
ssh -L 8080:127.0.0.1:8080 your-user@your-vps
```

Then open `http://localhost:8080`. To use this source-build mode, run it from
the project's `vps/` directory and keep the rest of the source tree next to it.

## Moving the existing Windows test data

Stop `grubdrops.exe` cleanly with `Ctrl+C`. Copy `data-test/miner.db` and
`data-test/master.key` into the VPS `vps/data/` directory, and ensure it is owned
by UID 65532. Keep the database and matching key together: without that key the
saved account sessions cannot be decrypted. Never commit or share `master.key`
or `.env`.

For a new installation, leave `data` empty. On first startup the miner creates
both `miner.db` and `master.key` there.

## Optional Kick browser sidecars

Twitch-only use does not need the miner to control Docker. Kick watch-time
sidecars are created on demand through `/var/run/docker.sock`. If you enable
Kick, add `-f compose.kick.yaml` to the Compose commands. For example:

```sh
docker compose -f compose.yaml -f compose.https.yaml -f compose.kick.yaml up -d
```

The Docker socket grants the miner broad control of Docker on the VPS. Enable it
only when Kick browser playback is needed. On ARM64 VPSes, a Kick browser
sidecar can use several gigabytes of RAM per active account.
