export const GITHUB_REPO = "sujan174/console.store";

export function ghAssetURL(tag, asset) {
  return `https://github.com/${GITHUB_REPO}/releases/download/${tag}/${asset}`;
}

// fetchSignedManifest returns a release's signed manifest envelope (JSON text),
// or null on failure. Both the manifest and checksum endpoints need it, and it
// was previously fetched from GitHub on EVERY request, uncached — that ~3s
// round-trip dominated their server-side latency (p50 ≈ 2.9s) and made installs
// and update-checks sluggish, worst for high-latency clients. Cache it 10 min
// (keyed by URL, which includes the tag, so a new release still fetches fresh)
// and bound it with a timeout so a stalled GitHub fetch can't hang the request.
export async function fetchSignedManifest(tag) {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(), 8000);
  try {
    const res = await fetch(ghAssetURL(tag, "console-manifest.json"), {
      headers: { "User-Agent": "consolestore-landing" },
      signal: ctrl.signal,
      next: { revalidate: 600 },
    });
    if (!res.ok) return null;
    return await res.text();
  } catch {
    return null;
  } finally {
    clearTimeout(timer);
  }
}

// channelMatches reports whether a tag belongs to a channel. stable = no
// -alpha/-beta suffix; beta/alpha = the matching prerelease suffix.
export function channelMatches(tag, channel) {
  const isAlpha = tag.includes("-alpha");
  const isBeta = tag.includes("-beta");
  if (channel === "alpha") return isAlpha;
  if (channel === "beta") return isBeta;
  return !isAlpha && !isBeta; // stable
}

// parseTag turns "v0.1.0-beta.10" into a comparable shape, or null if the tag
// isn't one of our release tags.
export function parseTag(tag) {
  const m = /^v(\d+)\.(\d+)\.(\d+)(?:-(alpha|beta)\.(\d+))?$/.exec(tag);
  if (!m) return null;
  return {
    base: [Number(m[1]), Number(m[2]), Number(m[3])],
    pre: m[4] || null, // null = stable
    num: m[4] ? Number(m[5]) : 0,
  };
}

// cmpTag orders two parsed tags: higher base wins; a stable release outranks a
// prerelease of the same base; otherwise the higher prerelease counter wins.
// Numeric, so beta.10 correctly beats beta.9 ("10" < "9" as strings does not).
function cmpTag(a, b) {
  for (let i = 0; i < 3; i++) {
    if (a.base[i] !== b.base[i]) return a.base[i] - b.base[i];
  }
  if (a.pre === null && b.pre !== null) return 1;
  if (a.pre !== null && b.pre === null) return -1;
  return a.num - b.num;
}

// pickLatestTag returns the highest-version tag for a channel. GitHub's
// /releases list order is NOT reliably newest-first (a fresh beta.10 can sort
// below beta.3), so we compare versions explicitly instead of taking the first
// match — otherwise a new release would never be served.
export function pickLatestTag(tags, channel) {
  let best = null;
  let bestParsed = null;
  for (const tag of tags) {
    if (!channelMatches(tag, channel)) continue;
    const p = parseTag(tag);
    if (!p) continue;
    if (best === null || cmpTag(p, bestParsed) > 0) {
      best = tag;
      bestParsed = p;
    }
  }
  return best;
}

// latestTag returns the highest-version release tag for a channel.
//
// This is the single dependency behind every channel endpoint (manifest,
// checksum, download). It calls GitHub's REST API, which is the thing to keep
// robust: unauthenticated calls share a 60-req/hour limit PER SOURCE IP, so
// Railway's egress IP — serving every tester's launch-time poll — was blowing
// that limit and making this return null, which surfaced as intermittent 404s
// ("no release") across all channels and blocked self-updates. Two guards:
//   1. Authenticate when GITHUB_TOKEN is set (raises the limit to 5000/hour).
//   2. Cache the list for 10 min, so even unauthenticated we make at most ~6
//      calls/hour regardless of how many testers poll.
export async function latestTag(channel) {
  const headers = {
    Accept: "application/vnd.github+json",
    "User-Agent": "consolestore-landing",
  };
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN || "";
  if (token) headers.Authorization = `Bearer ${token}`;
  // per_page must exceed the total release count: GitHub's list order is not
  // reliably newest-first, so a too-small window can drop a channel's latest tag
  // and silently serve a stale version. 100 is the API max — revisit with real
  // pagination before the repo ever holds 100+ releases.
  const res = await fetch(`https://api.github.com/repos/${GITHUB_REPO}/releases?per_page=100`, {
    headers,
    next: { revalidate: 600 },
  });
  if (!res.ok) return null;
  const releases = await res.json();
  const tags = releases.filter((r) => !r.draft).map((r) => r.tag_name);
  return pickLatestTag(tags, channel);
}

export function checkAlphaCode(code) {
  if (!code) return { ok: false, label: null };
  const raw = process.env.CONSOLE_ALPHA_CODES || "";
  for (const pair of raw.split(",")) {
    const [label, value] = pair.split(":");
    if (value && value === code) return { ok: true, label: label || "unknown" };
  }
  return { ok: false, label: null };
}

export function logAlphaGrant({ label, asset, version, ip, ua }) {
  console.log(
    `alpha-grant who=${label} asset=${asset || "-"} version=${version || "-"} ip=${ip || "-"} ua=${JSON.stringify(ua || "-")}`,
  );
}

export async function ghReleaseBody(tag) {
  const res = await fetch(`https://api.github.com/repos/${GITHUB_REPO}/releases/tags/${tag}`, {
    headers: { "User-Agent": "consolestore-landing" },
  });
  if (!res.ok) return null;
  const data = await res.json();
  const body = data.body;
  if (!body || !body.trim()) return null;
  return body;
}
