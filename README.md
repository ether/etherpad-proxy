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

## TODO
- [ ] Working Load Balancing
- [ ] Test in production
- [ ] API to create/destroy backends
- [ ] Consider if multiple backends should serve static files or fall back to 9001 (as per current)
- [ ] Don't die on failure to connect to backend
