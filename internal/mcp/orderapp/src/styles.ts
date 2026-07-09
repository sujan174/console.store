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

/* boot-center fills the fixed-height #app so the boot loader sits vertically
   centered. Used ONLY for the pre-seed boot render — #app itself is a plain
   scrolling block (never a flex container), so real screens lay out top-down. */
.boot-center { min-height: 100%; display: flex; align-items: center; justify-content: center; }

/* --- console.store brand bar: the persistent wordmark + prompt cursor that
   marks every screen as ours (a terminal storefront inside the host). Mono,
   quiet, one orange accent shared with the action color. --- */
.cs-brandbar {
  display: flex; align-items: center; justify-content: space-between;
  gap: 10px; margin-bottom: 14px;
}
.cs-wordmark {
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 14px; letter-spacing: .01em; color: var(--text-primary);
  white-space: nowrap;
}
.cs-wordmark .p { color: var(--sw-orange); }
.cs-wordmark .d { color: var(--text-muted); }
.cs-cursor { color: var(--sw-orange); animation: cs-blink 1.1s steps(1) infinite; }
@keyframes cs-blink { 50% { opacity: 0; } }
/* Leading prompt glyph on the command-line search inputs. */
.cs-prompt { color: var(--sw-orange); font-family: var(--font-mono, ui-monospace, monospace); font-size: 14px; flex: none; }

/* --- scooter-road loader: shared centered loading block --- */
.scooter-loader {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  gap: 14px; padding: 48px 0; min-height: 180px;
  color: var(--text-secondary); font-size: 13px;
  animation: fadeIn .15s ease both;
}
.scooter-track { position: relative; width: 240px; max-width: 72%; height: 26px; overflow: hidden; }
.scooter-road {
  position: absolute; left: 0; right: 0; bottom: 3px; height: 0;
  border-bottom: 2px dotted var(--border-strong);
}
.scooter-rider {
  position: absolute; bottom: 0; font-size: 20px; line-height: 1;
  animation: scooter-drive 1.6s linear infinite;
}
/* Drive by left-offset (% of the track), not a fixed px translate, so the
   scooter traverses the full road at ANY track width — the track is
   min(240px,72%), so a fixed 240px translate would clip it on narrow hosts. */
