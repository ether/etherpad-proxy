[![Proxy Test: 3 Pads <> 3 Unique Backends](https://github.com/ether/etherpad-proxy/actions/workflows/1maxPadPerInstance.yml/badge.svg)](https://github.com/ether/etherpad-proxy/actions/workflows/1maxPadPerInstance.yml) [![Proxy Test: 9 Pads <> 3 Backends](https://github.com/ether/etherpad-proxy/actions/workflows/3maxPadPerInstance.yml/badge.svg)](https://github.com/ether/etherpad-proxy/actions/workflows/3maxPadPerInstance.yml) [![Somewhat faster assignment Proxy Test: 3 Pads <> 3 Unique Backends](https://github.com/ether/etherpad-proxy/actions/workflows/rapidUniqueness.yml/badge.svg)](https://github.com/ether/etherpad-proxy/actions/workflows/rapidUniqueness.yml) [![Lint](https://github.com/ether/etherpad-proxy/actions/workflows/lint-package-lock.yml/badge.svg)](https://github.com/ether/etherpad-proxy/actions/workflows/lint-package-lock.yml)

# Sharding Reverse Proxy for Etherpad
This is a reverse proxy that which will shard pads based on padId(within query[currently in socket-namespace branch of Etherpad core]) to a pool of backends based on the availability of the backends which is based on the number of concurrent active Pads being edited.

It's likely that this project will only get to proof of concept stage(see V0) and then something that integrates with HAProxy/Varnish et al will replace it as NodeJS is probably not the right tool for the job but having the high level management system written in NodeJS makes sense to discover potential pitfalls and best practices.

## Getting started
Copy ``settings.json.template`` to ``settings.json`` and modify the values.

## Usage
```
node app.js
```

Visit http://localhost:9000

## Settings
Settings come from ``settings.json``, see ``settings.json.template`` for an example to modify for your environment.

``backends`` is your Backend Etherpad instances.

``maxPadsPerInstance`` is how many active pads you want to allow per instance.  This value should be between 1 and 20000 depending on the number of authors and words per minute that you limit or you wish to allow.  Once this limit is met then random instances will be used.

``checkInterval`` is how often to check every backend for availability.  You should set this to a low number if you have lower number of very active instances with short pad life.  You should set this to a high number if you have lost of instances with relatively long pad life expectancy.

For database settings/options please see UeberDB https://github.com/ether/ueberdb

## V0
- [ ] Don't try to route anything to any backends that aren't responding

## V1
- [ ] Test in production.
  - [ ] Export endpoints <-- pretty sure this is currently broken (as padid might not be right - I think it just needs a split)
  - [ ] Comments Plugin
- [ ] Remove backend if it's not available.
- [ ] Figure out why changing ``1000`` to ``200`` for ``checkInterval`` makes tests fail.
- [ ] Abstract http-proxy out / introduce support for other proxy software/services.
- [ ] API to create/destroy backends - REF: https://github.com/colyseus/proxy/blob/master/proxy.ts
- [ ] If no backends are available, send a message explaining "we're full up"
- [ ] Currently pads are stuck to backends permanently, this is bad if they are revisited,
 ergo pads should only have a certain staleness allowed at which point they should be nuked from the proxy database.

# License
Apache 2
