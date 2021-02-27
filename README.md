# Reverse Proxy for Etherpad
Runs A reverse proxy on port 9000 which will route pads based on padId(within query) to a pool of backends.

Currently tests against two backends on http://localhost:9001 and http://localhost:9002 - these can be modified in app.js

## Usage
```
node app.js
```

## TODO
- [ ] Test Coverage
    - [ ] Design
    - [ ] Make
- [ ] API to create/destroy backends
- [ ] Test in production
- [x] Test performance using loadTest tool - confirmed positive impact
- [x] Use database for persistence
- [x] Check which backends are free using /stats

## Test Coverage Notes
