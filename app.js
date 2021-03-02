'use strict';

const httpProxy = require('http-proxy');
const http = require('http');
const backends = [{
  host: 'localhost',
  port: 9001,
}, {
  host: 'localhost',
  port: 9002,
}];

const proxies = {};

for (const backendId in backends) {
  /* eslint-disable-next-line new-cap */
  proxies[backendId] = new httpProxy.createProxyServer({
    target: {
      host: backends[backendId].host,
      port: backends[backendId].port,
    },
  });
}

const proxyServer = http.createServer((req, res) => {
  const searchParams = new URLSearchParams(req.url);
  const padId = searchParams.get('/socket.io/?padId');
  if (padId === 'test1') {
    proxies[0].web(req, res);
  }
  if (padId === 'test2') {
    proxies[1].web(req, res);
  }
});

proxyServer.on('error', (e) => {
  console.log('proxyserver error', e);
});
/*
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
*/

//
// Listen to the `upgrade` event and proxy the
// WebSocket requests as well.
//
proxyServer.on('upgrade', (req, socket, head) => {
  const searchParams = new URLSearchParams(req.url);
  const padId = searchParams.get('/socket.io/?padId');
  if (padId === 'test1') {
    proxies[0].ws(req, socket, head);
  }
  if (padId === 'test2') {
    proxies[1].ws(req, socket, head);
  }
});

proxyServer.listen(9000);
