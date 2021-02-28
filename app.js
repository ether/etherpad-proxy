'use strict';

const ueberdb = require('ueberdb2');
const superagent = require('superagent');
let db;

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

const reqToBackend = (host, url, req) => {
  const searchParams = new URLSearchParams(req.url);
  let backend = backends[0];
  const padId = searchParams.get('/socket.io/?padId');
  if (!padId) return backend;

  db.get(`padId:${padId}`, (e, databaseBackend) => {
    if (databaseBackend) {
      // association exists already :)
      backend = databaseBackend.target;
    } else {
      console.log(`Associating ${padId} with ${mostFreeBackend.backend}`);
      // no association exists, we must make one
      db.set(`padId:${padId}`, {
        backend: mostFreeBackend.backend,
      });
    }
  });

  return backend;
};

// assign high priority
reqToBackend.priority = 100;

(async () => {
  db = new ueberdb.Database('dirty', {filename: './dirty.db'});
  await db.init();
})();

require('redbird')({
  port: 9000,
  resolvers: [
    reqToBackend,
    // uses the same priority as default resolver, so will be called after default resolver
    (host, url, req) => 'http://127.0.0.1:9001',
  ],
});


// TODO: I think some of this logic isn't quite right as the value never seems to increase
const checkAvailability = () => {
  setInterval(async () => {
    console.log(mostFreeBackend);
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
};

// every second check every backend to see which has the most availability.
checkAvailability();
