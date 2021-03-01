'use strict';

const ueberdb = require('ueberdb2');
const availability = require('./checkAvailability');
const httpProxy = require('http-proxy');
const http = require('http');
let db;

const proxyOne = new httpProxy.createProxyServer({
  target: {
    host: 'localhost',
    port: 9001,
  },
});

const proxyTwo = new httpProxy.createProxyServer({
  target: {
    host: 'localhost',
    port: 9002,
  },
});

const proxyServer = http.createServer((req, res) => {
  console.log(req);
  const backend = reqToBackend(req);
  if (backend === 'localhost:9001') {
    proxyOne.web(req, res);
  }
  if (backend === 'localhost:9002') {
    proxyTwo.web(req, res);
  }
});

//
// Listen to the `upgrade` event and proxy the
// WebSocket requests as well.
//
proxyServer.on('upgrade', (req, socket, head) => {
  const backend = reqToBackend(req);
  if (backend === 'localhost:9001') {
    proxyOne.ws(req, socket, head);
  }
  if (backend === 'localhost:9002') {
    proxyTwo.ws(req, socket, head);
  }
});

proxyServer.listen(9000);


// every second check every backend to see which has the most availability.
availability.checkAvailability();

// query database to see if we have a backend assigned for this padId
const reqToBackend = (req) => {
  let padId;
  let backend = availability.backends[0];
  // if it's a normal pad URL IE static file
  if (req.url.indexOf('/p/') !== -1) {
    padId = req.url.split('/p/')[1];
  }
  // if it's a websocket or specific connection
  if (!padId) {
    const searchParams = new URLSearchParams(req.url);
    padId = searchParams.get('/socket.io/?padId');
  }
  if (!padId) return backend;

  db.get(`padId:${padId}`, (e, result) => {
    if (result) {
      console.log('FOUND IN DB', result.backend);
      // association exists already :)
      backend = result.backend;
    } else {
      console.log(`Associating ${padId} with ${availability.mostFreeBackend.backend}`);
      // no association exists, we must make one
      db.set(`padId:${padId}`, {
        backend: availability.mostFreeBackend.backend,
      });
      backend = availability.mostFreeBackend.backend;
    }
  });
  return backend;
};

(async () => {
  db = new ueberdb.Database('dirty', {filename: './dirty.db'});
  await db.init();
})();
