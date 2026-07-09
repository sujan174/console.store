// Behaviour ported from Claude Design "Console Store Landing.dc.html",
// rewritten as a framework-free mount(root) that returns a cleanup fn.
// Owns: the ambient city canvas, the hero ASCII-cell wordmark, the footer
// scramble wordmark, the stats chip/drawer, and three demo animators whose
// every visible string is copied from the real app (see
// docs/superpowers/specs/2026-07-10-landing-authenticity-design.md):
//   startTerminal — the TUI replay (internal/tui screens)
//   startAgent    — Claude + MCP tool-call transcript (internal/mcp tools)
//   startCliStrip — headless `console order` / `console status` (internal/cli)

export function mount(root) {
  if (!root) return () => {};

  const S = { dead: false, timers: [], raf: 0, ambRaf: 0, ambResize: null, heroCleanup: null };
  const wait = (ms) => new Promise((r) => S.timers.push(setTimeout(r, ms)));
  const reduce =
    typeof window !== "undefined" &&
    window.matchMedia &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  // Phone-width: the ASCII particle wordmark can't resolve at this size (the
  // sample grid is coarser than the glyph strokes) — use the styled wordmark
  // with the scramble reveal instead, and thin the ambient field.
  const smallHero =
    typeof window !== "undefined" &&
    window.matchMedia &&
    window.matchMedia("(max-width: 700px)").matches;

  const refs = {
    root,
    ambient: root.querySelector('[data-ref="ambient"]'),
    hero3dwrap: root.querySelector('[data-ref="hero3dwrap"]'),
    hero3d: root.querySelector('[data-ref="hero3d"]'),
    wordmark: root.querySelector('[data-ref="wordmark"]'),
    footwm: root.querySelector('[data-ref="footwm"]'),
    term: root.querySelector('[data-ref="term"]'),
    key: root.querySelector('[data-ref="key"]'),
    clistrip: root.querySelector('[data-ref="clistrip"]'),
    agent: root.querySelector('[data-ref="agent"]'),
    toast: root.querySelector('[data-ref="toast"]'),
    statstab: root.querySelector('[data-ref="statstab"]'),
    statsback: root.querySelector('[data-ref="statsback"]'),
    statsdrawer: root.querySelector('[data-ref="statsdrawer"]'),
    statsclose: root.querySelector('[data-ref="statsclose"]'),
    statschart: root.querySelector('[data-ref="statschart"]'),
    chipspark: root.querySelector('[data-ref="chipspark"]'),
    chipval: root.querySelector('[data-ref="chipval"]'),
  };

  // OS detection
  const detectOS = () => {
    const ua = navigator.userAgent || "";
    const plat = navigator.platform || "";
    const touch = (navigator.maxTouchPoints || 0) > 1;
    if (/Android/i.test(ua) || /iPhone|iPad|iPod/i.test(ua)) return "mobile";
    if (/Win/i.test(plat) || /Windows/i.test(ua)) return "windows";
    if (/Mac/i.test(plat) || /Macintosh/i.test(ua)) return touch ? "mobile" : "unix";
    if (/Linux/i.test(plat) || /Linux/i.test(ua)) return touch ? "mobile" : "unix";
    return "unix";
  };
  // Beta channel install commands, auto-picked per OS. Unix/macOS pass --beta to
  // the install script; Windows sets the channel env before the PowerShell installer.
  const INSTALL = {
    unix: { cmd: "curl -fsSL consolestore.in/install | sh -s -- --beta", prompt: "$", hint: "macOS & Linux · beta channel · armed builds place real orders, the default stays safe." },
    windows: { cmd: '$env:CONSOLE_CHANNEL="beta"; irm consolestore.in/install.ps1 | iex', prompt: "PS>", hint: "Windows PowerShell · beta channel · armed builds place real orders, the default stays safe." },
    mobile: { cmd: "curl -fsSL consolestore.in/install | sh -s -- --beta", prompt: "$", hint: "beta channel · run this on your computer — macOS, Linux, or Windows (PowerShell)." },
  }[detectOS()];
  root.querySelectorAll("[data-install-cmd]").forEach((e) => (e.textContent = INSTALL.cmd));
  root.querySelectorAll("[data-install-prompt]").forEach((e) => (e.textContent = INSTALL.prompt));
  root.querySelectorAll("[data-install-hint]").forEach((e) => (e.textContent = INSTALL.hint));

  // Copy the OS-picked install command to the clipboard, with a toast + a brief
  // "copied" flip on the button label.
  const copyInstall = async () => {
    try {
      await navigator.clipboard.writeText(INSTALL.cmd);
    } catch (e) {
      // Fallback for non-secure contexts / older browsers.
      const ta = document.createElement("textarea");
      ta.value = INSTALL.cmd;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      try { document.execCommand("copy"); } catch (e2) {}
      ta.remove();
    }
    const msg = root.querySelector("[data-toast-msg]");
    if (msg) msg.textContent = "copied — paste into your terminal.";
    if (refs.toast) {
      refs.toast.style.display = "flex";
      S.timers.push(setTimeout(() => { if (!S.dead && refs.toast) refs.toast.style.display = "none"; }, 1800));
    }
    const labels = Array.from(root.querySelectorAll("[data-copy-label]"));
    labels.forEach((e) => (e.textContent = "copied ✓"));
    S.timers.push(setTimeout(() => { if (!S.dead) labels.forEach((e) => (e.textContent = "copy")); }, 1800));
  };
  const copyEls = Array.from(root.querySelectorAll('[data-action="copy"]'));
  copyEls.forEach((el) => el.addEventListener("click", copyInstall));

  // Phone CTA: you can't run curl on the phone you opened Instagram with — the
  // share sheet sends the install command to your computer (AirDrop, notes,
  // email, whatever the OS offers). Only shown where navigator.share exists.
  const shareBtn = root.querySelector('[data-action="share"]');
  if (shareBtn && navigator.share && detectOS() === "mobile") {
    const doShare = () => {
      navigator
        .share({ title: "consolestore", text: INSTALL.cmd + "\n\nconsolestore — order food from your terminal.", url: "https://consolestore.in" })
        .catch(() => {});
    };
    shareBtn.hidden = false;
    shareBtn.style.display = "flex";
    shareBtn.addEventListener("click", doShare);
    S.shareCleanup = () => shareBtn.removeEventListener("click", doShare);
  }

  // ===== SCRAMBLE REVEAL =====
  const GLYPHS = "abcdefghijklmnopqrstuvwxyz0123456789·:>_/".split("");
  const scrambleGroup = (g, startDelay) => {
    if (!g) return 0;
    const finalText = g.getAttribute("data-final") || g.textContent;
    g.setAttribute("data-final", finalText);
    g.textContent = "";
    const spans = [];
    for (const ch of finalText) {
      const s = document.createElement("span");
      s.textContent = ch;
      g.appendChild(s);
      spans.push(s);
    }
    spans.forEach((s, i) => {
      const finalCh = finalText[i];
      const lock = startDelay + i * 52 + 170 + Math.random() * 150;
      const t0 = performance.now();
      const tick = (now) => {
        if (S.dead) return;
        if (now - t0 >= lock) { s.textContent = finalCh; return; }
        s.textContent = GLYPHS[(Math.random() * GLYPHS.length) | 0];
        requestAnimationFrame(tick);
      };
      requestAnimationFrame(tick);
    });
    return finalText.length * 52 + 170;
  };
  const wmHandlers = [];
  const startWordmark = () => {
    const wrap = refs.wordmark;
    if (!wrap) return;
    const c = wrap.querySelector("[data-wm-console]");
    const s = wrap.querySelector("[data-wm-store]");
    const play = () => { const d = scrambleGroup(c, 0) || 480; scrambleGroup(s, d - 140); };
    play();
    wrap.addEventListener("click", play);
    wmHandlers.push([wrap, play]);
  };
  const scrambleObservers = [];
  const initFooterWordmark = () => {
    const wrap = refs.footwm;
    if (!wrap || !("IntersectionObserver" in window)) return;
    const c = wrap.querySelector("[data-wm-console]");
    const s = wrap.querySelector("[data-wm-store]");
    let done = false;
    const io = new IntersectionObserver(
      (ents) => {
        ents.forEach((e) => {
          if (e.isIntersecting && !done) {
            done = true;
            const d = scrambleGroup(c, 0) || 420;
            scrambleGroup(s, d - 120);
            io.disconnect();
          }
        });
      },
      { threshold: 0.45 }
    );
    io.observe(wrap);
    scrambleObservers.push(io);
  };

  // scroll reveal
  const initReveal = () => {
    if (reduce) return;
    root.querySelectorAll("[data-reveal]").forEach((el) => {
      el.style.animationName = "revealUp";
      el.style.animationFillMode = "both";
      el.style.animationTimingFunction = "cubic-bezier(.22,1,.36,1)";
      el.style.animationDuration = "1ms";
      el.style.animationTimeline = "view()";
      el.style.animationRange = "entry 2% entry 46%";
    });
  };

  // ---- hero pitch reveal ----
  // The slogan / value line / install card live in #pitch below the fold and
  // rise up as you scroll into them. This is done purely in CSS via a view()
  // scroll timeline (see styles.css) — the same mechanism the rest of the
  // sections use — so there are no JS layout-timing races. Reduced-motion and
  // browsers without scroll-driven animations get the content shown statically
  // (handled in CSS). Nothing to wire up here.

  // FAQ accordion
  const faqHandlers = [];
  const initFaq = () => {
    Array.from(root.querySelectorAll("[data-faq]")).forEach((item) => {
      const q = item.querySelector("[data-faq-q]");
      const a = item.querySelector("[data-faq-a]");
      const ic = item.querySelector("[data-faq-i]");
      const h = () => {
        const open = a.style.maxHeight && a.style.maxHeight !== "0px";
        if (open) { a.style.maxHeight = "0px"; ic.style.transform = "none"; ic.textContent = "+"; }
        else { a.style.maxHeight = a.scrollHeight + "px"; ic.style.transform = "rotate(45deg)"; }
      };
      q.addEventListener("click", h);
      faqHandlers.push([q, h]);
    });
  };

  // ---- live stats: one /stats fetch feeds the right-edge tab → pop-out drawer
  // (the single home for live stats). The tab is always shown; the drawer holds
  // totals + the per-channel breakdown. A failed/empty fetch leaves the drawer's
  // numbers at zero with a "no channel data yet" note. ----
  const statsHandlers = [];
  const initStats = () => {
    const fmt = (n) => Math.round(n).toLocaleString("en-US");
    const countUp = (el, to, dur) => {
      if (!el) return;
      const t0 = performance.now();
      const ease = (u) => 1 - Math.pow(1 - u, 3);
      const step = (now) => {
        if (S.dead) return;
        const u = Math.min((now - t0) / dur, 1);
        el.textContent = fmt(to * ease(u));
        if (u < 1) requestAnimationFrame(step);
        else el.textContent = fmt(to);
      };
      requestAnimationFrame(step);
    };

    // ----- readout chip + drawer -----
    const tab = refs.statstab,
      drawer = refs.statsdrawer,
      back = refs.statsback;
    const SVGNS = "http://www.w3.org/2000/svg";
    const drawerEls = {};
    if (drawer)
      ["orders", "installs", "active"].forEach(
        (k) => (drawerEls[k] = drawer.querySelector('[data-dstat="' + k + '"]'))
      );
    const deltaEls = {};
    if (drawer)
      ["orders", "installs"].forEach(
        (k) => (deltaEls[k] = drawer.querySelector('[data-ddelta="' + k + '"]'))
      );

    // svgPath builds a smooth-ish polyline path from values normalized into a
    // WxH box with padding; returns { line, area } d-strings.
    const svgPath = (vals, w, h, pad, maxOverride) => {
      const n = vals.length;
      if (n < 2) return { line: "", area: "" };
      const max = Math.max(1, maxOverride || 0, ...vals);
      const x = (i) => pad + (i * (w - 2 * pad)) / (n - 1);
      const y = (v) => h - pad - (v / max) * (h - 2 * pad);
      let line = "M " + x(0) + " " + y(vals[0]);
      for (let i = 1; i < n; i++) line += " L " + x(i) + " " + y(vals[i]);
      const area = line + " L " + x(n - 1) + " " + (h - pad) + " L " + x(0) + " " + (h - pad) + " Z";
      return { line, area, x, y };
    };

    // renderChart draws the cumulative installs + orders growth chart into the
    // drawer's SVG. Two area+line series (installs = blue, orders = gold) share
    // one vertical scale so their relative growth reads honestly.
    const renderChart = (series) => {
      const svg = refs.statschart;
      if (!svg) return;
      while (svg.firstChild) svg.removeChild(svg.firstChild);
      if (!series || series.length < 2) {
        const t = document.createElementNS(SVGNS, "text");
        t.setAttribute("x", "160"); t.setAttribute("y", "70");
        t.setAttribute("text-anchor", "middle"); t.setAttribute("fill", "#2d2f48");
        t.setAttribute("font-size", "12"); t.setAttribute("font-family", "JetBrains Mono, monospace");
        t.textContent = "growth data warming up…";
        svg.appendChild(t);
        return;
      }
      const W = 320, H = 132, PAD = 10;
      const installs = series.map((s) => s.installs);
      const orders = series.map((s) => s.orders);

      // faint baseline
      const base = document.createElementNS(SVGNS, "line");
      base.setAttribute("x1", PAD); base.setAttribute("x2", W - PAD);
      base.setAttribute("y1", H - PAD); base.setAttribute("y2", H - PAD);
      base.setAttribute("stroke", "rgba(147,168,255,0.1)"); base.setAttribute("stroke-width", "1");
      svg.appendChild(base);

      const defs = document.createElementNS(SVGNS, "defs");
      defs.innerHTML =
        '<linearGradient id="gi" x1="0" y1="0" x2="0" y2="1">' +
        '<stop offset="0" stop-color="#93a8ff" stop-opacity="0.28"/>' +
        '<stop offset="1" stop-color="#93a8ff" stop-opacity="0"/></linearGradient>' +
        '<linearGradient id="go" x1="0" y1="0" x2="0" y2="1">' +
        '<stop offset="0" stop-color="#eab560" stop-opacity="0.22"/>' +
        '<stop offset="1" stop-color="#eab560" stop-opacity="0"/></linearGradient>';
      svg.appendChild(defs);

      const shared = Math.max(1, ...installs, ...orders); // one scale for both series
      const draw = (vals, stroke, fill) => {
        const p = svgPath(vals, W, H, PAD, shared);
        const area = document.createElementNS(SVGNS, "path");
        area.setAttribute("d", p.area); area.setAttribute("fill", fill); area.setAttribute("stroke", "none");
        svg.appendChild(area);
        const line = document.createElementNS(SVGNS, "path");
        line.setAttribute("d", p.line); line.setAttribute("fill", "none");
        line.setAttribute("stroke", stroke); line.setAttribute("stroke-width", "2");
        line.setAttribute("stroke-linejoin", "round"); line.setAttribute("stroke-linecap", "round");
        if (!reduce) {
          const len = line.getTotalLength ? line.getTotalLength() : 400;
          line.style.strokeDasharray = len; line.style.strokeDashoffset = len;
          line.style.transition = "stroke-dashoffset .9s ease";
          requestAnimationFrame(() => { line.style.strokeDashoffset = "0"; });
        }
        svg.appendChild(line);
        // endpoint dot
        const dot = document.createElementNS(SVGNS, "circle");
        dot.setAttribute("cx", p.x(vals.length - 1)); dot.setAttribute("cy", p.y(vals[vals.length - 1]));
        dot.setAttribute("r", "3"); dot.setAttribute("fill", stroke);
        dot.setAttribute("class", "chart-dot");
        svg.appendChild(dot);
      };
      draw(installs, "#93a8ff", "url(#gi)");
      draw(orders, "#eab560", "url(#go)");
    };

    // renderChipSpark draws the mini installs sparkline inside the chip.
    const renderChipSpark = (series) => {
      const svg = refs.chipspark;
      if (!svg) return;
      while (svg.firstChild) svg.removeChild(svg.firstChild);
      if (!series || series.length < 2) return;
      const p = svgPath(series.map((s) => s.installs), 64, 20, 3);
      const area = document.createElementNS(SVGNS, "path");
      area.setAttribute("d", p.area); area.setAttribute("fill", "rgba(127,224,255,0.16)");
      svg.appendChild(area);
      const line = document.createElementNS(SVGNS, "path");
      line.setAttribute("d", p.line); line.setAttribute("fill", "none");
      line.setAttribute("stroke", "#7fe0ff"); line.setAttribute("stroke-width", "1.6");
      line.setAttribute("stroke-linejoin", "round"); line.setAttribute("stroke-linecap", "round");
      svg.appendChild(line);
    };

    const renderDeltas = (vals) => {
      const put = (el, n, unit) => {
        if (!el) return;
        if (n > 0) { el.textContent = "+" + fmt(n) + " " + unit; el.hidden = false; }
        else el.hidden = true;
      };
      put(deltaEls.installs, vals.installs_week, "this week");
      put(deltaEls.orders, vals.orders_week, "this week");
    };

    // The chip's headline: install count + a "this week" hint once data lands.
    const renderChip = (vals) => {
      if (refs.chipval) {
        refs.chipval.innerHTML = fmt(vals.installs) + " <small>installs</small>";
      }
      renderChipSpark(vals.series);
    };

    let lastFocus = null;
    const openDrawer = () => {
      if (!drawer) return;
      lastFocus = document.activeElement;
      if (back) back.classList.add("open");
      drawer.classList.add("open");
      drawer.setAttribute("aria-hidden", "false");
      if (tab) tab.setAttribute("aria-expanded", "true");
      const vals = statsState.data || EMPTY_STATS;
      renderChart(vals.series);
      renderDeltas(vals);
      ["orders", "installs", "active"].forEach((k) => {
        if (!drawerEls[k]) return;
        if (reduce) drawerEls[k].textContent = fmt(vals[k]);
        else countUp(drawerEls[k], vals[k], 900);
      });
      if (refs.statsclose) refs.statsclose.focus();
    };
    const closeDrawer = () => {
      if (!drawer) return;
      if (back) back.classList.remove("open");
      drawer.classList.remove("open");
      drawer.setAttribute("aria-hidden", "true");
      if (tab) tab.setAttribute("aria-expanded", "false");
      if (lastFocus && lastFocus.focus) lastFocus.focus();
    };

    // Expose so the keyboard controller (Tab) can drive the drawer.
    S.openStats = openDrawer;
    S.closeStats = closeDrawer;
    S.statsIsOpen = () => !!(drawer && drawer.classList.contains("open"));

    if (tab) { tab.addEventListener("click", openDrawer); statsHandlers.push([tab, "click", openDrawer]); }
    if (back) { back.addEventListener("click", closeDrawer); statsHandlers.push([back, "click", closeDrawer]); }
    if (refs.statsclose) { refs.statsclose.addEventListener("click", closeDrawer); statsHandlers.push([refs.statsclose, "click", closeDrawer]); }
    const onKey = (e) => { if (e.key === "Escape") closeDrawer(); };
    document.addEventListener("keydown", onKey);
    statsHandlers.push([document, "keydown", onKey]);

    const revealTab = () => {
      if (!tab) return;
      const show = () => {
        if (S.dead) return;
        tab.removeAttribute("hidden");
        requestAnimationFrame(() => tab.classList.add("is-in"));
      };
      S.timers.push(setTimeout(show, reduce ? 250 : 2600));
    };

    // ----- fetch feeds the chip preview + the drawer -----
    // The chip is always revealed (persistent affordance); when /stats lands it
    // fills the chip's mini sparkline + install count. The drawer renders the
    // full growth chart + weekly deltas on open. A failed/empty fetch just
    // leaves zeros and a "warming up" chart.
    revealTab();
    fetch("/stats", { headers: { accept: "application/json" } })
      .then((r) => (r.ok ? r.json() : null))
      .then((d) => {
        if (S.dead || !d) return;
        statsState.data = {
          orders: +d.orders || 0,
          installs: +d.installs || 0,
          active: +d.active_installs || 0,
          orders_week: +d.orders_week || 0,
          installs_week: +d.installs_week || 0,
          series: Array.isArray(d.series) ? d.series : [],
        };
        renderChip(statsState.data);
        // If the drawer is already open (opened before data landed), refresh it.
        if (S.statsIsOpen && S.statsIsOpen()) {
          renderChart(statsState.data.series);
          renderDeltas(statsState.data);
        }
      })
      .catch(() => {});

    S.statsCleanup = () => statsHandlers.forEach(([el, ev, h]) => el.removeEventListener(ev, h));
  };
  const EMPTY_STATS = { orders: 0, installs: 0, active: 0, orders_week: 0, installs_week: 0, series: [] };
  const statsState = { data: null };

  // Cinematic "console city" backdrop — a bespoke generative Tokyo-Night
  // skyline rendered on the single fixed [data-ref=ambient] canvas (replaces the
  // old rotating starfield). Three parallax depth bands of towers built from
  // lit terminal "windows", drifting embers, an aurora sky wash and a gold
  // horizon glow. Everything rides the shared rAF slot (S.ambRaf) + resize slot
  // (S.ambResize); pointer parallax eases and honours reduced-motion / phone.
  const startAmbient = () => {
    const cv = refs.ambient;
    if (!cv) return;
    const ctx = cv.getContext("2d");
    const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
    let W = 0, H = 0;

    const hex = (h, a) => { const n = parseInt(h.slice(1), 16); return "rgba(" + ((n >> 16) & 255) + "," + ((n >> 8) & 255) + "," + (n & 255) + "," + a + ")"; };
    // seeded PRNG so the skyline is stable frame-to-frame (rebuilt only on resize).
    const mulberry = (a) => () => { a |= 0; a = (a + 0x6d2b79f5) | 0; let t = Math.imul(a ^ (a >>> 15), 1 | a); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };

    // depth bands, far → near. base = fraction of H where a tower's foot sits;
    // hCap = tallest a tower may reach as a fraction of H; win = lit-window hue.
    const BANDS = [
      { seed: 11, col: "#222a55", win: "#4a5aa8", hCap: 0.12, base: 0.78, par: 26, alpha: 0.5 },
      { seed: 29, col: "#191634", win: "#7a5fd0", hCap: 0.2, base: 0.9, par: 62, alpha: 0.8 },
      { seed: 47, col: "#0b0916", win: "#eab560", hCap: 0.3, base: 1.04, par: 116, alpha: 1 },
    ];
    let towers = [];
    const buildTowers = () => {
      towers = BANDS.map((b) => {
        const rnd = mulberry(b.seed), list = [];
        const wBase = 44 + b.hCap * 130, gap = 10 + b.hCap * 22;
        let x = -wBase * 2;
        while (x < W + wBase * 3) {
          const tw = wBase * (0.6 + rnd() * 0.9), th = H * b.hCap * (0.4 + rnd() * 0.6);
          const cell = 9, pad = 7, cols = Math.max(1, Math.floor((tw - pad * 2) / cell)), rows = Math.max(1, Math.floor((th - pad * 2) / cell)), lit = [];
          for (let r = 0; r < rows; r++) for (let c = 0; c < cols; c++) if (rnd() < 0.4) lit.push({ c, r, ph: rnd() * 6.28 });
          list.push({ x, tw, th, cell, pad, lit });
          x += tw + gap + rnd() * gap * 1.5;
        }
        return list;
      });
    };

    const ecol = ["#3f4a86", "#54467e", "#5a4a32", "#4a3f78"];
    let embers = [];
    const buildEmbers = () => { embers = []; const n = smallHero ? 24 : 52; for (let i = 0; i < n; i++) embers.push({ x: Math.random(), y: Math.random(), z: Math.random(), col: ecol[(Math.random() * ecol.length) | 0], ph: Math.random() * 6.28, star: Math.random() < 0.16 }); };

    const resize = () => { const r = cv.getBoundingClientRect(); W = r.width; H = r.height; cv.width = W * dpr; cv.height = H * dpr; ctx.setTransform(dpr, 0, 0, dpr, 0, 0); buildTowers(); if (!embers.length) buildEmbers(); };
    resize();
    S.ambResize = resize;
    window.addEventListener("resize", resize);

    // eased pointer parallax (off under reduced-motion / phone)
    let tx = 0, ty = 0, cxp = 0, cyp = 0;
    const onPointer = (e) => { tx = (e.clientX / window.innerWidth) * 2 - 1; ty = (e.clientY / window.innerHeight) * 2 - 1; };
    if (!reduce && !smallHero) window.addEventListener("pointermove", onPointer, { passive: true });
    S.ambPointerCleanup = () => window.removeEventListener("pointermove", onPointer);

    let t = 0;
    const tick = () => {
      if (S.dead) return;
      t += 1;
      cxp += (tx - cxp) * 0.05; cyp += (ty - cyp) * 0.05;
      ctx.clearRect(0, 0, W, H);
      const scrollNow = window.scrollY || 0, vh = window.innerHeight || H;
      const lift = Math.min(scrollNow * 0.14, H * 0.42);
      // the cityscape stays visible the whole scroll (the animation must be seen)
      // but dims to a floor past the hero so content sections keep text contrast.
      cv.style.opacity = String(Math.max(0.42, Math.min(1, 1 - (scrollNow - vh * 0.5) / (vh * 0.9))));

      // aurora sky wash
      for (const a of [{ x: W * 0.74, y: H * 0.0, c: "#93a8ff", al: 0.07 }, { x: W * 0.1, y: H * 0.22, c: "#b08cf5", al: 0.055 }]) {
        const g = ctx.createRadialGradient(a.x + cxp * 22, a.y, 0, a.x, a.y, Math.max(W, H) * 0.6);
        g.addColorStop(0, hex(a.c, a.al)); g.addColorStop(1, "rgba(0,0,0,0)");
        ctx.fillStyle = g; ctx.fillRect(0, 0, W, H);
      }

      // embers drifting up
      for (const e of embers) {
        let ey = (e.y - t * 0.00006 * (0.4 + e.z)) % 1; if (ey < 0) ey += 1;
        const yy = ey * H, xx = e.x * W + Math.sin(t * 0.01 + e.ph) * 12 + cxp * (6 + e.z * 12), tw = 0.35 + 0.65 * (0.5 + 0.5 * Math.sin(t * 0.03 + e.ph)), rad = 1 + e.z * 2.2;
        ctx.globalAlpha = (0.05 + e.z * 0.22) * tw; ctx.fillStyle = e.col;
        if (e.star) { const s = rad * 1.7; ctx.fillRect(xx - s, yy - rad * 0.4, s * 2, rad * 0.8); ctx.fillRect(xx - rad * 0.4, yy - s, rad * 0.8, s * 2); }
        else { const s = Math.max(1, Math.round(rad * 1.6)); ctx.fillRect(xx | 0, yy | 0, s, s); }
      }
      ctx.globalAlpha = 1;

      // skyline bands (far → near); near occludes far for real depth
      for (let bi = 0; bi < BANDS.length; bi++) {
        const b = BANDS[bi], list = towers[bi], baseY = H * b.base - lift * (0.25 + bi * 0.3), ox = -cxp * b.par;
        for (const tw of list) {
          const x = tw.x + ox; if (x + tw.tw < -30 || x > W + 30) continue;
          const topY = baseY - tw.th;
          ctx.globalAlpha = b.alpha; ctx.fillStyle = b.col; ctx.fillRect(x, topY, tw.tw, tw.th + 80);
          ctx.fillStyle = b.win;
          for (const w of tw.lit) { const wa = 0.22 + 0.78 * (0.5 + 0.5 * Math.sin(t * 0.02 + w.ph)); ctx.globalAlpha = b.alpha * wa * 0.85; ctx.fillRect(x + tw.pad + w.c * tw.cell, topY + tw.pad + w.r * tw.cell, tw.cell - 3, tw.cell - 3); }
        }
      }
      ctx.globalAlpha = 1;

      // dark scrim across the lower third so the install pill + cue read cleanly
      // over the skyline (like zo's vignetted foreground), then a gold horizon glow.
      const sy = H * 0.6, sc = ctx.createLinearGradient(0, sy, 0, H);
      sc.addColorStop(0, "rgba(3,3,7,0)"); sc.addColorStop(0.55, "rgba(3,3,7,.55)"); sc.addColorStop(1, "rgba(3,3,7,.9)");
      ctx.fillStyle = sc; ctx.fillRect(0, sy, W, H - sy);

      const hy = H * BANDS[1].base - lift * 0.4, hg = ctx.createLinearGradient(0, hy - 130, 0, hy + 30);
      hg.addColorStop(0, "rgba(0,0,0,0)"); hg.addColorStop(0.72, hex("#eab560", 0.05)); hg.addColorStop(1, hex("#eab560", 0.13));
      ctx.fillStyle = hg; ctx.fillRect(0, hy - 130, W, 160);

      if (!reduce) S.ambRaf = requestAnimationFrame(tick);
    };
    tick();
  };

  // Shared demo palette (Tokyo Night, matches internal/tui/theme + internal/cli/style.go).
  const cliColors = { A: "#565b80", V: "#9aa0c4", B: "#e9ebf7", G: "#8ee08a", Cy: "#7fe0ff", Au: "#eab560" };

  // Agent chat animator — Claude ordering through the real console MCP tools.
  // The tool names and their order are the ACTUAL Instamart sequence
  // (internal/mcp: im_search_products → im_update_cart → im_prepare_order →
  // place_order; prepare returns the bill, place happens only after the yes).
  const agentParts = () => {
    const { A, V, G, Cy } = cliColors;
    const P = "#b08cf5";
    const cur = '<span style="display:inline-block;width:7px;height:13px;background:#93a8ff;vertical-align:middle;animation:blink 1s step-end infinite"></span>';
    const you = (t) => '<div style="margin:0 0 14px;text-align:right"><span style="display:inline-block;background:#14162a;border:1px solid rgba(147,168,255,.16);border-radius:12px 12px 3px 12px;padding:8px 13px;color:#e9ebf7;font-size:12.5px;max-width:80%">' + t + "</span></div>";
    const tool = (name, ok) => '<div style="margin:0 0 8px;color:' + A + ';font-size:11.5px"><span style="color:' + P + '">●</span> consolestore · <span style="color:' + V + '">' + name + "</span>" + (ok ? ' <span style="color:' + G + '">✓</span>' : "") + "</div>";
    const toolBody = (h) => '<div style="margin:-2px 0 12px 14px;color:' + A + ';font-size:11.5px;line-height:1.7">' + h + "</div>";
    const bot = (t) => '<div style="margin:0 0 14px;display:flex;gap:8px;align-items:flex-start"><span style="color:' + P + ';font-size:12px;flex:none;margin-top:1px">✳</span><span style="color:#cdd3f0;font-size:12.5px;line-height:1.6">' + t + "</span></div>";
    const bill = toolBody('Red Bull Energy Drink 250 ml ×4 · Instamart<br><span style="color:' + Cy + '">to pay ₹460</span> <span style="color:' + A + '">· to Home · COD</span>');
    const ask = '₹460 to Home — four Red Bulls from Instamart. place it?';
    const done = 'placed ✓ — order 1042. I’ll keep an eye on it.';
    return { P, cur, you, tool, toolBody, bot, bill, ask, done };
  };
  const startAgent = async () => {
    const el = refs.agent;
    if (!el) return;
    const p = agentParts();
    const set = (h) => { if (el) el.innerHTML = h; };
    const typeBot = async (acc, t) => {
      for (let i = 0; i <= t.length; i++) { if (S.dead) return acc; set(acc + '<div style="margin:0 0 14px;display:flex;gap:8px;align-items:flex-start"><span style="color:' + p.P + ';font-size:12px;flex:none;margin-top:1px">✳</span><span style="color:#cdd3f0;font-size:12.5px;line-height:1.6">' + t.slice(0, i) + p.cur + "</span></div>"); await wait(16); }
      return acc + p.bot(t);
    };
    while (!S.dead) {
      let acc = "";
      set(acc = p.you("grab me an energy drink from instamart")); await wait(750);
      acc += p.tool("im_search_products", true); set(acc); await wait(620);
      acc += p.tool("im_update_cart", true); set(acc); await wait(420);
      acc += p.tool("im_prepare_order", true); set(acc); await wait(320);
      acc += p.bill; set(acc); await wait(750);
      acc = await typeBot(acc, p.ask); if (S.dead) return; set(acc); await wait(1200);
      acc += p.you("yes, go"); set(acc); await wait(650);
      acc += p.tool("place_order", true); set(acc); await wait(520);
      acc = await typeBot(acc, p.done); if (S.dead) return; set(acc);
      await wait(3400);
      set(""); await wait(520);
    }
  };
  const staticAgent = () => {
    const el = refs.agent;
    if (!el) return;
    const p = agentParts();
    el.innerHTML =
      p.you("grab me an energy drink from instamart") +
      p.tool("im_search_products", true) +
      p.tool("im_update_cart", true) +
      p.tool("im_prepare_order", true) +
      p.bill +
      p.bot(p.ask) +
      p.you("yes, go") +
      p.tool("place_order", true) +
      p.bot(p.done);
  };

  // Headless-CLI strip — `console order dinner` + `console status`, output
  // mirrored from internal/cli (render.go bill labels, order.go confirm prompt
  // and placed line, status.go block format).
  const cliParts = () => {
    const { A, V, B, G, Au } = cliColors;
    const BL = "#93a8ff";
    const line = (h) => "<div>" + h + "</div>";
    const sp = (c, t, b) => '<span style="color:' + c + (b ? ";font-weight:600" : "") + '">' + t + "</span>";
    const row = (l, r, lc, rc, b) => '<div style="display:flex;justify-content:space-between;max-width:340px"><span style="color:' + lc + (b ? ";font-weight:600" : "") + '">&nbsp;&nbsp;' + l + '</span><span style="color:' + rc + (b ? ";font-weight:600" : "") + '">' + r + "</span></div>";
    const rule = '<div style="color:#2d2f48;max-width:340px;overflow:hidden;white-space:nowrap">&nbsp;&nbsp;' + "─".repeat(40) + "</div>";
    const prompt = sp(A, "~ %") + " ";
    const cur = '<span style="display:inline-block;width:8px;height:14px;background:#93a8ff;vertical-align:middle;animation:blink 1s step-end infinite"></span>';
    const orderOut =
      line("&nbsp;&nbsp;" + sp(Au, "Meghana Foods", true) + "&nbsp;&nbsp;" + sp(A, "→") + "&nbsp;&nbsp;" + sp(A, "Home")) +
      '<div style="height:8px"></div>' +
      row("2 × Chicken Biryani", "₹398", V, B) +
      rule +
      row("item total", "₹398", A, B) +
      row("delivery", "₹29", A, B) +
      rule +
      row("to pay", "₹427", Au, G, true);
    const confirm = line(sp(G, "press Enter to place this order") + " " + sp(A, "· Ctrl-C / n to cancel"));
    const placed = line(sp(G, "✓ order placed — 999 · eta 30-40 mins"));
    const statusOut =
      line(sp(A, "order ") + sp(BL, "999") + "  " + sp(Au, "Meghana Foods", true)) +
      row("status&nbsp;&nbsp;Out for delivery", "", V, V) +
      row("eta&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;11 mins", "", V, V) +
      row("total&nbsp;&nbsp;&nbsp;₹427", "", V, V);
    return { line, sp, prompt, cur, orderOut, confirm, placed, statusOut, gap: '<div style="height:12px"></div>' };
  };
  const staticCliStrip = () => {
    const el = refs.clistrip;
    if (!el) return;
    const p = cliParts();
    el.innerHTML =
      p.line(p.prompt + p.sp("#e9ebf7", "console order ") + p.sp("#eab560", "dinner")) +
      p.orderOut + p.confirm + p.placed + p.gap +
      p.line(p.prompt + p.sp("#e9ebf7", "console status")) +
      p.statusOut;
  };
  const startCliStrip = async () => {
    const el = refs.clistrip;
    if (!el) return;
    const p = cliParts();
    const set = (h) => { if (el) el.innerHTML = h; };
    const B = "#e9ebf7", Au = "#eab560";
    const typeCmd = async (head, cmd, argAt) => {
      for (let i = 0; i <= cmd.length; i++) {
        if (S.dead) return "";
        const done = cmd.slice(0, i);
        const colored = argAt && i > argAt ? p.sp(B, done.slice(0, argAt)) + p.sp(Au, done.slice(argAt)) : p.sp(B, done);
        set(head + "<div>" + p.prompt + colored + p.cur + "</div>");
        await wait(48);
      }
      const full = argAt ? p.sp(B, cmd.slice(0, argAt)) + p.sp(Au, cmd.slice(argAt)) : p.sp(B, cmd);
      return head + "<div>" + p.prompt + full + "</div>";
    };
    while (!S.dead) {
      let acc = await typeCmd("", "console order dinner", 14); if (S.dead) return;
      await wait(420);
      acc += p.orderOut; set(acc); await wait(900);
      acc += p.confirm; set(acc); await wait(1300);
      acc += p.placed; set(acc); await wait(1600);
      acc += p.gap;
      acc = await typeCmd(acc, "console status", 0); if (S.dead) return;
      await wait(380);
      acc += p.statusOut; set(acc);
      await wait(3200);
      set(""); await wait(500);
    }
  };

  // TUI screen factory — every frame mirrors the real bubbletea app
  // (internal/tui/screens: splash.go, brand.go, rail.go, menu.go,
  // restaurant.go, checkout.go, tracking.go/ordertrack.go). Strings are
  // copied verbatim from those files — if the app's copy changes, change it here.
  const screens = () => {
    const C = { text: "#a9b1d6", item: "#9aa5c4", bright: "#e9ebf7", dim: "#565b80", faint: "#2d2f48", blue: "#93a8ff", cyan: "#7dcfff", green: "#8ee08a", gold: "#eab560", red: "#ff7d96", purple: "#b08cf5", sel: "#1a1b2e" };
    const sp = (c, t, b) => '<span style="color:' + c + (b ? ";font-weight:600" : "") + '">' + (t || "") + "</span>";
    const line = (h) => "<div>" + (h || "&nbsp;") + "</div>";
    const row = (l, r, bg) => '<div style="display:flex;justify-content:space-between;gap:24px' + (bg ? ";background:" + C.sel + ";margin:0 -10px;padding:0 10px" : "") + '">' + l + "<span>" + r + "</span></div>";
    const gap = (h) => '<div style="height:' + h + 'px"></div>';
    const secRule = (label) => '<div style="text-align:center">' + sp(C.faint, "────── ") + sp(C.gold, label) + sp(C.faint, " ──────") + "</div>";
    const dense = (h) => '<div style="font-size:11.5px;line-height:1.62">' + h + "</div>";
    // ---- top brand banner (screens/brand.go BrandBanner): brand + version,
    // deliver-to segment, cart chip, full-width gold rule ----
    const chrome = (cart) =>
      '<div style="display:flex;align-items:center;padding-bottom:5px">' +
      sp(C.blue, "▍ ", true) + sp(C.bright, "consolestore.in", true) + sp(C.purple, "&nbsp;&nbsp;v0.1.0-beta.20") +
      '<span style="margin-left:auto;padding-left:14px;flex:none">' + sp(C.dim, "deliver to ") + sp(C.green, "⊕ ") + sp(C.bright, "Home") + sp(C.faint, " ⌄") + sp(C.faint, " &nbsp;·&nbsp; ") + cart + "</span></div>" +
      '<div style="border-bottom:1px solid rgba(234,181,96,.38)"></div>';
    const cartEmpty = sp(C.dim, "🛒 cart empty");
    const cartOne = sp(C.gold, "🛒 cart · 1 · ₹139");
    // ---- FOOD / Instamart tabs (menu.go verticalTabs + keycapHint) ----
    const goldTab = (t) => '<span style="background:' + C.gold + ';color:#1a1408;font-weight:700;padding:1px 8px;border-radius:3px">' + t + "</span>";
    const tabsRow = () =>
      '<div style="display:flex;align-items:center;padding:7px 0 8px">' +
      goldTab("FOOD") + "&nbsp;&nbsp;&nbsp;&nbsp;" + sp(C.dim, "Instamart") +
      '<span style="margin-left:auto">' + sp(C.dim, "tab") + sp(C.faint, " switch") + "</span></div>";
    const hintBar = (t) => '<div style="margin-top:12px">' + sp(C.faint, t) + "</div>";
    // ---- category rail (rail.go + config.DefaultCategories, exact order) ----
    const CATS = ["Home", "Coffee", "Burgers", "Pizza", "Sandwich", "Rolls", "Momos", "North Indian", "South Indian", "Chinese", "Biryani", "Shawarma", "Cake", "Shakes"];
    const rail = (sel) => {
      const items = CATS.map((c, i) => i === sel
        ? '<div style="display:flex"><span style="color:' + C.gold + '">▌ </span>' + sp(C.bright, c, true) + "</div>"
        : '<div style="padding-left:11px">' + sp(C.item, c) + "</div>").join("");
      return '<div style="flex:none;width:118px;padding-right:12px;border-right:1px solid ' + C.faint + '">' +
        sp(C.faint, "&nbsp;explore") + '<div style="height:4px"></div>' + sp(C.dim, "⌕ Search") +
        '<div style="border-bottom:1px solid ' + C.faint + ';margin:6px 0 7px;width:82px"></div>' +
        '<div style="line-height:1.72">' + items + "</div></div>";
    };
    const twoPane = (sel, content) => '<div style="display:flex;padding-top:8px">' + rail(sel) + '<div style="flex:1;padding-left:15px;min-width:0">' + content + "</div></div>";
    const COFFEE = ["Blue Tokai Coffee Roasters", "abcoffee", "Theobroma", "Third Wave Coffee", "Chaayos", "Starbucks Coffee", "Subko Coffee Roasters", "Krispy Kreme", "McDonald's", "Chai Point"];
    const MENU = [["Hot Espresso", "5.0", "139"], ["Hot Americano", "4.9", "139"], ["Hot Cappuccino", "4.6", "149"], ["Iced Americano", "4.4", "169"], ["Hot Latte", "4.2", "159"], ["Iced Vanilla Latte", "4.4", "209"], ["Iced Irish Latte", "5.0", "209"], ["Signature Cold Coffee", "4.4", "199"], ["Strawberry Milkshake", "5.0", "199"], ["Iced Mocha", "4.3", "199"], ["Hot Flat White", "5.0", "169"]];
    // ---- splash (splash.go): prompt line, wordmark + phrase box, tagline,
    // Swiggy provenance, home buttons, hint ----
    const splash = () =>
      [
        line(sp(C.dim, "~ % ") + sp(C.text, "console") + sp(C.faint, "&nbsp;&nbsp;&nbsp;# v0.1.0-beta.20 · beta")),
        gap(14),
        '<div style="display:flex;align-items:flex-end;gap:14px;flex-wrap:wrap"><div style="font-weight:800;font-size:26px;letter-spacing:-.02em;line-height:1.1"><span style="color:#aebcff">console</span><span style="color:#eab560;font-size:.62em;vertical-align:.06em">store</span></div><span style="border:1px dashed rgba(234,181,96,.5);color:' + C.gold + ';font-size:10px;padding:2px 8px;border-radius:4px;margin-bottom:3px">✦ Real devs eat in the terminal</span></div>',
        gap(8),
        line(sp(C.dim, "coffee · food · quick snacks")),
        line(sp(C.dim, "orders fulfilled through ") + sp(C.gold, "Swiggy", true)),
        gap(14),
        line(sp(C.blue, "▌", true) + '<span style="background:' + C.sel + ';color:#e9ebf7;padding:0 7px"> enter store </span>'),
        line(sp(C.dim, "&nbsp;&nbsp;settings")),
        gap(10),
        line(sp(C.faint, "&nbsp;&nbsp;? help&nbsp;&nbsp; · &nbsp;&nbsp;q quit")),
      ].join("");
    // ---- browse (menu.go): Home = usuals + popular; category = detail strip +
    // restaurant list. Selected row = blue "▌ > " cursor on the selection bg. ----
    const placeRows = (names, selIdx) => names.map((r, i) => i === selIdx
      ? '<div style="display:flex;background:' + C.sel + ';margin:0 -8px;padding:0 8px"><span style="color:' + C.blue + '">▌ </span>' + sp("#ffffff", "> " + r, true) + "</div>"
      : '<div style="padding-left:13px">' + sp(C.item, r) + "</div>").join("");
    const browse = (catSel) => {
      let content;
      if (catSel === 0) {
        content =
          secRule("your usuals") +
          '<div style="line-height:1.68">' + placeRows(["Blue Tokai Coffee Roasters", "Meghana Foods"], 0) + "</div>" +
          '<div style="height:6px"></div>' +
          secRule("popular near you") +
          '<div style="line-height:1.68">' + placeRows(["Theobroma", "Third Wave Coffee", "Chaayos"], -1) + "</div>";
      } else {
        const detail = "<div>" + sp(C.bright, "abcoffee", true) + sp(C.faint, "&nbsp; · &nbsp;") + sp(C.gold, "4.3 ★") + sp(C.faint, "&nbsp; · &nbsp;") + sp(C.dim, "35-40 mins") + sp(C.faint, "&nbsp; · &nbsp;") + sp(C.gold, "60% OFF") + "</div>";
        content = detail + secRule("Coffee") + '<div style="line-height:1.68">' + placeRows(COFFEE, 1) + "</div>";
      }
      return dense(chrome(cartEmpty) + tabsRow() + twoPane(catSel, content) + hintBar("↑↓ move &nbsp; ↵ open &nbsp; / search &nbsp; i info &nbsp; c cart &nbsp; : cmd"));
    };
    // ---- restaurant menu (restaurant.go): esc + name header, rating · eta,
    // category bar, item rows w/ fixed rating+price columns, in-cart stepper ----
    const resto = (added) => {
      const qty = added ? 1 : 0;
      const rows = MENU.map((m, i) => {
        const rating = sp(C.gold, m[1] + " ★");
        const price = sp(C.cyan, "₹" + m[2]);
        if (i === 0) {
          const step = qty > 0 ? sp(C.red, "−") + sp(C.green, " ×" + qty + " ", true) + sp(C.green, "+") : "";
          const bar = qty > 0 ? sp(C.green, "▌ ", true) : sp(C.gold, "▌ ", true);
          return '<div style="display:flex;align-items:center;background:' + C.sel + ';margin:0 -8px;padding:1px 8px">' + bar + sp("#ffffff", "> " + m[0], true) + '<span style="margin-left:auto;display:flex;gap:13px;align-items:center">' + step + rating + price + "</span></div>";
        }
        return '<div style="display:flex;align-items:center;padding-left:13px">' + sp(C.item, m[0]) + '<span style="margin-left:auto;display:flex;gap:13px">' + rating + price + "</span></div>";
      }).join("");
      return dense(
        chrome(qty > 0 ? cartOne : cartEmpty) +
        '<div style="padding:7px 0 3px">' + sp(C.cyan, "esc") + "&nbsp;&nbsp;" + sp(C.bright, "abcoffee", true) + "</div>" +
        '<div style="padding-bottom:5px">' + sp(C.gold, "4.3 ★") + sp(C.faint, "&nbsp; · &nbsp;") + sp(C.dim, "35-40 mins") + "</div>" +
        '<div style="padding-bottom:6px"><span style="color:' + C.gold + ';text-decoration:underline;text-underline-offset:3px">All</span>' + sp(C.faint, " · ") + sp(C.dim, "99 Store") + sp(C.faint, " · ") + sp(C.dim, "Items at 169") + sp(C.faint, " · ") + sp(C.dim, "Recommended") + sp(C.faint, " ›") + "</div>" +
        '<div style="line-height:1.7">' + rows + "</div>" +
        hintBar("↑↓ move &nbsp; ↵/+ add &nbsp; − remove &nbsp; ←→ category &nbsp; / search &nbsp; c cart &nbsp; esc back")
      );
    };
    // ---- merged cart/checkout (checkout.go + cart.go renderBill) ----
    const checkout = () =>
      dense(
        chrome(cartOne) +
        '<div style="padding:8px 0 2px">' + sp(C.bright, "checkout", true) + sp(C.faint, " &nbsp;·&nbsp; ") + sp(C.dim, "abcoffee") + "</div>" +
        '<div style="padding-bottom:6px">' + sp(C.dim, "deliver to ") + sp(C.text, "FD 46 Enclave") + sp(C.faint, " &nbsp;·&nbsp; ") + sp(C.dim, "Home") + "</div>" +
        row(sp(C.green, "▌ ", true) + sp(C.bright, "Hot Espresso", true) + "&nbsp;&nbsp;" + sp(C.red, "−") + sp(C.green, " ×1 ", true) + sp(C.green, "+"), sp(C.text, "₹139"), true) +
        gap(6) +
        line(sp(C.faint, "╌".repeat(34))) +
        row(sp(C.dim, "item total"), sp(C.text, "₹139")) +
        row(sp(C.dim, "delivery"), sp(C.text, "₹29")) +
        row(sp(C.dim, "taxes &amp; charges"), sp(C.text, "₹18")) +
        line(sp(C.faint, "╌".repeat(34))) +
        row(sp(C.bright, "to pay", true), sp(C.cyan, "₹186", true)) +
        gap(10) +
        line(sp(C.green, "▌", true) + '<span style="background:' + C.sel + ';color:#e9ebf7;padding:0 7px"> ❯ place order </span>') +
        gap(4) +
        line(sp(C.gold, "pay the rider — cash / UPI") + sp(C.faint, " &nbsp;·&nbsp; ") + sp(C.dim, "can\'t cancel once placed")) +
        hintBar("↑↓ move &nbsp; ←→ qty &nbsp; ⌫ remove &nbsp; ↵ place order &nbsp; esc back")
      );
    // ---- placed confirmation (checkout.go confirm view, incl. the speed receipt) ----
    const confirm = () =>
      dense(
        chrome(cartEmpty) +
        gap(12) +
        line(sp(C.green, "╔══════════════════════╗")) +
        line(sp(C.green, "║ &nbsp;&nbsp;order placed&nbsp; ✓ &nbsp;&nbsp; ║")) +
        line(sp(C.green, "╚══════════════════════╝")) +
        gap(8) +
        line(sp(C.text, "abcoffee") + sp(C.faint, " · ") + sp(C.dim, "ETA 25-30 min") + sp(C.faint, " · ") + sp(C.dim, "999")) +
        gap(10) +
        line(sp(C.gold, "⚡ ordered in 2.1s · 4 keystrokes")) +
        line(sp(C.dim, "this session best 1.8s &nbsp;·&nbsp; phone app ~45s")) +
        gap(12) +
        line(sp(C.faint, "↵ track &nbsp;&nbsp;&nbsp;&nbsp; esc back to menu"))
      );
    // ---- live tracking (tracking.go: rider sprite on the road + TrackStages +
    // StatusDisplay friendly phrases) ----
    const track = (step) => {
      const stages = ["order confirmed", "preparing", "out for delivery", "delivered"];
      const phrase = ["order confirmed", "kitchen\'s on it — preparing your order", "on the way to you · ~12 min", "delivered ✓ · enjoy your order!"][step];
      const N = 26;
      const pos = Math.min(N - 2, Math.round((step / 3) * (N - 2)));
      const wheels = ["◐◓", "◓◑", "◑◒", "◒◐"][step];
      const pad = (n) => "&nbsp;".repeat(Math.max(0, n));
      const stageRows = stages.map((s, i) => {
        let mark, col;
        if (i < step) { mark = "●"; col = C.green; }
        else if (i === step) { mark = "◐"; col = C.gold; }
        else { mark = "○"; col = C.faint; }
        return line(sp(col, mark + " ") + sp(i <= step ? C.bright : C.dim, s));
      }).join("");
      return dense(
        chrome(cartEmpty) +
        '<div style="padding:8px 0 2px;display:flex;justify-content:space-between">' + sp(C.cyan, "← tracking · 999") + sp(C.dim, "abcoffee") + "</div>" +
        '<div style="display:flex;justify-content:space-between">' + sp(C.gold, "abcoffee") + sp(C.cyan, "Home") + "</div>" +
        gap(10) +
        line(pad(pos + 2) + sp(C.text, "_o")) +
        line(pad(pos + 1) + sp(C.text, "-\\&lt;") + sp(C.faint, " ~&nbsp;~")) +
        line(sp(C.green, "═".repeat(pos)) + sp(C.gold, wheels) + sp(C.faint, "─".repeat(Math.max(0, N - pos - 2)))) +
        gap(10) +
        stageRows +
        gap(8) +
        line(sp(C.dim, step >= 3 ? "status&nbsp; " : "ETA&nbsp; ") + sp(step >= 3 ? C.green : C.gold, phrase)) +
        line(sp(C.faint, "rider contact &amp; live map → open the Swiggy app")) +
        hintBar("d done &nbsp; esc back to menu")
      );
    };
    return { splash, browse, resto, checkout, confirm, track };
  };

  const setKey = (label) => { if (refs.key) refs.key.textContent = label; };
  const staticTerminal = () => { if (refs.term) refs.term.innerHTML = screens().browse(1); };
  const startTerminal = async () => {
    const el = refs.term;
    if (!el) return;
    const Sc = screens();
    const set = (html) => { if (el) el.innerHTML = html; };
    const typeCmd = async () => {
      const cmd = "console";
      for (let i = 0; i <= cmd.length; i++) { if (S.dead) return; set('<div><span style="color:#565b80">~ % </span><span style="color:#a9b1d6">' + cmd.slice(0, i) + '</span><span style="display:inline-block;width:8px;height:14px;background:#93a8ff;vertical-align:middle;animation:blink 1s step-end infinite"></span></div>'); await wait(58); }
      await wait(420);
    };
    while (!S.dead) {
      setKey("run"); await typeCmd(); if (S.dead) return;
      set(Sc.splash()); setKey("boot"); await wait(2300); if (S.dead) return;
      setKey("↵ enter store"); set(Sc.browse(0)); await wait(1400); if (S.dead) return;
      setKey("↓ Coffee"); set(Sc.browse(1)); await wait(1600); if (S.dead) return;
      setKey("↵ open"); set(Sc.resto(false)); await wait(1400); if (S.dead) return;
      setKey("+ add"); set(Sc.resto(true)); await wait(1500); if (S.dead) return;
      setKey("c cart"); set(Sc.checkout()); await wait(2400); if (S.dead) return;
      setKey("↵ place"); set(Sc.confirm()); await wait(2000); if (S.dead) return;
      setKey("↵ track");
      for (let st = 0; st <= 3; st++) { if (S.dead) return; set(Sc.track(st)); await wait(900); }
      setKey("✓ delivered"); await wait(1800); if (S.dead) return;
    }
  };

  // ===== HERO ASCII WORDMARK (crisp text glyphs, left-to-right scramble assembly) =====
  const showWordmarkFallback = () => {
    const cv = refs.hero3d, wm = refs.wordmark;
    if (cv) cv.style.display = "none";
    if (wm) wm.style.display = "flex";
  };
  const startHeroAscii = () => {
    const wrap = refs.hero3dwrap, canvas = refs.hero3d;
    if (!wrap || !canvas) return false;
    let W = wrap.clientWidth, H = wrap.clientHeight;
    if (!W || !H) return false;
    const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
    const ctx = canvas.getContext("2d");
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const FONT = '"JetBrains Mono", ui-monospace, monospace';
    const GLYPHS = "consolestore/<>_-=+:;.()[]{}$#%&*\\|!?~^01abcdefghijklmnopqrstuvwxyz".split("");
    const hash = (n) => { const s = Math.sin(n) * 43758.5453; return s - Math.floor(s); };

    let cells = [], cellSz = 12, glyphPx = 16;

    const build = () => {
      W = wrap.clientWidth; H = wrap.clientHeight;
      if (!W || !H) return;
      canvas.width = Math.round(W * dpr);
      canvas.height = Math.round(H * dpr);
      // ---- render the wordmark to an offscreen mask ----
      const off = document.createElement("canvas"); off.width = W; off.height = H;
      const o = off.getContext("2d");
      o.textBaseline = "alphabetic";
      o.font = "800 100px " + FONT;
      const cw100 = o.measureText("console").width / 100;
      const sw100 = o.measureText("store").width / 100;
      const STORE = 0.58, GAPF = 0.05;
      const perFs = cw100 + GAPF + sw100 * STORE;
      const fsC = (W * 0.93) / perFs;
      const fsS = fsC * STORE;
      o.font = "800 " + fsC + "px " + FONT;
      const cwidth = o.measureText("console").width;
      o.font = "800 " + fsS + "px " + FONT;
      const swidth = o.measureText("store").width;
      const gap = fsC * GAPF;
      const total = cwidth + gap + swidth;
      const startX = (W - total) / 2;
      const baseline = H / 2 + fsC * 0.35;
      const cg = o.createLinearGradient(0, baseline - fsC * 0.7, 0, baseline);
      cg.addColorStop(0, "#b8c4ff"); cg.addColorStop(0.5, "#a6a3f7"); cg.addColorStop(1, "#bd9bf7");
      o.font = "800 " + fsC + "px " + FONT; o.fillStyle = cg; o.fillText("console", startX, baseline);
      o.font = "800 " + fsS + "px " + FONT; o.fillStyle = "#eab560"; o.fillText("store", startX + cwidth + gap, baseline);
      const data = o.getImageData(0, 0, W, H).data;

      // ---- sample into a grid of character cells ----
      // Denser grid = higher-resolution wordmark: smaller cells pack more
      // character glyphs into each letter so the strokes read solid instead of
      // scattered. Glyph slightly larger than the cell so neighbours overlap
      // and close the gaps between samples.
      cellSz = clamp(fsC * 0.05, 6, 10);
      glyphPx = cellSz * 1.55;
      cells = [];
      let minX = Infinity, maxX = -Infinity;
      const off2 = Math.floor(cellSz / 2);
      const rows = Math.floor((H - off2) / cellSz);
      const colsN = Math.floor((W - off2) / cellSz);
      for (let ry = 0; ry <= rows; ry++) {
        for (let rx = 0; rx <= colsN; rx++) {
          const gx = Math.round(off2 + rx * cellSz);
          const gy = Math.round(off2 + ry * cellSz);
          if (gx >= W || gy >= H) continue;
          const i = (gy * W + gx) * 4;
          if (data[i + 3] > 90) {
            const br = 1.18;
            const r = Math.min(255, data[i] * br) | 0;
            const g = Math.min(255, data[i + 1] * br) | 0;
            const b = Math.min(255, data[i + 2] * br) | 0;
            const sd = hash(gx * 12.9 + gy * 78.2) * 1000;
            cells.push({ x: gx, y: gy, color: "rgb(" + r + "," + g + "," + b + ")", gi: Math.floor(hash(sd) * GLYPHS.length), seed: sd, baseA: 0.62 + hash(sd * 1.3) * 0.38 });
            if (gx < minX) minX = gx;
            if (gx > maxX) maxX = gx;
          }
        }
      }
      const span = maxX - minX || 1;
      for (const c of cells) c.nx = (c.x - minX) / span;
    };

    build();
    if (!cells.length) return false;

    ctx.textAlign = "center";
    ctx.textBaseline = "middle";

    let assembled = false, hraf = 0, needRebuild = false;
    const onResize = () => { needRebuild = true; };
    window.addEventListener("resize", onResize);
    // If the hero started on the fallback mono metrics (fonts hadn't loaded
    // yet), let boot ask us to re-measure once JetBrains Mono finishes so the
    // wordmark settles into the real glyph shape.
    S.heroRebuild = () => {
      needRebuild = true;
      if (assembled) { needRebuild = false; build(); ctx.textAlign = "center"; ctx.textBaseline = "middle"; paint(DONE); }
    };

    const SWEEP = 1500, SNAP = 420, DONE = SWEEP + SNAP + 160;
    const t0 = performance.now();
    const paint = (T) => {
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, W, H);
      ctx.font = "600 " + glyphPx + "px " + FONT;
      const fr = Math.floor(T / 70);
      const intro = clamp(T / 260, 0, 1);
      for (let k = 0; k < cells.length; k++) {
        const c = cells[k];
        const lockT = c.nx * SWEEP;
        if (T < lockT) continue;
        const p = clamp((T - lockT) / SNAP, 0, 1);
        let gx, gy, glyph, a;
        if (p >= 1) { glyph = GLYPHS[c.gi]; gx = c.x; gy = c.y; a = c.baseA; }
        else {
          glyph = GLYPHS[(c.gi + fr + k) % GLYPHS.length];
          const jit = (1 - p) * cellSz * 1.1;
          gx = c.x + (hash(c.seed + fr) - 0.5) * jit;
          gy = c.y + (hash(c.seed * 1.7 + fr) - 0.5) * jit;
          a = c.baseA * (0.35 + 0.65 * p);
        }
        ctx.globalAlpha = a * intro;
        ctx.fillStyle = c.color;
        ctx.fillText(glyph, gx, gy);
      }
      ctx.globalAlpha = 1;
    };

    const tick = () => {
      if (S.dead) return;
      if (needRebuild) { needRebuild = false; build(); ctx.textAlign = "center"; ctx.textBaseline = "middle"; }
      const T = performance.now() - t0;
      paint(T);
      if (T >= DONE && !needRebuild) { assembled = true; paint(DONE); return; }
      hraf = requestAnimationFrame(tick);
    };
    tick();

    const settleWatch = setInterval(() => {
      if (S.dead) { clearInterval(settleWatch); return; }
      if (assembled && needRebuild) { needRebuild = false; build(); ctx.textAlign = "center"; ctx.textBaseline = "middle"; paint(DONE); }
    }, 200);
    S.timers.push(settleWatch);

    // Watchdog: if the rAF assembly hasn't settled within a few seconds (e.g.
    // the tab was backgrounded and rAF was throttled, or the loop stalled),
    // fall back to the static styled wordmark so visitors always see the brand.
    const watchdog = setTimeout(() => {
      if (!S.dead && !assembled) { showWordmarkFallback(); startWordmark(); }
    }, 3500);
    S.timers.push(watchdog);

    S.heroCleanup = () => {
      cancelAnimationFrame(hraf);
      clearInterval(settleWatch);
      clearTimeout(watchdog);
      window.removeEventListener("resize", onResize);
    };
    return true;
  };

  // ---- waitlist: hosted / Claude-web access capture. POSTs the email to
  // /event/waitlist (rate-limited, stored in Postgres) and shows an inline
  // status. Progressive enhancement: without JS the <form> simply does nothing. ----
  const initWaitlist = () => {
    const form = root.querySelector("[data-waitlist]");
    if (!form) return;
    const input = form.querySelector("[data-waitlist-email]");
    const btn = form.querySelector("[data-waitlist-submit]");
    const msg = root.querySelector("[data-waitlist-msg]");
    const say = (t, ok) => { if (msg) { msg.textContent = t; msg.style.color = ok ? "#8ee08a" : "#ff7d96"; } };
    const onSubmit = async (e) => {
      e.preventDefault();
      const email = (input && input.value ? input.value : "").trim();
      if (!email) return;
      if (btn) { btn.disabled = true; btn.textContent = "…"; }
      try {
        const r = await fetch("/event/waitlist", {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ email, source: "landing" }),
        });
        if (r.ok) { say("you're on the list — we'll email you when web access opens.", true); if (input) input.value = ""; }
        else if (r.status === 400) say("that doesn't look like a valid email.", false);
        else if (r.status === 429) say("one sec — try again in a moment.", false);
        else say("something went wrong — try again later.", false);
      } catch (err) { say("network hiccup — try again.", false); }
      if (btn) { btn.disabled = false; btn.textContent = "join waitlist"; }
    };
    form.addEventListener("submit", onSubmit);
    S.waitlistCleanup = () => form.removeEventListener("submit", onSubmit);
  };

  // boot
  if (refs.toast) refs.toast.style.display = "none";
  initReveal();
  initFaq();
  initStats();
  initWaitlist();

  // Brand lockup → back to the very top of the page (y=0, so the whole hero +
  // nav are in frame). Overrides the #top anchor, which would otherwise land a
  // hair below the sticky nav. The scramble replay is wired separately in
  // startWordmark(), and still fires on the same click.
  if (refs.hero3dwrap) {
    const toTop = (e) => {
      e.preventDefault();
      window.scrollTo({ top: 0, behavior: reduce ? "auto" : "smooth" });
    };
    refs.hero3dwrap.addEventListener("click", toTop);
    wmHandlers.push([refs.hero3dwrap, toTop]);
  }
  // Auto-scroll assistance removed by request: no keyboard section-paging, no
  // scroll-snap, no enter-to-explore jump. The page is a plain free-scroll page.
  // (initKeyboardNav / initScrollSnap intentionally not called.)

  if (!reduce) {
    initFooterWordmark();
    startAmbient();
    startTerminal();
    startCliStrip();
    startAgent();
    const fontsReady = document.fonts && document.fonts.ready ? document.fonts.ready : Promise.resolve();
    const bail = () => { showWordmarkFallback(); startWordmark(); };
    const tryHero = (n) => {
      if (S.dead) return;
      const w = refs.hero3dwrap;
      if (w && w.clientWidth > 1 && w.clientHeight > 1) {
        let ok = false;
        try { ok = startHeroAscii(); } catch (e) { console.warn("hero ascii failed", e); ok = false; }
        if (!ok) bail();
        return;
      }
      if (n > 40) { bail(); return; }
      S.raf = requestAnimationFrame(() => tryHero(n + 1));
    };
    // Start the hero as soon as fonts are ready — but never hang on it. Some
    // environments leave document.fonts.ready pending indefinitely, which would
    // otherwise mean the wordmark never appears. Race it against a short timeout
    // so the hero always starts; if JetBrains Mono lands later we re-measure.
    if (smallHero || !refs.hero3d) {
      // No hero canvas to measure (phone width, or the brand is the nav
      // wordmark lockup with no canvas) — reveal + scramble the styled
      // wordmark right away instead of waiting on font metrics.
      bail();
    } else {
      let started = false;
      const startHero = () => { if (started || S.dead) return; started = true; tryHero(0); };
      const fontTimeout = new Promise((res) => S.timers.push(setTimeout(res, 1500)));
      Promise.race([fontsReady, fontTimeout]).then(startHero);
      fontsReady.then(() => { if (!S.dead && S.heroRebuild) S.heroRebuild(); });
    }
  } else {
    showWordmarkFallback();
    startWordmark();
    staticTerminal();
    staticCliStrip();
    staticAgent();
  }

  return () => {
    S.dead = true;
    cancelAnimationFrame(S.raf);
    cancelAnimationFrame(S.ambRaf);
    S.timers.forEach(clearTimeout);
    if (S.ambResize) window.removeEventListener("resize", S.ambResize);
    if (S.ambPointerCleanup) S.ambPointerCleanup();
    copyEls.forEach((el) => el.removeEventListener("click", copyInstall));
    if (S.shareCleanup) S.shareCleanup();
    faqHandlers.forEach(([q, h]) => q.removeEventListener("click", h));
    wmHandlers.forEach(([el, h]) => el.removeEventListener("click", h));
    scrambleObservers.forEach((io) => io.disconnect());
    if (S.heroCleanup) S.heroCleanup();
    if (S.statsCleanup) S.statsCleanup();
    if (S.waitlistCleanup) S.waitlistCleanup();
  };
}
