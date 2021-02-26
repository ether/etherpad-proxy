'use strict';

const http = require('http');
const httpProxy = require('http-proxy');
const proxy = httpProxy.createProxyServer();
const ueberdb = require('ueberdb2');

(async () => {
  const db = new ueberdb.Database('dirty', {filename: './dirty.db'});
  await db.init();

  proxy.on('error', (e) => {
    console.error('Error', e);
  });

  const proxyServer = http.createServer({
    ws: true,
  }, (req, res) => {
  // NOTE TO SELF: putting async here is probably a terrible idea!
    const searchParams = new URLSearchParams(req.url);
    let target = 'ws://localhost:9001';
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
            target: 'ws://localhost:9002',
          });
        }
        console.log('!backend after!', backend);
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
