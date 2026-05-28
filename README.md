# Sharding Reverse Proxy for Etherpad
This is a reverse proxy that which will shard pads based on padId(within query[currently in socket-namespace branch of Etherpad core]) to a pool of backends based on the availability of the backends which is based on the number of concurrent active Pads being edited.

This is now a production grade reverse proxy written in Golang.

## Getting started for production

1. Install Docker
2. Install Docker Compose
3. Use the provided docker-compose
4. Copy ``settings.json.template`` to ``settings.json`` and modify the values.
5. Depending on the `port` value in `settings.json`, you may need to modify the `ports` section of the `docker-compose.yml` file.

```yaml
  reverse-proxy:
    container_name: reverse-proxy
    image: ghcr.io/ether/etherpad-proxy:latest
    ports:
      - "9000:9000"
    volumes:
      - ./reverse-proxy/db:/app/db
      - ./settings.json:/app/settings.json
```

Visit http://localhost:9000

## Installation without Docker

The proxy is a single static Go binary. You can run it directly on a host
without Docker.

### Option A: download a prebuilt binary

1. Go to the [Releases page](https://github.com/ether/etherpad-proxy/releases)
   and download the archive for your OS/architecture.
2. Extract it: `tar xzf etherpad-proxy_*.tar.gz` (or unzip on Windows).
3. Continue with **Configure** below.

### Option B: build from source

Requires Go 1.25 or newer.

```bash
git clone https://github.com/ether/etherpad-proxy.git
cd etherpad-proxy
go build -o etherpad-proxy .
```

### Configure

1. Copy the template and edit it:
   ```bash
   cp settings.json.template settings.json
   ```
2. Set `port` (proxy listen port), optionally `managementPort`
   (default `8081`, serves `/pads`, `/metrics`, `/healthz`, `/readyz`), your
   `backends`, and a database (see **Database** below).
3. The settings path defaults to `./settings.json`; override it with the
   `SETTINGS_FILE` environment variable.

### Run

```bash
SETTINGS_FILE=/path/to/settings.json ./etherpad-proxy
```

The proxy listens on `port`; management/metrics/health endpoints listen on
`managementPort`.

### Database

- **SQLite (default, single instance):** set `dbSettings.filename`, e.g.
  `"db/etherpad-proxy.db"`. No external services required. Create the directory
  first: `mkdir -p db`.
- **Postgres (recommended for multiple proxy instances):** set
  `dbSettings.postgresConnstr`. With a shared Postgres, several proxy instances
  share routing state and assign new pads atomically, so they never route the
  same pad to different backends.

  Provision a database and user, for example:
  ```sql
  CREATE USER proxy WITH PASSWORD 'changeme';
  CREATE DATABASE etherpad_proxy OWNER proxy;
  ```
  Then set:
  ```json
  "dbSettings": {
    "postgresConnstr": "postgres://proxy:changeme@db-host:5432/etherpad_proxy?sslmode=disable"
  }
  ```
  The proxy creates its tables automatically on first start. Set exactly one of
  `filename` or `postgresConnstr`.

### Run as a systemd service (Linux)

A sample unit is provided at `support/etherpad-proxy.service`.

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin etherpad-proxy
sudo mkdir -p /opt/etherpad-proxy/db
sudo cp etherpad-proxy /opt/etherpad-proxy/
sudo cp settings.json /opt/etherpad-proxy/
sudo cp support/etherpad-proxy.service /etc/systemd/system/
sudo chown -R etherpad-proxy:etherpad-proxy /opt/etherpad-proxy
sudo systemctl daemon-reload
sudo systemctl enable --now etherpad-proxy
```

Check status and logs:
```bash
systemctl status etherpad-proxy
journalctl -u etherpad-proxy -f
```

## Settings

Settings come from ``settings.json``, see ``settings.json.template`` for an example to modify for your environment.

``backends`` is your Backend Etherpad instances.

``maxPadsPerInstance`` is how many active pads you want to allow per instance.  This value should be between 1 and 20000 depending on the number of authors and words per minute that you limit or you wish to allow.  Once this limit is met then random instances will be used.

``checkInterval`` is how often to check every backend for availability.  You should set this to a low number if you have lower number of very active instances with short pad life.  You should set this to a high number if you have lots of instances with relatively long pad life expectancy.

``port`` is the port this service listens on.

## OAuth2 backed Etherpads

``clientId`` is the clientId used to authenticate with the backend Etherpad instances.  This should be a random string that is unique to this service.

``clientSecret`` is the clientSecret used to authenticate with the backend Etherpad instances.  This should be a random string that is unique to this service.
``tokenURL`` is the URL of the OAuth2 token endpoint. This is normally `http://<your-host>:<your-port>/oidc/token`

## Basic Auth-backed Etherpads

``username`` is the username used to authenticate with the backend Etherpad instances.  This should be a random string that is unique to this service.

``password`` is the password used to authenticate with the backend Etherpad instances.  This should be a random string that is unique to this service.

Pads will be fetched every checkInterval * seconds whereas the normal checkInterval runs every checkInterval * milliseconds.
If pads are deleted they are also deleted from the reverse proxy so it can be reassigned to another backend.


## Database support

- SQLite (default, file-based, no setup required) - specified by dbSettings.filename = "db/etherpad-proxy.db"
- Postgres - specified by dbSettings.postgresConnstr - e.g. postgres://user:password@localhost:5432/etherpad_proxy_db?sslmode=disable

# License
Apache 2
