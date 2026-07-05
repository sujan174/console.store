// Landing markup — ported faithfully from the Claude Design
// "Console Store Landing.dc.html". JetBrains-Mono-only, blue→violet (CONSOLE)
// / gold (STORE) gradient wordmark on pure black. Every data-ref / data-action
// / data-* hook is preserved so logic.js drives it unchanged. The design's
// DCLogic `style-hover` attributes are dropped here and re-expressed as CSS
// hover rules in styles.css (via .lnk / .install-* / [data-key] / [data-cmd]
// / [data-faq-q] / [data-marquee]). Mounted via dangerouslySetInnerHTML.
export const MARKUP = String.raw`
<div data-ref="root" style="position:relative;min-height:100vh;background:radial-gradient(1100px 620px at 72% -6%,rgba(147,168,255,.09),transparent 58%),radial-gradient(800px 500px at 12% 18%,rgba(176,140,245,.07),transparent 58%),#030307">

  <canvas data-ref="ambient" style="position:fixed;inset:0;width:100%;height:100%;pointer-events:none;z-index:0"></canvas>

  <div style="position:fixed;left:-8vw;top:-6vh;width:42vw;height:42vw;border-radius:50%;pointer-events:none;z-index:0;background:radial-gradient(circle,rgba(147,168,255,.09),transparent 60%);filter:blur(42px);animation:orbA 22s ease-in-out infinite"></div>
  <div style="position:fixed;right:-8vw;top:22vh;width:36vw;height:36vw;border-radius:50%;pointer-events:none;z-index:0;background:radial-gradient(circle,rgba(176,140,245,.08),transparent 60%);filter:blur(46px);animation:orbB 28s ease-in-out infinite"></div>

  <div style="position:fixed;inset:0;pointer-events:none;z-index:1;background:repeating-linear-gradient(0deg,rgba(0,0,0,0) 0px,rgba(0,0,0,0) 2px,rgba(0,0,0,.16) 3px,rgba(0,0,0,0) 4px);opacity:.36"></div>
  <div style="position:fixed;inset:0;pointer-events:none;z-index:1;background:radial-gradient(120% 120% at 50% 42%,transparent 58%,rgba(0,0,0,.5) 100%)"></div>

  <div style="position:absolute;top:0;left:0;right:0;height:100vh;pointer-events:none;z-index:0;background-image:linear-gradient(rgba(147,168,255,.04) 1px,transparent 1px),linear-gradient(90deg,rgba(147,168,255,.04) 1px,transparent 1px);background-size:44px 44px;mask-image:radial-gradient(ellipse 78% 66% at 50% 34%,#000 26%,transparent 76%);-webkit-mask-image:radial-gradient(ellipse 78% 66% at 50% 34%,#000 26%,transparent 76%);animation:gridPar both;animation-timeline:scroll(root)"></div>

  <div style="position:fixed;left:86%;top:27%;width:14px;height:14px;pointer-events:none;z-index:2;background:radial-gradient(closest-side,#f3cd84 0%,transparent 70%) center/40% 100% no-repeat,radial-gradient(closest-side,#f3cd84 0%,transparent 70%) center/100% 40% no-repeat;opacity:0;animation:twinkle 7.5s ease-in-out infinite 1.8s;filter:drop-shadow(0 0 5px #f3cd84)"></div>
  <div style="position:fixed;left:10%;top:55%;width:10px;height:10px;pointer-events:none;z-index:2;background:radial-gradient(closest-side,#cdd6ff 0%,transparent 70%) center/40% 100% no-repeat,radial-gradient(closest-side,#cdd6ff 0%,transparent 70%) center/100% 40% no-repeat;opacity:0;animation:twinkle 9s ease-in-out infinite 3.2s;filter:drop-shadow(0 0 4px #cdd6ff)"></div>

  <div class="scroll-bar"></div>

  <!-- NAV -->
  <nav class="site-nav" style="position:relative;z-index:5;display:flex;align-items:center;justify-content:space-between;gap:20px;max-width:1100px;margin:0 auto;padding:22px clamp(24px,6vw,56px);animation:introFade .7s ease both">
    <a href="#top" style="display:inline-flex;align-items:center;gap:10px">
      <svg width="24" height="24" viewBox="0 0 64 64" fill="none" shape-rendering="crispEdges" style="display:block;flex:none;filter:drop-shadow(0 0 5px rgba(147,168,255,.35))">
        <rect x="20" y="18" width="6" height="6" fill="#93a8ff"></rect><rect x="26" y="18" width="6" height="6" fill="#93a8ff"></rect>
        <rect x="26" y="24" width="6" height="6" fill="#9c9af4"></rect><rect x="32" y="24" width="6" height="6" fill="#9c9af4"></rect>
        <rect x="32" y="30" width="6" height="6" fill="#b08cf5"></rect><rect x="38" y="30" width="6" height="6" fill="#b08cf5"></rect>
        <rect x="26" y="36" width="6" height="6" fill="#b08cf5"></rect><rect x="32" y="36" width="6" height="6" fill="#b08cf5"></rect>
        <rect x="20" y="42" width="6" height="6" fill="#b08cf5"></rect><rect x="26" y="42" width="6" height="6" fill="#b08cf5"></rect>
        <rect x="30" y="48" width="18" height="5" fill="#eab560"></rect>
      </svg>
    </a>
    <div class="nav-links" style="display:flex;align-items:center;gap:26px;font-size:12.5px;color:#565b80">
      <a href="#run" class="lnk lnk-desk">run</a>
      <a href="#keys" class="lnk lnk-desk">terminal &amp; agent</a>
      <a href="/features" class="lnk">features</a>
      <a href="/how-to" class="lnk">how-to</a>
      <a href="#faq" class="lnk lnk-desk">faq</a>
      <a href="https://github.com/sujan174/console.store" class="lnk" target="_blank" rel="noopener noreferrer" aria-label="consolestore on GitHub" title="View source on GitHub" style="display:inline-flex;align-items:center">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" focusable="false" style="display:block"><path d="M12 .5C5.37.5 0 5.87 0 12.5c0 5.3 3.44 9.8 8.21 11.39.6.11.82-.26.82-.58 0-.29-.01-1.04-.02-2.05-3.34.72-4.04-1.61-4.04-1.61-.55-1.39-1.34-1.76-1.34-1.76-1.09-.75.08-.73.08-.73 1.2.09 1.84 1.24 1.84 1.24 1.07 1.84 2.81 1.31 3.5 1 .11-.78.42-1.31.76-1.61-2.67-.3-5.47-1.34-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.12-.3-.54-1.52.12-3.18 0 0 1.01-.32 3.3 1.23a11.5 11.5 0 0 1 6 0c2.29-1.55 3.3-1.23 3.3-1.23.66 1.66.24 2.88.12 3.18.77.84 1.24 1.91 1.24 3.22 0 4.61-2.81 5.63-5.49 5.93.43.37.81 1.1.81 2.22 0 1.6-.01 2.9-.01 3.29 0 .32.22.7.83.58A12.01 12.01 0 0 0 24 12.5C24 5.87 18.63.5 12 .5z"></path></svg>
      </a>
    </div>
  </nav>

  <!-- HERO — editorial screen: serif headline + explainer + install. The
       animated ASCII wordmark is relocated to the top-left corner (brand
       signature); the name is explained by the headline + paragraph copy. -->
  <header id="top" style="position:relative;z-index:2;min-height:calc(100vh - 72px);display:flex;flex-direction:column">

    <!-- corner brand signature — plain SOLID wordmark with the footer's
         scramble-reveal. No canvas, no gradient: logic.js bails on the missing
         hero canvas and drives [data-ref=wordmark] via startWordmark (scramble). -->
    <div data-ref="hero3dwrap" title="click to replay" style="position:absolute;left:clamp(20px,5vw,54px);top:15px;z-index:4;cursor:pointer;user-select:none;animation:introFade .9s ease both .2s">
      <div data-ref="wordmark" style="display:none;align-items:flex-end;font-weight:800;font-size:clamp(21px,2.7vw,28px);letter-spacing:-.035em;line-height:.92">
        <span data-wm-console style="background:linear-gradient(168deg,#aebcff 0%,#9c9af4 50%,#b08cf5 100%);-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text">console</span><span data-wm-store style="color:#eab560;font-size:.58em;align-self:flex-end;margin:0 0 .14em .05em">store</span>
      </div>
    </div>

    <div style="flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;width:100%;max-width:1100px;margin:0 auto;padding:18px clamp(24px,6vw,56px) 0">
      <h1 class="hero-head" style="font-weight:700;font-size:clamp(32px,5.8vw,66px);line-height:1.1;letter-spacing:-.025em;color:#e9ebf7;margin:0;animation:introUp .8s cubic-bezier(.22,1,.36,1) both .24s">Great food is just<br>a <em style="font-style:italic;color:#eab560">command</em> away.</h1>

      <p style="margin:28px 0 0;max-width:54ch;font-size:16.5px;line-height:1.8;color:#b6bce0;animation:introUp .8s cubic-bezier(.22,1,.36,1) both .5s">consolestore orders real food — <span style="color:#cdd3f0">coffee, dinner, quick snacks</span> — straight from your terminal, or through your AI agent. No browser tabs, no forms, no cookie walls. <span style="color:#cdd3f0">You type, you eat.</span></p>

      <!-- install command (OS-aware text set in logic.js; live stats live in the right drawer) -->
      <div style="margin-top:34px;display:flex;flex-direction:column;align-items:center;gap:13px;animation:introUp .8s cubic-bezier(.22,1,.36,1) both .7s">
        <div class="install-hero" style="display:inline-flex;align-items:stretch;border:1px solid rgba(147,168,255,.15);border-radius:10px;background:#09090f;box-shadow:0 0 48px rgba(147,168,255,.05),0 24px 64px rgba(0,0,0,.55);overflow:hidden;font-size:13.5px">
          <div data-action="copy" title="click to copy" class="install-cmdrow" style="display:flex;align-items:center;gap:11px;padding:14px 20px;cursor:pointer">
            <span data-install-prompt style="color:#2d2f48">$</span>
            <span data-ref="install" data-install-cmd style="color:#e9ebf7">curl -fsSL consolestore.in/install | sh -s -- --beta</span>
            <span style="margin-left:3px;font-size:10px;letter-spacing:1.2px;text-transform:uppercase;color:#7fe0ff;border:1px solid rgba(127,224,255,.32);border-radius:99px;padding:2px 8px;flex:none">beta</span>
          </div>
          <div class="install-actions" style="display:flex;align-items:stretch;flex:none">
            <div data-action="copy" title="click to copy" class="install-act install-act-gold" style="display:flex;align-items:center;justify-content:center;gap:7px;padding:0 18px;background:#eab560;border-left:1px solid rgba(234,181,96,.4);color:#1a1408;font-weight:600;font-size:11.5px;flex:none;cursor:pointer">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" style="flex:none"><rect x="9" y="9" width="11" height="11" rx="2" stroke="currentColor" stroke-width="1.8"/><path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1" stroke="currentColor" stroke-width="1.8"/></svg><span data-copy-label>copy</span>
            </div>
            <button data-action="share" class="install-act install-share" type="button" hidden style="display:none;align-items:center;justify-content:center;gap:7px;padding:0 16px;background:rgba(176,140,245,.1);border:0;border-left:1px solid rgba(176,140,245,.24);color:#b08cf5;font-size:11.5px;font-family:inherit;flex:none;cursor:pointer">
              <svg width="13" height="13" viewBox="0 0 24 24" fill="none" style="flex:none"><path d="M12 15V3m0 0L7 8m5-5 5 5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/><path d="M4 14v5a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg><span>send to your computer</span>
            </button>
          </div>
        </div>
        <div style="display:flex;align-items:center;gap:11px;font-size:12px;color:#565b80;letter-spacing:.3px">
          <span>one binary</span><span style="color:#2d2f48">·</span>
          <span>macOS</span><span style="color:#2d2f48">·</span>
          <span>Linux</span><span style="color:#2d2f48">·</span>
          <span>Windows</span>
        </div>
        <div data-install-hint style="font-size:11px;color:#2d2f48">beta channel · armed builds place real orders, the default stays safe.</div>
      </div>
    </div>
  </header>

  <!-- PITCH — one full screen: what it is + install (rises up on scroll) -->
  <section id="pitch" data-pitch style="position:relative;z-index:2;min-height:calc(100vh - 40px);display:flex;align-items:center;justify-content:center;padding:48px clamp(24px,6vw,56px)">
    <div class="pitch-grid" data-pitch-grid style="display:grid;grid-template-columns:1.02fr .98fr;gap:clamp(32px,5vw,60px);align-items:center;width:100%;max-width:1100px">
      <div class="pitch-left" style="text-align:left">
        <div data-pitch-item style="font-size:11px;letter-spacing:2px;color:#93a8ff;margin-bottom:6px">// what it is</div>
        <h1 data-pitch-item style="font-weight:700;font-size:clamp(28px,4.2vw,52px);line-height:1.1;letter-spacing:-.015em;color:#e9ebf7;margin:8px 0 0;max-width:16ch">dinner, piped through your <span style="color:#93a8ff">terminal</span> — or your <span style="color:#93a8ff">agent</span>.</h1>
        <p data-pitch-item style="max-width:52ch;color:#b6bce0;font-size:15px;line-height:1.8;margin:22px 0 0">consolestore is one binary: a CLI <span style="color:#cdd3f0">(type commands)</span> and full TUI <span style="color:#cdd3f0">(a keyboard-only menu)</span> for ordering real food through Swiggy without leaving your shell — plus an MCP server <span style="color:#cdd3f0">(a plug-in your AI agent talks to)</span> that lets it order for you. authorize once, then reorder a saved favourite with a keystroke, or just tell your agent <span style="color:#cdd3f0">“order my usual.”</span></p>
        <a href="/features" data-pitch-item class="pitch-cta" aria-label="explore all features" style="display:inline-flex;align-items:center;gap:10px;margin-top:32px;padding:14px 24px;border-radius:11px;border:1px solid rgba(147,168,255,.32);background:rgba(147,168,255,.09);color:#aebcff;font-weight:600;font-size:14px;letter-spacing:.2px;text-decoration:none">explore all 18 features <span style="font-size:16px;line-height:1">→</span></a>
      </div>
      <div data-pitch-item class="pitch-demo">
        <div class="demo-win" style="border:1px solid rgba(147,168,255,.14);border-radius:12px;background:linear-gradient(180deg,#0c0d18,#09090f);box-shadow:0 30px 90px rgba(0,0,0,.6);overflow:hidden">
          <div style="display:flex;align-items:center;gap:8px;padding:11px 15px;border-bottom:1px solid rgba(147,168,255,.07);background:#0e0f1c">
            <span class="win-mark">❯</span>
            <span style="font-size:11px;color:#b6bce0">zsh — one command, dinner's on the way</span>
          </div>
          <div style="padding:20px;font-family:'JetBrains Mono',monospace;font-size:13px;line-height:1.95;color:#a9b1d6;white-space:pre-wrap">
            <div><span style="color:#565b80">~ %</span> <span style="color:#e9ebf7">console order</span> <span style="color:#93a8ff">dinner</span></div>
            <div style="color:#565b80">  ↳ pushing preset <span style="color:#8ee08a">✓</span> · pulling the real bill…</div>
            <div style="margin:2px 0"><span style="color:#565b80">  bill</span>  Meghana Foods · <span style="color:#e9ebf7">₹438</span>   <span style="color:#565b80">confirm</span> <span style="color:#eab560">↵</span></div>
            <div><span style="color:#8ee08a">  ✓ order placed</span> <span style="color:#565b80">· ETA 35 min</span></div>
            <div style="margin-top:16px"><span style="color:#565b80">~ %</span> <span style="color:#e9ebf7">console status</span></div>
            <div style="color:#565b80">  » on the way · 12 min away</div>
          </div>
        </div>
      </div>
    </div>
  </section>


  <!-- TOAST -->
  <div data-ref="toast" style="display:none;position:fixed;left:50%;bottom:34px;transform:translateX(-50%);z-index:50;align-items:center;gap:10px;background:#0c0c18;border:1px solid rgba(234,181,96,.28);border-radius:10px;padding:13px 18px;font-size:13px;color:#e9ebf7;box-shadow:0 20px 60px rgba(0,0,0,.6)">
    <span style="color:#eab560">●</span><span data-toast-msg>coming soon — the install isn't live yet.</span>
  </div>

  <!-- LIVE-STATS readout chip + pop-out drawer — the single home for live stats.
       The chip is a bottom-right terminal status segment: a live dot, a mini
       growth sparkline, and the headline install count. It previews the data
       it opens. Revealed by logic.js after the wordmark settles; the mini
       sparkline + number are filled once /stats lands. -->
  <button data-ref="statstab" class="stats-chip" type="button" aria-haspopup="dialog" aria-expanded="false" aria-label="open live stats" hidden>
    <span class="stats-chip-dot" aria-hidden="true"></span>
    <span class="stats-chip-live">LIVE</span>
    <svg data-ref="chipspark" class="stats-chip-spark" viewBox="0 0 64 20" preserveAspectRatio="none" aria-hidden="true"></svg>
    <span class="stats-chip-val" data-ref="chipval">stats</span>
    <span class="stats-chip-key" aria-hidden="true">⇥</span>
  </button>

  <div data-ref="statsback" class="stats-backdrop"></div>

  <aside data-ref="statsdrawer" class="stats-drawer" role="dialog" aria-modal="true" aria-label="live stats" aria-hidden="true">
    <div class="stats-drawer-head">
      <span class="stats-drawer-title"><span class="stats-chip-dot"></span>live stats</span>
      <button data-ref="statsclose" class="stats-drawer-x" type="button" aria-label="close live stats">×</button>
    </div>
    <div class="stats-drawer-body">
      <div class="stats-big">
        <div class="stats-big-row">
          <b data-dstat="installs">0</b>
          <span class="stats-big-meta">installs<i class="stats-delta" data-ddelta="installs" hidden></i></span>
        </div>
        <div class="stats-big-row">
          <b class="gold" data-dstat="orders">0</b>
          <span class="stats-big-meta">orders placed<i class="stats-delta gold" data-ddelta="orders" hidden></i></span>
        </div>
        <div class="stats-big-row stats-big-row--sm">
          <b data-dstat="active">0</b>
          <span class="stats-big-meta">active this week</span>
        </div>
      </div>

      <!-- growth chart: cumulative installs + orders over the last 8 weeks -->
      <div class="stats-graph">
        <div class="stats-graph-head">
          <span class="stats-graph-title">growth</span>
          <span class="stats-graph-legend">
            <span class="stats-lg stats-lg--i"><i></i>installs</span>
            <span class="stats-lg stats-lg--o"><i></i>orders</span>
          </span>
        </div>
        <svg data-ref="statschart" class="stats-chart" viewBox="0 0 320 132" preserveAspectRatio="none" role="img" aria-label="cumulative installs and orders over the last 8 weeks"></svg>
        <div class="stats-graph-axis"><span>8 weeks ago</span><span>now</span></div>
      </div>

      <div class="stats-foot">anonymous · counted since launch · unified across alpha · beta · stable</div>
    </div>
  </aside>

  <!-- TERMINAL DEMO -->
  <section id="run" style="position:relative;z-index:2;max-width:1100px;margin:0 auto;padding:48px clamp(24px,6vw,56px) 32px" data-reveal>
    <div class="run-grid">
      <div>
        <div style="font-size:11px;letter-spacing:2px;color:#eab560;margin-bottom:14px">// watch it run</div>
        <h2 style="font-weight:800;font-size:clamp(22px,2.8vw,36px);letter-spacing:-.02em;margin:0 0 18px;color:#e9ebf7;line-height:1.1">the whole shop<br>is a <span style="color:#eab560">tui</span>.</h2>
        <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 28px">recreated frame-by-frame from the real bubbletea app. live demo videos land at launch.</p>
        <div style="display:flex;flex-direction:column;gap:11px">
          <div style="display:flex;align-items:center;gap:11px;font-size:12.5px;color:#565b80"><span style="width:6px;height:6px;min-width:6px;background:#8ee08a;border-radius:1px"></span>browse restaurants &amp; menus</div>
          <div style="display:flex;align-items:center;gap:11px;font-size:12.5px;color:#565b80"><span style="width:6px;height:6px;min-width:6px;background:#93a8ff;border-radius:1px"></span>manage cart with keyboard only</div>
          <div style="display:flex;align-items:center;gap:11px;font-size:12.5px;color:#565b80"><span style="width:6px;height:6px;min-width:6px;background:#eab560;border-radius:1px"></span>place &amp; track real orders</div>
        </div>
      </div>
      <div class="demo-win" style="position:relative;border:1px solid rgba(147,168,255,.12);border-radius:13px;background:linear-gradient(180deg,#0c0d18,#09090f);box-shadow:0 40px 120px rgba(0,0,0,.7),0 0 0 1px rgba(147,168,255,.04);overflow:hidden;animation:scaleIn both;animation-timeline:view();animation-range:entry 6% cover 26%">
        <div style="position:absolute;inset:0;pointer-events:none;background:linear-gradient(180deg,rgba(147,168,255,.05),transparent 16%);z-index:1"></div>
        <div style="display:flex;align-items:center;gap:14px;padding:12px 16px;border-bottom:1px solid rgba(147,168,255,.08);background:#0d0e1c">
          <span class="win-mark">❯</span>
          <span style="font-size:12px;color:#b6bce0">consolestore.in ~ %</span>
          <span style="margin-left:auto;display:inline-flex;align-items:center;gap:6px;font-size:10.5px;color:#7fe0ff"><span style="width:5px;height:5px;border-radius:99px;background:#7fe0ff;animation:pulseDot 1.6s ease-in-out infinite;flex:none"></span>live preview</span>
        </div>
        <div class="demo-body" style="position:relative;padding:20px 22px;min-height:380px">
          <div style="position:absolute;left:0;right:0;top:0;height:56px;pointer-events:none;background:linear-gradient(180deg,rgba(147,168,255,.04),transparent);animation:scan 5.5s linear infinite;z-index:0"></div>
          <div data-ref="term" style="position:relative;z-index:1;font-size:13px;line-height:1.65;color:#a9b1d6;white-space:pre-wrap"></div>
          <div data-ref="key" style="position:absolute;right:16px;bottom:12px;z-index:2;font-size:10.5px;color:#565b80;border:1px solid rgba(147,168,255,.12);border-radius:7px;padding:4px 10px;background:#0a0a12;min-width:72px;text-align:center"></div>
        </div>
      </div>
    </div>
  </section>

  <!-- AGENT — order from wherever you already work -->
  <section id="agent" style="position:relative;z-index:2;max-width:1100px;margin:0 auto;padding:48px clamp(24px,6vw,56px) 32px" data-reveal>
    <div class="run-grid">
      <div>
        <div style="font-size:11px;letter-spacing:2px;color:#93a8ff;margin-bottom:14px">// wherever you work</div>
        <h2 style="font-weight:800;font-size:clamp(22px,2.8vw,36px);letter-spacing:-.02em;margin:0 0 18px;color:#e9ebf7;line-height:1.1">order without leaving<br>your <span style="color:#93a8ff">editor</span>.</h2>
        <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 16px">consolestore drops an <span style="color:#e9ebf7">MCP server + skills</span> into the agent you already use — <span style="color:#e9ebf7">Claude, Cursor, Windsurf, Zed, VS Code</span>. ask in plain language, right in your editor or chat: it searches, builds the cart, shows the <span style="color:#e9ebf7">real bill</span>, and places the order <span style="color:#e9ebf7">only after you say yes</span>.</p>
        <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 22px">food or Instamart, same flow — <span style="color:#e9ebf7">"grab me an energy drink"</span> and it's on the way. never a charge you didn't approve.</p>
        <div style="display:flex;flex-wrap:wrap;gap:8px">
          <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">Claude · Cursor · Zed · VS Code</span>
          <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">plain language</span>
          <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">you approve every order</span>
        </div>
      </div>
      <div class="demo-win" style="border:1px solid rgba(176,140,245,.2);border-radius:12px;background:#0a0a12;box-shadow:0 30px 80px rgba(0,0,0,.5);overflow:hidden">
        <div style="display:flex;align-items:center;gap:8px;padding:11px 14px;border-bottom:1px solid rgba(147,168,255,.07);background:#0e0f1c">
          <span style="color:#b08cf5;font-size:13px">✳</span>
          <span style="font-size:11px;color:#565b80">your agent · consolestore mcp</span>
          <span style="margin-left:auto;width:7px;height:7px;border-radius:99px;background:#8ee08a;box-shadow:0 0 8px #8ee08a"></span>
        </div>
        <div style="padding:18px;font-size:12.5px;min-height:280px">
          <div style="margin:0 0 12px;text-align:right"><span style="display:inline-block;background:#14162a;border:1px solid rgba(147,168,255,.16);border-radius:12px 12px 3px 12px;padding:8px 13px;color:#e9ebf7">grab me an energy drink from instamart</span></div>
          <div style="margin:0 0 7px;color:#565b80;font-size:11.5px"><span style="color:#b08cf5">●</span> consolestore · <span style="color:#9aa0c4">im_search_products</span> <span style="color:#8ee08a">✓</span></div>
          <div style="margin:0 0 13px;color:#565b80;font-size:11.5px"><span style="color:#b08cf5">●</span> consolestore · <span style="color:#9aa0c4">im_prepare_order</span> <span style="color:#8ee08a">✓</span></div>
          <div style="margin:0 0 13px;display:flex;gap:8px;align-items:flex-start"><span style="color:#b08cf5;font-size:12px;flex:none;margin-top:1px">✳</span><span style="color:#cdd3f0;line-height:1.6">Red Bull ×4 · Instamart — <span style="color:#e9ebf7">₹460</span> to pay. place it?</span></div>
          <div style="margin:0 0 13px;text-align:right"><span style="display:inline-block;background:#14162a;border:1px solid rgba(147,168,255,.16);border-radius:12px 12px 3px 12px;padding:8px 13px;color:#e9ebf7">yes, go</span></div>
          <div style="margin:0 0 13px;color:#565b80;font-size:11.5px"><span style="color:#b08cf5">●</span> consolestore · <span style="color:#9aa0c4">place_order</span> <span style="color:#8ee08a">✓</span></div>
          <div style="display:flex;gap:8px;align-items:flex-start"><span style="color:#b08cf5;font-size:12px;flex:none;margin-top:1px">✳</span><span style="color:#cdd3f0;line-height:1.6">placed <span style="color:#8ee08a">✓</span> · on its way</span></div>
        </div>
      </div>
    </div>
  </section>

  <!-- TUI / CLI SCROLL-DRIVEN TOGGLE -->
  <section id="keys" class="keys-scrolly" style="position:relative;z-index:2;padding:0 clamp(24px,6vw,56px)">
    <div class="keys-sticky">
      <div style="max-width:1100px;margin:0 auto;width:100%;display:flex;flex-direction:column;align-items:center;gap:34px">
        <div style="text-align:center">
          <div style="font-size:11px;letter-spacing:2px;color:#93a8ff;margin-bottom:18px">// two ways to order</div>
          <div data-toggle-track style="position:relative;display:inline-flex;gap:4px;border:1px solid rgba(147,168,255,.14);border-radius:999px;padding:4px;background:#08080e">
            <div data-toggle-ind style="position:absolute;top:4px;left:4px;width:104px;height:calc(100% - 8px);border-radius:999px;background:rgba(147,168,255,.14);transition:transform .15s linear,width .2s"></div>
            <button data-toggle="tui" style="position:relative;z-index:1;border:0;cursor:pointer;font-family:'JetBrains Mono',monospace;font-weight:700;font-size:11px;letter-spacing:2px;padding:9px 22px;border-radius:999px;background:transparent;color:#e9ebf7;transition:color .25s">TERMINAL</button>
            <button data-toggle="cli" style="position:relative;z-index:1;border:0;cursor:pointer;font-family:'JetBrains Mono',monospace;font-weight:700;font-size:11px;letter-spacing:2px;padding:9px 22px;border-radius:999px;background:transparent;color:#565b80;transition:color .25s">AGENT</button>
          </div>
          <div data-keys-hint style="font-size:11.5px;color:#565b80;margin-top:14px">click to switch — order it yourself at the prompt, or hand it to your agent.</div>
        </div>
        <div data-panel-wrap style="position:relative;width:100%">

          <!-- TERMINAL panel · alias scripting -->
          <div data-panel="tui">
            <div class="keys-grid" style="display:grid;grid-template-columns:1fr 1fr;gap:44px;align-items:center">
              <div>
                <div style="font-size:11px;letter-spacing:2px;color:#93a8ff;margin-bottom:14px">// you, at the prompt</div>
                <h2 style="font-weight:800;font-size:clamp(22px,2.8vw,36px);letter-spacing:-.02em;margin:0 0 16px;color:#e9ebf7;line-height:1.1">order in <span style="color:#93a8ff">one command</span>.</h2>
                <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 16px">set an <span style="color:#e9ebf7">alias</span> once — <span style="color:#e9ebf7">:alias set</span> saves any cart, food <span style="color:#e9ebf7">or</span> Instamart — then it's <span style="color:#93a8ff">one command</span> forever. <span style="color:#e9ebf7">console order coffee</span> from your usual café, <span style="color:#e9ebf7">console order energy-drinks</span> from a specific Instamart, <span style="color:#e9ebf7">console order dinner</span> from your favourite kitchen.</p>
                <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 22px">each one pushes the cart, pulls the <span style="color:#e9ebf7">real bill</span>, and waits for a single <span style="color:#93a8ff">↵</span>. checking a live order? <span style="color:#e9ebf7">console status</span> — one command, live ETA, done.</p>
                <div style="display:flex;flex-wrap:wrap;gap:8px">
                  <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">aliases · one command</span>
                  <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">food + instamart</span>
                  <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">real bill · one ↵</span>
                </div>
              </div>
              <div class="demo-win" style="border:1px solid rgba(147,168,255,.14);border-radius:12px;background:#0a0a12;box-shadow:0 30px 80px rgba(0,0,0,.5);overflow:hidden">
                <div style="display:flex;align-items:center;gap:7px;padding:11px 14px;border-bottom:1px solid rgba(147,168,255,.07);background:#0e0f1c">
                  <span class="win-mark">❯</span>
                  <span style="font-size:11px;color:#b6bce0">zsh — your saved usuals, one command</span>
                </div>
                <div data-ref="cli" style="padding:18px;font-size:13px;line-height:1.95;min-height:280px"></div>
              </div>
            </div>
          </div>

          <!-- AGENT panel · agent ordering -->
          <div data-panel="cli" style="display:none">
            <div class="keys-grid" style="display:grid;grid-template-columns:1fr 1fr;gap:44px;align-items:center">
              <div>
                <span style="display:inline-flex;align-items:center;gap:8px;font-size:10.5px;letter-spacing:1.5px;color:#b08cf5;border:1px solid rgba(176,140,245,.28);border-radius:999px;padding:5px 12px;margin-bottom:18px">NEW · MCP + SKILLS</span>
                <div style="font-size:11px;letter-spacing:2px;color:#93a8ff;margin-bottom:14px">// hand it to your agent</div>
                <h2 style="font-weight:800;font-size:clamp(22px,2.8vw,36px);letter-spacing:-.02em;margin:0 0 16px;color:#e9ebf7;line-height:1.1">let your <span style="color:#b08cf5">agent</span><br>order dinner.</h2>
                <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 16px">consolestore installs an <span style="color:#e9ebf7">MCP server + skills</span> into your AI agent — Claude, Cursor, Windsurf, Zed, VS Code. ask in plain language; it searches, builds the cart, shows the <span style="color:#e9ebf7">real bill</span>, and places the order <span style="color:#e9ebf7">only after you say yes</span>.</p>
                <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 22px">your usual, hands-free — but never a charge you didn't approve. the agent proposes; <span style="color:#93a8ff">you</span> confirm.</p>
                <div style="display:flex;flex-wrap:wrap;gap:8px">
                  <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">Claude · Cursor · Zed</span>
                  <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">you approve every order</span>
                  <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">plain language</span>
                </div>
              </div>
              <div class="demo-win" style="border:1px solid rgba(176,140,245,.2);border-radius:12px;background:#0a0a12;box-shadow:0 30px 80px rgba(0,0,0,.5);overflow:hidden">
                <div style="display:flex;align-items:center;gap:8px;padding:11px 14px;border-bottom:1px solid rgba(147,168,255,.07);background:#0e0f1c">
                  <span style="color:#b08cf5;font-size:13px">✳</span>
                  <span style="font-size:11px;color:#565b80">your agent · consolestore mcp</span>
                  <span style="margin-left:auto;width:7px;height:7px;border-radius:99px;background:#8ee08a;box-shadow:0 0 8px #8ee08a"></span>
                </div>
                <div data-ref="agent" style="padding:18px;font-size:13px;line-height:1.6;min-height:280px"></div>
              </div>
            </div>
          </div>

        </div>
      </div>
    </div>
  </section>

  <!-- MARQUEE -->
  <section style="position:relative;z-index:2;overflow:hidden;border-top:1px solid rgba(147,168,255,.07);border-bottom:1px solid rgba(147,168,255,.07);margin-top:44px;background:#050509">
    <div data-marquee style="display:flex;width:max-content;animation:drift 28s linear infinite;will-change:transform">
      <span style="padding:20px 42px;font-size:16px;color:#565b80">terminal + agent ordering</span><span style="padding:20px 42px;font-size:16px;color:#93a8ff">·</span>
      <span style="padding:20px 42px;font-size:16px;color:#565b80">os keyring auth</span><span style="padding:20px 42px;font-size:16px;color:#b08cf5">·</span>
      <span style="padding:20px 42px;font-size:16px;color:#565b80">guarded live checkout</span><span style="padding:20px 42px;font-size:16px;color:#8ee08a">·</span>
      <span style="padding:20px 42px;font-size:16px;color:#565b80">no server to babysit</span><span style="padding:20px 42px;font-size:16px;color:#eab560">·</span>
      <span style="padding:20px 42px;font-size:16px;color:#565b80">swiggy-backed</span><span style="padding:20px 42px;font-size:16px;color:#7fe0ff">·</span>
      <span style="padding:20px 42px;font-size:16px;color:#565b80">terminal + agent ordering</span><span style="padding:20px 42px;font-size:16px;color:#93a8ff">·</span>
      <span style="padding:20px 42px;font-size:16px;color:#565b80">os keyring auth</span><span style="padding:20px 42px;font-size:16px;color:#b08cf5">·</span>
      <span style="padding:20px 42px;font-size:16px;color:#565b80">guarded live checkout</span><span style="padding:20px 42px;font-size:16px;color:#8ee08a">·</span>
      <span style="padding:20px 42px;font-size:16px;color:#565b80">no server to babysit</span><span style="padding:20px 42px;font-size:16px;color:#eab560">·</span>
      <span style="padding:20px 42px;font-size:16px;color:#565b80">swiggy-backed</span><span style="padding:20px 42px;font-size:16px;color:#7fe0ff">·</span>
    </div>
  </section>

  <!-- MANIFESTO -->
  <section id="why" style="position:relative;z-index:2;max-width:820px;margin:0 auto;padding:64px clamp(24px,6vw,56px)" data-reveal>
    <div style="font-size:11px;letter-spacing:2px;color:#eab560;margin-bottom:24px;text-align:center">// why it's different</div>
    <p style="font-weight:800;font-size:clamp(24px,3.8vw,46px);line-height:1.18;letter-spacing:-.02em;text-align:center;margin:0">
      <span style="color:#e9ebf7">your terminal is open. your agent is listening.</span><br>
      <span style="color:#3a3d5c">why open a browser</span> <span style="color:#eab560">to get dinner?</span>
    </p>
    <p style="max-width:54ch;margin:30px auto 0;text-align:center;color:#b6bce0;font-size:13.5px;line-height:1.78">no tab-hunting, no cookie banners, no context switch — whether you type the command or your agent does. every order is a real Swiggy bill you approve before a rupee moves. the site can be loud because the product underneath is intentionally strict.</p>
  </section>

  <!-- FAQ -->
  <section id="faq" style="position:relative;z-index:2;max-width:760px;margin:0 auto;padding:24px clamp(24px,6vw,56px) 54px" data-reveal>
    <div style="font-size:11px;letter-spacing:2px;color:#93a8ff;margin-bottom:14px">// questions</div>
    <h2 style="font-weight:800;font-size:clamp(20px,2.4vw,30px);letter-spacing:-.02em;margin:0 0 30px;background:linear-gradient(168deg,#aebcff 0%,#9c9af4 52%,#b08cf5 100%);-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text">the obvious ones.</h2>
    <div style="border-top:1px solid rgba(147,168,255,.08)">
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">is it live yet?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">early access. the command above is auto-picked for your OS — <span style="color:#e9ebf7">curl … | sh</span> on macOS/Linux, <span style="color:#e9ebf7">irm … | iex</span> on Windows. it installs a signed binary that self-updates on launch. the public (stable) channel is rolling out; alpha &amp; beta are invite-only.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">can I order without opening the app?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">yes — that's the point for power users. save a cart as a preset once (<span style="color:#e9ebf7">:alias set dinner</span> in the TUI), then <span style="color:#e9ebf7">console order dinner</span> shows the real bill and places it on ↵. <span style="color:#e9ebf7">console status</span> prints your live ETA. no TUI, no browser.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">can my AI agent order for me?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">yes. consolestore installs a <span style="color:#e9ebf7">console mcp</span> server + skills into Claude Desktop/Code, Cursor, Windsurf, Zed, VS Code, and Codex. ask in plain language — the agent searches, builds the cart, and shows the real Swiggy bill, then places the order <span style="color:#e9ebf7">only after you confirm</span>. same broker, same guardrails as the CLI.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">does it actually place real orders?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">only the armed <span style="color:#e9ebf7">console</span> build, and only after you set <span style="color:#e9ebf7">CONSOLE_LIVE_ORDERS=1</span>. plain builds stay at browse + cart. placement always needs an explicit yes — your ↵, or your agent's confirmation — and is never silently retried.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">where do my tokens go?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">into your os keyring via go-keyring. there is no server and no database — auth is a one-time loopback browser handoff with pkce, refreshed locally.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">what backs the menus?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">swiggy's food + instamart mcp api, brokered in-process by the cli — real restaurants, real prices, real delivery estimates.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">which terminals work?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">anything truecolor. kitty graphics renders the hero art where supported; everywhere else falls back to a portable half-block render. macos, linux, and windows terminal are all detected.</p></div>
      </div>
    </div>
  </section>

  <!-- FOOTER -->
  <footer style="position:relative;z-index:2;border-top:1px solid rgba(147,168,255,.07);margin-top:40px;background:#040408">
    <div style="max-width:1100px;margin:0 auto;padding:60px clamp(24px,6vw,56px) 50px">
      <div style="display:flex;justify-content:space-between;gap:44px;flex-wrap:wrap">
        <div style="max-width:420px">
          <div data-ref="footwm" style="display:flex;align-items:flex-end;font-weight:800;font-size:clamp(34px,5.4vw,60px);letter-spacing:-.035em;line-height:.92;margin-bottom:14px">
            <span data-wm-console style="background:linear-gradient(168deg,#aebcff 0%,#9c9af4 50%,#b08cf5 100%);-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text">console</span><span data-wm-store style="color:#eab560;font-size:.58em;align-self:flex-end;margin:0 0 .14em .05em">store</span>
          </div>
          <div style="font-size:13px;color:#565b80;margin-bottom:20px"><span style="color:#3a3d5c">&gt;</span> see you in the shell <span style="display:inline-block;width:7px;height:13px;background:#93a8ff;vertical-align:middle;animation:blink 1.05s step-end infinite"></span></div>
          <div data-action="copy" title="click to copy" class="install-foot" style="display:inline-flex;align-items:center;gap:10px;border:1px solid rgba(147,168,255,.12);border-radius:8px;background:#0a0a12;padding:12px 16px;font-size:12.5px;cursor:pointer;margin-bottom:16px">
            <span data-install-prompt style="color:#2d2f48">$</span>
            <span data-install-cmd style="color:#e9ebf7">curl -fsSL consolestore.in/install | sh -s -- --beta</span>
            <span style="font-size:10px;letter-spacing:1.2px;text-transform:uppercase;color:#7fe0ff;border:1px solid rgba(127,224,255,.32);border-radius:99px;padding:2px 8px;flex:none">beta</span>
            <span style="display:flex;align-items:center;gap:6px;color:#93a8ff;font-size:11px;border-left:1px solid rgba(147,168,255,.18);padding-left:11px;flex:none"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" style="flex:none"><rect x="9" y="9" width="11" height="11" rx="2" stroke="currentColor" stroke-width="1.8"/><path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1" stroke="currentColor" stroke-width="1.8"/></svg><span data-copy-label>copy</span></span>
          </div>
          <div style="font-size:11.5px;color:#2d2f48">// not affiliated with swiggy. preview build, no warranty, no real orders on this page.</div>
        </div>
        <div style="display:flex;gap:40px;font-size:12.5px;color:#b6bce0;flex-wrap:wrap">
          <div style="display:flex;flex-direction:column;gap:11px">
            <span style="color:#565b80;font-size:10.5px;letter-spacing:1px">PRODUCT</span>
            <a href="#run" class="lnk">run</a>
            <a href="#keys" class="lnk">terminal &amp; agent</a>
            <a href="#features" class="lnk">features</a>
            <a href="/how-to" class="lnk">how-to</a>
          </div>
          <div style="display:flex;flex-direction:column;gap:11px">
            <span style="color:#565b80;font-size:10.5px;letter-spacing:1px">SUPPORT</span>
            <a href="mailto:consolestore.in@gmail.com" class="lnk">consolestore.in@gmail.com</a>
            <a href="#faq" class="lnk">faq</a>
          </div>
          <div style="display:flex;flex-direction:column;gap:11px">
            <span style="color:#565b80;font-size:10.5px;letter-spacing:1px">STATUS</span>
            <span style="display:inline-flex;align-items:center;gap:7px"><span style="width:6px;height:6px;border-radius:99px;background:#7fe0ff;animation:pulseDot 2.4s ease-in-out infinite;flex:none"></span>beta</span>
          </div>
        </div>
      </div>
    </div>
  </footer>

</div>
`;
