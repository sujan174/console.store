// Design tokens for the order app, re-skinned to look native to Claude
// Desktop ("cloud theme") with a Swiggy accent — see the redesign spec at
// .superpowers/sdd/order-app-redesign-spec.md for the full rationale.
//
// OUR var names (--bg, --surface-1, --text-primary, …) are kept stable so
// screens.ts never has to change when the theme does; each one now resolves
// against the HOST's own `--color-*` tokens (applied by app.ts via
// applyHostStyleVariables) with a Swiggy/Claude-paper fallback for when the
// host doesn't supply one (e.g. during local dev / before connect fires).
// `prefers-color-scheme` covers the case before `data-theme` is set;
// `applyDocumentTheme` (app.ts) then flips `:root[data-theme]` from the
// host's reported theme.

const LIGHT = `
  --bg:             var(--color-background-primary,   #faf9f6);
  --surface-1:      var(--color-background-tertiary,  #f1efe8);
  --surface-2:      var(--color-background-secondary, #ffffff);
  --border:         var(--color-border-primary,       #e8e4d9);
  --border-strong:  var(--color-border-secondary,     #d9d4c6);
  --text-primary:   var(--color-text-primary,         #262521);
  --text-secondary: var(--color-text-secondary,       #6b6658);
  --text-muted:     var(--color-text-tertiary,        #a8a294);
  --text-success:   var(--color-text-success,         #1a7f4b);
  --bg-success:     var(--color-background-success,   rgba(26, 127, 75, .10));
  --text-danger:    var(--color-text-danger,          #c0392b);
  --bg-danger:      var(--color-background-danger,    rgba(192, 57, 43, .10));
  --text-warning:   var(--color-text-warning,         #b7791f);
  --ring:           var(--color-ring-primary,         var(--sw-orange));
  --shadow: 0 1px 2px rgba(30, 25, 15, .04), 0 4px 16px rgba(30, 25, 15, .06);
`;

const DARK = `
  --bg:             var(--color-background-primary,   #1c1b18);
  --surface-1:      var(--color-background-tertiary,  #232019);
  --surface-2:      var(--color-background-secondary, #26241f);
  --border:         var(--color-border-primary,       #33302a);
  --border-strong:  var(--color-border-secondary,     #423e36);
  --text-primary:   var(--color-text-primary,         #ece7db);
  --text-secondary: var(--color-text-secondary,       #a6a08f);
  --text-muted:     var(--color-text-tertiary,        #6f6a5c);
  --text-success:   var(--color-text-success,         #6fce9a);
  --bg-success:     var(--color-background-success,   rgba(111, 206, 154, .14));
  --text-danger:    var(--color-text-danger,          #f2857a);
  --bg-danger:      var(--color-background-danger,    rgba(242, 133, 122, .14));
  --text-warning:   var(--color-text-warning,         #e0af68);
  --ring:           var(--color-ring-primary,         var(--sw-orange));
  --shadow: 0 1px 2px rgba(0, 0, 0, .20), 0 6px 20px rgba(0, 0, 0, .28);
`;

// Brand + geometry are theme-independent — one place, never re-derived per
// mode. The Swiggy orange is a FIXED brand accent (not themed by the host).
const BRAND = `
  --sw-orange: #fc8019;
  --sw-orange-press: #e06d0a;
  --sw-on-orange: #fff;
  --veg: #3aab5a;
  --nonveg: #b02b2b;
  --radius: 14px;
  --radius-sm: 10px;
  --pill: 999px;

  /* Kept for any leftover reference — now Swiggy orange, not the old blue. */
  --text-accent: var(--sw-orange);
  --border-accent: var(--sw-orange);
  --bg-accent: rgba(252, 128, 25, .12);
`;

