import GuidedTour from "./GuidedTour";

export const metadata = {
  title: "how to use consolestore — install, order, reorder",
  description:
    "A step-by-step guide to consolestore: install in one line, browse and order in the terminal app, save presets and reorder from your shell, and wire it into your AI agent.",
  alternates: { canonical: "/how-to" }
};

// ─── data ────────────────────────────────────────────────────────────────────

const KEYMAP_GROUPS = [
  {
    title: "move & select",
    rows: [
      { keys: ["↑", "↓", "k", "j"], desc: "move the cursor / list" },
      { keys: ["←", "→", "h", "l"], desc: "switch column · category · quantity" },
      { keys: ["↵"], desc: "select · confirm · add" },
      { keys: ["esc"], desc: "back a step  ·  esc esc jumps home" },
      { keys: ["tab"], desc: "switch Restaurants ⟷ Instamart" },
      { keys: ["ctrl-c"], desc: "quit" }
    ]
  },
  {
    title: "browse restaurants",
    rows: [
      { keys: ["/"], desc: "search restaurants" },
      { keys: ["i"], desc: "restaurant info" },
      { keys: ["c"], desc: "open cart" },
      { keys: ["a"], desc: "change delivery address" }
    ]
  },
  {
    title: "inside a restaurant",
    rows: [
      { keys: ["↵", "+"], desc: "add the dish" },
      { keys: ["−"], desc: "remove one" },
      { keys: ["←", "→"], desc: "change category" },
      { keys: ["/"], desc: "search dishes" },
      { keys: ["v"], desc: "veg only" },
      { keys: ["i"], desc: "dish info" },
      { keys: ["c"], desc: "open cart" },
      { keys: ["esc"], desc: "back" }
    ]
  },
  {
    title: "cart & checkout",
    rows: [
      { keys: ["↑", "↓"], desc: "pick a line" },
      { keys: ["←", "→", "+", "−"], desc: "change quantity" },
      { keys: ["⌫"], desc: "remove the line" },
      { keys: ["↵"], desc: "place the order (cash on delivery)" }
    ]
  },
  {
    title: "tracking",
    rows: [
      { keys: ["d"], desc: "dismiss a delivered order" },
      { keys: ["esc"], desc: "back" }
    ]
  }
];

const CLI_COMMANDS = [
  { cmd: "console status", desc: "show your live order status (or none)" },
  { cmd: "console order <name>", desc: "order a saved preset (lists them if several share the name)" },
  { cmd: "console order <name> <n>", desc: "order the nth same-named preset directly" },
  { cmd: "console alias list", desc: "list your saved presets" },
  { cmd: "console alias rm <name> [n]", desc: "remove preset <name> (the nth, if several share it)" },
  { cmd: "console whoami", desc: "show connection + saved addresses" },
  { cmd: "console logout", desc: "disconnect your Swiggy account" },
  { cmd: "console version", desc: "print version + channel" },
  {
    cmd: "console update [--channel stable|beta|alpha [--code X]]",
    desc: "switch channel, or check for updates now"
  },
  { cmd: "console agents [install|list|remove]", desc: "wire console into your AI agents (MCP + skills)" },
  { cmd: "console help", desc: "show this help" }
];

// ─── shared SVG mark (from features/page.jsx, verbatim) ──────────────────────

function SvgMark() {
  return (
    <svg
      width="24"
      height="24"
      viewBox="0 0 64 64"
      fill="none"
      shapeRendering="crispEdges"
      style={{ display: "block", flex: "none", filter: "drop-shadow(0 0 5px rgba(147,168,255,.35))" }}
    >
      <rect x="20" y="18" width="6" height="6" fill="#93a8ff" />
      <rect x="26" y="18" width="6" height="6" fill="#93a8ff" />
      <rect x="26" y="24" width="6" height="6" fill="#9c9af4" />
      <rect x="32" y="24" width="6" height="6" fill="#9c9af4" />
      <rect x="32" y="30" width="6" height="6" fill="#b08cf5" />
      <rect x="38" y="30" width="6" height="6" fill="#b08cf5" />
      <rect x="26" y="36" width="6" height="6" fill="#b08cf5" />
      <rect x="32" y="36" width="6" height="6" fill="#b08cf5" />
      <rect x="20" y="42" width="6" height="6" fill="#b08cf5" />
      <rect x="26" y="42" width="6" height="6" fill="#b08cf5" />
      <rect x="30" y="48" width="18" height="5" fill="#eab560" />
    </svg>
  );
}

// ─── reusable: fake-terminal mockup ───────────────────────────────────────────

