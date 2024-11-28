'use strict';

import httpProxy, {type ErrorCallback} from 'http-proxy';
import http from 'http';
import {Database} from 'ueberdb2';
import {checkAvailability} from './checkAvailability.ts'
import fs from 'fs'
// load the settings
const loadSettings = () => {
  let settings;
  try {
    settings = fs.readFileSync('settings.json', 'utf8');
    return JSON.parse(settings);
  } catch (e) {
    console.error('Please create your own settings.json file');
    settings = fs.readFileSync('settings.json.template', 'utf8');
    return JSON.parse(settings);
  }
};

const settings = loadSettings();
console.debug(settings);
if (settings.dbType === 'dirty') console.error('DirtyDB is not recommend for production');

const backendIds = Object.keys(settings.backends);

// An object of our proxy instances
const proxies: { [key: string]: httpProxy<http.IncomingMessage, http.ServerResponse<http.IncomingMessage>> } = {};

// Making availableBackend globally available.
let availableBackends: {
  up: Array<string>,
  available: Array<string>,
};
(async () => {
  checkAvailability(
      settings.backends,
      settings.checkInterval,
      settings.maxPadsPerInstance);
})();
// And now grab them every X duration
setInterval(async () => {
  availableBackends = await checkAvailability(
      settings.backends,
      settings.checkInterval,
      settings.maxPadsPerInstance);
}, settings.checkInterval);

// Creating our database connection
const db = new Database(settings.dbType, settings.dbSettings);

// Initiate the proxy routes to the backends
const initiateRoute = (backend: string, req:http.IncomingMessage, res: http.ServerResponse, socket:any, head:any) => {
  if (res) {
    // console.log('backend: ', backend);
    if (proxies[backend]) {
      proxies[backend].web(req, res,{}, (e) => {
        console.error(e);
      });
    }
  }
  if (socket && head) {
    if (proxies[backend]) {
      proxies[backend].ws(req, socket, head, {}, (e) => {
        console.error(e);
      });
    }
  }
};

// Create dynamically assigned routes based on padIds and ensure that a route for
// unique padIds are re-used and stuck to a backend -- padId <> backend association.
const createRoute = (padId: string | null, req, res, socket, head) => {
  // If the route isn't for a specific padID IE it's for a static file
  // we can use any of the backends but now let's use the first :)
  if (!padId) {
    const newBackend = availableBackends.available[
        Math.floor(Math.random() * availableBackends.available.length)];
    return initiateRoute(newBackend, req, res, socket, head);
  }

  // pad specific backend required, do we have a backend already?
  // @ts-ignore
  db.get(`padId:${padId}`, (e, r) => {
    if (r && r.backend) {
      // console.log(`database hit: ${padId} <> ${r.backend}`);
      if (!availableBackends) {
        return console.log('Request made during startup.');
      }
      if (availableBackends.up.indexOf(r.backend) !== -1) {
        initiateRoute(r.backend, req, res, socket, head);
      } else {
        // not available..  actually it might be up but not available..
        console.log(`hit backend not available: ${padId} <> ${r.backend}`);
        const newBackend = availableBackends.up[
            Math.floor(Math.random() * availableBackends.up.length)];
        // set and store a new backend
        db.set(`padId:${padId}`, {
          backend: newBackend,
        });
        console.log(`creating new association: ${padId} <> ${newBackend}`);
        initiateRoute(newBackend, req, res, socket, head);
      }
    } else {
      // if no backend is stored for this pad, create a new connection
      console.log(availableBackends);
      const newBackend = availableBackends.available[
          Math.floor(Math.random() * availableBackends.available.length)];
      db.set(`padId:${padId}`, {
        backend: newBackend,
      });
      if (!availableBackends.available) console.log('no available backends!');
      console.log(`database miss, initiating new association: ${padId} <> ${newBackend}`);
      initiateRoute(newBackend, req, res, socket, head);
    }
  });
};

db.init();

  // Create the backends.
  for (const backendId of backendIds) {
    /* eslint-disable-next-line new-cap */
    proxies[backendId] = httpProxy.createProxyServer({
      target: {
        host: settings.backends[backendId].host,
        port: settings.backends[backendId].port,
      },
    });
  }
  // Create the routes for web traffic to those backends.
  const proxyServer = http.createServer((req, res) => {
    let padId: string | null = null;
    if (req.url!.indexOf('/p/') !== -1) {
      padId = req.url!.split('/p/')[1].split('?')[0].split('/')[0];
      console.log(`initial request to /p/${padId}`);
    }
    if (padId === null) {
      const searchParams = new URLSearchParams(req.url);
      padId = searchParams.get('/socket.io/?padId');
    }
    createRoute(padId as string, req, res, null, null);
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

  proxyServer.listen(settings.port);
