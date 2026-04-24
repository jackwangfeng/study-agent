// Minimal service worker — only here so iPhone Safari treats the site as a
// proper PWA (which requires a registered SW). We deliberately do NOT cache
// API responses or app shell yet: that adds a bunch of stale-data footguns
// and isn't needed until the kid reports "couldn't open the app on a flaky
// network". When that day comes, swap to a Workbox precache + runtime cache.

self.addEventListener("install", (event) => {
  // Activate immediately on first install so the page doesn't have to
  // reload to start using the SW.
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil(self.clients.claim());
});

// Pass-through fetch — present so the SW counts as "controlling fetches",
// which is required for installability on some browsers.
self.addEventListener("fetch", (event) => {
  // Default browser behavior. No interception, no caching.
});
