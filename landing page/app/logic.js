// Behaviour ported from Claude Design "Console Store Landing.dc.html".
// The DCLogic React class is rewritten as a framework-free mount(root) that
// returns a cleanup fn. Canvas + animation code is kept verbatim: ambient
// particle field, the hero ASCII-cell wordmark (left-to-right scramble
// assembly), the scramble-reveal footer wordmark, the live TUI terminal, the
// animated headless CLI, and the scroll-driven TUI/CLI toggle.

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
    herovid: root.querySelector('[data-ref="herovid"]'),
    hero3dwrap: root.querySelector('[data-ref="hero3dwrap"]'),
    hero3d: root.querySelector('[data-ref="hero3d"]'),
    wordmark: root.querySelector('[data-ref="wordmark"]'),
    footwm: root.querySelector('[data-ref="footwm"]'),
    term: root.querySelector('[data-ref="term"]'),
    key: root.querySelector('[data-ref="key"]'),
    palette: root.querySelector('[data-ref="palette"]'),
    cli: root.querySelector('[data-ref="cli"]'),
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

  // ===== TUI / CLI scroll-driven toggle =====
  let cleanupKeys = () => {};
  (() => {
    const section = root.querySelector("#keys");
    const wrap = root.querySelector("[data-panel-wrap]");
    const ind = root.querySelector("[data-toggle-ind]");
    const hint = root.querySelector("[data-keys-hint]");
    const btn = { tui: root.querySelector('[data-toggle="tui"]'), cli: root.querySelector('[data-toggle="cli"]') };
    const panel = { tui: root.querySelector('[data-panel="tui"]'), cli: root.querySelector('[data-panel="cli"]') };
    if (!section || !wrap || !panel.tui || !panel.cli) return;
    const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
    const smooth = (x, a, b) => { const u = clamp((x - a) / (b - a), 0, 1); return u * u * (3 - 2 * u); };
    const placeInd = (t) => {
      if (!ind || !btn.tui || !btn.cli) return;
      ind.style.width = btn.tui.offsetWidth + "px";
      ind.style.transform = "translateX(" + (btn.cli.offsetLeft - btn.tui.offsetLeft) * t + "px)";
    };
    const colorBtns = (which) => {
      if (btn.tui) btn.tui.style.color = which === "tui" ? "#e9ebf7" : "#565b80";
      if (btn.cli) btn.cli.style.color = which === "cli" ? "#e9ebf7" : "#565b80";
    };
    const desktop = typeof window !== "undefined" && window.matchMedia && window.matchMedia("(min-width: 821px)").matches;
    if (!desktop) {
      const show = (which) => {
        panel.tui.style.display = which === "tui" ? "block" : "none";
        panel.cli.style.display = which === "cli" ? "block" : "none";
        placeInd(which === "cli" ? 1 : 0);
        colorBtns(which);
      };
      const handlers = [];
      Object.keys(btn).forEach((k) => { if (!btn[k]) return; const h = () => show(k); btn[k].addEventListener("click", h); handlers.push([btn[k], h]); });
      show("tui");
      requestAnimationFrame(() => placeInd(0));
      if (hint) hint.textContent = "tap to switch — order it yourself at the prompt, or hand it to your agent.";
      cleanupKeys = () => handlers.forEach(([b, h]) => b.removeEventListener("click", h));
      return;
    }
    wrap.style.position = "relative";
    [panel.tui, panel.cli].forEach((p) => {
      p.style.position = "absolute";
      p.style.top = "0";
      p.style.left = "0";
      p.style.right = "0";
      p.style.display = "block";
      p.style.transition = "opacity .25s ease, transform .25s ease";
      p.style.willChange = "opacity, transform";
    });
    const sizeWrap = () => { wrap.style.height = Math.max(panel.tui.offsetHeight, panel.cli.offsetHeight) + "px"; };
    const apply = () => {
      const total = section.offsetHeight - window.innerHeight;
      const passed = -section.getBoundingClientRect().top;
      const p = total > 0 ? clamp(passed / total, 0, 1) : 0;
      const t = smooth(p, 0.35, 0.65);
      panel.tui.style.opacity = String(1 - t);
      panel.tui.style.transform = "translateY(" + -16 * t + "px)";
      panel.tui.style.pointerEvents = t < 0.5 ? "auto" : "none";
      panel.cli.style.opacity = String(t);
      panel.cli.style.transform = "translateY(" + 16 * (1 - t) + "px)";
      panel.cli.style.pointerEvents = t >= 0.5 ? "auto" : "none";
      placeInd(t);
      colorBtns(t < 0.5 ? "tui" : "cli");
    };
    sizeWrap();
    const onResize = () => { sizeWrap(); apply(); };
    window.addEventListener("scroll", apply, { passive: true });
    window.addEventListener("resize", onResize);
    const clickHandlers = [];
    const scrollToPhase = (which) => {
      const total = section.offsetHeight - window.innerHeight;
      const top = section.getBoundingClientRect().top + window.scrollY;
      window.scrollTo({ top: top + total * (which === "cli" ? 0.82 : 0.16), behavior: "smooth" });
    };
    Object.keys(btn).forEach((k) => { if (!btn[k]) return; const h = () => scrollToPhase(k); btn[k].addEventListener("click", h); clickHandlers.push([btn[k], h]); });
    apply();
    cleanupKeys = () => {
      window.removeEventListener("scroll", apply);
      window.removeEventListener("resize", onResize);
      clickHandlers.forEach(([b, h]) => b.removeEventListener("click", h));
    };
  })();

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

  // ---- scroll nudge: fade it out once scrolling starts (fallback for browsers
  // without scroll-driven CSS timelines; supporting ones fade via @supports) ----
  const initNudge = () => {
    const nudge = root.querySelector("[data-scroll-nudge]");
    if (!nudge) return;
    const supported = typeof CSS !== "undefined" && CSS.supports && CSS.supports("animation-timeline: scroll()");
    if (supported) return;
    const onScroll = () => { nudge.style.transition = "opacity .3s"; nudge.style.opacity = window.scrollY > 140 ? "0" : "1"; };
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    S.nudgeCleanup = () => window.removeEventListener("scroll", onScroll);
  };

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

  // command-palette click-to-type
  let setPaletteOverride = null;
  const cmdHandlers = [];
  const initCmdClicks = () => {
    root.querySelectorAll("[data-cmd]").forEach((rowEl) => {
      const name = rowEl.getAttribute("data-cmd-name");
      if (!name) return;
      rowEl.style.cursor = "pointer";
      const h = () => { if (setPaletteOverride) setPaletteOverride(name); };
      rowEl.addEventListener("click", h);
      cmdHandlers.push([rowEl, h]);
    });
  };

  // The pitch "feature map" window: clicking flies into it — the window scales
  // up and the rest of the pitch fades to the starfield — then lands on
  // /features. Reduced-motion (or missing element) just follows the link.
  const initFeaturesZoom = () => {
    const win = root.querySelector('[data-action="features-zoom"]');
    if (!win) return;
    const href = win.getAttribute("href") || "/features";
    const pitch = root.querySelector("#pitch");
    // Clearing the zoom state is critical for the browser Back button: when the
    // page is restored from the back-forward cache, its DOM comes back with the
    // `.zooming` classes still applied — the window scaled away and the pitch at
    // opacity 0 — leaving the whole section blank. pageshow fires on that restore.
    const resetZoom = () => { win.classList.remove("zooming"); if (pitch) pitch.classList.remove("zooming"); };
    const h = (e) => {
      if (reduce || e.metaKey || e.ctrlKey || e.shiftKey || e.button === 1) return; // let the browser handle new-tab / reduced-motion
      e.preventDefault();
      if (pitch) pitch.classList.add("zooming");
      win.classList.add("zooming");
      S.timers.push(setTimeout(() => { if (!S.dead) window.location.assign(href); }, 560));
    };
    win.addEventListener("click", h);
    window.addEventListener("pageshow", resetZoom);
    cmdHandlers.push([win, h]);
    S.featZoomCleanup = () => window.removeEventListener("pageshow", resetZoom);
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

  // ---- keyboard navigation: terminal-style section paging. ↑/↓ (or PageUp/Down)
  // step between sections; ↵ / Space advances; Tab toggles the live-stats drawer;
  // Home/End jump to ends. Reinforces the keyboard-driven product. The "press ↵"
  // cue + the bottom legend teach it; both fade once the visitor starts driving. ----
  const navHandlers = [];
  const initKeyboardNav = () => {
    const sections = Array.from(
      root.querySelectorAll("#top, #pitch, #run, #keys, #features, #why, #faq, footer")
    );
    if (!sections.length) return;
    const cue = root.querySelector('[data-ref="enterCue"]');
    const legend = root.querySelector('[data-ref="keyhint"]');
    let used = false;
    const markUsed = () => {
      if (used || S.dead) return;
      used = true;
      if (cue) cue.classList.add("spent");
      if (legend) legend.classList.add("dim");
    };
    // Stops = the Y positions ↑/↓ snap to. A short section is CENTERED in the
    // viewport; a full-height / tall one is top-aligned; the #keys scrolly gets
    // TWO stops (TUI at 16% through it, CLI at 82%) so arrows step through its
    // transition instead of skipping it. Rebuilt on resize / after layout settles.
    // Absolute document top from layout (offsetTop chain) — unaffected by the
    // `data-reveal` scroll-timeline transforms, unlike getBoundingClientRect.
    const docTop = (el) => {
      let y = 0, n = el;
      while (n) { y += n.offsetTop; n = n.offsetParent; }
      return y;
    };
    const buildStops = () => {
      const vh = window.innerHeight;
      const maxY = Math.max(0, document.documentElement.scrollHeight - vh);
      const clampY = (y) => Math.max(0, Math.min(maxY, Math.round(y)));
      const raw = [];
      sections.forEach((s, idx) => {
        const topY = docTop(s);
        const h = s.offsetHeight;
        if (idx === 0) {
          raw.push(0); // hero → the very top, so the nav stays in frame
        } else if (s.id === "keys") {
          const total = Math.max(1, h - vh);
          raw.push(clampY(topY + total * 0.16)); // TUI view
          raw.push(clampY(topY + total * 0.82)); // CLI view
        } else if (h >= vh * 0.82) {
          raw.push(clampY(topY)); // near-full / tall section → align top
        } else {
          raw.push(clampY(topY - (vh - h) / 2)); // short section → center it
        }
      });
      raw.sort((a, b) => a - b);
      const stops = [];
      raw.forEach((y) => { if (!stops.length || y - stops[stops.length - 1] > 6) stops.push(y); });
      return stops;
    };
    let stops = buildStops();
    const rebuild = () => { stops = buildStops(); };
    // goY drives every programmatic jump (keys + scroll-snap settle). It marks a
    // short suppression window so the wheel/touch scroll-snap doesn't fire on the
    // smooth-scroll churn it itself produces. Exposed on S so initScrollSnap reuses
    // the exact same stop model (centered short / top-aligned tall / two #keys stops).
    const goY = (y) => {
      S.suppressSnapUntil = performance.now() + 700;
      window.scrollTo({ top: y, behavior: reduce ? "auto" : "smooth" });
    };
    S.navStops = () => stops;
    S.navGoY = goY;
    const nextStop = () => {
      const y = window.scrollY + 6;
      const t = stops.find((s) => s > y);
      goY(t != null ? t : stops[stops.length - 1]);
    };
    const prevStop = () => {
      const y = window.scrollY - 6;
      let t = stops[0];
      for (const s of stops) { if (s < y) t = s; }
      goY(t);
    };
    const onKey = (e) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      const t = e.target || {};
      if (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable) return;
      if (e.key === "Tab") {
        e.preventDefault();
        if (S.statsIsOpen && S.statsIsOpen()) S.closeStats && S.closeStats();
        else S.openStats && S.openStats();
        markUsed();
        return;
      }
      if (S.statsIsOpen && S.statsIsOpen()) return; // drawer owns keys while open (Esc closes)
      switch (e.key) {
        case "ArrowDown":
        case "PageDown":
        case "Enter":
        case " ":
          e.preventDefault(); nextStop(); markUsed(); break;
        case "ArrowUp":
        case "PageUp":
          e.preventDefault(); prevStop(); markUsed(); break;
        case "Home":
          e.preventDefault(); goY(stops[0]); markUsed(); break;
        case "End":
          e.preventDefault(); goY(stops[stops.length - 1]); markUsed(); break;
      }
    };
    document.addEventListener("keydown", onKey);
    navHandlers.push([document, "keydown", onKey]);
    // Recompute stops when layout changes (resize) and once more after the hero /
    // fonts settle, since section offsets shift during the opening animation.
    const onResize = () => rebuild();
    window.addEventListener("resize", onResize);
    navHandlers.push([window, "resize", onResize]);
    S.timers.push(setTimeout(rebuild, 1200));
    S.timers.push(setTimeout(rebuild, 3200));
    // Mouse/touch affordances: the cue advances, the legend's stats chip opens it.
    if (cue) {
      const h = () => { nextStop(); markUsed(); };
      cue.addEventListener("click", h);
      navHandlers.push([cue, "click", h]);
    }
    const statBtn = legend && legend.querySelector("[data-open-stats]");
    if (statBtn) {
      const h = () => { if (S.openStats) S.openStats(); markUsed(); };
      statBtn.addEventListener("click", h);
      navHandlers.push([statBtn, "click", h]);
    }
    // fade the cue + legend in once the hero has settled (transition-driven so the
    // later .spent/.dim class swaps aren't fighting a held keyframe).
    if (cue) S.timers.push(setTimeout(() => { if (!S.dead) cue.classList.add("show"); }, reduce ? 200 : 1700));
    if (legend) S.timers.push(setTimeout(() => { if (!S.dead) legend.classList.add("show"); }, reduce ? 250 : 2100));
    S.navCleanup = () => navHandlers.forEach(([el, ev, h]) => el.removeEventListener(ev, h));
  };

  // ---- gentle scroll-snap: free-scroll settles onto the SAME section stops the
  // keyboard nav uses. We never preventDefault the wheel/touch (that's the janky
  // path) — native momentum runs, then ~150ms after the gesture stops we ease to
  // the nearest stop *only if it's within ~0.62vh* (proximity, not mandatory: you
  // can still rest mid-way through tall content without being yanked). The #keys
  // 180vh scrolly is handled for free — its TUI/CLI stops are part of the model,
  // so a scroll there settles onto one phase instead of stranding mid-transition.
  // Disabled under reduced-motion. ----
  const initScrollSnap = () => {
    if (reduce) return;
    let idleTimer = 0;
    const proximity = () => window.innerHeight * 0.62;
    const snap = () => {
      if (S.dead) return;
      if (performance.now() < (S.suppressSnapUntil || 0)) return;
      if (S.statsIsOpen && S.statsIsOpen()) return; // drawer open → leave the page alone
      const stops = S.navStops ? S.navStops() : null;
      if (!stops || !stops.length) return;
      const y = window.scrollY;
      let best = stops[0], bd = Math.abs(stops[0] - y);
      for (const s of stops) { const d = Math.abs(s - y); if (d < bd) { bd = d; best = s; } }
      if (bd <= 3) return;            // already parked on a stop
      if (bd > proximity()) return;   // mid long-content → don't yank
      (S.navGoY || ((t) => window.scrollTo({ top: t, behavior: "smooth" })))(best);
    };
    const onScroll = () => {
      if (performance.now() < (S.suppressSnapUntil || 0)) return; // our own smooth-scroll
      clearTimeout(idleTimer);
      idleTimer = setTimeout(snap, 150); // settle once the gesture stops
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    S.snapCleanup = () => { window.removeEventListener("scroll", onScroll); clearTimeout(idleTimer); };
  };

  // Full-bleed cinematic hero video. Fades in only once real frame data lands
  // (canplay/loadeddata), then stops the starfield rAF to save CPU. If the asset
  // is missing / errors / doesn't decode within the grace window it hides itself
  // and the ambient starfield stays as the fallback. Reduced-motion shows the
  // poster frame paused (no playback), never black.
  const startHeroVideo = () => {
    const v = refs.herovid;
    if (!v) return;
    if (reduce) v.removeAttribute("autoplay");
    let shown = false;
    const reveal = () => {
      if (shown || S.dead) return;
      shown = true;
      v.style.opacity = "1";
      cancelAnimationFrame(S.ambRaf); // video is the backdrop now — drop the starfield
      if (reduce) { try { v.pause(); } catch (e) {} }
    };
    const fail = () => { if (shown) return; v.style.display = "none"; }; // keep starfield
    v.addEventListener("loadeddata", () => { if (v.readyState >= 2) reveal(); });
    v.addEventListener("canplay", reveal);
    v.addEventListener("error", fail);
    v.querySelectorAll("source").forEach((s) => s.addEventListener("error", () => { if (v.networkState === 3 /* NO_SOURCE */) fail(); }));
    // no event fires when every source 404s on some browsers — bail after a grace window.
    S.timers.push(setTimeout(() => { if (!shown && !S.dead) fail(); }, 2600));
  };

  // ambient particle field
  const startAmbient = () => {
    const cv = refs.ambient;
    if (!cv) return;
    const ctx = cv.getContext("2d");
    const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
    let W = 0, H = 0;
    const cols = ["#3a4476", "#4a3f78", "#54467e", "#5a4a32", "#3f4a86"];
    const pts = [];
    for (let i = 0; i < (smallHero ? 44 : 78); i++)
      pts.push({ x: Math.random() * 2 - 1, y: Math.random() * 2 - 1, z: Math.random() * 2 - 1, col: cols[(Math.random() * cols.length) | 0], ph: Math.random() * Math.PI * 2, star: Math.random() < 0.14 });
    const resize = () => { const r = cv.getBoundingClientRect(); W = r.width; H = r.height; cv.width = W * dpr; cv.height = H * dpr; ctx.setTransform(dpr, 0, 0, dpr, 0, 0); };
    resize();
    S.ambResize = resize;
    window.addEventListener("resize", resize);
    let ang = 0;
    const tick = () => {
      if (S.dead) return;
      ang += 0.0009;
      ctx.clearRect(0, 0, W, H);
      const cx = W / 2, cy = H / 2, focal = Math.max(W, H) * 0.9, cos = Math.cos(ang), sin = Math.sin(ang);
      for (const p of pts) {
        const rx = p.x * cos - p.z * sin, rz = p.x * sin + p.z * cos, yy = p.y + Math.sin(ang * 1.4 + p.ph) * 0.05;
        const sc = focal / ((rz + 2.4) * 2.0), X = cx + rx * sc * 1.3, Y = cy + yy * sc * 1.3, depth = (rz + 1) / 2, rad = 1 + depth * 2.6;
        ctx.globalAlpha = 0.06 + depth * 0.22;
        ctx.fillStyle = p.col;
        if (p.star) { const a = rad * 1.6; ctx.fillRect(X - a, Y - rad * 0.4, a * 2, rad * 0.8); ctx.fillRect(X - rad * 0.4, Y - a, rad * 0.8, a * 2); }
        else { const s = Math.max(1, Math.round(rad * 1.7)); ctx.fillRect(Math.round(X), Math.round(Y), s, s); }
      }
      ctx.globalAlpha = 1;
      S.ambRaf = requestAnimationFrame(tick);
    };
    tick();
  };

  // command palette typer
  const startPalette = () => {
    const el = refs.palette;
    if (!el) return;
    const cmds = ["checkout", "track", "address", "help", "arm"];
    let ci = 0;
    let overrideUntil = 0;
    setPaletteOverride = (txt) => { el.textContent = txt; overrideUntil = performance.now() + 2800; };
    const typeOne = async () => {
      if (S.dead) return;
      if (performance.now() < overrideUntil) { await wait(220); return typeOne(); }
      const word = cmds[ci % cmds.length];
      for (let i = 0; i <= word.length; i++) { if (S.dead) return; if (performance.now() < overrideUntil) return typeOne(); el.textContent = word.slice(0, i); await wait(70); }
      await wait(1400);
      if (performance.now() < overrideUntil) return typeOne();
      for (let i = word.length; i >= 0; i--) { if (S.dead) return; el.textContent = word.slice(0, i); await wait(34); }
      await wait(300);
      ci++;
      typeOne();
    };
    typeOne();
  };

  // CLI animator
  const cliColors = { A: "#565b80", V: "#9aa0c4", B: "#e9ebf7", G: "#8ee08a", Cy: "#7fe0ff", Au: "#eab560" };
  const startCli = async () => {
    const el = refs.cli;
    if (!el) return;
    const { A, V, B, G, Cy, Au } = cliColors;
    const cur = '<span style="display:inline-block;width:8px;height:14px;background:#93a8ff;vertical-align:middle;animation:blink 1s step-end infinite"></span>';
    const rowB = (l, r, rc) => '<div style="display:flex;justify-content:space-between"><span style="color:' + A + '">&nbsp;&nbsp;' + l + '</span><span style="color:' + rc + '">' + r + "</span></div>";
    const note = (c, t) => '<div style="color:' + c + '">&nbsp;&nbsp;' + t + "</div>";
    const set = (h) => { if (el) el.innerHTML = h; };
    const prompt = '<span style="color:' + A + '">~ %</span> ';
    const orderCmd = "console order dinner";
    const colorOrder = (n) => { const head = orderCmd.slice(0, Math.min(n, 13)); const arg = n > 14 ? orderCmd.slice(14, n) : ""; return '<span style="color:' + B + '">' + head + "</span>" + (n > 13 ? " " : "") + (arg ? '<span style="color:' + Au + '">' + arg + "</span>" : ""); };
    // A saved alias list, so the reorder command reads as "scripting your usuals".
    const aliasCmd = "console alias list";
    while (!S.dead) {
      // 1) list saved aliases (presets)
      for (let i = 0; i <= aliasCmd.length; i++) { if (S.dead) return; set("<div>" + prompt + '<span style="color:' + B + '">' + aliasCmd.slice(0, i) + "</span>" + cur + "</div>"); await wait(46); }
      let acc = "<div>" + prompt + '<span style="color:' + B + '">' + aliasCmd + "</span></div>";
      await wait(340);
      acc += rowB("dinner", "Meghana Foods · 3 items", V);
      acc += rowB("coffee", "Blue Tokai · 1 item", V);
      set(acc);
      await wait(1100);
      acc += '<div style="height:12px"></div>';
      // 2) order a saved alias
      const head0 = acc;
      for (let i = 0; i <= orderCmd.length; i++) { if (S.dead) return; set(head0 + "<div>" + prompt + colorOrder(i) + cur + "</div>"); await wait(50); }
      acc = head0 + "<div>" + prompt + colorOrder(orderCmd.length) + "</div>";
      await wait(360);
      const billLines = [rowB("from", "Meghana Foods", V), rowB("2 × Chicken Biryani", "₹398", G), rowB("to pay", "₹438", Cy), note(A, "press ↵ to place · ⌃C to cancel")];
      for (const ln of billLines) { if (S.dead) return; acc += ln; set(acc); await wait(280); }
      await wait(720);
      acc += note(G, "✓ order placed · arriving ~35 min");
      set(acc);
      await wait(1500);
      acc += '<div style="height:12px"></div>';
      // 3) check status
      const statusCmd = "console status";
      for (let i = 0; i <= statusCmd.length; i++) { if (S.dead) return; set(acc + "<div>" + prompt + '<span style="color:' + B + '">' + statusCmd.slice(0, i) + "</span>" + cur + "</div>"); await wait(52); }
      acc += "<div>" + prompt + '<span style="color:' + B + '">' + statusCmd + "</span></div>";
      await wait(340);
      acc += '<div><span style="color:' + Cy + '">&nbsp;&nbsp;◐ on the way to you</span><span style="color:' + V + '"> · 6 mins</span></div>';
      set(acc);
      await wait(2800);
      set("");
      await wait(520);
    }
  };
  const staticCli = () => {
    const el = refs.cli;
    if (!el) return;
    const { A, V, B, G, Cy, Au } = cliColors;
    el.innerHTML =
      '<div><span style="color:' + A + '">~ %</span> <span style="color:' + B + '">console order</span> <span style="color:' + Au + '">dinner</span></div>' +
      '<div style="display:flex;justify-content:space-between"><span style="color:' + A + '">&nbsp;&nbsp;to pay</span><span style="color:' + Cy + '">₹438</span></div>' +
      '<div style="color:' + G + '">&nbsp;&nbsp;✓ order placed</div>' +
      '<div style="height:14px"></div>' +
      '<div><span style="color:' + A + '">~ %</span> <span style="color:' + B + '">console status</span></div>' +
      '<div><span style="color:' + Cy + '">&nbsp;&nbsp;◐ on the way to you</span><span style="color:' + V + '"> · 6 mins</span></div>';
  };

  // Agent chat animator — a mock AI agent ordering food through the console MCP
  // tools: user asks, agent calls tools, shows the real bill, places on approval.
  const startAgent = async () => {
    const el = refs.agent;
    if (!el) return;
    const { A, V, B, G, Cy, Au } = cliColors;
    const P = "#b08cf5";
    const set = (h) => { if (el) el.innerHTML = h; };
    const cur = '<span style="display:inline-block;width:7px;height:13px;background:#93a8ff;vertical-align:middle;animation:blink 1s step-end infinite"></span>';
    const you = (t) => '<div style="margin:0 0 14px;text-align:right"><span style="display:inline-block;background:#14162a;border:1px solid rgba(147,168,255,.16);border-radius:12px 12px 3px 12px;padding:8px 13px;color:#e9ebf7;font-size:12.5px;max-width:80%">' + t + "</span></div>";
    const tool = (name, ok) => '<div style="margin:0 0 8px;color:' + A + ';font-size:11.5px"><span style="color:' + P + '">●</span> consolestore · <span style="color:' + V + '">' + name + "</span>" + (ok ? ' <span style="color:' + G + '">✓</span>' : "") + "</div>";
    const toolBody = (h) => '<div style="margin:-4px 0 10px 14px;color:' + A + ';font-size:11.5px;line-height:1.7">' + h + "</div>";
    const bot = (t) => '<div style="margin:0 0 14px;display:flex;gap:8px;align-items:flex-start"><span style="color:' + P + ';font-size:12px;flex:none;margin-top:1px">✳</span><span style="color:#cdd3f0;font-size:12.5px;line-height:1.6">' + t + "</span></div>";
    const typeBot = async (acc, t) => {
      for (let i = 0; i <= t.length; i++) { if (S.dead) return acc; set(acc + '<div style="margin:0 0 14px;display:flex;gap:8px;align-items:flex-start"><span style="color:' + P + ';font-size:12px;flex:none;margin-top:1px">✳</span><span style="color:#cdd3f0;font-size:12.5px;line-height:1.6">' + t.slice(0, i) + cur + "</span></div>"); await wait(16); }
      return acc + bot(t);
    };
    while (!S.dead) {
      let acc = "";
      set(acc = you("order my usual dinner")); await wait(700);
      acc += tool("search_restaurants"); set(acc); await wait(650);
      acc += tool("prepare_order"); set(acc); await wait(300);
      acc += toolBody('Meghana Foods · Chicken Biryani ×2, Butter Naan<br><span style="color:' + Cy + '">to pay ₹438</span> <span style="color:' + A + '">· to Home</span>'); set(acc); await wait(700);
      acc = await typeBot(acc, "That’s ₹438 to Home — want me to place it?"); if (S.dead) return; set(acc); await wait(1100);
      acc += you("yes, go ahead"); set(acc); await wait(650);
      acc += tool("place_order", true); set(acc); await wait(500);
      acc = await typeBot(acc, "Ordered — Meghana Foods, arriving in ~35 min. I’ll track it."); if (S.dead) return; set(acc);
      await wait(3200);
      set(""); await wait(520);
    }
  };
  const staticAgent = () => {
    const el = refs.agent;
    if (!el) return;
    const { A, V, Cy, G } = cliColors;
    const P = "#b08cf5";
    el.innerHTML =
      '<div style="margin:0 0 14px;text-align:right"><span style="display:inline-block;background:#14162a;border:1px solid rgba(147,168,255,.16);border-radius:12px 12px 3px 12px;padding:8px 13px;color:#e9ebf7;font-size:12.5px">order my usual dinner</span></div>' +
      '<div style="margin:0 0 8px;color:' + A + ';font-size:11.5px"><span style="color:' + P + '">●</span> consolestore · <span style="color:' + V + '">place_order</span> <span style="color:' + G + '">✓</span></div>' +
      '<div style="margin:0 0 14px;display:flex;gap:8px"><span style="color:' + P + '">✳</span><span style="color:#cdd3f0;font-size:12.5px">Ordered — arriving in ~35 min.</span></div>';
  };

  // TUI screen factory
  const screens = () => {
    const C = { text: "#a9b1d6", item: "#9aa5c4", bright: "#e9ebf7", dim: "#565b80", faint: "#2d2f48", blue: "#93a8ff", cyan: "#7fe0ff", green: "#8ee08a", gold: "#eab560", red: "#ff7d96", purple: "#b08cf5", sel: "#1a1b2e" };
    const sp = (c, t, b) => '<span style="color:' + c + (b ? ";font-weight:600" : "") + '">' + (t || "") + "</span>";
    const line = (h) => "<div>" + (h || "&nbsp;") + "</div>";
    const row = (l, r, bg) => '<div style="display:flex;justify-content:space-between;gap:24px' + (bg ? ";background:" + C.sel + ";margin:0 -10px;padding:0 10px" : "") + '">' + l + "<span>" + r + "</span></div>";
    const gap = (h) => '<div style="height:' + h + 'px"></div>';
    const div = (label) => line(sp(C.faint, "──────────") + " " + sp(C.dim, label) + " " + sp(C.faint, "──────────"));
    const splash = () =>
      [
        line(sp(C.dim, "~ % ") + sp(C.text, "store")),
        gap(14),
        '<div style="font-weight:800;font-size:26px;letter-spacing:-.02em;line-height:1.1"><span style="color:#aebcff">console</span><span style="color:#eab560;font-size:.62em;vertical-align:.06em">store</span></div>',
        gap(8),
        line(sp(C.dim, "coffee · food · quick snacks")),
        gap(16),
        line(sp(C.blue, "▌", true) + '<span style="background:' + C.sel + ';color:#e9ebf7;padding:0 7px"> enter store </span>'),
        line(sp(C.dim, "&nbsp;&nbsp;settings")),
        gap(10),
        line(sp(C.faint, "&nbsp;&nbsp;q quit")),
      ].join("");
    const browse = (cur) => {
      const places = [["Meghana Foods", "28 min"], ["Third Wave Coffee", "19 min"], ["Empire Restaurant", "24 min"]];
      const rows = places.map((p, i) => (i === cur ? row(sp(C.blue, "▌ ", true) + sp("#ffffff", "> " + p[0], true), sp(C.dim, p[1]), true) : row(sp(C.item, "&nbsp;&nbsp;&nbsp;&nbsp;" + p[0]), sp(C.dim, p[1]), false))).join("");
      return [
        row(sp(C.dim, "deliver to ") + sp(C.blue, "⊕ ") + sp(C.bright, "4th Cross, Indiranagar") + sp(C.dim, " · home") + sp(C.faint, " ⌄"), ""),
        gap(10),
        row(sp(C.gold, "coffee") + sp(C.dim, " 4") + sp(C.faint, " │ ") + sp(C.gold, "food", true) + sp(C.dim, " 5") + sp(C.faint, " │ ") + sp(C.dim, "quick snacks 5"), sp(C.dim, "🛒 cart empty")),
        gap(10),
        rows,
        gap(14),
        line(sp(C.faint, "↑↓ move   ←→ category   ↵ open   / search   c cart   ") + sp(C.purple, ":") + sp(C.faint, " cmd")),
      ].join("");
    };
    const search = (q) => {
      const res = [["Meghana Foods", "★4.4 · 28 min"], ["Biryani Blues", "★4.2 · 31 min"], ["Paradise", "★4.5 · 26 min"]];
      const caret = '<span style="display:inline-block;width:8px;height:14px;background:' + C.blue + ';vertical-align:middle;animation:blink 1s step-end infinite"></span>';
      const rows = res.map((p, i) => (i === 0 ? row(sp(C.blue, "▌ ", true) + sp("#ffffff", "> " + p[0], true), sp(C.gold, p[1]), true) : row(sp(C.item, "&nbsp;&nbsp;&nbsp;&nbsp;" + p[0]), sp(C.dim, p[1]), false))).join("");
      return [line(sp(C.blue, "⌕ " + q) + caret), gap(6), line(sp(C.dim, "3 results")), gap(10), rows, gap(14), line(sp(C.faint, "↑↓ move   ↵ open   esc back"))].join("");
    };
    const resto = (added) => {
      const dishes = [["Chicken Biryani", "₹326"], ["Mutton Biryani", "₹389"], ["Paneer Biryani", "₹289"]];
      const rows = dishes.map((p, i) => (i === 0 ? row(sp(C.blue, "▌ ", true) + sp("#ffffff", "> " + p[0], true), sp(C.green, p[1]), true) : row(sp(C.item, "&nbsp;&nbsp;&nbsp;&nbsp;" + p[0]), sp(C.green, p[1]), false))).join("");
      return [
        line(sp(C.blue, "‹ ") + sp(C.bright, "Meghana Foods", true) + sp(C.faint, "  ·  ") + sp(C.gold, "★ 4.4") + sp(C.faint, "  ·  ") + sp(C.dim, "28 min")),
        gap(10),
        div("biryani"),
        rows,
        gap(14),
        row(sp(C.faint, "↵ add   c cart   esc back"), added ? sp(C.gold, "🛒 1 · ₹326") : sp(C.dim, "🛒 cart empty")),
      ].join("");
    };
    const checkout = () =>
      [
        line(sp(C.bright, "cart · checkout", true)),
        gap(8),
        line(sp(C.dim, "Meghana Foods")),
        row(sp(C.item, "Chicken Biryani ") + sp(C.dim, "×1"), sp(C.green, "₹326")),
        gap(6),
        line(sp(C.faint, "─────────────────────────────")),
        row(sp(C.dim, "item total"), sp(C.text, "₹326")),
        row(sp(C.dim, "delivery"), sp(C.text, "₹39")),
        row(sp(C.dim, "coupon ") + sp(C.purple, "WELCOME50"), sp(C.green, "-₹50")),
        line(sp(C.faint, "─────────────────────────────")),
        row(sp(C.bright, "to pay", true), sp(C.cyan, "₹315", true)),
        gap(12),
        line(sp(C.red, "▌", true) + '<span style="background:rgba(224,175,104,.14);color:' + C.gold + ';padding:0 7px"> place order </span>' + sp(C.faint, "  armed")),
        gap(10),
        line(sp(C.faint, "↵ confirm   esc back")),
      ].join("");
    const track = (step) => {
      const steps = [["confirmed", "order placed"], ["preparing", "kitchen is on it"], ["on the way", "~12 min"], ["delivered", "enjoy"]];
      const rows = steps
        .map((s, i) => {
          let mark, col;
          if (i < step) { mark = "●"; col = C.green; }
          else if (i === step) { mark = "◐"; col = C.gold; }
          else { mark = "○"; col = C.faint; }
          return row(sp(col, mark + " ") + sp(i <= step ? C.bright : C.dim, s[0]), sp(C.dim, s[1]));
        })
        .join("");
      const filled = Math.round((step / 3) * 28);
      return [line(sp(C.green, "✓ ") + sp(C.bright, "order placed", true) + sp(C.dim, " · Meghana Foods")), gap(12), rows, gap(12), line(sp(C.green, "█".repeat(filled)) + sp(C.faint, "░".repeat(28 - filled)))].join("");
    };
    return { splash, browse, search, resto, checkout, track };
  };

  const setKey = (label) => { if (refs.key) refs.key.textContent = label; };
  const staticTerminal = () => { if (refs.term) refs.term.innerHTML = screens().browse(0); };
  const startTerminal = async () => {
    const el = refs.term;
    if (!el) return;
    const Sc = screens();
    const set = (html) => { if (el) el.innerHTML = html; };
    const typeSsh = async () => {
      const cmd = "store";
      for (let i = 0; i <= cmd.length; i++) { if (S.dead) return; set('<div><span style="color:#565b80">~ % </span><span style="color:#a9b1d6">' + cmd.slice(0, i) + '</span><span style="display:inline-block;width:8px;height:14px;background:#93a8ff;vertical-align:middle;animation:blink 1s step-end infinite"></span></div>'); await wait(58); }
      await wait(420);
    };
    const typeSearch = async () => {
      const q = "biryani";
      for (let i = 0; i <= q.length; i++) { if (S.dead) return; set(Sc.search(q.slice(0, i))); setKey("/ " + q.slice(0, i)); await wait(95); }
      await wait(500);
    };
    while (!S.dead) {
      setKey("run"); await typeSsh(); if (S.dead) return;
      set(Sc.splash()); setKey("boot"); await wait(2100); if (S.dead) return;
      setKey("↵ enter"); set(Sc.browse(0)); await wait(900); if (S.dead) return;
      setKey("j move"); set(Sc.browse(1)); await wait(750); if (S.dead) return;
      set(Sc.browse(2)); await wait(850); if (S.dead) return;
      setKey("/ search"); await typeSearch(); if (S.dead) return;
      set(Sc.search("biryani")); await wait(1400); if (S.dead) return;
      setKey("↵ open"); set(Sc.resto(false)); await wait(1100); if (S.dead) return;
      setKey("↵ add"); set(Sc.resto(true)); await wait(1300); if (S.dead) return;
      setKey(": checkout"); set(Sc.checkout()); await wait(2200); if (S.dead) return;
      setKey("↵ confirm");
      for (let st = 0; st <= 3; st++) { if (S.dead) return; set(Sc.track(st)); await wait(820); }
      setKey("✓ done"); await wait(1800); if (S.dead) return;
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

  // boot
  if (refs.toast) refs.toast.style.display = "none";
  initReveal();
  initNudge();
  initFaq();
  initCmdClicks();
  initFeaturesZoom();
  initStats();
  initKeyboardNav();
  initScrollSnap();

  // hero video runs in both motion modes (it pauses on the poster under reduce);
  // this also stops an invisible autoplay from burning CPU when the block below
  // is skipped for reduced-motion.
  startHeroVideo();

  if (!reduce) {
    initFooterWordmark();
    startAmbient();
    startTerminal();
    startPalette();
    startCli();
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
    if (smallHero) {
      // phone: no canvas to measure — scramble the styled wordmark right away
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
    staticCli();
    staticAgent();
    if (refs.palette) refs.palette.textContent = "checkout";
  }

  return () => {
    S.dead = true;
    cancelAnimationFrame(S.raf);
    cancelAnimationFrame(S.ambRaf);
    S.timers.forEach(clearTimeout);
    if (S.ambResize) window.removeEventListener("resize", S.ambResize);
    copyEls.forEach((el) => el.removeEventListener("click", copyInstall));
    if (S.shareCleanup) S.shareCleanup();
    faqHandlers.forEach(([q, h]) => q.removeEventListener("click", h));
    cmdHandlers.forEach(([el, h]) => el.removeEventListener("click", h));
    wmHandlers.forEach(([el, h]) => el.removeEventListener("click", h));
    scrambleObservers.forEach((io) => io.disconnect());
    if (S.heroCleanup) S.heroCleanup();
    if (S.nudgeCleanup) S.nudgeCleanup();
    if (S.statsCleanup) S.statsCleanup();
    if (S.navCleanup) S.navCleanup();
    if (S.snapCleanup) S.snapCleanup();
    if (S.featZoomCleanup) S.featZoomCleanup();
    cleanupKeys();
  };
}
