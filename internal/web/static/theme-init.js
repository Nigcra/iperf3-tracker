'use strict';
// Setzt data-theme vor dem ersten Zeichnen (synchron im <head> geladen, da die
// CSP keine Inline-Skripte erlaubt). Gespeicherte Wahl in localStorage
// "wedigo-theme", sonst immer Hell (nicht die Systemeinstellung).
// Der frühere Schlüssel "iperf-theme" wird einmalig nach "wedigo-theme" übernommen.
(function () {
  var KEY = 'wedigo-theme';
  var saved = null;
  try {
    saved = localStorage.getItem(KEY);
    var old = localStorage.getItem('iperf-theme');
    if (old !== null) {
      if (saved === null && (old === 'light' || old === 'dark')) { saved = old; localStorage.setItem(KEY, old); }
      localStorage.removeItem('iperf-theme');
    }
  } catch (e) {}
  var root = document.documentElement;
  root.setAttribute('data-theme', saved === 'dark' ? 'dark' : 'light');
  window.wedigoTheme = {
    key: KEY,
    // saved meldet, ob der Benutzer selbst ein Theme gewählt hat.
    saved: function () { try { return localStorage.getItem(KEY); } catch (e) { return null; } },
  };
})();
