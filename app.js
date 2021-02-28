'use strict';

const ueberdb = require('ueberdb2');
const availability = require('./checkAvailability');
let db;

// every second check every backend to see which has the most availability.
availability.checkAvailability();

// query database to see if we have a backend assigned for this padId
const reqToBackend = (host, url, req) => {
  const searchParams = new URLSearchParams(req.url);
  const backend = availability.backends[0];
  const padId = searchParams.get('/socket.io/?padId');
  if (!padId) return backend;

  db.get(`padId:${padId}`, (e, result) => {
    if (result) {
      console.log('FOUND IN DB', result.backend);
      // association exists already :)
      return result.backend;
    } else {
      console.log(`Associating ${padId} with ${availability.mostFreeBackend.backend}`);
      // no association exists, we must make one
      db.set(`padId:${padId}`, {
        backend: availability.mostFreeBackend.backend,
      });
      return availability.mostFreeBackend.backend;
    }
  });
  console.log('RETURNING', backend);
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
    (host, url, req) => 'http://127.0.0.1:9002',
  ],
});