function TermMock({ title, lines, accent = "#93a8ff", width }) {
  return (
    <div
      className="howto-term"
      style={{
        background: "#0a0a12",
        border: `1px solid ${hexA(accent, 0.16)}`,
        borderRadius: "12px",
        boxShadow: "0 14px 40px rgba(0,0,0,.35)",
        overflow: "hidden",
        width: width || "100%",
        maxWidth: "100%"
      }}
    >
      {/* title bar */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "10px",
          padding: "11px 14px",
          borderBottom: `1px solid ${hexA(accent, 0.1)}`,
          background: "rgba(255,255,255,.015)"
        }}
      >
        <span style={{ display: "inline-flex", gap: "6px" }}>
          <i style={dotStyle("#ff5f56")} />
          <i style={dotStyle("#ffbd2e")} />
          <i style={dotStyle("#27c93f")} />
        </span>
        <span
          style={{
            fontSize: "12px",
            color: "#565b80",
            letterSpacing: ".2px",
            marginLeft: "4px"
          }}
        >
          {title}
        </span>
      </div>
      {/* body */}
      <div
        style={{
          padding: "16px 18px",
          fontFamily:
            "ui-monospace,SFMono-Regular,'SF Mono',Menlo,Consolas,monospace",
          fontSize: "14px",
          lineHeight: 1.85,
          whiteSpace: "pre-wrap",
          wordBreak: "break-word"
        }}
      >
        {lines.map((segs, i) => (
          <div key={i}>
            {segs.length === 0 ? " " : segs.map((seg, j) => <Seg key={j} seg={seg} accent={accent} />)}
          </div>
        ))}
      </div>
    </div>
  );
}

function dotStyle(color) {
  return {
    width: "10px",
    height: "10px",
    borderRadius: "50%",
    background: color,
    display: "inline-block"
  };
}

// a line "segment": { t: "dim" | "text" | "accent" | "plain", v: string }
function Seg({ seg, accent }) {
  const color =
    seg.t === "dim" ? "#565b80" : seg.t === "text" ? "#8a8fb4" : seg.t === "accent" ? accent : "#e9ebf7";
  const weight = seg.t === "accent" ? 700 : 400;
  return (
    <span style={{ color, fontWeight: weight }}>
      {seg.v}
    </span>
  );
}

function hexA(hex, a) {
  const h = hex.replace("#", "");
  const r = parseInt(h.slice(0, 2), 16);
  const g = parseInt(h.slice(2, 4), 16);
  const b = parseInt(h.slice(4, 6), 16);
  return `rgba(${r},${g},${b},${a})`;
}

// ─── reusable: keymap row ──────────────────────────────────────────────────

function KeyRow({ keys, desc, accent }) {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        flexWrap: "wrap",
        gap: "10px",
        padding: "10px 0",
        borderBottom: "1px solid rgba(255,255,255,.04)"
      }}
    >
      <div style={{ display: "flex", gap: "6px", flexWrap: "wrap", flex: "none", minWidth: "112px" }}>
        {keys.map((k, i) => (
          <kbd
            key={i}
            style={{
              display: "inline-block",
              fontFamily:
                "ui-monospace,SFMono-Regular,'SF Mono',Menlo,Consolas,monospace",
              fontSize: "12.5px",
              color: accent,
              background: "#0a0a12",
              border: `1px solid ${hexA(accent, 0.3)}`,
              borderRadius: "5px",
              padding: "3px 7px",
              lineHeight: 1.4
            }}
          >
            {k}
          </kbd>
        ))}
      </div>
      <span style={{ fontSize: "14px", color: "#8a8fb4", lineHeight: 1.6 }}>{desc}</span>
    </div>
  );
}

// ─── reusable: section header ─────────────────────────────────────────────

function SectionHeader({ kicker, glyph, title, sub, accent }) {
  return (
    <div className="howto-section-hdr" style={{ marginBottom: "34px" }}>
      <div
        style={{
          fontSize: "12px",
          letterSpacing: "2px",
          color: accent,
          marginBottom: "10px"
        }}
      >
        {kicker}
      </div>
      <div
        style={{
          display: "flex",
          alignItems: "baseline",
          gap: "12px",
          marginBottom: "10px"
        }}
      >
        <span
          style={{
            color: accent,
            fontSize: "18px",
            fontWeight: 700,
            lineHeight: 1
          }}
        >
          {glyph}
        </span>
        <h2
          style={{
            margin: 0,
            fontWeight: 800,
            fontSize: "clamp(22px,2.8vw,34px)",
            letterSpacing: "-.02em",
            color: "#e9ebf7"
          }}
        >
          {title}
        </h2>
      </div>
      {sub && (
        <p
          style={{
            maxWidth: "56ch",
            margin: 0,
            fontSize: "15px",
            color: "#8a8fb4",
            lineHeight: 1.7
          }}
        >
          {sub}
        </p>
      )}
    </div>
  );
}

// ─── reusable: numbered step ───────────────────────────────────────────────

