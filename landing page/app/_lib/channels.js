export const GITHUB_REPO = "sujan174/console.store";

export function ghAssetURL(tag, asset) {
  return `https://github.com/${GITHUB_REPO}/releases/download/${tag}/${asset}`;
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
export async function latestTag(channel) {
  // per_page must exceed the total release count: GitHub's list order is not
  // reliably newest-first, so a too-small window can drop a channel's latest tag
  // and silently serve a stale version. 100 is the API max — revisit with real
  // pagination before the repo ever holds 100+ releases.
  const res = await fetch(`https://api.github.com/repos/${GITHUB_REPO}/releases?per_page=100`, {
    headers: { Accept: "application/vnd.github+json", "User-Agent": "consolestore-landing" },
    next: { revalidate: 60 },
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
