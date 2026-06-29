import { latestTag, ghAssetURL, checkAlphaCode, logAlphaGrant } from "../../_lib/channels.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const CHANNELS = new Set(["stable", "beta", "alpha"]);

export async function GET(req, { params }) {
  const { channel } = await params;
  if (!CHANNELS.has(channel)) {
    return new Response("unknown channel", { status: 404 });
  }

  const code = new URL(req.url).searchParams.get("code") || req.headers.get("x-console-code") || "";
  let label = null;
  if (channel === "alpha") {
    const chk = checkAlphaCode(code);
    if (!chk.ok) return new Response("alpha is invite-only", { status: 403 });
    label = chk.label;
  }

  const tag = await latestTag(channel);
  if (!tag) return new Response("no release for channel", { status: 404 });

  // The signed envelope is a release asset; pass it through verbatim so the
  // ed25519 signature stays valid.
  const upstream = await fetch(ghAssetURL(tag, "console-manifest.json"), {
    headers: { "User-Agent": "consolestore-landing" },
  });
  if (!upstream.ok) return new Response("manifest missing", { status: 502 });
  const body = await upstream.text();

  if (channel === "alpha") {
    logAlphaGrant({
      label, asset: "manifest", version: tag,
      ip: req.headers.get("x-forwarded-for"), ua: req.headers.get("user-agent"),
    });
  }
  return new Response(body, {
    status: 200,
    headers: { "content-type": "application/json", "cache-control": "no-store" },
  });
}
