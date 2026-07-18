import crypto from "node:crypto";

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
// MAX_RELEASE_PAGES bounds pagination so a runaway can't loop forever. 10 pages
// × 100 = 1000 releases, far beyond any realistic count.
const MAX_RELEASE_PAGES = 10;

// nextPageURL extracts the `rel="next"` URL from a GitHub Link header, or null
// when there are no more pages.
function nextPageURL(linkHeader) {
  if (!linkHeader) return null;
  for (const part of linkHeader.split(",")) {
    const m = /<([^>]+)>\s*;\s*rel="next"/.exec(part);
    if (m) return m[1];
  }
  return null;
}

export async function latestTag(channel) {
  const headers = {
    Accept: "application/vnd.github+json",
    "User-Agent": "consolestore-landing",
  };
  const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN || "";
  if (token) headers.Authorization = `Bearer ${token}`;
  // Page through ALL releases following the Link: rel="next" header. GitHub's
  // list order is not reliably newest-first (promotion re-tags an older commit,
  // which sorts down by created_at), so a single 100-item page can drop a
  // channel's true latest tag once the repo crosses 100 releases — the
  // "stuck-on-a-stale-version" incident class. Cache each page 10 min so even
  // unauthenticated we make few calls/hour regardless of poll volume.
  let url = `https://api.github.com/repos/${GITHUB_REPO}/releases?per_page=100`;
  const tags = [];
  for (let page = 0; page < MAX_RELEASE_PAGES && url; page++) {
    const res = await fetch(url, { headers, next: { revalidate: 600 } });
    if (!res.ok) {
      // A later page failing shouldn't discard tags already collected; only a
      // first-page failure yields null (no data at all).
      if (page === 0) return null;
      break;
    }
    const releases = await res.json();
    for (const r of releases) {
      if (!r.draft) tags.push(r.tag_name);
    }
    url = nextPageURL(res.headers.get("link"));
  }
  return pickLatestTag(tags, channel);
}

// constantTimeEqual compares two strings without an early-exit timing side
// channel. Both sides are sha256-hashed first so the buffers are always the same
// length (timingSafeEqual throws on a length mismatch).
function constantTimeEqual(a, b) {
  const ha = crypto.createHash("sha256").update(String(a)).digest();
  const hb = crypto.createHash("sha256").update(String(b)).digest();
  return crypto.timingSafeEqual(ha, hb);
}

export function checkAlphaCode(code) {
  if (!code) return { ok: false, label: null };
  const raw = process.env.CONSOLE_ALPHA_CODES || "";
  let matched = { ok: false, label: null };
  for (const pair of raw.split(",")) {
    const idx = pair.indexOf(":");
    if (idx < 0) continue;
    const label = pair.slice(0, idx);
    const value = pair.slice(idx + 1);
    // Constant-time compare, and DON'T break early — scan every configured code
    // so the total work doesn't depend on which (or whether) one matched.
    if (value && constantTimeEqual(value, code)) {
      matched = { ok: true, label: label || "unknown" };
    }
  }
  return matched;
}

// The ed25519 public key that verifies release manifests, as raw base64 (the
// 32-byte key). MUST stay in sync with internal/updater/pubkey.go
// (signingPubKeyB64). The private counterpart lives only in the CONSOLE_SIGN_KEY
// CI secret.
const SIGN_PUBKEY_B64 = "2eKjjdwLlQcgyxWZZZNcxIzv7wFFAYQfncuW3wgdNu4=";

// ED25519 SPKI DER prefix for a raw 32-byte public key.
const ED25519_SPKI_PREFIX = Buffer.from("302a300506032b6570032100", "hex");

function signingKey() {
  const raw = Buffer.from(SIGN_PUBKEY_B64, "base64");
  return crypto.createPublicKey({
    key: Buffer.concat([ED25519_SPKI_PREFIX, raw]),
    format: "der",
    type: "spki",
  });
}

// verifyManifest checks a signed manifest envelope's ed25519 signature and, on
// success, returns the DECODED payload object. Returns null on any failure
// (malformed JSON, missing fields, bad signature) — the caller must treat null
// as "do not trust". The signature is over the RAW base64-decoded payload bytes,
// matching internal/updater/manifest.go Envelope.Verify.
export function verifyManifest(envelopeText) {
  let env;
  try {
    env = JSON.parse(envelopeText);
  } catch {
    return null;
  }
  if (!env || typeof env.payload !== "string" || typeof env.sig !== "string") {
    return null;
  }
  let payloadBytes, sigBytes;
  try {
    payloadBytes = Buffer.from(env.payload, "base64");
    sigBytes = Buffer.from(env.sig, "base64");
  } catch {
    return null;
  }
  let ok = false;
  try {
    ok = crypto.verify(null, payloadBytes, signingKey(), sigBytes);
  } catch {
    return null;
  }
  if (!ok) return null;
  try {
    return JSON.parse(payloadBytes.toString("utf8"));
  } catch {
    return null;
  }
}

export function logAlphaGrant({ label, asset, version, ip, ua }) {
  console.log(
    `alpha-grant who=${label} asset=${asset || "-"} version=${version || "-"} ip=${ip || "-"} ua=${JSON.stringify(ua || "-")}`,
  );
}

export async function ghReleaseBody(tag) {
  // Validate the tag before interpolating it into the GitHub API URL — sibling
  // routes validate their params (asset/channel) and this one must too.
  if (!parseTag(tag)) return null;
  const res = await fetch(`https://api.github.com/repos/${GITHUB_REPO}/releases/tags/${encodeURIComponent(tag)}`, {
    headers: { "User-Agent": "consolestore-landing" },
  });
  if (!res.ok) return null;
  const data = await res.json();
  const body = data.body;
  if (!body || !body.trim()) return null;
  return body;
}
