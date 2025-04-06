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

## Basic Auth backed Etherpads

``username`` is the username used to authenticate with the backend Etherpad instances.  This should be a random string that is unique to this service.

``password`` is the password used to authenticate with the backend Etherpad instances.  This should be a random string that is unique to this service.

Pads will be fetched every checkInterval * seconds whereas the normal checkInterval runs every checkInterval * milliseconds.
If pads are deleted they are also deleted from the reverse proxy so it can be reassigned to another backend.

# License
Apache 2
