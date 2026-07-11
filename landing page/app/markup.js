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
  <nav class="site-nav">
   <div class="site-nav-inner" style="animation:introFade .7s ease both">
    <!-- brand lockup: pixel mark + scramble-reveal wordmark. This IS the only
         brand on the page. logic.js finds [data-ref=hero3dwrap] has no canvas
         inside it, so it reveals + scrambles [data-ref=wordmark] right away.
         Lives inside the nav flex so it stays aligned with the centered content
         column on every viewport — no absolute corner element to drift.
         Clicking it scrolls to the very top (logic.js) + replays the scramble. -->
    <a href="#top" data-ref="hero3dwrap" title="back to top" class="nav-brand" style="display:inline-flex;align-items:center;gap:11px;flex:none;cursor:pointer;user-select:none">
      <svg width="22" height="22" viewBox="0 0 64 64" fill="none" shape-rendering="crispEdges" style="display:block;flex:none;filter:drop-shadow(0 0 5px rgba(147,168,255,.35))">
        <rect x="20" y="18" width="6" height="6" fill="#93a8ff"></rect><rect x="26" y="18" width="6" height="6" fill="#93a8ff"></rect>
        <rect x="26" y="24" width="6" height="6" fill="#9c9af4"></rect><rect x="32" y="24" width="6" height="6" fill="#9c9af4"></rect>
        <rect x="32" y="30" width="6" height="6" fill="#b08cf5"></rect><rect x="38" y="30" width="6" height="6" fill="#b08cf5"></rect>
        <rect x="26" y="36" width="6" height="6" fill="#b08cf5"></rect><rect x="32" y="36" width="6" height="6" fill="#b08cf5"></rect>
        <rect x="20" y="42" width="6" height="6" fill="#b08cf5"></rect><rect x="26" y="42" width="6" height="6" fill="#b08cf5"></rect>
        <rect x="30" y="48" width="18" height="5" fill="#eab560"></rect>
      </svg>
      <div data-ref="wordmark" class="nav-wordmark" style="display:flex;align-items:flex-end;font-weight:800;font-size:clamp(18px,1.85vw,22px);letter-spacing:-.035em;line-height:.92">
        <span data-wm-console style="background:linear-gradient(168deg,#aebcff 0%,#9c9af4 50%,#b08cf5 100%);-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text">console</span><span data-wm-store style="color:#eab560;font-size:.58em;align-self:flex-end;margin:0 0 .14em .05em">store</span>
      </div>
    </a>
    <div class="nav-links">
      <a href="#agent" class="lnk lnk-desk">claude</a>
      <a href="#run" class="lnk lnk-desk">terminal</a>
      <a href="#faq" class="lnk lnk-desk">faq</a>
      <span class="nav-divider lnk-desk" aria-hidden="true"></span>
      <a href="/features" class="lnk nav-page">features</a>
      <a href="/how-to" class="lnk nav-page">how-to</a>
      <a href="https://github.com/sujan174/console.store" class="lnk nav-gh" target="_blank" rel="noopener noreferrer" aria-label="consolestore on GitHub" title="View source on GitHub" style="display:inline-flex;align-items:center">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" focusable="false" style="display:block"><path d="M12 .5C5.37.5 0 5.87 0 12.5c0 5.3 3.44 9.8 8.21 11.39.6.11.82-.26.82-.58 0-.29-.01-1.04-.02-2.05-3.34.72-4.04-1.61-4.04-1.61-.55-1.39-1.34-1.76-1.34-1.76-1.09-.75.08-.73.08-.73 1.2.09 1.84 1.24 1.84 1.24 1.07 1.84 2.81 1.31 3.5 1 .11-.78.42-1.31.76-1.61-2.67-.3-5.47-1.34-5.47-5.93 0-1.31.47-2.38 1.24-3.22-.12-.3-.54-1.52.12-3.18 0 0 1.01-.32 3.3 1.23a11.5 11.5 0 0 1 6 0c2.29-1.55 3.3-1.23 3.3-1.23.66 1.66.24 2.88.12 3.18.77.84 1.24 1.91 1.24 3.22 0 4.61-2.81 5.63-5.49 5.93.43.37.81 1.1.81 2.22 0 1.6-.01 2.9-.01 3.29 0 .32.22.7.83.58A12.01 12.01 0 0 0 24 12.5C24 5.87 18.63.5 12 .5z"></path></svg>
      </a>
    </div>
   </div>
  </nav>

  <!-- HERO — editorial screen: mono headline + explainer + install. The brand
       wordmark lives in the nav lockup above; the name is reinforced by the
       headline + paragraph copy. -->
  <header id="top" style="position:relative;z-index:2;min-height:calc(100vh - 72px);display:flex;flex-direction:column">

    <div style="flex:1;display:flex;flex-direction:column;align-items:center;justify-content:center;text-align:center;width:100%;max-width:1100px;margin:0 auto;padding:18px clamp(24px,6vw,56px) 16vh">
      <h1 class="hero-head" style="font-weight:700;font-size:clamp(32px,5.8vw,66px);line-height:1.1;letter-spacing:-.025em;color:#e9ebf7;margin:0;animation:introUp .8s cubic-bezier(.22,1,.36,1) both .24s">Great food is now just<br>a <span class="hero-cmd">command<span class="hero-caret" aria-hidden="true"></span></span> away.</h1>

      <p style="margin:28px 0 0;max-width:56ch;font-size:16.5px;line-height:1.8;color:#b6bce0;animation:introUp .8s cubic-bezier(.22,1,.36,1) both .5s">consolestore orders real food and groceries through Swiggy — <span style="color:#cdd3f0">coffee, dinner, Instamart runs</span>. two ways in: a <span style="color:#cdd3f0">terminal-native shop</span> for the keyboard cult, or <span style="color:#cdd3f0">just tell Claude</span>. real restaurants, the real bill, and your explicit yes before a rupee moves.</p>

      <!-- the two paths, spelled out before the CTA — one binary serves both -->
      <div style="margin-top:24px;display:flex;gap:10px;flex-wrap:wrap;justify-content:center;animation:introUp .8s cubic-bezier(.22,1,.36,1) both .6s">
        <a href="#agent" class="way-chip" style="display:inline-flex;align-items:center;gap:9px;border:1px solid rgba(176,140,245,.28);border-radius:10px;background:#0d0c18;padding:10px 16px;font-size:12.5px;text-decoration:none"><span style="color:#b08cf5">✳ ask Claude</span><span style="color:#565b80">“get me dinner”</span></a>
        <a href="#run" class="way-chip" style="display:inline-flex;align-items:center;gap:9px;border:1px solid rgba(234,181,96,.26);border-radius:10px;background:#0d0c18;padding:10px 16px;font-size:12.5px;text-decoration:none"><span style="color:#eab560">$ console</span><span style="color:#565b80">the terminal shop</span></a>
      </div>

      <!-- install command (OS-aware text set in logic.js; live stats live in the right drawer) -->
      <div style="margin-top:34px;display:flex;flex-direction:column;align-items:center;gap:13px;animation:introUp .8s cubic-bezier(.22,1,.36,1) both .7s">
        <div class="install-hero" style="display:inline-flex;align-items:stretch;border:1px solid rgba(147,168,255,.15);border-radius:10px;background:#09090f;box-shadow:0 0 48px rgba(147,168,255,.05),0 24px 64px rgba(0,0,0,.55);overflow:hidden;font-size:13.5px">
          <div data-action="copy" title="click to copy" class="install-cmdrow" style="display:flex;align-items:center;gap:11px;padding:14px 20px;cursor:pointer">
            <span data-install-prompt style="color:#2d2f48">$</span>
            <span data-ref="install" data-install-cmd style="color:#e9ebf7">curl -fsSL consolestore.in/install | sh</span>
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
          <span>Windows</span><span style="color:#2d2f48">·</span>
          <span style="color:#b08cf5">+ Claude</span>
        </div>
        <div data-install-hint style="font-size:11.5px;color:#8088b0">armed builds place real orders, the default stays safe.</div>
      </div>
    </div>
  </header>

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

  <!-- AGENT — order from wherever you already work -->
  <section id="agent" style="position:relative;z-index:2;max-width:1100px;margin:0 auto;padding:72px clamp(24px,6vw,56px) 56px" data-reveal>
    <div class="run-grid">
      <div>
        <span style="display:inline-flex;align-items:center;gap:8px;font-size:10.5px;letter-spacing:1.5px;color:#b08cf5;border:1px solid rgba(176,140,245,.3);border-radius:999px;padding:5px 13px;margin-bottom:16px">✳ AI AGENT · MCP + SKILLS</span>
        <div style="font-size:11px;letter-spacing:2px;color:#b08cf5;margin-bottom:14px">// hand it to your ai</div>
        <h2 style="font-weight:800;font-size:clamp(22px,2.8vw,36px);letter-spacing:-.02em;margin:0 0 18px;color:#e9ebf7;line-height:1.1">just tell<br><span style="color:#b08cf5">Claude</span>.</h2>
        <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 16px">one install plugs consolestore into <span style="color:#e9ebf7">Claude Desktop and Claude Code</span>. say <span style="color:#e9ebf7">"order biryani from Meghana Foods"</span> or <span style="color:#e9ebf7">"grab me an energy drink"</span> — Claude searches, builds the cart, shows the <span style="color:#e9ebf7">real Swiggy bill</span>, and places it <span style="color:#e9ebf7">only after you say yes</span>.</p>
        <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 22px">restaurants or <span style="color:#e9ebf7">Instamart groceries</span>, same flow. you see every tool call as it runs, and <span style="color:#e9ebf7">nothing gets charged without your yes</span>.</p>
        <div style="display:flex;flex-wrap:wrap;gap:8px">
          <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">Claude Desktop · Claude Code</span>
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
        <div data-ref="agent" style="padding:18px;font-size:12.5px;min-height:322px"></div>
      </div>
    </div>
  </section>

  <!-- IN-CLAUDE ORDERING APP — replica of the real orderapp widget (Swiggy-orange
       on dark paper, mono wordmark + bill), NOT the page's Tokyo Night theme:
       the contrast is the point — it reads as the actual product embedded in chat. -->
  <section id="fast" style="position:relative;z-index:2;max-width:1100px;margin:0 auto;padding:72px clamp(24px,6vw,56px) 56px" data-reveal>
    <div class="run-grid">
      <div>
        <span style="display:inline-flex;align-items:center;gap:8px;font-size:10.5px;letter-spacing:1.5px;color:#b08cf5;border:1px solid rgba(176,140,245,.3);border-radius:999px;padding:5px 13px;margin-bottom:16px">✳ IN CLAUDE · A REAL APP</span>
        <div style="font-size:11px;letter-spacing:2px;color:#b08cf5;margin-bottom:14px">// the fast path</div>
        <h2 style="font-weight:800;font-size:clamp(22px,2.8vw,36px);letter-spacing:-.02em;margin:0 0 18px;color:#e9ebf7;line-height:1.1">dinner in<br><span style="color:#b08cf5">one message</span>.</h2>
        <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 16px">consolestore puts a <span style="color:#e9ebf7">full ordering app right inside the chat</span>. browse menus, customize dishes, edit the cart — no tabs, no re-typing your address.</p>
        <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 22px">the checkout shows the <span style="color:#e9ebf7">real Swiggy bill</span>, and the button says what it means: <span style="color:#e9ebf7">nothing places before you press it</span>. a few words in chat and dinner is moving in <span style="color:#e9ebf7">seconds, not minutes</span>.</p>
        <div style="display:flex;flex-wrap:wrap;gap:8px">
          <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">a real app, inside Claude</span>
          <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">the real bill, not a guess</span>
          <span style="font-size:12px;color:#b6bce0;border:1px solid rgba(147,168,255,.14);border-radius:7px;padding:5px 11px;background:#0d0d18">you press place</span>
        </div>
      </div>
      <div class="demo-win" style="border:1px solid rgba(176,140,245,.2);border-radius:12px;background:#0a0a12;box-shadow:0 30px 80px rgba(0,0,0,.5);overflow:hidden">
        <div style="display:flex;align-items:center;gap:8px;padding:11px 14px;border-bottom:1px solid rgba(147,168,255,.07);background:#0e0f1c">
          <span style="color:#b08cf5;font-size:13px">✳</span>
          <span style="font-size:11px;color:#565b80">Claude · consolestore</span>
          <span style="margin-left:auto;width:7px;height:7px;border-radius:99px;background:#8ee08a;box-shadow:0 0 8px #8ee08a"></span>
        </div>
        <div style="padding:16px 16px 18px">
          <div style="margin:0 0 14px;text-align:right"><span style="display:inline-block;background:#14162a;border:1px solid rgba(147,168,255,.16);border-radius:12px 12px 3px 12px;padding:8px 13px;color:#e9ebf7;font-size:12.5px">get me biryani from Meghana Foods</span></div>
          <!-- ↓ the widget: real tokens from internal/mcp/orderapp/src/styles.ts (dark) -->
          <div style="border:1px solid #33302a;border-radius:14px;background:#1c1b18;overflow:hidden;box-shadow:0 12px 40px rgba(0,0,0,.5);font-family:ui-sans-serif,system-ui,sans-serif">
            <div style="display:flex;align-items:center;justify-content:space-between;padding:11px 14px;border-bottom:1px solid #33302a;background:#232019">
              <span style="font-family:inherit;font-size:13px;color:#ece7db"><span style="font-family:ui-monospace,monospace"><span style="color:#fc8019">~ %</span> consolestore<span style="display:inline-block;width:7px;height:13px;background:#fc8019;vertical-align:-2px;margin-left:1px;animation:blink 1.1s step-end infinite"></span></span></span>
              <span style="font-size:11px;color:#a6a08f;display:inline-flex;align-items:center;gap:5px"><svg width="11" height="11" viewBox="0 0 24 24" fill="none" style="flex:none"><path d="M12 21s-7-5.1-7-11a7 7 0 1 1 14 0c0 5.9-7 11-7 11z" stroke="currentColor" stroke-width="1.8"/><circle cx="12" cy="10" r="2.6" stroke="currentColor" stroke-width="1.8"/></svg>Home <span style="color:#6f6a5c">⌄</span></span>
            </div>
            <div style="padding:13px 14px 5px">
              <div style="color:#ece7db;font-size:14px;font-weight:600">Meghana Foods</div>
              <div style="color:#6f6a5c;font-size:11px;margin-top:2px">your order · the real bill</div>
            </div>
            <div style="padding:10px 14px 12px;font-family:ui-monospace,monospace;font-size:12px">
              <div style="display:flex;justify-content:space-between;margin-bottom:6px;color:#ece7db"><span><span style="display:inline-block;width:8px;height:8px;border:1px solid #b02b2b;margin-right:7px;position:relative;vertical-align:0"><span style="position:absolute;inset:1.5px;border-radius:99px;background:#b02b2b"></span></span>2 × Chicken Biryani</span><span>₹398</span></div>
              <div style="display:flex;justify-content:space-between;margin-bottom:10px;color:#ece7db"><span><span style="display:inline-block;width:8px;height:8px;border:1px solid #3aab5a;margin-right:7px;position:relative;vertical-align:0"><span style="position:absolute;inset:1.5px;border-radius:99px;background:#3aab5a"></span></span>2 × Butter Naan</span><span>₹80</span></div>
              <div style="border-top:1px solid #33302a;padding-top:8px;display:flex;justify-content:space-between;color:#a6a08f;margin-bottom:4px"><span>item total</span><span>₹478</span></div>
              <div style="display:flex;justify-content:space-between;color:#a6a08f;margin-bottom:4px"><span>delivery</span><span>₹29</span></div>
              <div style="display:flex;justify-content:space-between;color:#a6a08f;margin-bottom:8px"><span>taxes &amp; charges</span><span>₹31</span></div>
              <div style="border-top:1px solid #423e36;padding-top:8px;display:flex;justify-content:space-between;color:#ece7db;font-weight:700"><span>to pay</span><span>₹538</span></div>
            </div>
            <div style="padding:0 14px 13px">
              <div style="display:inline-flex;align-items:center;gap:6px;border:1px solid #33302a;border-radius:999px;background:#232019;padding:4px 10px;font-size:11px;color:#a6a08f;margin-bottom:10px"><svg width="10" height="10" viewBox="0 0 24 24" fill="none" style="flex:none"><path d="M12 21s-7-5.1-7-11a7 7 0 1 1 14 0c0 5.9-7 11-7 11z" stroke="currentColor" stroke-width="1.8"/><circle cx="12" cy="10" r="2.6" stroke="currentColor" stroke-width="1.8"/></svg>deliver to · Home</div>
              <button style="display:block;width:100%;background:#fc8019;border:0;border-radius:10px;padding:11px;color:#fff;font-weight:700;font-family:inherit;font-size:13px;cursor:pointer">place order · ₹538</button>
              <div style="margin-top:7px;text-align:center;font-size:10.5px;color:#6f6a5c">pressing this is your confirmation — nothing places before it</div>
            </div>
          </div>
          <div style="margin:12px 2px 0;color:#565b80;font-size:11px"><span style="color:#b08cf5">✳</span> biryani from Meghana Foods, naan on the side — press place order when ready. I'll track it.</div>
        </div>
      </div>
    </div>
  </section>

  <!-- TERMINAL DEMO -->
  <section id="run" style="position:relative;z-index:2;max-width:1100px;margin:0 auto;padding:72px clamp(24px,6vw,56px) 56px" data-reveal>
    <div class="run-grid">
      <div>
        <div style="font-size:11px;letter-spacing:2px;color:#eab560;margin-bottom:14px">// watch it run</div>
        <h2 style="font-weight:800;font-size:clamp(22px,2.8vw,36px);letter-spacing:-.02em;margin:0 0 18px;color:#e9ebf7;line-height:1.1">the whole shop<br>is a <span style="color:#eab560">tui</span>.</h2>
        <p style="color:#b6bce0;font-size:13.5px;line-height:1.72;margin:0 0 28px">prefer to stay at the prompt? type <span style="color:#e9ebf7">console</span> and the whole shop is a keyboard-only terminal app — browse, cart, checkout, track. what you're watching is the <span style="color:#e9ebf7">actual UI</span>, same screens and same keys.</p>
        <div style="display:flex;flex-direction:column;gap:11px">
          <div style="display:flex;align-items:center;gap:11px;font-size:12.5px;color:#565b80"><span style="width:6px;height:6px;min-width:6px;background:#8ee08a;border-radius:1px"></span>restaurants + instamart, one <span style="color:#9aa0c4">tab</span> apart</div>
          <div style="display:flex;align-items:center;gap:11px;font-size:12.5px;color:#565b80"><span style="width:6px;height:6px;min-width:6px;background:#93a8ff;border-radius:1px"></span>cart to checkout without touching the mouse</div>
          <div style="display:flex;align-items:center;gap:11px;font-size:12.5px;color:#565b80"><span style="width:6px;height:6px;min-width:6px;background:#eab560;border-radius:1px"></span>live order tracking, rider and all</div>
        </div>
      </div>
      <div class="demo-win" style="position:relative;border:1px solid rgba(147,168,255,.12);border-radius:13px;background:linear-gradient(180deg,#0c0d18,#09090f);box-shadow:0 40px 120px rgba(0,0,0,.7),0 0 0 1px rgba(147,168,255,.04);overflow:hidden;animation:scaleIn both;animation-timeline:view();animation-range:entry 6% cover 26%">
        <div style="position:absolute;inset:0;pointer-events:none;background:linear-gradient(180deg,rgba(147,168,255,.05),transparent 16%);z-index:1"></div>
        <div style="display:flex;align-items:center;gap:14px;padding:12px 16px;border-bottom:1px solid rgba(147,168,255,.08);background:#0d0e1c">
          <span class="win-mark">❯</span>
          <span style="font-size:12px;color:#b6bce0">consolestore.in ~ %</span>
          <span style="margin-left:auto;font-size:10.5px;color:#565b80">replay · real screens</span>
        </div>
        <div class="demo-body" style="position:relative;padding:20px 22px;min-height:380px">
          <div style="position:absolute;left:0;right:0;top:0;height:56px;pointer-events:none;background:linear-gradient(180deg,rgba(147,168,255,.04),transparent);animation:scan 5.5s linear infinite;z-index:0"></div>
          <div data-ref="term" style="position:relative;z-index:1;font-size:13px;line-height:1.65;color:#a9b1d6;white-space:pre-wrap"></div>
          <div data-ref="key" style="position:absolute;right:16px;bottom:12px;z-index:2;font-size:10.5px;color:#565b80;border:1px solid rgba(147,168,255,.12);border-radius:7px;padding:4px 10px;background:#0a0a12;min-width:72px;text-align:center"></div>
        </div>
      </div>
    </div>

    <!-- HEADLESS CLI — presets from the shell, no TUI. Output mirrors internal/cli. -->
    <div style="margin-top:22px">
      <div style="border:1px solid rgba(147,168,255,.12);border-radius:12px;background:#09090f;box-shadow:0 24px 70px rgba(0,0,0,.5);overflow:hidden">
        <div style="display:flex;align-items:center;gap:12px;padding:10px 16px;border-bottom:1px solid rgba(147,168,255,.08);background:#0d0e1c">
          <span class="win-mark">❯</span>
          <span style="font-size:11.5px;color:#b6bce0">no TUI needed — reorder is one line</span>
          <span style="margin-left:auto;font-size:10.5px;color:#565b80">replay · real output</span>
        </div>
        <div style="overflow-x:auto">
          <div data-ref="clistrip" style="padding:16px 20px;font-size:12.5px;line-height:1.7;min-height:236px;min-width:420px"></div>
        </div>
      </div>
      <div style="margin:12px 4px 0;font-size:12px;color:#565b80">save any cart once — <span style="color:#b08cf5">:alias set dinner</span> in the TUI — then <span style="color:#e9ebf7">console order dinner</span> from any shell, forever. <span style="color:#9aa0c4">console status</span> prints your live ETA.</div>
    </div>
  </section>

  <!-- WAITLIST — hosted / Claude-web access is coming; capture demand -->
  <section id="waitlist" style="position:relative;z-index:2;max-width:640px;margin:0 auto;padding:24px clamp(24px,6vw,56px) 40px" data-reveal>
    <div style="border:1px solid rgba(176,140,245,.2);border-radius:16px;background:linear-gradient(180deg,#0c0b16,#09090f);padding:clamp(28px,5vw,44px);text-align:center;box-shadow:0 30px 90px rgba(0,0,0,.5)">
      <span style="display:inline-flex;align-items:center;gap:8px;font-size:10.5px;letter-spacing:1.5px;color:#b08cf5;border:1px solid rgba(176,140,245,.3);border-radius:999px;padding:5px 13px;margin-bottom:18px">✳ COMING NEXT</span>
      <h2 style="font-weight:800;font-size:clamp(22px,3vw,34px);letter-spacing:-.02em;margin:0 0 14px;color:#e9ebf7;line-height:1.12">use it in <span style="color:#b08cf5">Claude on the web?</span></h2>
      <p style="max-width:46ch;margin:0 auto 24px;color:#b6bce0;font-size:14px;line-height:1.72">today consolestore runs as a local install — <span style="color:#e9ebf7">Claude Desktop &amp; Claude Code</span>. a hosted version for <span style="color:#e9ebf7">Claude web &amp; mobile</span> is next. drop your email and you'll be first in.</p>
      <form data-waitlist style="display:flex;gap:9px;max-width:420px;margin:0 auto;flex-wrap:wrap;justify-content:center">
        <input data-waitlist-email type="email" required placeholder="you@example.com" aria-label="your email" style="flex:1;min-width:200px;background:#08080e;border:1px solid rgba(147,168,255,.18);border-radius:10px;padding:13px 15px;color:#e9ebf7;font-family:inherit;font-size:13.5px;outline:none" />
        <button data-waitlist-submit type="submit" style="flex:none;background:#b08cf5;border:0;border-radius:10px;padding:0 22px;height:44px;color:#160e28;font-weight:700;font-family:inherit;font-size:13px;letter-spacing:.3px;cursor:pointer">join waitlist</button>
      </form>
      <div data-waitlist-msg role="status" aria-live="polite" style="min-height:16px;margin-top:13px;font-size:12px;color:#565b80"></div>
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
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">the command above is auto-picked for your OS — <span style="color:#e9ebf7">curl … | sh</span> on macOS/Linux, <span style="color:#e9ebf7">irm … | iex</span> on Windows. it installs a signed binary on the <span style="color:#e9ebf7">stable channel</span> that self-updates on launch. alpha &amp; beta channels stay invite-only for early builds.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">can I order without opening the app?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">yes — that's the point for power users. save a cart as a preset once (<span style="color:#e9ebf7">:alias set dinner</span> in the TUI), then <span style="color:#e9ebf7">console order dinner</span> shows the real bill and places it on ↵. <span style="color:#e9ebf7">console status</span> prints your live ETA. no TUI, no browser.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">can my AI agent order for me?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">yes. consolestore installs a <span style="color:#e9ebf7">console mcp</span> server + skills into <span style="color:#e9ebf7">Claude Desktop and Claude Code</span>. ask in plain language — Claude searches, builds the cart, and shows the real Swiggy bill, then places the order <span style="color:#e9ebf7">only after you confirm</span>. same broker, same guardrails as the CLI.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">can I use it in Claude on the web?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">not yet. today consolestore runs as a <span style="color:#e9ebf7">local install</span>, so it works in <span style="color:#e9ebf7">Claude Desktop and Claude Code</span>. Claude web &amp; mobile need a hosted version — that's what's coming next. <span style="color:#e9ebf7">join the waitlist above</span> and we'll ping you the moment it opens.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">does it actually place real orders?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">yes — the installed <span style="color:#e9ebf7">console</span> binary places real Swiggy orders, and only after an explicit yes: <span style="color:#e9ebf7">your ↵ in the terminal, or your confirmation to Claude</span>. builds from source stay at browse + cart unless you arm them. placement is never silently retried.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(147,168,255,.08)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#e9ebf7;font-size:14.5px">where do my Swiggy credentials live?<span data-faq-i style="color:#565b80;transition:transform .2s;flex:none">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#b6bce0;font-size:13px;line-height:1.72">in your <span style="color:#e9ebf7">OS keyring</span>, on your machine. sign-in is a one-time browser handoff (OAuth + PKCE), refreshed locally. there is no consolestore server and no database — nothing of yours leaves your computer.</p></div>
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
            <span data-install-cmd style="color:#e9ebf7">curl -fsSL consolestore.in/install | sh</span>
            <span style="display:flex;align-items:center;gap:6px;color:#93a8ff;font-size:11px;border-left:1px solid rgba(147,168,255,.18);padding-left:11px;flex:none"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" style="flex:none"><rect x="9" y="9" width="11" height="11" rx="2" stroke="currentColor" stroke-width="1.8"/><path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1" stroke="currentColor" stroke-width="1.8"/></svg><span data-copy-label>copy</span></span>
          </div>
          <div style="font-size:11.5px;color:#565b80;line-height:1.65;max-width:430px">consolestore is an independent, unofficial project — not affiliated with, endorsed by, sponsored by, or partnered with Swiggy. it connects to Swiggy's own MCP APIs; restaurants, menus, prices, orders, and delivery are all provided and fulfilled by Swiggy. "Swiggy" and "Instamart" are trademarks of Bundl Technologies Pvt. Ltd., used here only to describe what consolestore connects to.<div style="color:#2d2f48;margin-top:6px">// this page is a preview — no real orders are placed here.</div></div>
        </div>
        <div style="display:flex;gap:40px;font-size:12.5px;color:#b6bce0;flex-wrap:wrap">
          <div style="display:flex;flex-direction:column;gap:11px">
            <span style="color:#565b80;font-size:10.5px;letter-spacing:1px">PRODUCT</span>
            <a href="#agent" class="lnk">claude</a>
            <a href="#run" class="lnk">terminal</a>
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
            <span style="display:inline-flex;align-items:center;gap:7px"><span style="width:6px;height:6px;border-radius:99px;background:#8ee08a;animation:pulseDot 2.4s ease-in-out infinite;flex:none"></span>stable</span>
          </div>
        </div>
      </div>
    </div>
  </footer>

</div>
`;
