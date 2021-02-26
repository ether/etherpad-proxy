# Reverse Proxy for Etherpad

Runs A reverse proxy on port 9000 which will route pads based on padId to a pool of backends.

Currently tests against two backends on http://localhost:9001 and http://localhost:9002

!! WIP !!

## TODO
- [ ] Test performance using loadTest tool
- [ ] Use database for persistence
- [ ] Check which backends are free using /stats
- [ ] API to create/destroy backends
