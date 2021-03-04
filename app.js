'use strict';

const httpProxy = require('http-proxy');
const http = require('http');
const ueberdb = require('ueberdb2');
const checkAvailability = require('./checkAvailability').checkAvailability;

const checkInterval = 1000;
const maxPadsPerInstance = 1;
const backends = {
  backend1: {
    host: 'localhost',
    port: 9001,
  },
  backend2: {
    host: 'localhost',
    port: 9002,
  },
  backend3: {
    host: 'localhost',
    port: 9003,
  },
};

const proxies = {};
let availableBackend = null;
setInterval(async () => {
  availableBackend = await checkAvailability(backends, checkInterval, maxPadsPerInstance);
}, checkInterval);

const db = new ueberdb.Database('dirty', {filename: './dirty.db'});

const initiateRoute = (backend, req, res, socket, head) => {
  if (res) {
    proxies[backend].web(req, res, (e) => {
      console.error(e);
    });
  }
  if (socket && head) {
    proxies[backend].ws(req, socket, head, (e) => {
      console.error(e);
    });
  }
};

const createRoute = (padId, req, res, socket, head) => {
  db.get(`padId:${padId}`, (e, r) => {
    if (r && r.backend) {
      initiateRoute(r.backend, req, res, socket, head);
    } else {
      // if no backend is stored for this pad, create a new connection
      console.log('availableBackend', availableBackend);
      db.set(`padId:${padId}`, {
        backend: availableBackend,
      });
      initiateRoute(availableBackend, req, res, socket, head);
    }
  });
};

db.init(() => {
  // Create the backends.
  for (const backendId of Object.keys(backends)) {
    /* eslint-disable-next-line new-cap */
    proxies[backendId] = new httpProxy.createProxyServer({
      target: {
        host: backends[backendId].host,
        port: backends[backendId].port,
      },
    });
  }
  // Create the routes for web traffic to those backends.
  const proxyServer = http.createServer((req, res) => {
    const searchParams = new URLSearchParams(req.url);
    const padId = searchParams.get('/socket.io/?padId');
    createRoute(padId, req, res, null, null);
  });

  proxyServer.on('error', (e) => {
    console.log('proxyserver error', e);
  });

  // Create a route for upgrade / websockets
  proxyServer.on('upgrade', (req, socket, head) => {
    const searchParams = new URLSearchParams(req.url);
    const padId = searchParams.get('/socket.io/?padId');
    createRoute(padId, req, null, socket, head);
  });

  // Finally listen on port 9000 :)
  proxyServer.listen(9000);
});
