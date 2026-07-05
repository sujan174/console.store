export const metadata = {
  title: "features — everything consolestore does",
  description:
    "Every feature consolestore ships: full TUI browsing, keyboard-only ordering, preset reorder, live tracking, MCP agent ordering, and more. One binary, two ways.",
  alternates: { canonical: "/features" }
};

// ─── data ────────────────────────────────────────────────────────────────────

const TERMINAL_CARDS = [
  {
    n: "01",
    title: "Browse real menus",
    desc: "A full-screen terminal app for real Swiggy restaurants, prices, and delivery times. Move with the arrow keys; never touch the mouse.",
    tag: "TUI"
  },
  {
    n: "02",
    title: "Order in a few keystrokes",
    desc: "Build a cart, see the real Swiggy bill, and place it with a single ↵. From craving to confirmed in seconds.",
    tag: "TUI"
  },
  {
    n: "03",
    title: "Save & reorder your usuals",
    desc: "Snapshot any cart as a named preset once; then reorder it forever with one short command.",
    tag: "CLI"
  },
  {
    n: "04",
    title: "Live delivery tracking",
    desc: "Watch your order's status and ETA — courier and all — right inside the terminal.",
    tag: "TUI"
  },
  {
    n: "05",
    title: "One-glance order status",
    desc: "Check any in-progress order's live ETA with a single command. No app, no browser.",
    tag: "CLI"
  },
  {
    n: "06",
    title: "The command palette",
    desc: "A : palette for search, checkout, tracking, and saving presets — every action a keystroke away.",
    tag: "TUI"
  },
  {
    n: "07",
    title: "Guarded live checkout",
    desc: "Real orders are off by default. Arm them explicitly, and every placement still needs your confirmation.",
    tag: "SAFETY"
  },
  {
    n: "08",
    title: "Private sign-in",
    desc: "A one-time browser authorize; your token lives in your OS keyring. No server, no database, ever.",
    tag: "AUTH"
  },
  {
    n: "09",
    title: "Always up to date",
    desc: "Every launch quietly pulls the latest signed build. Switch between stable and beta channels anytime.",
    tag: "CLI"
  }
];

const AGENT_CARDS = [
  {
    n: "01",
    title: "Order by chatting",
    desc: 'Tell your AI agent "order my usual dinner" in plain language, and it handles the rest.',
    tag: "ANY AGENT"
  },
  {
    n: "02",
    title: "Works in your tools",
    desc: "Claude Desktop & Code, Cursor, Windsurf, Zed, VS Code, Codex, and Hermes — wherever you already work.",
    tag: "MCP"
  },
  {
    n: "03",
    title: "Searches & builds the cart",
    desc: "Your agent finds the restaurant, picks the items, and assembles the whole order for you.",
    tag: "MCP"
  },
  {
    n: "04",
    title: "Real bill, up front",
    desc: "It always surfaces the true Swiggy total — items, delivery, taxes — before anything is placed.",
    tag: "MCP"
  },
  {
    n: "05",
    title: "You approve every order",
    desc: "The agent proposes; nothing is ever ordered without your explicit yes.",
    tag: "SAFETY"
  },
  {
    n: "06",
    title: "Remembers your taste",
    desc: "A local card of your default address, favorite spots, and dietary preferences, built from real orders.",
    tag: "MEMORY"
  },
  {
    n: "07",
    title: "Reorder presets by name",
    desc: "The same saved carts from the terminal, orderable through your agent by name.",
    tag: "MCP"
  },
  {
    n: "08",
    title: "Track from the chat",
    desc: 'Ask "where\'s my order?" and get live status and ETA without leaving the conversation.',
    tag: "MCP"
  },
  {
    n: "09",
    title: "One-step setup",
    desc: "Installing wires the server and skills into every detected agent — and refreshes them automatically on update.",
    tag: "SETUP"
  }
];

// ─── shared SVG mark (from markup.js, verbatim) ──────────────────────────────

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

// ─── card component ───────────────────────────────────────────────────────────

