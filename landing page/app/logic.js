// Behavior ported from Claude Design "Console Store Landing.dc.html".
// The DCLogic React class is rewritten as a framework-free mount(root) that
// returns a cleanup fn. Canvas + animation code is kept verbatim.

export function mount(root) {
  if (!root) return () => {};

  const S = {
    dead: false,
    timers: [],
    raf: 0,
    ambRaf: 0,
    onResize: null,
    ambResize: null,
  };
  const wait = (ms) => new Promise((r) => S.timers.push(setTimeout(r, ms)));
  const reduce =
    typeof window !== "undefined" &&
    window.matchMedia &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  const refs = {
    root,
    ambient: root.querySelector('[data-ref="ambient"]'),
    canvas: root.querySelector('[data-ref="canvas"]'),
    term: root.querySelector('[data-ref="term"]'),
    key: root.querySelector('[data-ref="key"]'),
    palette: root.querySelector('[data-ref="palette"]'),
    install: root.querySelector('[data-ref="install"]'),
    toast: root.querySelector('[data-ref="toast"]'),
  };

  // ---- toast / install ping ----
  const pingInstall = () => {
    if (!refs.toast) return;
    refs.toast.style.display = "flex";
    S.timers.push(
      setTimeout(() => {
        if (!S.dead && refs.toast) refs.toast.style.display = "none";
      }, 2200)
    );
  };
  const installEls = Array.from(root.querySelectorAll('[data-action="install"]'));
  installEls.forEach((el) => el.addEventListener("click", pingInstall));

  // ---- scroll reveal: CSS scroll-driven (view timeline) ----
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

  // ---- faq accordion ----
  const faqHandlers = [];
  const initFaq = () => {
    Array.from(root.querySelectorAll("[data-faq]")).forEach((item) => {
      const q = item.querySelector("[data-faq-q]");
      const a = item.querySelector("[data-faq-a]");
      const ic = item.querySelector("[data-faq-i]");
      const h = () => {
        const open = a.style.maxHeight && a.style.maxHeight !== "0px";
        if (open) {
          a.style.maxHeight = "0px";
          ic.style.transform = "none";
          ic.textContent = "+";
        } else {
          a.style.maxHeight = a.scrollHeight + "px";
          ic.style.transform = "rotate(45deg)";
        }
      };
      q.addEventListener("click", h);
      faqHandlers.push([q, h]);
    });
  };

  // ---- ambient floating dot field ----
  const startAmbient = () => {
    const cv = refs.ambient;
    if (!cv) return;
    const ctx = cv.getContext("2d");
    const dpr = Math.min(window.devicePixelRatio || 1, 1.5);
    let W = 0,
      H = 0;
    const cols = ["#2b2f4d", "#33406a", "#3a3357", "#2f4a6b"];
    const pts = [];
    for (let i = 0; i < 70; i++) {
      pts.push({
        x: Math.random() * 2 - 1,
        y: Math.random() * 2 - 1,
        z: Math.random() * 2 - 1,
        col: cols[(Math.random() * cols.length) | 0],
        ph: Math.random() * Math.PI * 2,
      });
    }
    const resize = () => {
      const r = cv.getBoundingClientRect();
      W = r.width;
      H = r.height;
      cv.width = W * dpr;
      cv.height = H * dpr;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    };
    resize();
    S.ambResize = resize;
    window.addEventListener("resize", resize);
    let ang = 0;
    const tick = () => {
      if (S.dead) return;
      ang += 0.0009;
      ctx.clearRect(0, 0, W, H);
      const cx = W / 2,
        cy = H / 2,
        focal = Math.max(W, H) * 0.9;
      const cos = Math.cos(ang),
        sin = Math.sin(ang);
      for (const p of pts) {
        const rx = p.x * cos - p.z * sin;
        const rz = p.x * sin + p.z * cos;
        const yy = p.y + Math.sin(ang * 1.4 + p.ph) * 0.05;
        const sc = focal / ((rz + 2.4) * 2.0);
        const X = cx + rx * sc * 1.3;
        const Y = cy + yy * sc * 1.3;
        const depth = (rz + 1) / 2;
        const rad = 1 + depth * 2.6;
        ctx.globalAlpha = 0.06 + depth * 0.2;
        ctx.fillStyle = p.col;
        ctx.beginPath();
        ctx.arc(X, Y, rad, 0, 6.2832);
        ctx.fill();
      }
      ctx.globalAlpha = 1;
      S.ambRaf = requestAnimationFrame(tick);
    };
    tick();
  };

  // ============ ASCII PARTICLE WORDMARK ============
  const startParticles = () => {
    const cv = refs.canvas;
    if (!cv) return;
    const ctx = cv.getContext("2d");
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    const glyphs = "consolestore.in$>_#/·▌░▒█01".split("");
    const wordPal = [
      "#7aa2f7",
      "#7aa2f7",
      "#7aa2f7",
      "#9aa5c4",
      "#9aa5c4",
      "#c0caf5",
      "#7dcfff",
      "#bb9af7",
      "#9ece6a",
      "#e0af68",
    ];
    let W = 0,
      H = 0,
      parts = [];

    const buildTargets = () => {
      if (W < 10 || H < 10) return;
      const word = "consolestore";
      let fs = Math.min((W * 0.92) / (word.length * 0.6), H * 0.74);
      fs = Math.max(26, Math.min(fs, 150));
      const o = document.createElement("canvas");
      o.width = W;
      o.height = H;
      const octx = o.getContext("2d");
      octx.font = `700 ${fs}px 'JetBrains Mono', monospace`;
      octx.textAlign = "center";
      octx.textBaseline = "middle";
      octx.fillStyle = "#fff";
      octx.fillText(word, W / 2, H / 2 + 2);
      const data = octx.getImageData(0, 0, W, H).data;
      const gap = fs > 96 ? 7 : 6;
      const targets = [];
      for (let y = 0; y < H; y += gap)
        for (let x = 0; x < W; x += gap) {
          if (data[(y * W + x) * 4 + 3] > 130) targets.push({ x, y });
        }
      const next = [];
      for (let i = 0; i < targets.length; i++) {
        const old = parts[i];
        const a0 = Math.random() * Math.PI * 2,
          r0 = 130 + Math.random() * 300;
        next.push({
          x: old ? old.x : W / 2 + Math.cos(a0) * r0,
          y: old ? old.y : H / 2 + Math.sin(a0) * r0 * 0.7 + 60,
          vx: old ? old.vx : -Math.sin(a0) * 3.4,
          vy: old ? old.vy : Math.cos(a0) * 3.4,
          tx: targets[i].x,
          ty: targets[i].y,
          ch: glyphs[(Math.random() * glyphs.length) | 0],
          col: wordPal[(Math.random() * wordPal.length) | 0],
          amb: false,
        });
      }
      const amb = Math.min(120, Math.floor(W / 14));
      for (let i = 0; i < amb; i++) {
        next.push({
          x: Math.random() * W,
          y: Math.random() * H,
          vx: (Math.random() - 0.5) * 0.3,
          vy: (Math.random() - 0.5) * 0.3,
          ch: glyphs[(Math.random() * glyphs.length) | 0],
          col: "#2a2c44",
          amb: true,
        });
      }
      parts = next;
    };

    const resize = () => {
      const r = cv.getBoundingClientRect();
      W = Math.floor(r.width);
      H = Math.floor(r.height);
      cv.width = W * dpr;
      cv.height = H * dpr;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      buildTargets();
      // After the wordmark has settled the rAF loop is stopped, so a later
      // resize (e.g. a mobile browser collapsing its address bar) would leave
      // the canvas blank. Repaint the static frame once on every settled resize.
      if (settled) tick();
    };

    S.onResize = resize;
    window.addEventListener("resize", resize);

    let settled = false;
    const t0 = performance.now();

    const tick = () => {
      if (S.dead) return;
      if (parts.length === 0) buildTargets();
      ctx.clearRect(0, 0, W, H);
      ctx.font = `600 12.5px 'JetBrains Mono', monospace`;
      ctx.textAlign = "center";
      ctx.textBaseline = "middle";
      const elapsed = performance.now() - t0;
      const sweepX =
        elapsed > 1500 && elapsed < 2700
          ? ((elapsed - 1500) / 1100) * (W + 120) - 60
          : -99999;
      for (let i = 0; i < parts.length; i++) {
        const p = parts[i];
        if (settled) {
          if (!p.amb) {
            p.x = p.tx;
            p.y = p.ty;
          }
        } else {
          if (!p.amb) {
            p.vx += (p.tx - p.x) * 0.03;
            p.vy += (p.ty - p.y) * 0.03;
          } else {
            p.vx += (Math.random() - 0.5) * 0.04;
            p.vy += (Math.random() - 0.5) * 0.04;
          }
          p.vx *= 0.9;
          p.vy *= 0.9;
          p.x += p.vx;
          p.y += p.vy;
          if (p.amb) {
            if (p.x < 0) p.x = W;
            if (p.x > W) p.x = 0;
            if (p.y < 0) p.y = H;
            if (p.y > H) p.y = 0;
          }
        }
        let col = p.col,
          a = p.amb ? 0.5 : 1;
        if (!p.amb && Math.abs(p.tx - sweepX) < 46) {
          col = "#ffffff";
          a = 1;
        }
        ctx.globalAlpha = a;
        ctx.fillStyle = col;
        ctx.fillText(p.ch, p.x, p.y);
      }
      ctx.globalAlpha = 1;
      if (!settled) S.raf = requestAnimationFrame(tick);
    };

    resize();
    tick();
    S.timers.push(
      setTimeout(() => {
        if (!S.dead) {
          settled = true;
          tick();
        }
      }, 2700)
    );
  };

  // ============ COMMAND PALETTE TYPER ============
  const startPalette = () => {
    const el = refs.palette;
    if (!el) return;
    const cmds = ["checkout", "track", "address", "help", "arm"];
    let ci = 0;
    const typeOne = async () => {
      if (S.dead) return;
      const word = cmds[ci % cmds.length];
      for (let i = 0; i <= word.length; i++) {
        if (S.dead) return;
        el.textContent = word.slice(0, i);
        await wait(70);
      }
      await wait(1400);
      for (let i = word.length; i >= 0; i--) {
        if (S.dead) return;
        el.textContent = word.slice(0, i);
        await wait(34);
      }
      await wait(300);
      ci++;
      typeOne();
    };
    typeOne();
  };

  // ============ TUI SCREENS ============
  const screens = () => {
    const C = {
      text: "#a9b1d6",
      item: "#9aa5c4",
      bright: "#c0caf5",
      dim: "#565f89",
      faint: "#3b3b5a",
      blue: "#7aa2f7",
      cyan: "#7dcfff",
      green: "#9ece6a",
      gold: "#e0af68",
      red: "#f7768e",
      purple: "#bb9af7",
      sel: "#1f2335",
    };
    const sp = (c, t, b) =>
      `<span style="color:${c}${b ? ";font-weight:600" : ""}">${t}</span>`;
    const line = (h) => `<div>${h || "&nbsp;"}</div>`;
    const row = (l, r, bg) =>
      `<div style="display:flex;justify-content:space-between;gap:24px${
        bg ? ";background:" + C.sel + ";margin:0 -10px;padding:0 10px" : ""
      }">${l}<span>${r}</span></div>`;
    const gap = (h) => `<div style="height:${h}px"></div>`;
    const div = (label) =>
      line(sp(C.faint, "──────────") + " " + sp(C.dim, label) + " " + sp(C.faint, "──────────"));

    const splash = () =>
      [
        line(sp(C.dim, "~ % ") + sp(C.text, "ssh consolestore.in")),
        gap(14),
        `<div style="font-size:30px;letter-spacing:5px;font-weight:600;color:#c0caf5;line-height:1.1">consolestore</div>`,
        gap(8),
        line(sp(C.dim, "coffee · food · quick snacks")),
        gap(16),
        line(
          sp(C.blue, "▌", true) +
            `<span style="background:${C.sel};color:#c0caf5;padding:0 7px"> enter store </span>`
        ),
        line(sp(C.dim, "&nbsp;&nbsp;settings")),
        gap(10),
        line(sp(C.faint, "&nbsp;&nbsp;q quit")),
      ].join("");

    const browse = (cur) => {
      const places = [
        ["Meghana Foods", "28 min"],
        ["Third Wave Coffee", "19 min"],
        ["Empire Restaurant", "24 min"],
      ];
      const rows = places
        .map((p, i) => {
          if (i === cur)
            return row(
              sp(C.blue, "▌ ", true) + sp("#ffffff", "> " + p[0], true),
              sp(C.dim, p[1]),
              true
            );
          return row(sp(C.item, "&nbsp;&nbsp;&nbsp;&nbsp;" + p[0]), sp(C.dim, p[1]), false);
        })
        .join("");
      return [
        row(
          sp(C.dim, "deliver to ") +
            sp(C.blue, "⊕ ") +
            sp(C.bright, "4th Cross, Indiranagar") +
            sp(C.dim, " · home") +
            sp(C.faint, " ⌄"),
          ""
        ),
        gap(10),
        row(
          sp(C.gold, "coffee") +
            sp(C.dim, " 4") +
            sp(C.faint, " │ ") +
            sp(C.gold, "food", true) +
            sp(C.dim, " 5") +
            sp(C.faint, " │ ") +
            sp(C.dim, "quick snacks 5"),
          sp(C.dim, "🛒 cart empty")
        ),
        gap(10),
        rows,
        gap(14),
        line(
          sp(C.faint, "↑↓ move   ←→ category   ↵ open   / search   c cart   ") +
            sp(C.purple, ":") +
            sp(C.faint, " cmd")
        ),
      ].join("");
    };

    const search = (q) => {
      const res = [
        ["Meghana Foods", "★4.4 · 28 min"],
        ["Biryani Blues", "★4.2 · 31 min"],
        ["Paradise", "★4.5 · 26 min"],
      ];
      const caret = `<span style="display:inline-block;width:8px;height:14px;background:${C.blue};vertical-align:middle;animation:blink 1s step-end infinite"></span>`;
      const rows = res
        .map((p, i) =>
          i === 0
            ? row(
                sp(C.blue, "▌ ", true) + sp("#ffffff", "> " + p[0], true),
                sp(C.gold, p[1]),
                true
              )
            : row(sp(C.item, "&nbsp;&nbsp;&nbsp;&nbsp;" + p[0]), sp(C.dim, p[1]), false)
        )
        .join("");
      return [
        line(sp(C.blue, "⌕ " + q) + caret),
        gap(6),
        line(sp(C.dim, "3 results")),
        gap(10),
        rows,
        gap(14),
        line(sp(C.faint, "↑↓ move   ↵ open   esc back")),
      ].join("");
    };

    const resto = (added) => {
      const dishes = [
        ["Chicken Biryani", "₹326"],
        ["Mutton Biryani", "₹389"],
        ["Paneer Biryani", "₹289"],
      ];
      const rows = dishes
        .map((p, i) =>
          i === 0
            ? row(
                sp(C.blue, "▌ ", true) + sp("#ffffff", "> " + p[0], true),
                sp(C.green, p[1]),
                true
              )
            : row(sp(C.item, "&nbsp;&nbsp;&nbsp;&nbsp;" + p[0]), sp(C.green, p[1]), false)
        )
        .join("");
      return [
        line(
          sp(C.blue, "‹ ") +
            sp(C.bright, "Meghana Foods", true) +
            sp(C.faint, "  ·  ") +
            sp(C.gold, "★ 4.4") +
            sp(C.faint, "  ·  ") +
            sp(C.dim, "28 min")
        ),
        gap(10),
        div("biryani"),
        rows,
        gap(14),
        row(
          sp(C.faint, "↵ add   c cart   esc back"),
          added ? sp(C.gold, "🛒 1 · ₹326") : sp(C.dim, "🛒 cart empty")
        ),
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
        line(
          sp(C.red, "▌", true) +
            `<span style="background:rgba(224,175,104,0.14);color:${C.gold};padding:0 7px"> place order </span>` +
            sp(C.faint, "  armed")
        ),
        gap(10),
        line(sp(C.faint, "↵ confirm   esc back")),
      ].join("");

    const track = (step) => {
      const steps = [
        ["confirmed", "order placed"],
        ["preparing", "kitchen is on it"],
        ["on the way", "~12 min"],
        ["delivered", "enjoy"],
      ];
      const rows = steps
        .map((s, i) => {
          let mark, col;
          if (i < step) {
            mark = "●";
            col = C.green;
          } else if (i === step) {
            mark = "◐";
            col = C.gold;
          } else {
            mark = "○";
            col = C.faint;
          }
          return row(
            sp(col, mark + " ") + sp(i <= step ? C.bright : C.dim, s[0]),
            sp(C.dim, s[1])
          );
        })
        .join("");
      const filled = Math.round((step / 3) * 28);
      const bar =
        sp(C.green, "█".repeat(filled)) + sp(C.faint, "░".repeat(28 - filled));
      return [
        line(
          sp(C.green, "✓ ") +
            sp(C.bright, "order placed", true) +
            sp(C.dim, " · Meghana Foods")
        ),
        gap(12),
        rows,
        gap(12),
        line(bar),
      ].join("");
    };

    return { splash, browse, search, resto, checkout, track };
  };

  const setKey = (label) => {
    if (refs.key) refs.key.textContent = label;
  };

  const staticTerminal = () => {
    if (refs.term) refs.term.innerHTML = screens().browse(0);
  };

  const startTerminal = async () => {
    const el = refs.term;
    if (!el) return;
    const Sc = screens();
    const set = (html) => {
      if (el) el.innerHTML = html;
    };

    const typeSsh = async () => {
      const cmd = "ssh consolestore.in";
      for (let i = 0; i <= cmd.length; i++) {
        if (S.dead) return;
        set(
          `<div><span style="color:#565f89">~ % </span><span style="color:#a9b1d6">${cmd.slice(
            0,
            i
          )}</span><span style="display:inline-block;width:8px;height:14px;background:#7aa2f7;vertical-align:middle;animation:blink 1s step-end infinite"></span></div>`
        );
        await wait(58);
      }
      await wait(420);
    };

    const typeSearch = async () => {
      const q = "biryani";
      for (let i = 0; i <= q.length; i++) {
        if (S.dead) return;
        set(Sc.search(q.slice(0, i)));
        setKey("/ " + q.slice(0, i));
        await wait(95);
      }
      await wait(500);
    };

    while (!S.dead) {
      setKey("ssh");
      await typeSsh();
      if (S.dead) return;
      set(Sc.splash());
      setKey("boot");
      await wait(2100);
      if (S.dead) return;

      setKey("↵ enter");
      set(Sc.browse(0));
      await wait(900);
      if (S.dead) return;
      setKey("j move");
      set(Sc.browse(1));
      await wait(750);
      if (S.dead) return;
      set(Sc.browse(2));
      await wait(850);
      if (S.dead) return;

      setKey("/ search");
      await typeSearch();
      if (S.dead) return;
      set(Sc.search("biryani"));
      await wait(1400);
      if (S.dead) return;

      setKey("↵ open");
      set(Sc.resto(false));
      await wait(1100);
      if (S.dead) return;
      setKey("↵ add");
      set(Sc.resto(true));
      await wait(1300);
      if (S.dead) return;

      setKey(": checkout");
      set(Sc.checkout());
      await wait(2200);
      if (S.dead) return;

      setKey("↵ confirm");
      for (let st = 0; st <= 3; st++) {
        if (S.dead) return;
        set(Sc.track(st));
        await wait(820);
      }
      setKey("✓ done");
      await wait(1800);
      if (S.dead) return;
    }
  };

  // ---- boot ----
  if (refs.toast) refs.toast.style.display = "none";
  initReveal();
  initFaq();
  if (!reduce) {
    startAmbient();
    startParticles();
    startTerminal();
    startPalette();
  } else {
    staticTerminal();
  }

  // ---- cleanup ----
  return () => {
    S.dead = true;
    cancelAnimationFrame(S.raf);
    cancelAnimationFrame(S.ambRaf);
    S.timers.forEach(clearTimeout);
    if (S.onResize) window.removeEventListener("resize", S.onResize);
    if (S.ambResize) window.removeEventListener("resize", S.ambResize);
    installEls.forEach((el) => el.removeEventListener("click", pingInstall));
    faqHandlers.forEach(([q, h]) => q.removeEventListener("click", h));
  };
}