export const STYLE_TEXT = `
:root { ${BRAND} ${DARK} }
@media (prefers-color-scheme: light) {
  :root { ${LIGHT} }
}
@media (prefers-color-scheme: dark) {
  :root { ${DARK} }
}
:root[data-theme="light"] { ${LIGHT} }
:root[data-theme="dark"] { ${DARK} }

* { box-sizing: border-box; }
html, body { margin: 0; padding: 0; height: 100%; }
body {
  font-family: var(--font-sans, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif);
  font-size: 14px;
  line-height: 1.4;
  color: var(--text-primary);
  background: var(--bg);
  /* The FRAME never scrolls — #app is the single scroll surface (below).
     This is what keeps the widget one fixed size: the host measures the
     document, not each screen's content, so swapping screens (menu ->
     conflict -> cart) can't grow or shrink the window. */
  overflow: hidden;
}
/* #app is the fixed-size, internally-scrolling shop frame. It fills the
   host viewport (fullscreen or inline) at a constant height; content that
   overflows scrolls INSIDE it instead of resizing the iframe. Generous
   symmetric padding gives the shop presence and breathing room on all
   sides; the extra bottom pad clears the sticky cart bar. */
#app {
  /* Inline embeds size the iframe to our content, so a CONCRETE height is
     what makes the frame both usable-tall AND fixed (100dvh would collapse to
     the host's tiny inline slot — the bug that made this a sliver). Content
     that overflows scrolls inside this fixed box. Fullscreen overrides to
     100dvh below. */
  height: 620px;
  max-width: 520px;
  margin: 0 auto;
  padding: 24px 26px 34px;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-width: thin;
}
/* Fullscreen has the whole host viewport — fill it, more width for the
   home's sidebar layout. */
:root[data-display="fullscreen"] #app { height: 100dvh; max-width: 760px; padding: 28px 32px 40px; }

/* --- modal overlay (conflict keep/clear pops OVER the menu, so the frame
   height never changes) --- */
.overlay {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: color-mix(in srgb, var(--bg) 60%, transparent);
  backdrop-filter: blur(3px);
  animation: fadeIn .15s ease both;
}
.overlay > .card {
  width: 100%;
  max-width: 380px;
  animation: riseIn .2s ease both;
}

/* --- loading spinner + centered loading block --- */
.ring {
  width: 24px;
  height: 24px;
  border: 2.5px solid var(--border-strong);
  border-top-color: var(--sw-orange);
  border-radius: 50%;
  animation: spin .7s linear infinite;
  flex: none;
}
.loading-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 48px 0;
  color: var(--text-secondary);
  font-size: 13px;
  animation: fadeIn .15s ease both;
}

.num {
  font-family: var(--font-mono, ui-monospace, "SF Mono", Menlo, Consolas, monospace);
  font-variant-numeric: tabular-nums;
}

input {
  font-family: inherit;
  color: var(--text-primary);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}

/* --- buttons --- */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-family: inherit;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  background: transparent;
  border: 1px solid var(--border-strong);
  border-radius: var(--radius-sm);
  padding: 6px 12px;
  cursor: pointer;
  transition: transform .06s ease, background .15s ease, border-color .15s ease;
}
.btn:hover { border-color: var(--sw-orange); }
.btn:active { transform: scale(.97); }
.btn:disabled { cursor: not-allowed; opacity: .5; }
.btn:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }

.btn-primary {
  background: var(--sw-orange);
  border-color: var(--sw-orange);
  color: var(--sw-on-orange);
  font-weight: 600;
  box-shadow: 0 1px 2px rgba(224, 109, 10, .25);
}
.btn-primary:hover { background: var(--sw-orange-press); border-color: var(--sw-orange-press); }

.btn-ghost { /* default .btn look; hover picks up the orange ring */ }

.btn-block { width: 100%; height: 44px; font-size: 15px; justify-content: center; }

/* --- category / segmented tabs --- */
.tabrow {
  display: flex;
  gap: 7px;
  overflow-x: auto;
  padding-bottom: 8px;
  scrollbar-width: thin;
}
.tab {
  cursor: pointer;
  font-family: inherit;
  font-size: 13px;
  padding: 6px 12px;
  border-radius: var(--pill);
  border: 1px solid var(--border-strong);
  background: transparent;
  color: inherit;
  white-space: nowrap;
  transition: background .15s ease, border-color .15s ease, color .15s ease;
}
.tab.on {
  color: var(--sw-orange);
  border-color: var(--sw-orange);
  background: rgba(252, 128, 25, .10);
  font-weight: 600;
}

/* --- store home layout (Task 7 scaffold; Tasks 8–10 fill the slots) --- */
.store-layout { display: flex; gap: 14px; }
.sidebar { width: 150px; flex: none; display: flex; flex-direction: column; gap: 4px; }
.content { flex: 1; min-width: 0; }
/* Narrow / inline-embed fallback — stack instead of side-by-side. */
:root[data-display="inline"] .store-layout { flex-direction: column; }
:root[data-display="inline"] .sidebar { width: auto; flex-direction: row; overflow-x: auto; padding-bottom: 4px; }

/* --- store home: category sidebar (Task 9) --- */
.side-item {
  display: block;
  width: 100%;
  text-align: left;
  cursor: pointer;
  font-family: inherit;
  font-size: 13px;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-secondary);
  transition: background .15s ease, border-color .15s ease, color .15s ease;
}
.side-item:hover { background: var(--surface-1); color: var(--text-primary); }
.side-item:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }
.side-item.on {
  color: var(--sw-orange);
  border-color: var(--sw-orange);
  background: rgba(252, 128, 25, .10);
  font-weight: 600;
}
:root[data-display="inline"] .side-item { width: auto; flex: none; white-space: nowrap; }

/* --- store home: restaurant list (Task 9) --- */
.rest-card { margin-bottom: 10px; }
.rest-card--closed { opacity: .55; }

/* --- store home: recent orders (Task 10) --- */
.recent-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}
.recent-row:last-child { margin-bottom: 0; }

/* --- menu rows --- */
.tile {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 12px 0;
  border-top: 1px solid var(--border);
  transition: background .12s ease;
}
.tile:hover { background: color-mix(in srgb, var(--sw-orange) 4%, transparent); }

/* --- cards / surfaces --- */
.card {
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 16px 18px;
  box-shadow: var(--shadow);
}

.cartbar {
  position: sticky;
  bottom: 0;
  background: var(--surface-2);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 11px 14px;
  margin-top: 10px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: var(--shadow);
}

/* --- stepper (qty +/-) --- */
.stepper {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  border: 1px solid var(--sw-orange);
  border-radius: var(--pill);
  overflow: hidden;
  flex: none;
}
.stepper button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  color: var(--sw-orange);
  background: rgba(252, 128, 25, .08);
  padding: 4px 11px;
  cursor: pointer;
  font-family: inherit;
}
.stepper button:focus-visible { outline: 2px solid var(--ring); outline-offset: -2px; }
.stepper .num { min-width: 14px; text-align: center; padding: 0 2px; }

/* --- segmented control / choice chips --- */
.seg, .chip {
  cursor: pointer;
  font-family: inherit;
  border: 1px solid var(--border-strong);
  background: transparent;
  color: inherit;
  transition: background .15s ease, border-color .15s ease, color .15s ease;
}
.seg {
  font-size: 13px;
  padding: 7px 12px;
  border-radius: var(--radius-sm);
  text-align: center;
}
.chip {
  font-size: 12px;
  padding: 5px 10px;
  border-radius: var(--pill);
}
.seg.on, .chip.on {
  color: var(--sw-orange);
  border-color: var(--sw-orange);
  background: rgba(252, 128, 25, .10);
  font-weight: 600;
}
.seg:focus-visible, .chip:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }

/* --- veg / non-veg mark --- */
.veg {
  width: 15px;
  height: 15px;
  border-radius: 3px;
  border: 1.5px solid var(--veg);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: none;
}
.veg span { width: 5px; height: 5px; border-radius: 50%; background: var(--veg); }
.veg--nonveg { border-color: var(--nonveg); }
.veg--nonveg span { background: var(--nonveg); }

.badge-soldout {
  font-size: 12px;
  background: var(--bg-danger);
  color: var(--text-danger);
  padding: 2px 8px;
  border-radius: var(--pill);
  flex: none;
}

/* --- bill rows --- */
.bill-row {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
  font-size: 14px;
  color: var(--text-secondary);
}
.bill-total {
  display: flex;
  justify-content: space-between;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  border-top: 1px solid var(--border);
  padding: 10px 0;
}

/* --- motion (CSS-only, reduced-motion guarded) --- */
@keyframes riseIn { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: none; } }
@keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
@keyframes spin { to { transform: rotate(360deg); } }
.cartbar { animation: riseIn .18s ease both; }
.stagger > * { animation: riseIn .22s ease both; animation-delay: calc(var(--i, 0) * 22ms); }
.card { animation: fadeIn .18s ease both; }
.spin { animation: spin 1s linear infinite; display: inline-block; }
@media (prefers-reduced-motion: reduce) {
  * { animation: none !important; }
}
`;

const STYLE_TAG_ID = "consolestore-order-app-styles";

// injectStyles appends the <style> tag once; safe to call more than once.
export function injectStyles(): void {
  if (document.getElementById(STYLE_TAG_ID)) return;
  const tag = document.createElement("style");
  tag.id = STYLE_TAG_ID;
  tag.textContent = STYLE_TEXT;
  document.head.appendChild(tag);
}
