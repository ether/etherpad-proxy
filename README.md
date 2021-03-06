[![Proxy Test](https://github.com/ether/etherpad-proxy/actions/workflows/backend-tests.yml/badge.svg)](https://github.com/ether/etherpad-proxy/actions/workflows/backend-tests.yml)

# Experimental Reverse Proxy for Etherpad
This is a reverse proxy that runs on port 9000 which will route(shard) pads based on padId(within query[currently a branch of Etherpad core]) to a pool(currently hardcoded[in app.js]) of backends.

To add or remove backends, modify app.js - In the future this will be API driven.

It's likely that this project will only get to proof of concept stage and then something that integrates with HAProxy/Varnish et al will replace it as NodeJS is probably not the right tool for the job but having the high level management system written in NodeJS makes sense.

## Usage
```
node app.js
```

## settings
Settings come from settings.json, see settings.json.template for an example to modify for your environment.

``backends`` is your Backend Etherpad instances.

``maxPadsPerInstance`` is how many active pads you want to allow per instance.  This value should be between 1 and 20000 depending on the number of authors and words per minute that you limit or you wish to allow.  Once this limit is met then random instances will be used.

``checkInterval`` is how often to check every backend for availability.  You should set this to a low number if you have lower number of very active instances with short pad life.  You should set this to a high number if you have lost of instances with relatively long pad life expectancy.

For database settings/options please see UeberDB https://github.com/etherpad-lite

## V1
- [ ] Test in production.
- [ ] Remove backend if it's not available.
- [ ] Figure out why changing ``1000`` to ``200`` for ``checkInterval`` makes tests fail.
- [ ] Abstract http-proxy out / introduce support for other proxy software/services.
- [ ] API to create/destroy backends - REF: https://github.com/colyseus/proxy/blob/master/proxy.ts
- [ ] If no backends are available, send a message explaining "we're full up"
- [ ] Currently pads are stuck to backends permanently, this is bad if they are revisited,
 ergo pads should only have a certain staleness allowed at which point they should be nuked from the proxy database.

## V2
- [ ] Consider if multiple backends should serve static files or fall back to 9001 (as per current)
    - Advantage of single point = Only have to update plugin files there.
    - Disadvantage - single point of failure, plugins might be out of sync, plugin backend/frontend might bt out of sync.
    - Conclusion: I think a pads static content should come from where it gets it's content from, but then there are more complications especially surrounding export URIs..  
