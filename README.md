[![Proxy Test](https://github.com/ether/etherpad-proxy/actions/workflows/backend-tests.yml/badge.svg)](https://github.com/ether/etherpad-proxy/actions/workflows/backend-tests.yml)

# Reverse Proxy for Etherpad
Not currently functional.  Need to ref: https://github.com/colyseus/proxy/blob/master/proxy.ts

Runs A reverse proxy on port 9000 which will route pads based on padId(within query) to a pool(currently hardcoded) of backends.

Currently tests against two backends on http://localhost:9001 and http://localhost:9002 - these must be manually modified.

## Usage
```
node app.js
```

## TODO
- [ ] Working Load Balancing
- [ ] Test in production
- [ ] API to create/destroy backends

## Test Coverage Notes
