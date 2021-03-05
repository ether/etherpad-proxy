'use strict';

const httpProxy = require('http-proxy');
const http = require('http');
const ueberdb = require('ueberdb2');
const checkAvailability = require('./checkAvailability').checkAvailability;
// load the settings
const loadSettings = () => {
  const fs = require('fs');
  let settings;
  try {
    fs.readFileSync('settings.json', 'utf8');
    return JSON.parse(settings);
  } catch (e) {
    console.error('Please create your own settings.json file');
    const settings = fs.readFileSync('settings.json.template', 'utf8');
    return JSON.parse(settings);
  }
};

const settings = loadSettings();
console.log(settings);
if (settings.dbType === 'dirty') console.error('DirtyDB is not recommend for production');

const backendIds = Object.keys(settings.backends);

// An object of our proxy instances
const proxies = {};

// Making availableBackend globally available.
let availableBackend = backendIds[Math.floor(Math.random() * backendIds.length)];
setInterval(async () => {
  availableBackend = await checkAvailability(
      settings.backends,
      settings.checkInterval,
      settings.maxPadsPerInstance);
}, settings.checkInterval);

// Creating our database connection
const db = new ueberdb.Database(settings.dbType, settings.dbSettings);

// Initiate the proxy routes to the backends
const initiateRoute = (backend, req, res, socket, head) => {
  if (res) {
    console.log('backend: ', backend);
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

// Create dynamically assigned routes based on padIds and ensure that a route for
// unique padIds are re-used and stuck to a backend -- padId <> backend association.
const createRoute = (padId, req, res, socket, head) => {
  // If the route isn't for a specific padID IE it's for a static file
  // we can use any of the backends but now let's use the first :)
  if (!padId) {
    return initiateRoute(availableBackend, req, res, socket, head);
  }

  // pad specific backend required, do we have a backend already?
  db.get(`padId:${padId}`, (e, r) => {
    if (r && r.backend) {
      console.log(`database hit: ${padId} <> ${r.backend}`);
      initiateRoute(r.backend, req, res, socket, head);
    } else {
      if (!availableBackend) {
        availableBackend =
            backendIds[Math.floor(Math.random() * backendIds.length)];
      }
      // if no backend is stored for this pad, create a new connection
      db.set(`padId:${padId}`, {
        backend: availableBackend,
      });
      console.log(`database miss: ${padId} <> ${availableBackend}`);
      initiateRoute(availableBackend ||
        backendIds[Math.floor(Math.random() * backendIds)]
      , req, res, socket, head);
    }
  });
};

db.init(() => {
  // Create the backends.
  for (const backendId of backendIds) {
    /* eslint-disable-next-line new-cap */
    proxies[backendId] = new httpProxy.createProxyServer({
      target: {
        host: settings.backends[backendId].host,
        port: settings.backends[backendId].port,
      },
    });
  }
  // Create the routes for web traffic to those backends.
  const proxyServer = http.createServer((req, res) => {
    let padId;
    if (req.url.indexOf('/p/') !== -1) {
      padId = req.url.split('/p/')[1].split('?')[0];
      console.log(`initial request to /p/${padId}`);
    }
    const searchParams = new URLSearchParams(req.url);
    padId = searchParams.get('/socket.io/?padId');
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