// A clean feature ROW — number + title + plain-language line, separated by a
// hairline rule. No boxes: the page reads as one calm list, not a wall of cards.
function FeatureRow({ card, accent }) {
  return (
    <div
      className="feat-row"
      style={{
        display: "flex",
        gap: "18px",
        padding: "24px 4px",
        borderTop: "1px solid rgba(147,168,255,.08)"
      }}
    >
      <span
        style={{
          color: accent,
          fontSize: "12.5px",
          fontWeight: 700,
          flex: "none",
          width: "24px",
          lineHeight: 1.5,
          fontVariantNumeric: "tabular-nums"
        }}
      >
        {card.n}
      </span>
      <div>
        <h3
          style={{
            margin: "0 0 7px",
            fontSize: "16px",
            fontWeight: 700,
            color: "#e9ebf7",
            lineHeight: 1.3
          }}
        >
          {card.title}
        </h3>
        <p style={{ margin: 0, fontSize: "13.5px", color: "#b6bce0", lineHeight: 1.72 }}>
          {card.desc}
        </p>
      </div>
    </div>
  );
}

// ─── page ─────────────────────────────────────────────────────────────────────

export default function FeaturesPage() {
  return (
    <>
      {/* inline keyframes + card hover rules — injected once into the head via
          a <style> tag inside the component. Safe for RSC / static export. */}
      <style>{`
        @keyframes featFadeUp {
          from { opacity: 0; transform: translateY(28px); }
          to   { opacity: 1; transform: none; }
        }
        @keyframes headerFade {
          from { opacity: 0; transform: translateY(18px); }
          to   { opacity: 1; transform: none; }
        }

        .feat-card {
          opacity: 1;
          transition: transform 0.22s cubic-bezier(.22,1,.36,1),
                      border-color 0.22s,
                      box-shadow 0.22s;
        }
        @supports (animation-timeline: view()) {
          .feat-card {
            animation: featFadeUp 0.55s cubic-bezier(.22,1,.36,1) both;
            animation-timeline: view();
            animation-range: entry 0% entry 60%;
          }
        }
        /* fallback: stagger on load for browsers without scroll-driven animations */
        @supports not (animation-timeline: view()) {
          .feat-card {
            opacity: 0;
            animation: featFadeUp 0.55s cubic-bezier(.22,1,.36,1) both;
          }
        }
        .feat-card--blue:hover {
          transform: translateY(-3px);
          border-color: rgba(147,168,255,.42) !important;
          box-shadow: 0 12px 40px rgba(147,168,255,.10), 0 2px 0 rgba(147,168,255,.08);
        }
        .feat-card--violet:hover {
          transform: translateY(-3px);
          border-color: rgba(176,140,245,.42) !important;
          box-shadow: 0 12px 40px rgba(176,140,245,.12), 0 2px 0 rgba(176,140,245,.08);
        }

        .feat-section-hdr {
          animation: headerFade 0.7s cubic-bezier(.22,1,.36,1) both;
        }

        @media (max-width: 820px) {
          .feat-nav { display: none !important; }
          .feat-header-title {
            font-size: clamp(28px, 8vw, 44px) !important;
          }
        }
        @media (prefers-reduced-motion: reduce) {
          .feat-card, .feat-section-hdr {
            animation: none !important;
            opacity: 1 !important;
            transform: none !important;
          }
        }
      `}</style>

      {/* ── ambient background (same radials as homepage root) ── */}
      <div
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
        <nav className="feat-nav site-nav">
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
              {/* active: features */}
              <a href="/features" className="lnk nav-page" aria-current="page">
                features
              </a>
              <a href="/how-to" className="lnk nav-page">how-to</a>
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
            className="feat-section-hdr"
            style={{
              fontSize: "11px",
              letterSpacing: "2px",
              color: "#93a8ff",
              textTransform: "lowercase",
              marginBottom: "18px"
            }}
          >
            // everything it does
          </div>
          <h1
            className="feat-header-title"
            style={{
              margin: "0 0 20px",
              fontWeight: 800,
              fontSize: "clamp(30px,5vw,58px)",
              letterSpacing: "-.02em",
              lineHeight: 1.1,
              color: "#e9ebf7"
            }}
          >
            one download. <span style={{ color: "#93a8ff" }}>two ways</span> to order.
          </h1>
          <p
            style={{
              maxWidth: "56ch",
              margin: "0 auto",
              fontSize: "15px",
              color: "#b6bce0",
              lineHeight: 1.8
            }}
          >
            consolestore lets you order real food — and groceries from Instamart —
            without leaving your keyboard. Type a short command, or just ask your AI
            assistant to do it. No app, no browser, no forms. Here&apos;s everything
            it does, in plain terms.
          </p>
        </header>

        {/* ── TERMINAL SECTION ── */}
        <section
          style={{
            position: "relative",
            zIndex: 2,
            maxWidth: "1100px",
            margin: "0 auto",
            padding: "72px clamp(24px,6vw,56px) 48px"
          }}
        >
          {/* section header */}
          <div
            className="feat-section-hdr"
            style={{ marginBottom: "36px" }}
          >
            <div
              style={{
                fontSize: "11px",
                letterSpacing: "2px",
                color: "#93a8ff",
                marginBottom: "10px"
              }}
            >
              // you, at the prompt
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
                  color: "#93a8ff",
                  fontSize: "18px",
                  fontWeight: 700,
                  lineHeight: 1
                }}
              >
                ◆
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
                the terminal
              </h2>
            </div>
            <p
              style={{
                maxWidth: "52ch",
                margin: 0,
                fontSize: "13.5px",
                color: "#8a8fb4",
                lineHeight: 1.7
              }}
            >
              You type; it orders. Browse real restaurants, build a cart, pay, and
              track the delivery — all with the keyboard, right in your terminal. No
              app, no browser.
            </p>
          </div>

          {/* feature rows */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit,minmax(330px,1fr))",
              columnGap: "48px"
            }}
          >
            {TERMINAL_CARDS.map((card) => (
              <FeatureRow key={card.n} card={card} accent="#93a8ff" />
            ))}
          </div>
        </section>

        {/* section divider */}
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
              background:
                "linear-gradient(90deg,transparent,rgba(176,140,245,.18),transparent)"
            }}
          />
        </div>

        {/* ── AGENT SECTION ── */}
        <section
          style={{
            position: "relative",
            zIndex: 2,
            maxWidth: "1100px",
            margin: "0 auto",
            padding: "72px clamp(24px,6vw,56px) 80px"
          }}
        >
          {/* section header */}
          <div
            className="feat-section-hdr"
            style={{ marginBottom: "36px" }}
          >
            <div
              style={{
                fontSize: "11px",
                letterSpacing: "2px",
                color: "#93a8ff",
                marginBottom: "10px"
              }}
            >
              // hand it to your agent
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
                  color: "#b08cf5",
                  fontSize: "18px",
                  fontWeight: 700,
                  lineHeight: 1
                }}
              >
                ✳
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
                the agent
              </h2>
            </div>
            <p
              style={{
                maxWidth: "52ch",
                margin: 0,
                fontSize: "13.5px",
                color: "#8a8fb4",
                lineHeight: 1.7
              }}
            >
              Already using an AI assistant like Claude, Cursor or VS Code? Just ask
              it — in plain words — to order for you. It finds the food, builds the
              cart, shows the price, and only orders once you say yes.
            </p>
          </div>

          {/* feature rows */}
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit,minmax(330px,1fr))",
              columnGap: "48px"
            }}
          >
            {AGENT_CARDS.map((card) => (
              <FeatureRow key={card.n} card={card} accent="#b08cf5" />
            ))}
          </div>
        </section>

        {/* ── FOOTER (verbatim from markup.js) ── */}
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
                <div style={{ fontSize: "13px", color: "#565b80", marginBottom: "20px" }}>
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
                    fontSize: "12.5px",
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
                      fontSize: "10px",
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
                <div style={{ fontSize: "11.5px", color: "#2d2f48" }}>
                  // not affiliated with swiggy. preview build, no warranty, no real orders on
                  this page.
                </div>
              </div>

              {/* right: columns */}
              <div
                style={{
                  display: "flex",
                  gap: "40px",
                  fontSize: "12.5px",
                  color: "#8a8fb4",
                  flexWrap: "wrap"
                }}
              >
                {/* PRODUCT */}
                <div style={{ display: "flex", flexDirection: "column", gap: "11px" }}>
                  <span
                    style={{
                      color: "#565b80",
                      fontSize: "10.5px",
                      letterSpacing: "1px"
                    }}
                  >
                    PRODUCT
                  </span>
                  <a href="/#run" className="lnk">run</a>
                  <a href="/#keys" className="lnk">terminal &amp; agent</a>
                  <a href="/features" className="lnk" style={{ color: "#e9ebf7" }}>
                    features
                  </a>
                  <a href="/how-to" className="lnk">how-to</a>
                </div>

                {/* SUPPORT */}
                <div style={{ display: "flex", flexDirection: "column", gap: "11px" }}>
                  <span
                    style={{
                      color: "#565b80",
                      fontSize: "10.5px",
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
                      fontSize: "10.5px",
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
    </>
  );
}
