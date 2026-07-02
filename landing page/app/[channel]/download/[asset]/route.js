import { latestTag, ghAssetURL, checkAlphaCode, logAlphaGrant } from "../../../_lib/channels.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const CHANNELS = new Set(["stable", "beta", "alpha"]);
const ASSET_RE = /^store_[a-z0-9]+_[a-z0-9]+(\.exe)?$/;

// ghFetchAsset fetches a release asset for server-side streaming (the alpha
// path), with a bounded connection timeout and a few retries. Railway's egress
// to GitHub's asset CDN intermittently returns a transient non-ok or stalls;
// unretried, that surfaced to the client updater as a 502 (or an open-ended
// hang) and silently aborted the self-update, stranding testers on the old
// build. The timeout guards only the connection/header phase — it's cleared the
// moment the response resolves, so it never aborts an in-flight body stream.
async function ghFetchAsset(url, attempts = 3) {
  for (let i = 0; i < attempts; i++) {
    const ctrl = new AbortController();
    const timer = setTimeout(() => ctrl.abort(), 20000);
    try {
      const res = await fetch(url, {
        headers: { "User-Agent": "consolestore-landing" },
        signal: ctrl.signal,
        redirect: "follow",
      });
      clearTimeout(timer);
      if (res.ok && res.body) return res;
    } catch {
      clearTimeout(timer);
      // transient (abort/reset) — fall through to the next attempt
    }
  }
  return null;
}

export async function GET(req, { params }) {
  const { channel, asset } = await params;
  if (!CHANNELS.has(channel) || !ASSET_RE.test(asset)) {
    return new Response("bad request", { status: 400 });
  }

  // Alpha: gate on the code BEFORE revealing whether a release exists.
  let label = null;
  if (channel === "alpha") {
    const code = new URL(req.url).searchParams.get("code") || req.headers.get("x-console-code") || "";
    const chk = checkAlphaCode(code);
    if (!chk.ok) return new Response("alpha is invite-only", { status: 403 });
    label = chk.label;
  }

  const tag = await latestTag(channel);
  if (!tag) return new Response("no release", { status: 404 });
  const target = ghAssetURL(tag, asset);

  if (channel !== "alpha") {
    return Response.redirect(target, 302);
  }

  const upstream = await ghFetchAsset(target);
  if (!upstream) return new Response("asset temporarily unavailable", { status: 502 });

  logAlphaGrant({
    label, asset, version: tag,
    ip: req.headers.get("x-forwarded-for"), ua: req.headers.get("user-agent"),
  });
  return new Response(upstream.body, {
    status: 200,
    headers: {
      "content-type": "application/octet-stream",
      "content-disposition": `attachment; filename="${asset}"`,
      "cache-control": "no-store",
    },
  });
}
