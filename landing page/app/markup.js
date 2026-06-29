// Ported from Claude Design "Console Store Landing.dc.html", then refined.
// Raw markup, mounted via dangerouslySetInnerHTML. data-ref / data-action /
// class hooks are wired up in logic.js — keep them in sync.
export const MARKUP = String.raw`
<div data-ref="root" style="--accent:#7aa2f7;position:relative;min-height:100vh;background:radial-gradient(1100px 620px at 78% -8%, rgba(122,162,247,0.13), transparent 60%),radial-gradient(900px 560px at 8% 18%, rgba(187,154,247,0.10), transparent 60%),#07070c">

  <!-- ambient floating 3D glyph field (site-wide, non-interactive) -->
  <canvas data-ref="ambient" style="position:fixed;inset:0;width:100%;height:100%;pointer-events:none;z-index:0"></canvas>
  <!-- drifting aurora orbs (depth) -->
  <div style="position:fixed;left:-8vw;top:-6vh;width:46vw;height:46vw;border-radius:50%;pointer-events:none;z-index:0;background:radial-gradient(circle, rgba(122,162,247,0.12), transparent 62%);filter:blur(36px);animation:orbA 24s ease-in-out infinite"></div>
  <div style="position:fixed;right:-10vw;top:24vh;width:40vw;height:40vw;border-radius:50%;pointer-events:none;z-index:0;background:radial-gradient(circle, rgba(187,154,247,0.11), transparent 62%);filter:blur(40px);animation:orbB 30s ease-in-out infinite"></div>
  <div style="position:fixed;left:24vw;bottom:-14vh;width:38vw;height:38vw;border-radius:50%;pointer-events:none;z-index:0;background:radial-gradient(circle, rgba(125,207,255,0.08), transparent 62%);filter:blur(44px);animation:orbC 27s ease-in-out infinite"></div>

  <!-- scroll progress bar -->
  <div style="position:fixed;left:0;top:0;height:2px;width:100%;transform-origin:left;transform:scaleX(0);background:linear-gradient(90deg,var(--accent),#7dcfff);z-index:60;animation:growX both;animation-timeline:scroll(root)"></div>

  <!-- grain / grid overlay -->
  <div style="position:absolute;inset:-60px 0;pointer-events:none;z-index:0;background-image:linear-gradient(rgba(122,162,247,0.05) 1px,transparent 1px),linear-gradient(90deg,rgba(122,162,247,0.05) 1px,transparent 1px);background-size:54px 54px;mask-image:radial-gradient(1200px 760px at 60% 0%, #000 30%, transparent 78%);-webkit-mask-image:radial-gradient(1200px 760px at 60% 0%, #000 30%, transparent 78%);animation:gridPar both;animation-timeline:scroll(root)"></div>

  <!-- NAV -->
  <nav class="site-nav" style="position:relative;z-index:5;display:flex;align-items:center;justify-content:space-between;gap:20px;max-width:1180px;margin:0 auto;padding:26px 28px;animation:introFade .8s ease both">
    <a href="#top" style="display:inline-flex;align-items:center;gap:11px">
      <svg width="30" height="30" viewBox="0 0 64 64" fill="none" style="display:block;flex:none">
        <rect x="2" y="2" width="60" height="60" rx="16" fill="#0e0f18" stroke="#2a2e47" stroke-width="2"></rect>
        <path d="M19 21 L31 32 L19 43" stroke="#c0caf5" stroke-width="6" stroke-linecap="round" stroke-linejoin="round"></path>
        <rect x="34.5" y="37" width="15" height="6" rx="3" fill="#7aa2f7"></rect>
      </svg>
      <span style="color:#c0caf5;font-weight:600;letter-spacing:.4px">consolestore<span style="color:var(--accent)">.in</span></span>
    </a>
    <div class="nav-mid" style="display:flex;align-items:center;gap:26px;font-size:13px;color:#8b93b8">
      <a href="#run" style="transition:color .15s" class="lnk">run</a>
      <a href="#keys" style="transition:color .15s" class="lnk">keys</a>
      <a href="#features" style="transition:color .15s" class="lnk">features</a>
      <a href="#faq" style="transition:color .15s" class="lnk">faq</a>
    </div>
    <div style="display:inline-flex;align-items:center;gap:8px;font-size:11.5px;color:#8b93b8;border:1px solid rgba(122,162,247,0.18);padding:7px 11px;border-radius:999px">
      <span style="width:7px;height:7px;border-radius:99px;background:#e0af68;animation:pulseDot 2.4s ease-in-out infinite"></span>
      v0 · invite-only
    </div>
  </nav>

  <!-- HERO -->
  <header id="top" style="position:relative;z-index:2;max-width:1180px;margin:0 auto;padding:24px 28px 40px">
    <div style="display:flex;flex-direction:column;align-items:center;text-align:center">
      <div style="font-size:12.5px;letter-spacing:2px;color:var(--accent);margin:18px 0 6px;animation:introUp .8s cubic-bezier(.22,1,.36,1) both .15s">// terminal-native ordering</div>
      <!-- particle wordmark: glyphs decode left-to-right into the wordmark, then freeze -->
      <div style="position:relative;width:100%;height:clamp(150px,26vh,300px);margin:4px 0 2px;animation:introPop 1.3s cubic-bezier(.22,1,.36,1) both .05s">
        <canvas data-ref="canvas" style="position:absolute;inset:0;width:100%;height:100%;display:block"></canvas>
      </div>
      <h1 style="font-family:'Space Grotesk',sans-serif;font-weight:500;font-size:clamp(34px,5.4vw,62px);line-height:1.02;letter-spacing:-1.5px;color:#c0caf5;margin:6px 0 0;max-width:18ch;animation:introUp .9s cubic-bezier(.22,1,.36,1) both .65s">dinner, piped through your terminal.</h1>
      <p style="max-width:60ch;color:#8b93b8;font-size:15px;line-height:1.7;margin:20px 0 0;animation:introUp .9s cubic-bezier(.22,1,.36,1) both .85s">a CLI and a full TUI for ordering real food through Swiggy — without leaving your shell. authorize once, then browse, reorder a saved favourite, and track delivery straight from the terminal.</p>

      <!-- install -->
      <div style="margin:30px 0 0;display:flex;flex-direction:column;align-items:center;gap:12px;animation:introUp .9s cubic-bezier(.22,1,.36,1) both 1.05s">
        <div data-action="install" style="display:inline-flex;align-items:center;gap:11px;border:1px solid rgba(122,162,247,0.22);border-radius:9px;background:#0b0b13;box-shadow:0 18px 50px rgba(0,0,0,.5);cursor:pointer;padding:14px 18px;font-size:14px">
          <span style="color:#565f89">$</span>
          <span data-ref="install" style="color:#c0caf5">curl -fsSL consolestore.in/install | sh</span>
        </div>
        <div style="font-size:12px;color:#565f89">armed builds place real orders. the default stays safe.</div>
      </div>

      <div style="margin-top:26px;display:flex;align-items:center;gap:12px;flex-wrap:wrap;justify-content:center;font-size:12.5px;color:#8b93b8;animation:introUp .9s cubic-bezier(.22,1,.36,1) both 1.25s">
        <span>one binary</span><span style="color:#3b3b5a">·</span>
        <span>no mouse</span><span style="color:#3b3b5a">·</span>
        <span>no cookie walls</span><span style="color:#3b3b5a">·</span>
        <span>no checkout forms</span><span style="color:#3b3b5a">·</span>
        <span style="color:#c0caf5">just&nbsp;<span style="color:var(--accent)">↵</span></span>
      </div>
    </div>
  </header>

  <!-- toast -->
  <div data-ref="toast" style="display:none;position:fixed;left:50%;bottom:34px;transform:translateX(-50%);z-index:50;align-items:center;gap:10px;background:#0d0d16;border:1px solid rgba(224,175,104,0.3);border-radius:10px;padding:13px 18px;font-size:13px;color:#c0caf5;box-shadow:0 20px 60px rgba(0,0,0,.6)">
    <span style="color:#e0af68">●</span> coming soon — the curl line is a placeholder.
  </div>

  <!-- LIVE TERMINAL -->
  <section id="run" style="position:relative;z-index:2;max-width:1180px;margin:0 auto;padding:60px 28px 30px" data-reveal>
    <div style="display:flex;align-items:flex-end;justify-content:space-between;gap:24px;flex-wrap:wrap;margin-bottom:26px">
      <div>
        <div style="font-size:12.5px;letter-spacing:2px;color:var(--accent);margin-bottom:10px">// watch it run</div>
        <h2 style="font-family:'Space Grotesk',sans-serif;font-weight:500;font-size:clamp(26px,3.4vw,40px);letter-spacing:-1px;color:#c0caf5;margin:0">the whole shop is a tui.</h2>
      </div>
      <p style="max-width:38ch;color:#8b93b8;font-size:13.5px;line-height:1.65;margin:0">recreated frame-by-frame from the real bubbletea app. live demo videos land at launch.</p>
    </div>

    <!-- terminal window -->
    <div style="position:relative;border:1px solid rgba(122,162,247,0.16);border-radius:13px;background:linear-gradient(180deg,#0c0d15,#0a0a11);box-shadow:0 40px 120px rgba(0,0,0,.6);overflow:hidden;animation:scaleIn both;animation-timeline:view();animation-range:entry 6% cover 26%">
      <div style="position:absolute;inset:0;pointer-events:none;background:linear-gradient(180deg,rgba(122,162,247,0.05),transparent 14%);z-index:1"></div>
      <div style="display:flex;align-items:center;gap:14px;padding:13px 16px;border-bottom:1px solid rgba(122,162,247,0.1);background:#0e0f18">
        <div style="display:flex;gap:8px">
          <span style="width:12px;height:12px;border-radius:99px;background:#f7768e"></span>
          <span style="width:12px;height:12px;border-radius:99px;background:#e0af68"></span>
          <span style="width:12px;height:12px;border-radius:99px;background:#9ece6a"></span>
        </div>
        <span style="font-size:12.5px;color:#565f89">consolestore.in — store</span>
        <span style="margin-left:auto;display:inline-flex;align-items:center;gap:7px;font-size:11px;color:#7dcfff"><span style="width:6px;height:6px;border-radius:99px;background:#7dcfff;animation:pulseDot 1.6s ease-in-out infinite"></span>live preview</span>
      </div>
      <div style="position:relative;padding:24px 26px;min-height:392px">
        <div style="position:absolute;left:0;right:0;top:0;height:60px;pointer-events:none;background:linear-gradient(180deg,rgba(122,162,247,0.05),transparent);animation:scan 5.5s linear infinite;z-index:0"></div>
        <div data-ref="term" style="position:relative;z-index:1;font-size:13.5px;line-height:1.62;color:#a9b1d6;white-space:pre-wrap"></div>
        <div data-ref="key" style="position:absolute;right:18px;bottom:14px;z-index:2;font-size:11px;color:#565f89;border:1px solid rgba(122,162,247,0.16);border-radius:7px;padding:5px 10px;background:#0c0c14;min-width:74px;text-align:center"></div>
      </div>
    </div>
  </section>

  <!-- TUI / CLI showcase (toggle) -->
  <section id="keys" style="position:relative;z-index:2;max-width:1180px;margin:0 auto;padding:60px 28px 20px" data-reveal>
    <div style="text-align:center;margin-bottom:38px">
      <div style="font-size:12.5px;letter-spacing:2px;color:var(--accent);margin-bottom:16px">// two ways to drive it</div>
      <div style="display:inline-flex;gap:5px;border:1px solid rgba(122,162,247,0.2);border-radius:999px;padding:5px;background:#0b0b13">
        <button data-toggle="tui" style="border:0;cursor:pointer;font-family:inherit;font-size:13px;letter-spacing:1px;padding:8px 24px;border-radius:999px;background:rgba(122,162,247,0.18);color:#c0caf5;transition:background .2s,color .2s">TUI</button>
        <button data-toggle="cli" style="border:0;cursor:pointer;font-family:inherit;font-size:13px;letter-spacing:1px;padding:8px 24px;border-radius:999px;background:transparent;color:#565f89;transition:background .2s,color .2s">CLI</button>
      </div>
      <div style="font-size:12px;color:#565f89;margin-top:14px">the full interactive app — or two commands from your shell.</div>
    </div>

    <!-- TUI panel -->
    <div data-panel="tui">
      <div class="keys-grid" style="display:grid;grid-template-columns:1fr 1fr;gap:46px;align-items:center">
        <div>
          <div style="font-size:12.5px;letter-spacing:2px;color:var(--accent);margin-bottom:12px">// the interactive app</div>
          <h2 style="font-family:'Space Grotesk',sans-serif;font-weight:500;font-size:clamp(26px,3.4vw,40px);letter-spacing:-1px;color:#c0caf5;margin:0 0 16px">no mouse. just home row.</h2>
          <p style="color:#8b93b8;font-size:14px;line-height:1.7;margin:0 0 26px">move with the <span style="color:#c0caf5">arrow keys</span>, change quantity with <span style="color:#c0caf5">+ / −</span>, jump to the cart with <span style="color:#c0caf5">c</span>, open with <span style="color:#c0caf5">↵</span>, step back with <span style="color:#c0caf5">esc</span>, and drop into a <span style="color:#bb9af7">:</span> command palette for search, checkout, tracking, and saving presets.</p>
          <div style="display:flex;flex-wrap:wrap;gap:9px">
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#c0caf5;font-size:13px">↑</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#c0caf5;font-size:13px">↓</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#c0caf5;font-size:13px">←</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#c0caf5;font-size:13px">→</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#c0caf5;font-size:13px">↵</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#c0caf5;font-size:13px">c</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#c0caf5;font-size:13px">+</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#c0caf5;font-size:13px">−</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#c0caf5;font-size:13px">⌫</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#c0caf5;font-size:13px">/</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(187,154,247,0.35);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#bb9af7;font-size:13px">:</span>
            <span data-key style="display:inline-grid;place-items:center;min-width:34px;height:34px;padding:0 9px;border:1px solid rgba(122,162,247,0.2);border-bottom-width:3px;border-radius:7px;background:#0d0d16;color:#565f89;font-size:13px">esc</span>
          </div>
        </div>
        <!-- command palette mock -->
        <div style="border:1px solid rgba(122,162,247,0.16);border-radius:11px;background:#0a0a11;box-shadow:0 30px 80px rgba(0,0,0,.5);overflow:hidden">
          <div style="display:flex;align-items:center;gap:8px;padding:14px 16px;border-bottom:1px solid rgba(122,162,247,0.1)">
            <span style="color:#bb9af7">:</span><span data-ref="palette" style="color:#c0caf5"></span><span style="display:inline-block;width:8px;height:15px;background:#7aa2f7;animation:blink 1s step-end infinite;vertical-align:middle"></span>
          </div>
          <div style="padding:8px 0">
            <div data-cmd style="display:flex;justify-content:space-between;padding:10px 16px;font-size:13px;color:#a9b1d6"><span><span style="color:#7aa2f7">checkout</span>  review cart &amp; pay</span><span style="color:#565f89">↵</span></div>
            <div data-cmd style="display:flex;justify-content:space-between;padding:10px 16px;font-size:13px;color:#a9b1d6"><span><span style="color:#7aa2f7">track</span>     follow a live order</span><span style="color:#565f89">↵</span></div>
            <div data-cmd style="display:flex;justify-content:space-between;padding:10px 16px;font-size:13px;color:#a9b1d6"><span><span style="color:#7aa2f7">alias set</span> save this cart as a preset</span><span style="color:#565f89">↵</span></div>
            <div data-cmd style="display:flex;justify-content:space-between;padding:10px 16px;font-size:13px;color:#a9b1d6"><span><span style="color:#7aa2f7">arm</span>       enable live checkout</span><span style="color:#e0af68">!</span></div>
            <div data-cmd style="display:flex;justify-content:space-between;padding:10px 16px;font-size:13px;color:#a9b1d6"><span><span style="color:#7aa2f7">help</span>      keys &amp; commands</span><span style="color:#565f89">↵</span></div>
          </div>
        </div>
      </div>
    </div>

    <!-- CLI panel (hidden until toggled) -->
    <div data-panel="cli" style="display:none">
      <div class="keys-grid" style="display:grid;grid-template-columns:1fr 1fr;gap:46px;align-items:center">
        <div>
          <span style="display:inline-flex;align-items:center;gap:8px;font-size:11px;letter-spacing:1.5px;color:#bb9af7;border:1px solid rgba(187,154,247,0.3);border-radius:999px;padding:5px 12px;margin-bottom:18px">★ FOR POWER USERS</span>
          <h2 style="font-family:'Space Grotesk',sans-serif;font-weight:500;font-size:clamp(26px,3.4vw,40px);letter-spacing:-1px;color:#c0caf5;margin:0 0 16px">order without the app.</h2>
          <p style="color:#8b93b8;font-size:14px;line-height:1.7;margin:0 0 16px">save a cart once as a <span style="color:#c0caf5">preset</span>, and you never have to open the app to eat again. <span style="color:#c0caf5">store order &lt;name&gt;</span> pushes it to your cart, shows the real bill, and waits for a single <span style="color:var(--accent)">↵</span> — two or three keystrokes, five to ten seconds.</p>
          <p style="color:#8b93b8;font-size:14px;line-height:1.7;margin:0 0 22px">checking a live order? <span style="color:#c0caf5">store status</span> — one command, live ETA, done. for a developer it's the fastest path from craving to confirmed: no app, no browser, no breaking flow.</p>
          <div style="display:flex;flex-wrap:wrap;gap:8px">
            <span style="font-size:12px;color:#9aa5c4;border:1px solid rgba(122,162,247,0.18);border-radius:7px;padding:6px 11px;background:#0d0d16">reorder in ~8s</span>
            <span style="font-size:12px;color:#9aa5c4;border:1px solid rgba(122,162,247,0.18);border-radius:7px;padding:6px 11px;background:#0d0d16">status in one command</span>
            <span style="font-size:12px;color:#9aa5c4;border:1px solid rgba(122,162,247,0.18);border-radius:7px;padding:6px 11px;background:#0d0d16">no app · no browser</span>
          </div>
        </div>
        <!-- animated headless terminal -->
        <div style="border:1px solid rgba(187,154,247,0.22);border-radius:11px;background:#0a0a11;box-shadow:0 30px 80px rgba(0,0,0,.5);overflow:hidden">
          <div style="display:flex;align-items:center;gap:8px;padding:11px 14px;border-bottom:1px solid rgba(122,162,247,0.1);background:#0e0f18">
            <span style="width:10px;height:10px;border-radius:99px;background:#f7768e"></span>
            <span style="width:10px;height:10px;border-radius:99px;background:#e0af68"></span>
            <span style="width:10px;height:10px;border-radius:99px;background:#9ece6a"></span>
            <span style="margin-left:6px;font-size:11.5px;color:#565f89">zsh — no TUI, just the shell</span>
          </div>
          <div data-ref="cli" style="padding:18px 18px 20px;font-size:13px;line-height:1.95;min-height:268px"></div>
        </div>
      </div>
    </div>
  </section>

  <!-- marquee -->
  <section style="position:relative;z-index:2;overflow:hidden;border-top:1px solid rgba(122,162,247,0.1);border-bottom:1px solid rgba(122,162,247,0.1);margin-top:54px;background:#090910">
    <div style="display:flex;width:max-content;animation:drift 26s linear infinite;will-change:transform">
      <span style="padding:22px 44px;font-size:18px;color:#565f89">terminal-native ordering</span><span style="padding:22px 44px;font-size:18px;color:#7aa2f7">·</span>
      <span style="padding:22px 44px;font-size:18px;color:#565f89">os keyring auth</span><span style="padding:22px 44px;font-size:18px;color:#bb9af7">·</span>
      <span style="padding:22px 44px;font-size:18px;color:#565f89">guarded live checkout</span><span style="padding:22px 44px;font-size:18px;color:#9ece6a">·</span>
      <span style="padding:22px 44px;font-size:18px;color:#565f89">no server to babysit</span><span style="padding:22px 44px;font-size:18px;color:#e0af68">·</span>
      <span style="padding:22px 44px;font-size:18px;color:#565f89">swiggy-backed</span><span style="padding:22px 44px;font-size:18px;color:#7dcfff">·</span>
      <span style="padding:22px 44px;font-size:18px;color:#565f89">terminal-native ordering</span><span style="padding:22px 44px;font-size:18px;color:#7aa2f7">·</span>
      <span style="padding:22px 44px;font-size:18px;color:#565f89">os keyring auth</span><span style="padding:22px 44px;font-size:18px;color:#bb9af7">·</span>
      <span style="padding:22px 44px;font-size:18px;color:#565f89">guarded live checkout</span><span style="padding:22px 44px;font-size:18px;color:#9ece6a">·</span>
      <span style="padding:22px 44px;font-size:18px;color:#565f89">no server to babysit</span><span style="padding:22px 44px;font-size:18px;color:#e0af68">·</span>
      <span style="padding:22px 44px;font-size:18px;color:#565f89">swiggy-backed</span><span style="padding:22px 44px;font-size:18px;color:#7dcfff">·</span>
    </div>
  </section>

  <!-- FEATURES -->
  <section id="features" style="position:relative;z-index:2;max-width:1180px;margin:0 auto;padding:70px 28px 40px" data-reveal>
    <div style="font-size:12.5px;letter-spacing:2px;color:var(--accent);margin-bottom:12px">// why it's faster</div>
    <h2 style="font-family:'Space Grotesk',sans-serif;font-weight:500;font-size:clamp(26px,3.6vw,42px);letter-spacing:-1px;color:#c0caf5;margin:0 0 14px;max-width:22ch">a browser order is a chore. this is three keystrokes.</h2>
    <p style="color:#8b93b8;font-size:14px;line-height:1.7;max-width:60ch;margin:0 0 34px">no tabs to dig through, no address re-entry, no cookie wall, no “are you still here?”. you stay on home row and dinner is already moving.</p>

    <!-- the browser way  vs  consolestore -->
    <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:14px;margin-bottom:14px">
      <div style="border:1px solid rgba(247,118,142,0.16);border-radius:14px;background:#0c0a0d;padding:26px 26px 22px;display:flex;flex-direction:column">
        <div style="display:flex;align-items:center;gap:9px;margin-bottom:18px">
          <span style="width:8px;height:8px;border-radius:99px;background:#f7768e"></span>
          <span style="font-size:12px;letter-spacing:1px;color:#f7768e">THE BROWSER WAY</span>
        </div>
        <div style="display:flex;flex-direction:column;gap:8px;font-size:13px;color:#8b93b8;line-height:1.4;flex:1">
          <span>open a tab, search swiggy</span>
          <span>log in, clear the captcha</span>
          <span>re-pick your address</span>
          <span>scroll past 40 “sponsored”</span>
          <span>open, add, open the cart</span>
          <span>hunt for a coupon code</span>
          <span>pay, dismiss two pop-ups</span>
        </div>
        <div style="margin-top:18px;padding-top:14px;border-top:1px solid rgba(247,118,142,0.14);font-size:12px;color:#565f89">~14 clicks · 4 tabs · ~2 min · mouse required</div>
      </div>
      <div style="border:1px solid rgba(158,206,106,0.2);border-radius:14px;background:#0a0c0a;padding:26px 26px 22px;display:flex;flex-direction:column">
        <div style="display:flex;align-items:center;gap:9px;margin-bottom:18px">
          <span style="width:8px;height:8px;border-radius:99px;background:#9ece6a"></span>
          <span style="font-size:12px;letter-spacing:1px;color:#9ece6a">CONSOLESTORE</span>
        </div>
        <div style="background:#070908;border:1px solid rgba(158,206,106,0.12);border-radius:9px;padding:15px 16px;font-size:13.5px;line-height:2;flex:1">
          <div><span style="color:#565f89">$</span> <span style="color:#c0caf5">store order</span> <span style="color:#e0af68">dinner</span></div>
          <div><span style="color:#565f89">  real bill, your saved preset</span></div>
          <div><span style="color:#7aa2f7">↵</span> <span style="color:#9ece6a">order placed.</span></div>
          <div><span style="color:#565f89">$</span> <span style="color:#c0caf5">store status</span> <span style="color:#7dcfff">→ 6 mins</span></div>
        </div>
        <div style="margin-top:18px;padding-top:14px;border-top:1px solid rgba(158,206,106,0.14);font-size:12px;color:#565f89">2–3 keystrokes · 0 tabs · ~9s · hands on home row</div>
      </div>
    </div>

    <!-- facts, man-page style (the rest — security sits under the speed story) -->
    <div style="border:1px solid rgba(122,162,247,0.12);border-radius:14px;background:#0a0a12;padding:24px 28px;color:#8b93b8">
      <div style="display:flex;justify-content:space-between;align-items:center;color:#565f89;font-size:12.5px;border-bottom:1px solid rgba(122,162,247,0.1);padding-bottom:13px;margin-bottom:18px">
        <span style="color:#c0caf5">consolestore(1)</span><span style="letter-spacing:1px">FACTS, NOT FEATURES</span>
      </div>
      <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(250px,1fr));gap:26px 44px;font-size:13px;line-height:1.5">

        <div>
          <div style="color:#7aa2f7;letter-spacing:1.5px;font-size:11.5px;margin-bottom:10px">SECURITY</div>
          <div style="display:flex;gap:12px;margin-bottom:8px"><span style="color:#9aa5c4;min-width:74px">tokens</span><span>os keyring (go-keyring). no server, no database.</span></div>
          <div style="display:flex;gap:12px"><span style="color:#9aa5c4;min-width:74px">checkout</span><span>gated — armed build + <span style="color:#c0caf5">CONSOLE_LIVE_ORDERS=1</span>.</span></div>
        </div>

        <div>
          <div style="color:#bb9af7;letter-spacing:1.5px;font-size:11.5px;margin-bottom:10px">RUNTIME</div>
          <div style="display:flex;gap:12px;margin-bottom:8px"><span style="color:#9aa5c4;min-width:74px">process</span><span>one local process. no server, no database, no docker.</span></div>
          <div style="display:flex;gap:12px"><span style="color:#9aa5c4;min-width:74px">backend</span><span>swiggy food + instamart, brokered live.</span></div>
        </div>

        <div>
          <div style="color:#e0af68;letter-spacing:1.5px;font-size:11.5px;margin-bottom:10px">BUILDS</div>
          <div style="display:flex;gap:12px;margin-bottom:8px"><span style="color:#9aa5c4;min-width:74px">store</span><span>armed — places real orders.</span></div>
          <div style="display:flex;gap:12px"><span style="color:#9aa5c4;min-width:74px">safestore</span><span>disarmed — browse + cart only.</span></div>
        </div>

        <div>
          <div style="color:#9ece6a;letter-spacing:1.5px;font-size:11.5px;margin-bottom:10px">SURFACE</div>
          <div style="display:flex;gap:12px;margin-bottom:8px"><span style="color:#9aa5c4;min-width:74px">tui</span><span>browse, customize, cart, checkout, track.</span></div>
          <div style="display:flex;gap:12px"><span style="color:#9aa5c4;min-width:74px">cli</span><span><span style="color:#c0caf5">store order</span> · <span style="color:#c0caf5">store status</span> · <span style="color:#c0caf5">alias</span>.</span></div>
        </div>

      </div>
    </div>
  </section>

  <!-- MANIFESTO -->
  <section style="position:relative;z-index:2;max-width:980px;margin:0 auto;padding:90px 28px" data-reveal>
    <div style="font-size:12.5px;letter-spacing:2px;color:var(--accent);margin-bottom:22px;text-align:center">// why terminal-native</div>
    <p style="font-family:'Space Grotesk',sans-serif;font-weight:500;font-size:clamp(26px,4.2vw,50px);line-height:1.12;letter-spacing:-1.4px;color:#c0caf5;text-align:center;margin:0">your terminal is already open.<br><span style="color:#565f89">why leave it</span> to get lunch?</p>
    <p style="max-width:54ch;margin:30px auto 0;text-align:center;color:#8b93b8;font-size:14px;line-height:1.7">no tab-hunting, no cookie banners, no context switch. one command, a real Swiggy bill, and one ↵ — food is moving. the site can be loud because the product underneath is intentionally strict.</p>
  </section>

  <!-- FAQ -->
  <section id="faq" style="position:relative;z-index:2;max-width:880px;margin:0 auto;padding:30px 28px 40px" data-reveal>
    <div style="font-size:12.5px;letter-spacing:2px;color:var(--accent);margin-bottom:12px">// questions</div>
    <h2 style="font-family:'Space Grotesk',sans-serif;font-weight:500;font-size:clamp(24px,3vw,34px);letter-spacing:-1px;color:#c0caf5;margin:0 0 30px">the obvious ones.</h2>
    <div style="border-top:1px solid rgba(122,162,247,0.12)">
      <div data-faq style="border-bottom:1px solid rgba(122,162,247,0.12)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#c0caf5;font-size:15px">is it live yet?<span data-faq-i style="color:#565f89;transition:transform .2s">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#8b93b8;font-size:13.5px;line-height:1.7">not yet — this is a private preview. the curl line on this page is a placeholder and the install endpoint isn't serving the binary.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(122,162,247,0.12)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#c0caf5;font-size:15px">can I order without opening the app?<span data-faq-i style="color:#565f89;transition:transform .2s">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#8b93b8;font-size:13.5px;line-height:1.7">yes — that's the point for power users. save a cart as a preset once (<span style="color:#c0caf5">:alias set dinner</span> in the TUI), then <span style="color:#c0caf5">store order dinner</span> shows the real bill and places it on ↵. <span style="color:#c0caf5">store status</span> prints your live ETA. no TUI, no browser.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(122,162,247,0.12)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#c0caf5;font-size:15px">does it actually place real orders?<span data-faq-i style="color:#565f89;transition:transform .2s">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#8b93b8;font-size:13.5px;line-height:1.7">only the armed <span style="color:#c0caf5">store</span> build, and only after you set <span style="color:#c0caf5">CONSOLE_LIVE_ORDERS=1</span>. plain builds and <span style="color:#c0caf5">safestore</span> stay at browse + cart. placement always needs an explicit ↵ and is never silently retried.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(122,162,247,0.12)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#c0caf5;font-size:15px">where do my tokens go?<span data-faq-i style="color:#565f89;transition:transform .2s">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#8b93b8;font-size:13.5px;line-height:1.7">into your os keyring via go-keyring. there is no server and no database — auth is a one-time loopback browser handoff with pkce, refreshed locally.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(122,162,247,0.12)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#c0caf5;font-size:15px">what backs the menus?<span data-faq-i style="color:#565f89;transition:transform .2s">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#8b93b8;font-size:13.5px;line-height:1.7">swiggy's food + instamart mcp api, brokered in-process by the cli — real restaurants, real prices, real delivery estimates.</p></div>
      </div>
      <div data-faq style="border-bottom:1px solid rgba(122,162,247,0.12)">
        <div data-faq-q style="display:flex;justify-content:space-between;align-items:center;gap:18px;padding:20px 4px;cursor:pointer;color:#c0caf5;font-size:15px">which terminals work?<span data-faq-i style="color:#565f89;transition:transform .2s">+</span></div>
        <div data-faq-a style="max-height:0;overflow:hidden;transition:max-height .3s ease"><p style="margin:0 4px 20px;color:#8b93b8;font-size:13.5px;line-height:1.7">anything truecolor. kitty graphics renders the hero art where supported; everywhere else falls back to a portable half-block render. macos, linux, and windows terminal are all detected.</p></div>
      </div>
    </div>
  </section>

  <!-- FOOTER -->
  <footer style="position:relative;z-index:2;border-top:1px solid rgba(122,162,247,0.12);margin-top:40px;background:#090910">
    <div style="max-width:1180px;margin:0 auto;padding:64px 28px 48px">
      <div style="font-family:'Space Grotesk',sans-serif;font-weight:600;font-size:clamp(40px,9vw,120px);letter-spacing:-3px;line-height:.9;margin-bottom:40px;background:linear-gradient(90deg,#3a3f5e,#6b73a0,#3a3f5e);background-size:220% 100%;-webkit-background-clip:text;background-clip:text;-webkit-text-fill-color:transparent;color:transparent;animation:sheen 6s linear infinite">consolestore</div>
      <div style="display:flex;justify-content:space-between;gap:30px;flex-wrap:wrap;align-items:flex-end">
        <div>
          <div data-action="install" style="display:inline-flex;align-items:center;gap:11px;border:1px solid rgba(122,162,247,0.18);border-radius:8px;background:#0b0b13;padding:12px 16px;font-size:13px;cursor:pointer;margin-bottom:16px">
            <span style="color:#565f89">$</span><span style="color:#c0caf5">curl -fsSL consolestore.in/install | sh</span>
          </div>
          <div style="font-size:12px;color:#565f89">// not affiliated with swiggy. preview build, no warranty, no real orders on this page.</div>
        </div>
        <div style="display:flex;gap:34px;font-size:13px;color:#8b93b8">
          <div style="display:flex;flex-direction:column;gap:10px">
            <span style="color:#565f89;font-size:11px;letter-spacing:1px">product</span>
            <a href="#run" style="transition:color .15s" class="lnk">run</a>
            <a href="#keys" style="transition:color .15s" class="lnk">keyboard &amp; cli</a>
            <a href="#features" style="transition:color .15s" class="lnk">features</a>
          </div>
          <div style="display:flex;flex-direction:column;gap:10px">
            <span style="color:#565f89;font-size:11px;letter-spacing:1px">status</span>
            <span style="display:inline-flex;align-items:center;gap:7px"><span style="width:7px;height:7px;border-radius:99px;background:#e0af68;animation:pulseDot 2.4s ease-in-out infinite"></span>v0 · invite-only</span>
            <span style="color:#565f89">launching soon</span>
          </div>
        </div>
      </div>
    </div>
  </footer>

</div>
`;
