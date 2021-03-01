'use strict';

const ueberdb = require('ueberdb2');
const availability = require('./checkAvailability');
const httpProxy = require('http-proxy');
const http = require('http');
let db;

/* eslint-disable-next-line new-cap */
const proxyOne = new httpProxy.createProxyServer({
  target: {
    host: 'localhost',
    port: 9001,
  },
});

/* eslint-disable-next-line new-cap */
const proxyTwo = new httpProxy.createProxyServer({
  target: {
    host: 'localhost',
    port: 9002,
  },
});

const proxyServer = http.createServer((req, res) => {
  const backend = reqToBackend(req);
  if (backend === 'localhost:9001') {
    proxyOne.web(req, res);
  }
  if (backend === 'localhost:9002') {
    proxyTwo.web(req, res);
  }
});

proxyServer.on('error', (e) => {
  console.log('proxyserver error', e);
});

proxyOne.on('error', (e, req, res) => {
  console.log('proxyOne error', e);
  res.writeHead(500, {
    'Content-Type': 'text/plain',
  });
  res.end('Something went wrong. And we are reporting a custom error message.');
});

proxyTwo.on('error', (e, req, res) => {
  console.log('proxyTwo error', e);
  res.writeHead(500, {
    'Content-Type': 'text/plain',
  });
  res.end('Something went wrong. And we are reporting a custom error message.');
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
  let backend = availability.mostFreeBackend.backend;
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
      console.log(`Found in Databas ${result.backend}`);
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
