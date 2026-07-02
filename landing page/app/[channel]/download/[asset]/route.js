import { latestTag, ghAssetURL, checkAlphaCode, logAlphaGrant } from "../../../_lib/channels.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const CHANNELS = new Set(["stable", "beta", "alpha"]);
const ASSET_RE = /^store_[a-z0-9]+_[a-z0-9]+(\.exe)?$/;

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

  // All channels REDIRECT to the public GitHub asset rather than proxying the
  // bytes. Alpha used to stream server-side to keep the code gate meaningful,
  // but proxying a ~10MB binary through the single Railway Node process stalled
  // intermittently (body stream never completing) — the client updater then
  // hung or 502'd and silently aborted the self-update, stranding testers on the
  // old build. The release assets are public on a public repo (the GitHub URL is
  // guessable from the tag), so streaming added no real secrecy — we still gate
  // on the invite code and log the grant here, then hand off the bytes to
  // GitHub's CDN, which is what beta/stable already do reliably.
  if (channel === "alpha") {
    logAlphaGrant({
      label, asset, version: tag,
      ip: req.headers.get("x-forwarded-for"), ua: req.headers.get("user-agent"),
    });
  }
  return Response.redirect(target, 302);
}
