'use strict';

const httpProxy = require('http-proxy');
const http = require('http');
const ueberdb = require('ueberdb2');
const backends = {
  backend1: {
    host: 'localhost',
    port: 9001,
  },
  backend2: {
    host: 'localhost',
    port: 9002,
  },
};

const proxies = {};
const db = new ueberdb.Database('dirty', {filename: './dirty.db'});
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
    db.get(`padId:${padId}`, (e, r) => {
      if (r) {
        proxies[r.backend].web(req, res, (e) => {
          console.error(e);
        });
      }
    });
  });
  proxyServer.on('error', (e) => {
    console.log('proxyserver error', e);
  });

  // Create a route for upgrade / websockets
  proxyServer.on('upgrade', (req, socket, head) => {
    const searchParams = new URLSearchParams(req.url);
    const padId = searchParams.get('/socket.io/?padId');
    db.get(`padId:${padId}`, (e, r) => {
      if (r) {
        proxies[r.backend].ws(req, socket, head, (e) => {
          console.error(e);
        });
      }
    });
  });

  // Finally listen on port 9000 :)
  proxyServer.listen(9000);
});