@keyframes scooter-drive {
  from { left: -28px; }
  to   { left: 100%; }
}
.scooter-shimmer {
  width: min(240px, 72%); height: 2px; border-radius: 2px;
  background: linear-gradient(90deg, transparent, var(--sw-orange, #ff5200), transparent);
  background-size: 45% 100%; background-repeat: no-repeat;
  background-position: 50% 0;
  animation: scooter-shimmer 1.6s linear infinite;
}
@keyframes scooter-shimmer {
  from { background-position: -45% 0; }
  to   { background-position: 145% 0; }
}
.scooter-label { font-family: var(--font-mono, ui-monospace, monospace); letter-spacing: .01em; }
/* Terminal cursor after the loader label — ties every loading view to the
   console.store prompt motif. Static under reduced-motion (the * rule below
   kills the blink). */
.scooter-label::after {
  content: "\\2588"; color: var(--sw-orange); margin-left: 3px;
  animation: cs-blink 1.1s steps(1) infinite;
}
@media (prefers-reduced-motion: reduce) {
  /* No motion: park the scooter centered on the road and show the shimmer as
     a clean centered highlight (background-position 50% above) rather than a
     frozen off-screen frame. */
  .scooter-rider, .scooter-shimmer { animation: none; }
  .scooter-rider { left: 50%; transform: translateX(-50%); }
}

/* load-screen fills the fixed-height #app as a flex column so a loading view
   centers its scooter in the space BELOW the chrome (brand bar + back button),
   instead of top-aligning it right under the header (which read as "too high").
   .load-body takes the remaining height and centers the loader within it. */
.load-screen { min-height: 100%; display: flex; flex-direction: column; }
.load-body { flex: 1 1 auto; display: flex; align-items: center; justify-content: center; }

/* consolestore boot screen: an oversized wordmark that rises + breathes, the
   driving scooter beneath it, and one status line that cycles the current
   action. All CSS-only (the boot render is set once, never re-rendered). */
.boot-wrap { display: flex; flex-direction: column; align-items: center; gap: 20px; }

/* The wordmark — big, mono, one orange prompt. Rises in from a tighter tracked
   state, then breathes a soft orange glow so it feels alive during the wait. */
.boot-brand {
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 30px; font-weight: 600; letter-spacing: .01em;
  color: var(--text-primary); white-space: nowrap; line-height: 1;
  animation: boot-brand-in .6s cubic-bezier(.2,.8,.2,1) both,
             boot-brand-glow 3.4s ease-in-out .6s infinite;
}
.boot-brand .p { color: var(--sw-orange); }
@keyframes boot-brand-in {
  from { opacity: 0; transform: translateY(10px) scale(.94); letter-spacing: .18em; }
  to   { opacity: 1; transform: none; letter-spacing: .01em; }
}
@keyframes boot-brand-glow {
  0%, 100% { text-shadow: 0 0 0 rgba(255, 82, 0, 0); }
  50%      { text-shadow: 0 0 16px rgba(255, 82, 0, .38); }
}

/* Scooter block: reuse the shared scooter-track/road/rider/shimmer motif, just
   stacked tighter under the wordmark than the standalone loadingBlock. */
.boot-scoot { display: flex; flex-direction: column; align-items: center; gap: 9px; }

/* Status slot: a single fixed-height line. The three phases are stacked in the
   same spot and crossfade on one shared timeline so exactly one "current
   action" shows at a time — a looping proxy for live progress. */
.boot-status {
  position: relative; height: 20px; min-width: 210px;
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 13px; color: var(--text-secondary); text-align: center;
}
.boot-phase {
  position: absolute; left: 0; right: 0; top: 0; opacity: 0;
  animation: boot-phase 5.4s ease-in-out infinite;
  animation-delay: calc(var(--i) * 1.8s);
}
.boot-phase::before { content: "\\25B8 "; color: var(--sw-orange); }
.boot-phase::after {
  content: "\\2588"; color: var(--sw-orange); margin-left: 3px;
  animation: cs-blink 1.1s steps(1) infinite;
}
@keyframes boot-phase {
  0%   { opacity: 0; transform: translateY(3px); }
  6%   { opacity: 1; transform: none; }
  28%  { opacity: 1; transform: none; }
  34%  { opacity: 0; transform: translateY(-3px); }
  100% { opacity: 0; }
}

/* Loader escape hatch: a fixed corner button shown only while a loading view is
   up (boot / menu open / home search). Lives on <body>, outside the rewritten
   #app root, so it stays put across every screen. Hidden by default; the
   .is-visible class (toggled from render()) fades + slides it in. */
.loader-home-btn {
  position: fixed; top: 12px; right: 12px; z-index: 50;
  display: inline-flex; align-items: center; gap: 6px;
  padding: 6px 11px; border-radius: 999px;
  font-family: var(--font-mono, ui-monospace, monospace); font-size: 12px; line-height: 1;
  color: var(--text-secondary);
  background: var(--surface-2, rgba(20, 20, 28, .82));
  border: 1px solid var(--border, rgba(255, 255, 255, .12));
  box-shadow: var(--shadow, 0 2px 10px rgba(0, 0, 0, .3));
  cursor: pointer;
  opacity: 0; transform: translateY(-6px); pointer-events: none;
  transition: opacity .18s ease, transform .18s ease, color .15s ease, border-color .15s ease;
}
.loader-home-btn.is-visible { opacity: 1; transform: none; pointer-events: auto; }
.loader-home-btn:hover { color: var(--text-primary); border-color: var(--sw-orange); }
.loader-home-btn:active { transform: translateY(1px); }
.loader-home-btn svg { width: 13px; height: 13px; flex: none; }
@media (prefers-reduced-motion: reduce) {
  .loader-home-btn { transition: opacity .18s ease; transform: none; }
  .loader-home-btn.is-visible { transform: none; }
}

/* cs-line: the mono orange prompt line used on the recovery / status card. */
.cs-line { font-family: var(--font-mono, ui-monospace, monospace); font-size: 13px; color: var(--sw-orange); }

@media (prefers-reduced-motion: reduce) {
  .boot-brand { animation: none; }
  .boot-phase { animation: none; }
  .boot-phase:not(:first-child) { display: none; }
  .boot-phase:first-child { opacity: 1; }
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
  font-family: var(--font-mono, ui-monospace, monospace);
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
  font-family: var(--font-mono, ui-monospace, monospace);
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
  font-family: var(--font-mono, ui-monospace, monospace);
}
.bill-total {
  display: flex;
  justify-content: space-between;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  border-top: 1px solid var(--border);
  padding: 10px 0;
  font-family: var(--font-mono, ui-monospace, monospace);
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
