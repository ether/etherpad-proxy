[![Proxy Test](https://github.com/ether/etherpad-proxy/actions/workflows/backend-tests.yml/badge.svg)](https://github.com/ether/etherpad-proxy/actions/workflows/backend-tests.yml)

# Experimental Reverse Proxy for Etherpad
Not currently functional.  Need to ref: https://github.com/colyseus/proxy/blob/master/proxy.ts

Runs A reverse proxy on port 9000 which will route pads based on padId(within query) to a pool(currently hardcoded) of backends.

Currently tests against two backends on http://localhost:9001 and http://localhost:9002 - these must be manually modified.

It's likely that this project will only get to proof of concept stage and then something that integrates with HAProxy/Varnish et al will replace it as NodeJS is probably not the right tool for the job but having the high level management system written in NodeJS makes sense.

## Usage
```
node app.js
```

## V0
- [ ] Working Load Balancing

## V1
- [ ] Abstract http-proxy out / introduce support for other proxy software/services.
- [ ] Test in production
- [ ] API to create/destroy backends - REF: https://github.com/colyseus/proxy/blob/master/proxy.ts

## V2
- [ ] Consider if multiple backends should serve static files or fall back to 9001 (as per current)
    - Advantage of single point = Only have to update plugin files there.
    - Disadvantage - single point of failure, plugins might be out of sync, plugin backend/frontend might bt out of sync.
    - Conclusion: I think a pads static content should come from where it gets it's content from, but then there are more complications especially surrounding export URIs..  
