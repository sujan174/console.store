import { checkAlphaCode, logAlphaGrant, ghReleaseBody, parseTag } from "../../../_lib/channels.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const CHANNELS = new Set(["stable", "beta", "alpha"]);

export async function GET(req, { params }) {
  const { channel, version } = await params;
  if (!CHANNELS.has(channel)) {
    return new Response("unknown channel", { status: 404 });
  }
  // Validate the version tag before it reaches the GitHub API URL (defense in
  // depth — ghReleaseBody also rejects it). Sibling routes validate their params.
  if (!parseTag(version)) {
    return new Response("bad version", { status: 400 });
  }

  const code = new URL(req.url).searchParams.get("code") || req.headers.get("x-console-code") || "";
  let label = null;
  if (channel === "alpha") {
    const chk = checkAlphaCode(code);
    if (!chk.ok) return new Response("alpha is invite-only", { status: 403 });
    label = chk.label;
  }

  const body = await ghReleaseBody(version);
  if (!body) return new Response("no notes", { status: 404 });

  if (channel === "alpha") {
    logAlphaGrant({
      label, asset: "notes", version,
      ip: req.headers.get("x-forwarded-for"), ua: req.headers.get("user-agent"),
    });
  }

  const cacheControl = channel === "alpha" ? "no-store" : "s-maxage=300, stale-while-revalidate=600";
  return new Response(body, {
    status: 200,
    headers: { "content-type": "text/markdown; charset=utf-8", "cache-control": cacheControl },
  });
}
