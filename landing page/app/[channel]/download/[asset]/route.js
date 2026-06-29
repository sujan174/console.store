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
  const tag = await latestTag(channel);
  if (!tag) return new Response("no release", { status: 404 });
  const target = ghAssetURL(tag, asset);

  if (channel !== "alpha") {
    return Response.redirect(target, 302);
  }

  const code = new URL(req.url).searchParams.get("code") || req.headers.get("x-console-code") || "";
  const chk = checkAlphaCode(code);
  if (!chk.ok) return new Response("alpha is invite-only", { status: 403 });

  const upstream = await fetch(target, { headers: { "User-Agent": "consolestore-landing" } });
  if (!upstream.ok || !upstream.body) return new Response("asset missing", { status: 502 });

  logAlphaGrant({
    label: chk.label, asset, version: tag,
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
