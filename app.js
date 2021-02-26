'use strict';

const http = require('http');
const httpProxy = require('http-proxy');
const url = require('url');
const proxy = httpProxy.createProxyServer();
const ueberdb = require('ueberdb2');
const db = new ueberdb.Database('dirty', {filename: 'var/dirty.db'});

proxy.on('error', function(e){
  console.error("Error", e);
})

// example associations, this will be replaced with ueber at some point
const assocs = {
  'test1': 'ws://localhost:9001',
  'test2': 'ws://localhost:9002',
};

const proxyServer = http.createServer(
  {
    ws: true,
  }, function (req, res) {
  const parsedURL = url.parse(req.url, true);
  let target = 'ws://localhost:9001';
  if (parsedURL.query && parsedURL.query.padId) {
    const padId = parsedURL.query.padId;
    if (assocs[padId]) {
      target = assocs[padId];
    }
  }
  proxy.web(req, res, {
    target,
  });
}).listen(9000);

//
// Listen to the `upgrade` event and proxy the
// WebSocket requests as well.
//
// proxyServer.on('upgrade', function (req, socket, head) {
//   proxy.ws(req, socket, head);
// });

proxyServer.on('error', function(e){
  console.error('proxy server error')
})

proxy.on('close', function (res, socket, head) {
  // view disconnected websocket connections
  console.log('Client disconnected');
});
