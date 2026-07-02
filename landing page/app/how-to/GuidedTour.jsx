"use client";

// GuidedTour — a spotlight walkthrough for the how-to page.
//
// Activated ONLY by the secret link `/how-to?guide` (no on-page button). It reads
// the query param on the client, then dims the page and moves a spotlight from
// section to section, auto-scrolling each into view. A fixed control card at the
// bottom narrates each step with Back / Next / Done and a progress counter.
//
// The page itself stays a server component; this is a self-contained client island
// rendered once at the end of page.jsx. It only touches `window`/`document` inside
// effects, so static export + hydration stay clean (nothing renders until the
// effect flips `active` on).

import { useCallback, useEffect, useRef, useState } from "react";

// Each step targets a section id on the page (null = a full-page dim, no spotlight,
// used for the intro/outro). Keep the narration accurate to the real commands/keys.
const STEPS = [
  {
    id: null,
    accent: "#93a8ff",
    title: "welcome to the walkthrough",
    body:
      "I'll guide you through consolestore end to end — installing it, ordering in the terminal, saving reorders, the full keymap, the shell commands, and handing it to your AI agent. Use Next and Back, or press Esc to leave anytime."
  },
  {
    id: "install",
    accent: "#93a8ff",
    title: "1 · install",
    body:
      "One line drops a signed binary that self-updates on every launch. The first run opens your browser to authorize once; your token then lives in your OS keyring — no server, no database."
  },
  {
    id: "app",
    accent: "#93a8ff",
    title: "2 · the app",
    body:
      "Run console with no arguments to open the full-screen terminal app. Browse real Swiggy restaurants, step into one, add dishes, and check out — all from the keyboard, no mouse."
  },
  {
    id: "controls",
    accent: "#93a8ff",
    title: "3 · every control",
    body:
      "The full keymap. Arrows or h j k l move; ↵ selects, confirms and adds; / searches; c opens the cart; a changes the address; esc steps back (esc esc jumps home). The cart, checkout and tracking keys live here too."
  },
  {
    id: "alias",
    accent: "#b08cf5",
    title: "4 · presets",
    body:
      "Build a cart once, press : and run alias set <name> to snapshot it. Then reorder it forever from your shell with console order <name>. Presets remember their restaurant and delivery address."
  },
  {
    id: "cli",
    accent: "#93a8ff",
    title: "5 · the command line",
    body:
      "Every headless command: console status for a live order and ETA, console order to reorder a preset, console alias to manage them, plus whoami, update and agents. No app required."
  },
  {
    id: "agent",
    accent: "#b08cf5",
    title: "6 · your agent",
    body:
      'One command — console agents install — wires consolestore into Claude, Cursor, Codex, Zed and more as an MCP server. Then just tell your agent "order my usual dinner" and approve the bill.'
  },
  {
    id: null,
    accent: "#eab560",
    title: "that's the whole tour",
    body:
      "Everything here runs from your terminal. Install it, order dinner, and never leave the shell. Press Done to explore on your own."
  }
];

const PAD = 12; // spotlight padding around the target rect

