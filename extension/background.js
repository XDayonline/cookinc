// Cookinc Bridge — Chrome Extension Service Worker
// chrome.cookies API handles all decryption (App-Bound Encryption included).
// Uses chrome.alarms to keep the service worker alive in MV3.

const SINK_URL = 'http://localhost:19999';
let allowlist = [];

function pushCookies() {
  fetch(SINK_URL + '/config')
    .then(r => r.json())
    .then(cfg => { allowlist = cfg.allowlist || []; })
    .catch(() => {});

  if (!allowlist.length) return;

  chrome.cookies.getAll({}, cookies => {
    const matched = cookies.filter(c =>
      allowlist.some(allowed =>
        c.domain === allowed || c.domain === '.' + allowed || c.domain.endsWith('.' + allowed)
      )
    );
    if (!matched.length) return;

    fetch(SINK_URL + '/cookies', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(matched),
    }).catch(() => {});
  });
}

// Alarm-based timer (keeps SW alive across cycles)
chrome.alarms.create('cookinc-sync', { periodInMinutes: 1 / 12 }); // every 5s
chrome.alarms.onAlarm.addListener(alarm => {
  if (alarm.name === 'cookinc-sync') pushCookies();
});

// Also push on install/startup
pushCookies();
