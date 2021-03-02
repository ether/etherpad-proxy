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
  if (padId === 'test1') {
    proxies[0].web(req, res, (e) => {
      console.error(e);
    });
  }
  if (padId === 'test2') {
    proxies[1].web(req, res, (e) => {
      console.error(e);
    });
  }
});
proxyServer.on('error', (e) => {
  console.log('proxyserver error', e);
});

// Create a route for upgrade / websockets
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

// Finally listen on port 9000 :)
proxyServer.listen(9000);
