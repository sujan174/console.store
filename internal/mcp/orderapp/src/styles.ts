// Inline design tokens for the order app, ported from
// internal/agents/bundles/console-order/references/surface-kit.md.
//
// The ext-apps host does not guarantee these variable names (it exposes its
// own `--color-*` tokens via applyHostStyleVariables) — these are OURS, kept
// in sync with the terminal Tokyo Night palette (internal/tui/theme/tokyonight.go)
// so the app matches the rest of consolestore. `app.ts` flips `data-theme` on
// <html> from the host's reported theme; `prefers-color-scheme` is the
// fallback before that fires.

const DARK = `
  --bg: #15161f;
  --surface-1: #10111a;
  --surface-2: #191a24;
  --border: #232539;
  --border-strong: #2c2e44;
  --text-primary: #a9b1d6;
  --text-secondary: #565f89;
  --text-muted: #3b3b5a;
  --text-accent: #7aa2f7;
  --bg-accent: rgba(122, 162, 247, 0.16);
  --border-accent: #7aa2f7;
  --text-success: #9ece6a;
  --bg-success: rgba(158, 206, 106, 0.16);
  --text-danger: #f7768e;
  --bg-danger: rgba(247, 118, 142, 0.16);
  --text-warning: #e0af68;
  --radius: 10px;
`;

const LIGHT = `
  --bg: #eef0f5;
  --surface-1: #e2e5ee;
  --surface-2: #ffffff;
  --border: #d7dae3;
  --border-strong: #c0c4d2;
  --text-primary: #343b58;
  --text-secondary: #5a5f78;
  --text-muted: #9195a8;
  --text-accent: #3760bf;
  --bg-accent: rgba(55, 96, 191, 0.1);
  --border-accent: #3760bf;
  --text-success: #2f7d4f;
  --bg-success: rgba(47, 125, 79, 0.1);
  --text-danger: #c4384d;
  --bg-danger: rgba(196, 56, 77, 0.1);
  --text-warning: #966300;
  --radius: 10px;
`;

export const STYLE_TEXT = `
:root { ${DARK} }
@media (prefers-color-scheme: light) {
  :root { ${LIGHT} }
}
:root[data-theme="dark"] { ${DARK} }
:root[data-theme="light"] { ${LIGHT} }

* { box-sizing: border-box; }
html, body { margin: 0; padding: 0; }
body {
  font-family: ui-monospace, "SF Mono", "Cascadia Code", "JetBrains Mono", Menlo, Consolas, monospace;
  font-size: 14px;
  line-height: 1.4;
  color: var(--text-primary);
  background: var(--bg);
}
#app { padding: 14px; max-width: 480px; margin: 0 auto; }
button {
  font-family: inherit;
  font-size: 13px;
  font-weight: 400;
  color: var(--text-primary);
  background: transparent;
  border: 0.5px solid var(--border-strong);
  border-radius: var(--radius);
  padding: 6px 12px;
  cursor: pointer;
}
button:hover { border-color: var(--border-accent); }
button:disabled { cursor: not-allowed; opacity: .5; }
button:focus-visible { outline: 2px solid var(--border-accent); outline-offset: 1px; }
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