export default function GuidedTour() {
  const [active, setActive] = useState(false);
  const [i, setI] = useState(0);
  const [rect, setRect] = useState(null); // spotlight rect in viewport coords, or null
  const iRef = useRef(0); // live step index, read by measure() to avoid stale closures
  const cardRef = useRef(null); // the narration card, for focus management / trap
  const [reduced, setReduced] = useState(false); // prefers-reduced-motion

  const step = STEPS[i];

  // Keep the ref in sync so measure() always sees the current step.
  useEffect(() => {
    iRef.current = i;
  }, [i]);

  // Honor prefers-reduced-motion: skip smooth scroll + spotlight travel animation.
  useEffect(() => {
    try {
      const mq = window.matchMedia("(prefers-reduced-motion: reduce)");
      setReduced(mq.matches);
      const onChange = (e) => setReduced(e.matches);
      mq.addEventListener("change", onChange);
      return () => mq.removeEventListener("change", onChange);
    } catch {
      /* no-op */
    }
  }, []);

  // Move focus into the dialog when the tour opens (aria-modal needs it).
  useEffect(() => {
    if (active) cardRef.current?.focus();
  }, [active]);

  // Activate only when the URL carries `?guide` (any value, or bare flag).
  useEffect(() => {
    try {
      const params = new URLSearchParams(window.location.search);
      if (params.has("guide")) {
        setActive(true);
        setI(0);
      }
    } catch {
      /* no-op: SSR / no window */
    }
  }, []);

  const close = useCallback(() => {
    setActive(false);
    setRect(null);
    // Strip ?guide so a refresh won't relaunch the tour.
    try {
      const url = new URL(window.location.href);
      url.searchParams.delete("guide");
      // Keep any OTHER query params (url.search is "" or "?rest" after the delete).
      window.history.replaceState({}, "", url.pathname + url.search + url.hash);
    } catch {
      /* no-op */
    }
  }, []);

  const next = useCallback(() => {
    setI((n) => (n < STEPS.length - 1 ? n + 1 : n));
  }, []);
  const prev = useCallback(() => {
    setI((n) => (n > 0 ? n - 1 : n));
  }, []);

  // Measure the CURRENT step's target (read fresh via iRef, so there are no stale
  // per-step closures to race) and position the spotlight over it. Event-driven —
  // no requestAnimationFrame, which some environments throttle or pause.
  const measure = useCallback(() => {
    const s = STEPS[iRef.current];
    const el = s.id ? document.getElementById(s.id) : null;
    if (!el) {
      setRect(null);
      return;
    }
    const r = el.getBoundingClientRect();
    setRect({
      top: r.top - PAD,
      left: r.left - PAD,
      width: r.width + PAD * 2,
      height: r.height + PAD * 2
    });
  }, []);

  // On step change: scroll the target into view, then re-measure a few times to
  // catch the smooth-scroll settling (setTimeout keeps working even if rAF is idle).
  useEffect(() => {
    if (!active) return;
    const s = STEPS[i];
    const el = s.id ? document.getElementById(s.id) : null;
    if (el) el.scrollIntoView({ behavior: reduced ? "auto" : "smooth", block: "center" });
    const timers = [0, 120, 260, 420, 620, 850].map((d) => setTimeout(measure, d));
    return () => timers.forEach(clearTimeout);
  }, [active, i, measure, reduced]);

  // Keep the spotlight glued during manual scroll / resize. Listeners are added
  // once per active session (not per step) so nothing stale lingers. `true`
  // captures scrolls on the body/inner scroller too.
  useEffect(() => {
    if (!active) return;
    window.addEventListener("scroll", measure, true);
    window.addEventListener("resize", measure);
    return () => {
      window.removeEventListener("scroll", measure, true);
      window.removeEventListener("resize", measure);
    };
  }, [active, measure]);

  // Keyboard: Esc closes, →/Enter next, ← back, and Tab is trapped inside the
  // dialog (keeps focus off the dimmed page behind it — aria-modal contract).
  useEffect(() => {
    if (!active) return;
    const onKey = (e) => {
      if (e.key === "Escape") {
        e.preventDefault();
        close();
      } else if (e.key === "ArrowRight" || e.key === "Enter") {
        e.preventDefault();
        i < STEPS.length - 1 ? next() : close();
      } else if (e.key === "ArrowLeft") {
        e.preventDefault();
        prev();
      } else if (e.key === "Tab") {
        const card = cardRef.current;
        if (!card) return;
        const items = card.querySelectorAll(
          'button:not([disabled]), [href], [tabindex]:not([tabindex="-1"])'
        );
        if (!items.length) return;
        const first = items[0];
        const last = items[items.length - 1];
        const activeEl = document.activeElement;
        // Wrap focus, and pull it back in if it has escaped to the page.
        if (e.shiftKey && (activeEl === first || !card.contains(activeEl))) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && (activeEl === last || !card.contains(activeEl))) {
          e.preventDefault();
          first.focus();
        }
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [active, i, next, prev, close]);

  if (!active) return null;

  const isLast = i === STEPS.length - 1;
  const accent = step.accent;

  return (
    <div style={{ position: "fixed", inset: 0, zIndex: 9999 }} aria-live="polite">
      {/* Dim layer. With a rect, a transparent "hole" casts the dim via a huge
          box-shadow — the spotlight. Without a rect, a plain full dim. */}
      {rect ? (
        <div
          style={{
            position: "fixed",
            top: rect.top,
            left: rect.left,
            width: rect.width,
            height: rect.height,
            borderRadius: "14px",
            boxShadow: `0 0 0 9999px rgba(3,3,7,.86), 0 0 0 1px ${accent}, 0 0 34px ${accent}66`,
            pointerEvents: "none",
            transition: reduced
              ? "none"
              : "top .18s ease, left .18s ease, width .18s ease, height .18s ease"
          }}
        />
      ) : (
        <div
          style={{
            position: "fixed",
            inset: 0,
            background: "rgba(3,3,7,.86)",
            pointerEvents: "none"
          }}
        />
      )}

      {/* Click-catcher: clicking the dimmed area advances (feels natural). Sits
          under the control card, above the page. */}
      <button
        onClick={isLast ? close : next}
        aria-hidden="true"
        tabIndex={-1}
        style={{
          position: "fixed",
          inset: 0,
          background: "transparent",
          border: "none",
          cursor: "pointer",
          padding: 0,
          margin: 0
        }}
      />

      {/* Control / narration card — fixed bottom-center. */}
      <div
        ref={cardRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="howto-tour-title"
        aria-describedby="howto-tour-desc"
        tabIndex={-1}
        style={{
          position: "fixed",
          bottom: "28px",
          left: "50%",
          transform: "translateX(-50%)",
          width: "calc(100% - 40px)",
          maxWidth: "540px",
          background: "#0a0a12",
          border: `1px solid ${accent}55`,
          borderRadius: "14px",
          boxShadow: `0 18px 60px rgba(0,0,0,.55), 0 0 0 1px rgba(255,255,255,.02)`,
          padding: "20px 22px 18px",
          fontFamily: '"JetBrains Mono", ui-monospace, monospace',
          animation: "howtoTourIn .32s cubic-bezier(.22,1,.36,1) both"
        }}
      >
        <style>{`
          @keyframes howtoTourIn { from { opacity:0; transform:translate(-50%,12px);} to {opacity:1; transform:translate(-50%,0);} }
          @media (prefers-reduced-motion: reduce){
            [role="dialog"]{animation:none !important}
          }
        `}</style>

        {/* top row: progress + close */}
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            marginBottom: "10px"
          }}
        >
          <span
            style={{
              fontSize: "11px",
              letterSpacing: "1.5px",
              textTransform: "uppercase",
              color: accent
            }}
          >
            guided tour · {i + 1} / {STEPS.length}
          </span>
          <button
            onClick={close}
            aria-label="close tour"
            style={{
              background: "transparent",
              border: "none",
              color: "#565b80",
              cursor: "pointer",
              fontSize: "15px",
              lineHeight: 1,
              padding: "2px 4px"
            }}
          >
            esc ✕
          </button>
        </div>

        {/* progress bar */}
        <div
          style={{
            height: "3px",
            borderRadius: "99px",
            background: "rgba(255,255,255,.06)",
            marginBottom: "14px",
            overflow: "hidden"
          }}
        >
          <div
            style={{
              height: "100%",
              width: `${((i + 1) / STEPS.length) * 100}%`,
              background: accent,
              borderRadius: "99px",
              transition: "width .28s ease"
            }}
          />
        </div>

        <h3
          id="howto-tour-title"
          style={{
            margin: "0 0 8px",
            fontSize: "17px",
            fontWeight: 800,
            color: "#e9ebf7",
            letterSpacing: "-.01em"
          }}
        >
          {step.title}
        </h3>
        <p
          id="howto-tour-desc"
          style={{
            margin: "0 0 16px",
            fontSize: "14px",
            lineHeight: 1.7,
            color: "#a2a7c8"
          }}
        >
          {step.body}
        </p>

        {/* controls */}
        <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
          <button
            onClick={prev}
            disabled={i === 0}
            style={{
              fontFamily: "inherit",
              fontSize: "13px",
              padding: "9px 16px",
              borderRadius: "8px",
              border: "1px solid rgba(255,255,255,.10)",
              background: "transparent",
              color: i === 0 ? "#3a3d5c" : "#8a8fb4",
              cursor: i === 0 ? "default" : "pointer"
            }}
          >
            ← back
          </button>
          <div style={{ flex: 1 }} />
          <button
            onClick={isLast ? close : next}
            style={{
              fontFamily: "inherit",
              fontSize: "13px",
              fontWeight: 700,
              padding: "9px 20px",
              borderRadius: "8px",
              border: `1px solid ${accent}`,
              background: `${accent}1f`,
              color: "#e9ebf7",
              cursor: "pointer"
            }}
          >
            {isLast ? "done ✓" : "next →"}
          </button>
        </div>
      </div>
    </div>
  );
}
