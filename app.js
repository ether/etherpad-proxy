'use strict';

const http = require('http');
const httpProxy = require('http-proxy');
const proxy = httpProxy.createProxyServer();
const ueberdb = require('ueberdb2');
const superagent = require('superagent');

// Check instance availability once a second
const checkAvailabilityInterval = 1000;

// Hard coded backends for now.
const backends = [
  'localhost:9001',
  'localhost:9002',
];

const mostFreeBackend = {
  activePads: 0,
  backend: backends[0],
};

(async () => {
  setInterval(async () => {
    console.log(mostFreeBackend);
    // every second check every backend
    for (const backend of backends) {
      // query if it's free
      const stats = await superagent.get(`http://${backend}/stats`);
      const activePads = JSON.parse(stats.text).activePads;
      if (activePads === 0) {
        // console.log(`Free backend: ${backend} with ${activePads} active pads`);
        mostFreeBackend.activePads = activePads;
        mostFreeBackend.backend = backend;
      }
      if (activePads <= mostFreeBackend.activePads) {
        // console.log(`Free backend: ${backend} with ${activePads} active pads`);
        mostFreeBackend.activePads = activePads;
        mostFreeBackend.backend = backend;
        return;
      }
    }
  }, checkAvailabilityInterval);
})();

(async () => {
  const db = new ueberdb.Database('dirty', {filename: './dirty.db'});
  await db.init();

  proxy.on('error', (e) => {
    console.error('Error', e);
  });

  const proxyServer = http.createServer({
    ws: true,
  }, (req, res) => {
    const searchParams = new URLSearchParams(req.url);
    let target = `ws://${backends[0]}`;
    const padId = searchParams.get('/socket.io/?padId');
    if (padId) {
      db.get(`padId:${padId}`, (e, backend) => {
        if (backend) {
          // association exists already :)
          target = backend.target;
        } else {
          console.log(`Associating ${padId} with new backend`);
          // no association exists, we must make one
          db.set(`padId:${padId}`, {
            target: `ws://${mostFreeBackend.backend}`,
          });
        }
      });
    }
    proxy.web(req, res, {
      target,
    });
  }).listen(9000);

  proxyServer.on('error', (e) => {
    console.error('proxy server error');
  });

  proxy.on('close', (res, socket, head) => {
  // view disconnected websocket connections
    console.log('Client disconnected');
  });
})();