function Step({ n, children, accent }) {
  return (
    <div style={{ display: "flex", gap: "14px", alignItems: "flex-start" }}>
      <span
        style={{
          flex: "none",
          width: "26px",
          height: "26px",
          borderRadius: "7px",
          border: `1px solid ${hexA(accent, 0.3)}`,
          background: "rgba(0,0,0,.25)",
          color: accent,
          fontSize: "12px",
          fontWeight: 700,
          display: "flex",
          alignItems: "center",
          justifyContent: "center"
        }}
      >
        {n}
      </span>
      <p style={{ margin: 0, fontSize: "15px", color: "#8a8fb4", lineHeight: 1.75, paddingTop: "3px" }}>
        {children}
      </p>
    </div>
  );
}

// ─── page ─────────────────────────────────────────────────────────────────────

export default function HowToPage() {
  return (
    <>
      <style>{`
        html { scroll-behavior: smooth; }
        #install, #app, #controls, #alias, #cli, #agent {
          scroll-margin-top: 84px;
        }

        @keyframes howtoFadeUp {
          from { opacity: 0; transform: translateY(28px); }
          to   { opacity: 1; transform: none; }
        }
        @keyframes headerFade {
          from { opacity: 0; transform: translateY(18px); }
          to   { opacity: 1; transform: none; }
        }
        @keyframes blink {
          0%, 49% { opacity: 1; }
          50%, 100% { opacity: 0; }
        }
        @keyframes pulseDot {
          0%, 100% { opacity: 1; }
          50% { opacity: .35; }
        }

        .howto-section-hdr {
          animation: headerFade 0.7s cubic-bezier(.22,1,.36,1) both;
        }
        .howto-term, .howto-card {
          opacity: 1;
          transition: transform 0.22s cubic-bezier(.22,1,.36,1),
                      border-color 0.22s,
                      box-shadow 0.22s;
        }
        @supports (animation-timeline: view()) {
          .howto-term, .howto-card {
            animation: howtoFadeUp 0.55s cubic-bezier(.22,1,.36,1) both;
            animation-timeline: view();
            animation-range: entry 0% entry 60%;
          }
        }
        @supports not (animation-timeline: view()) {
          .howto-term, .howto-card {
            opacity: 0;
            animation: howtoFadeUp 0.55s cubic-bezier(.22,1,.36,1) both;
          }
        }

        .howto-chip {
          transition: transform 0.18s cubic-bezier(.22,1,.36,1), border-color 0.18s, color 0.18s;
        }
        .howto-chip:hover {
          transform: translateY(-2px);
          border-color: rgba(147,168,255,.42);
          color: #e9ebf7 !important;
        }

        @media (max-width: 820px) {
          .howto-nav { display: none !important; }
          .howto-header-title {
            font-size: clamp(28px, 8vw, 44px) !important;
          }
          .howto-grid-2 {
            grid-template-columns: 1fr !important;
          }
          .howto-chip-row {
            gap: 8px !important;
          }
        }
        @media (prefers-reduced-motion: reduce) {
          .howto-term, .howto-card, .howto-section-hdr, .howto-chip {
            animation: none !important;
            opacity: 1 !important;
            transform: none !important;
          }
          /* quiet the footer's decorative blink/pulseDot too */
          * { animation-duration: 0.001ms !important; animation-iteration-count: 1 !important; }
          html { scroll-behavior: auto; }
        }
      `}</style>

      {/* ── ambient background (same radials as features page) ── */}
      <div
        id="top"
        style={{
          position: "relative",
          minHeight: "100vh",
          background:
            "radial-gradient(1100px 620px at 72% -6%,rgba(147,168,255,.09),transparent 58%),radial-gradient(800px 500px at 12% 18%,rgba(176,140,245,.07),transparent 58%),#030307"
        }}
      >
        {/* orb A */}
        <div
          style={{
            position: "fixed",
            left: "-8vw",
            top: "-6vh",
            width: "42vw",
            height: "42vw",
            borderRadius: "50%",
            pointerEvents: "none",
            zIndex: 0,
            background: "radial-gradient(circle,rgba(147,168,255,.09),transparent 60%)",
            filter: "blur(42px)"
          }}
        />
        {/* orb B */}
        <div
          style={{
            position: "fixed",
            right: "-8vw",
            top: "22vh",
            width: "36vw",
            height: "36vw",
            borderRadius: "50%",
            pointerEvents: "none",
            zIndex: 0,
            background: "radial-gradient(circle,rgba(176,140,245,.08),transparent 60%)",
            filter: "blur(46px)"
          }}
        />
        {/* scanlines */}
        <div
          style={{
            position: "fixed",
            inset: 0,
            pointerEvents: "none",
            zIndex: 1,
            background:
              "repeating-linear-gradient(0deg,rgba(0,0,0,0) 0px,rgba(0,0,0,0) 2px,rgba(0,0,0,.16) 3px,rgba(0,0,0,0) 4px)",
            opacity: 0.36
          }}
        />
        {/* vignette */}
        <div
          style={{
            position: "fixed",
            inset: 0,
            pointerEvents: "none",
            zIndex: 1,
            background:
              "radial-gradient(120% 120% at 50% 42%,transparent 58%,rgba(0,0,0,.5) 100%)"
          }}
        />

        {/* ── NAV ── */}
        <nav className="howto-nav site-nav">
          <div className="site-nav-inner">
            {/* wordmark → home (the "start" for a sub-page) */}
            <a
              href="/"
              title="back to home"
              style={{ display: "inline-flex", alignItems: "center", gap: "10px" }}
            >
              <SvgMark />
              <span style={{ fontWeight: 800, fontSize: "14px", letterSpacing: "-.02em" }}>
                <span
                  style={{
                    background: "linear-gradient(170deg,#a6b8ff 0%,#ad8cf2 100%)",
                    WebkitBackgroundClip: "text",
                    WebkitTextFillColor: "transparent",
                    backgroundClip: "text"
                  }}
                >
                  console
                </span>
                <span style={{ color: "#eab560" }}>store</span>
              </span>
            </a>

            {/* links: in-page section anchors, then a divider, then the page tabs */}
            <div className="nav-links">
              <a href="/#run" className="lnk">run</a>
              <a href="/#keys" className="lnk">terminal &amp; agent</a>
              <a href="/#faq" className="lnk">faq</a>
              <span className="nav-divider" aria-hidden="true" />
              <a href="/features" className="lnk nav-page">features</a>
              {/* active: how-to */}
              <a href="/how-to" className="lnk nav-page" aria-current="page">
                how-to
              </a>
            </div>
          </div>
        </nav>

        {/* ── PAGE HEADER ── */}
        <header
          style={{
            position: "relative",
            zIndex: 2,
            maxWidth: "1100px",
            margin: "0 auto",
            padding: "56px clamp(24px,6vw,56px) 0",
            textAlign: "center"
          }}
        >
          <div
            className="howto-section-hdr"
            style={{
              fontSize: "12px",
              letterSpacing: "2px",
              color: "#93a8ff",
              textTransform: "lowercase",
              marginBottom: "18px"
            }}
          >
            // how to actually use it
          </div>
          <h1
            className="howto-header-title"
            style={{
              margin: "0 0 20px",
              fontWeight: 800,
              fontSize: "clamp(30px,5vw,58px)",
              letterSpacing: "-.02em",
              lineHeight: 1.1,
              background:
                "linear-gradient(168deg,#aebcff 0%,#9c9af4 52%,#b08cf5 100%)",
              WebkitBackgroundClip: "text",
              WebkitTextFillColor: "transparent",
              backgroundClip: "text"
            }}
          >
            from install to dinner, in a few keys
          </h1>
          <p
            style={{
              maxWidth: "58ch",
              margin: "0 auto",
              fontSize: "16px",
              color: "#8a8fb4",
              lineHeight: 1.75
            }}
          >
            a practical walkthrough — install, browse and order in the terminal
            app, save a preset and reorder from your shell, and hand it to your
            AI agent.
          </p>
        </header>

        {/* ── SELECTOR CHIP ROW ── */}
        <div
          style={{
            position: "relative",
            zIndex: 2,
            maxWidth: "1100px",
            margin: "0 auto",
            padding: "34px clamp(24px,6vw,56px) 0"
          }}
        >
          <div
            className="howto-chip-row"
            style={{
              display: "flex",
              flexWrap: "wrap",
              justifyContent: "center",
              gap: "12px"
            }}
          >
            {[
              { href: "#install", label: "install" },
              { href: "#app", label: "the app" },
              { href: "#cli", label: "the command line" },
              { href: "#agent", label: "your agent" }
            ].map((c) => (
              <a
                key={c.href}
                href={c.href}
                className="howto-chip"
                style={{
                  display: "inline-block",
                  fontSize: "13px",
                  color: "#8a8fb4",
                  border: "1px solid rgba(147,168,255,.16)",
                  borderRadius: "99px",
                  padding: "9px 18px",
                  background: "#0a0a12"
                }}
              >
                {c.label}
              </a>
            ))}
          </div>
        </div>

        {/* ── § INSTALL ── */}
        <section
          id="install"
          style={{
            position: "relative",
            zIndex: 2,
            maxWidth: "1100px",
            margin: "0 auto",
            padding: "72px clamp(24px,6vw,56px) 48px"
          }}
        >
          <SectionHeader
            kicker="// one command"
            glyph="◆"
            title="install"
            sub="One line gets you a signed binary that self-updates on every launch."
            accent="#93a8ff"
          />

          <div className="howto-grid-2" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "20px", marginBottom: "28px" }}>
            <TermMock
              title="stable"
              accent="#93a8ff"
              lines={[
                [{ t: "dim", v: "$ " }, { t: "plain", v: "curl -fsSL consolestore.in/install | sh" }]
              ]}
            />
            <TermMock
              title={"beta"}
              accent="#93a8ff"
              lines={[
                [
                  { t: "dim", v: "$ " },
                  { t: "plain", v: "curl -fsSL consolestore.in/install | sh -s -- " },
                  { t: "accent", v: "--beta" }
                ]
              ]}
            />
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: "16px", maxWidth: "70ch" }}>
            <Step n="1" accent="#93a8ff">
              First run opens your browser to authorize once against Swiggy.
            </Step>
            <Step n="2" accent="#93a8ff">
              Your token is stored in your OS keyring &mdash; no server, no
              database, ever.
            </Step>
            <Step n="3" accent="#93a8ff">
              That&apos;s it. Run <code style={{ color: "#e9ebf7" }}>console</code>{" "}
              any time after &mdash; it&apos;s a one-time step.
            </Step>
          </div>
        </section>

        {/* divider */}
        <Divider color="rgba(147,168,255,.18)" />

        {/* ── § THE APP ── */}
        <section
          id="app"
          style={{
            position: "relative",
            zIndex: 2,
            maxWidth: "1100px",
            margin: "0 auto",
            padding: "72px clamp(24px,6vw,56px) 48px"
          }}
        >
          <SectionHeader
            kicker="// the full-screen app"
            glyph="◆"
            title="the app"
            sub="Run console with no arguments and you get a full terminal app for real Swiggy restaurants."
            accent="#93a8ff"
          />

          <div style={{ display: "flex", flexDirection: "column", gap: "16px", maxWidth: "70ch", marginBottom: "36px" }}>
            <Step n="1" accent="#93a8ff">
              Open it: run <code style={{ color: "#e9ebf7" }}>console</code> with no
              arguments.
            </Step>
            <Step n="2" accent="#93a8ff">
              Browse real Swiggy restaurants near your saved address &mdash;
              names, ratings, delivery time, all live.
            </Step>
            <Step n="3" accent="#93a8ff">
              Enter a restaurant and add dishes to the cart.
            </Step>
            <Step n="4" accent="#93a8ff">
              Cart and checkout are one page &mdash; see the real Swiggy bill,
              place it with a single <code style={{ color: "#e9ebf7" }}>↵</code>.
            </Step>
            <Step n="5" accent="#93a8ff">
              Track the live order &mdash; status and ETA &mdash; right in the
              terminal.
            </Step>
          </div>

          <div className="howto-grid-2" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "20px", marginBottom: "48px" }}>
            <TermMock
              title="restaurants — Koramangala"
              accent="#93a8ff"
              lines={[
                [{ t: "dim", v: "/ " }, { t: "text", v: "search restaurants" }],
                [],
                [{ t: "accent", v: "❯ " }, { t: "plain", v: "Meghana Foods" }, { t: "dim", v: "        4.3 ★  32 min" }],
                [{ t: "plain", v: "  Truffles" }, { t: "dim", v: "               4.5 ★  28 min" }],
                [{ t: "plain", v: "  Empire Restaurant" }, { t: "dim", v: "       4.1 ★  35 min" }],
                [{ t: "plain", v: "  Third Wave Coffee" }, { t: "dim", v: "      4.4 ★  20 min" }],
                [],
                [{ t: "dim", v: "↵ open   c cart   a address   tab instamart" }]
              ]}
            />
            <TermMock
              title="cart — Meghana Foods"
              accent="#93a8ff"
              lines={[
                [{ t: "plain", v: "2×  Boneless Biryani" }, { t: "dim", v: "     ₹" }, { t: "text", v: "498" }],
                [{ t: "plain", v: "1×  Chicken 65" }, { t: "dim", v: "            ₹" }, { t: "text", v: "249" }],
                [{ t: "plain", v: "1×  Butter Naan" }, { t: "dim", v: "            ₹" }, { t: "text", v: "60" }],
                [],
                [{ t: "dim", v: "item total" }, { t: "text", v: "              ₹807" }],
                [{ t: "dim", v: "delivery fee" }, { t: "text", v: "             ₹35" }],
                [{ t: "dim", v: "taxes" }, { t: "text", v: "                   ₹41" }],
                [{ t: "accent", v: "total" }, { t: "accent", v: "                   ₹883" }],
                [],
                [{ t: "dim", v: "place order  " }, { t: "accent", v: "↵" }]
              ]}
            />
          </div>

          {/* keymap */}
          <div style={{ marginBottom: "10px", fontSize: "12px", letterSpacing: "1.5px", color: "#565b80", textTransform: "uppercase" }}>
            keymap
          </div>
          <div
            id="controls"
            className="howto-grid-2"
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              gap: "28px 40px"
            }}
          >
            {KEYMAP_GROUPS.map((group) => (
              <div key={group.title}>
                <div
                  style={{
                    fontSize: "14px",
                    fontWeight: 700,
                    color: "#e9ebf7",
                    marginBottom: "4px"
                  }}
                >
                  {group.title}
                </div>
                <div>
                  {group.rows.map((row, i) => (
                    <KeyRow key={i} keys={row.keys} desc={row.desc} accent="#93a8ff" />
                  ))}
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* divider */}
        <Divider color="rgba(176,140,245,.18)" />

        {/* ── § PRESETS / ALIAS ── */}
        <section
          id="alias"
          style={{
            position: "relative",
            zIndex: 2,
            maxWidth: "1100px",
            margin: "0 auto",
            padding: "72px clamp(24px,6vw,56px) 48px",
            background:
              "radial-gradient(700px 360px at 15% 0%,rgba(176,140,245,.05),transparent 65%)"
          }}
        >
          <SectionHeader
            kicker="// save a cart once"
            glyph="✳"
            title="presets & the alias"
            sub="Save a cart once, reorder forever — from the app or straight from your shell."
            accent="#b08cf5"
          />

          <div style={{ display: "flex", flexDirection: "column", gap: "16px", maxWidth: "70ch", marginBottom: "28px" }}>
            <Step n="1" accent="#b08cf5">
              Build a cart in the app &mdash; add whatever you&apos;d order again.
            </Step>
            <Step n="2" accent="#b08cf5">
              Press <code style={{ color: "#e9ebf7" }}>:</code> to open the command
              palette.
            </Step>
            <Step n="3" accent="#b08cf5">
              Run <code style={{ color: "#e9ebf7" }}>alias set &lt;name&gt;</code>{" "}
              &mdash; that cart is now a named preset.
            </Step>
          </div>

          <div style={{ marginBottom: "28px", maxWidth: "70ch" }}>
            <TermMock
              title="command palette"
              accent="#b08cf5"
              lines={[
                [{ t: "dim", v: ": " }, { t: "accent", v: "alias set" }, { t: "plain", v: " usual-dinner" }],
                [{ t: "dim", v: "  saved · Meghana Foods · HSR Layout, BLR" }]
              ]}
            />
          </div>

          <p style={{ maxWidth: "62ch", fontSize: "15px", color: "#8a8fb4", lineHeight: 1.75, marginBottom: "28px" }}>
            A preset is bound to the restaurant and delivery address it was saved
            with, so <code style={{ color: "#e9ebf7" }}>console order usual-dinner</code>{" "}
            always reorders exactly what you snapshotted.
          </p>

          <div style={{ maxWidth: "70ch" }}>
            <TermMock
              title="shell"
              accent="#b08cf5"
              lines={[
                [{ t: "dim", v: "$ " }, { t: "accent", v: "console order" }, { t: "plain", v: " usual-dinner" }],
                [{ t: "dim", v: "$ " }, { t: "accent", v: "console alias list" }],
                [{ t: "dim", v: "$ " }, { t: "accent", v: "console alias rm" }, { t: "plain", v: " usual-dinner" }]
              ]}
            />
          </div>
        </section>

        {/* divider */}
        <Divider color="rgba(147,168,255,.18)" />

        {/* ── § THE COMMAND LINE ── */}
        <section
          id="cli"
          style={{
            position: "relative",
            zIndex: 2,
            maxWidth: "1100px",
            margin: "0 auto",
            padding: "72px clamp(24px,6vw,56px) 48px"
          }}
        >
          <SectionHeader
            kicker="// headless, no TUI"
            glyph="◆"
            title="the command line"
            sub="Every subcommand runs headless — plain text, no full-screen app."
            accent="#93a8ff"
          />

          <div
            style={{
              border: "1px solid rgba(147,168,255,.10)",
              borderRadius: "12px",
              background: "#0a0a12",
              overflow: "hidden",
              marginBottom: "36px"
            }}
          >
            {CLI_COMMANDS.map((row, i) => (
              <div
                key={row.cmd}
                style={{
                  display: "flex",
                  flexWrap: "wrap",
                  gap: "6px 18px",
                  padding: "13px 18px",
                  borderBottom:
                    i === CLI_COMMANDS.length - 1 ? "none" : "1px solid rgba(255,255,255,.04)"
                }}
              >
                <code
                  style={{
                    flex: "0 0 300px",
                    maxWidth: "100%",
                    fontFamily:
                      "ui-monospace,SFMono-Regular,'SF Mono',Menlo,Consolas,monospace",
                    fontSize: "14px",
                    color: "#93a8ff"
                  }}
                >
                  {row.cmd}
                </code>
                <span style={{ fontSize: "15px", color: "#8a8fb4", lineHeight: 1.6, flex: "1 1 260px" }}>
                  {row.desc}
                </span>
              </div>
            ))}
          </div>

          <div className="howto-grid-2" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "20px" }}>
            <TermMock
              title="console status"
              accent="#93a8ff"
              lines={[
                [{ t: "dim", v: "$ " }, { t: "accent", v: "console status" }],
                [{ t: "plain", v: "Meghana Foods" }, { t: "dim", v: "  ·  out for delivery" }],
                [{ t: "text", v: "ETA " }, { t: "accent", v: "12 min" }]
              ]}
            />
            <TermMock
              title="console order <name>"
              accent="#93a8ff"
              lines={[
                [{ t: "dim", v: "$ " }, { t: "accent", v: "console order" }, { t: "plain", v: " usual-dinner" }],
                [{ t: "plain", v: "Meghana Foods" }, { t: "dim", v: "  ·  HSR Layout, BLR" }],
                [{ t: "text", v: "2× Boneless Biryani, 1× Chicken 65, 1× Butter Naan" }],
                [{ t: "text", v: "total " }, { t: "accent", v: "₹883" }],
                [{ t: "dim", v: "confirm  " }, { t: "accent", v: "↵" }]
              ]}
            />
          </div>
        </section>

        {/* divider */}
        <Divider color="rgba(176,140,245,.18)" />

        {/* ── § YOUR AGENT ── */}
        <section
          id="agent"
          style={{
            position: "relative",
            zIndex: 2,
            maxWidth: "1100px",
            margin: "0 auto",
            padding: "72px clamp(24px,6vw,56px) 80px",
            background:
              "radial-gradient(700px 360px at 85% 0%,rgba(176,140,245,.05),transparent 65%)"
          }}
        >
          <SectionHeader
            kicker="// wire it into your tools"
            glyph="✳"
            title="your agent"
            sub="One command sets up the MCP server and skills across every AI tool you use."
            accent="#b08cf5"
          />

          <div style={{ maxWidth: "70ch", marginBottom: "28px" }}>
            <TermMock
              title="shell"
              accent="#b08cf5"
              lines={[
                [{ t: "dim", v: "$ " }, { t: "accent", v: "console agents install" }],
                [{ t: "text", v: "found Claude Code, Cursor, Codex" }],
                [{ t: "plain", v: "✓ " }, { t: "text", v: "MCP server wired" }],
                [{ t: "plain", v: "✓ " }, { t: "text", v: "skills installed" }],
                [{ t: "dim", v: "3 agents ready" }]
              ]}
            />
          </div>

          <p style={{ maxWidth: "62ch", fontSize: "15px", color: "#8a8fb4", lineHeight: 1.75, marginBottom: "22px" }}>
            It auto-detects Claude Desktop &amp; Code, Cursor, Windsurf, Zed, VS
            Code, Codex, and Hermes, and wires in the MCP server + skills. It
            re-runs itself on update, so your agents stay current.
          </p>

          <div style={{ maxWidth: "70ch", marginBottom: "22px" }}>
            <TermMock
              title="you, to your agent"
              accent="#b08cf5"
              lines={[
                [{ t: "dim", v: "you   " }, { t: "text", v: "order my usual dinner" }],
                [{ t: "dim", v: "agent " }, { t: "text", v: "searching · building the cart · found the bill" }],
                [{ t: "accent", v: "total ₹883" }, { t: "dim", v: "  — place it?" }],
                [{ t: "dim", v: "you   " }, { t: "text", v: "yes" }]
              ]}
            />
          </div>

          <p style={{ maxWidth: "62ch", fontSize: "15px", color: "#8a8fb4", lineHeight: 1.75, margin: 0 }}>
            See the full agent tool list on the{" "}
            <a href="/features" className="lnk" style={{ color: "#b08cf5" }}>
              features page
            </a>
            .
          </p>
        </section>

        {/* ── FOOTER (verbatim from features/page.jsx, how-to link added) ── */}
        <footer
          style={{
            position: "relative",
            zIndex: 2,
            borderTop: "1px solid rgba(147,168,255,.07)",
            marginTop: "40px",
            background: "#040408"
          }}
        >
          <div
            style={{
              maxWidth: "1100px",
              margin: "0 auto",
              padding: "60px clamp(24px,6vw,56px) 50px"
            }}
          >
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                gap: "44px",
                flexWrap: "wrap"
              }}
            >
              {/* left: wordmark + install */}
              <div style={{ maxWidth: "420px" }}>
                <div
                  style={{
                    display: "flex",
                    alignItems: "flex-end",
                    fontWeight: 800,
                    fontSize: "clamp(34px,5.4vw,60px)",
                    letterSpacing: "-.035em",
                    lineHeight: 0.92,
                    marginBottom: "14px"
                  }}
                >
                  <span
                    style={{
                      background:
                        "linear-gradient(168deg,#aebcff 0%,#9c9af4 50%,#b08cf5 100%)",
                      WebkitBackgroundClip: "text",
                      WebkitTextFillColor: "transparent",
                      backgroundClip: "text"
                    }}
                  >
                    console
                  </span>
                  <span
                    style={{
                      color: "#eab560",
                      fontSize: "0.58em",
                      alignSelf: "flex-end",
                      margin: "0 0 .14em .05em"
                    }}
                  >
                    store
                  </span>
                </div>
                <div style={{ fontSize: "15px", color: "#565b80", marginBottom: "20px" }}>
                  <span style={{ color: "#3a3d5c" }}>&gt;</span> see you in the shell{" "}
                  <span
                    style={{
                      display: "inline-block",
                      width: "7px",
                      height: "13px",
                      background: "#93a8ff",
                      verticalAlign: "middle",
                      animation: "blink 1.05s step-end infinite"
                    }}
                  />
                </div>
                <div
                  className="install-foot"
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: "10px",
                    border: "1px solid rgba(147,168,255,.12)",
                    borderRadius: "8px",
                    background: "#0a0a12",
                    padding: "12px 16px",
                    fontSize: "13px",
                    marginBottom: "16px",
                    flexWrap: "wrap"
                  }}
                >
                  <span style={{ color: "#2d2f48" }}>$</span>
                  <span style={{ color: "#e9ebf7" }}>
                    curl -fsSL consolestore.in/install | sh -s -- --beta
                  </span>
                  <span
                    style={{
                      fontSize: "11px",
                      letterSpacing: "1.2px",
                      textTransform: "uppercase",
                      color: "#7fe0ff",
                      border: "1px solid rgba(127,224,255,.32)",
                      borderRadius: "99px",
                      padding: "2px 8px",
                      flex: "none"
                    }}
                  >
                    beta
                  </span>
                </div>
                <div style={{ fontSize: "12px", color: "#2d2f48" }}>
                  // not affiliated with swiggy. preview build, no warranty, no real orders on
                  this page.
                </div>
              </div>

              {/* right: columns */}
              <div
                style={{
                  display: "flex",
                  gap: "40px",
                  fontSize: "13px",
                  color: "#8a8fb4",
                  flexWrap: "wrap"
                }}
              >
                {/* PRODUCT */}
                <div style={{ display: "flex", flexDirection: "column", gap: "11px" }}>
                  <span
                    style={{
                      color: "#565b80",
                      fontSize: "11px",
                      letterSpacing: "1px"
                    }}
                  >
                    PRODUCT
                  </span>
                  <a href="/#run" className="lnk">run</a>
                  <a href="/#keys" className="lnk">terminal &amp; agent</a>
                  <a href="/features" className="lnk">features</a>
                  <a href="/how-to" className="lnk" style={{ color: "#e9ebf7" }}>
                    how-to
                  </a>
                </div>

                {/* SUPPORT */}
                <div style={{ display: "flex", flexDirection: "column", gap: "11px" }}>
                  <span
                    style={{
                      color: "#565b80",
                      fontSize: "11px",
                      letterSpacing: "1px"
                    }}
                  >
                    SUPPORT
                  </span>
                  <a href="mailto:consolestore.in@gmail.com" className="lnk">
                    consolestore.in@gmail.com
                  </a>
                  <a href="/#faq" className="lnk">faq</a>
                </div>

                {/* STATUS */}
                <div style={{ display: "flex", flexDirection: "column", gap: "11px" }}>
                  <span
                    style={{
                      color: "#565b80",
                      fontSize: "11px",
                      letterSpacing: "1px"
                    }}
                  >
                    STATUS
                  </span>
                  <span style={{ display: "inline-flex", alignItems: "center", gap: "7px" }}>
                    <span
                      style={{
                        width: "6px",
                        height: "6px",
                        borderRadius: "99px",
                        background: "#7fe0ff",
                        animation: "pulseDot 2.4s ease-in-out infinite",
                        flex: "none"
                      }}
                    />
                    beta
                  </span>
                </div>
              </div>
            </div>
          </div>
        </footer>
      </div>

      {/* guided walkthrough — activates only via the secret `?guide` link */}
      <GuidedTour />
    </>
  );
}

function Divider({ color }) {
  return (
    <div
      style={{
        maxWidth: "1100px",
        margin: "0 auto",
        padding: "0 clamp(24px,6vw,56px)"
      }}
    >
      <div
        style={{
          height: "1px",
          background: `linear-gradient(90deg,transparent,${color},transparent)`
        }}
      />
    </div>
  );
}
