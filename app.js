'use strict';

const http = require('http');
const httpProxy = require('http-proxy');
const url = require('url');
const proxy = httpProxy.createProxyServer();

proxy.on('error', function(e){
  console.error("Error", e)
})

// example associations, this will be replaced with ueber at some point
const assocs = {
  'test1': 'http://localhost:9001',
  'test2': 'http://localhost:9002',
};

const proxyServer = http.createServer(
  {
    ws: true,
  }, function (req, res) {
  const parsedURL = url.parse(req.url, true);
  let target = 'http://localhost:9001';
  if (parsedURL.query && parsedURL.query.padId) {
    const padId = parsedURL.query.padId;
    if (assocs[padId]) {
      target = assocs[padId];
    }
  }
  proxy.web(req, res, {
    target
  });
}).listen(9000);

//
// Listen to the `upgrade` event and proxy the
// WebSocket requests as well.
//
proxyServer.on('upgrade', function (req, socket, head) {
  proxy.ws(req, socket, head);
});
