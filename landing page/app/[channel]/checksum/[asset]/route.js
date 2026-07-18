import { latestTag, fetchSignedManifest, checkAlphaCode, verifyManifest } from "../../../_lib/channels.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const CHANNELS = new Set(["stable", "beta", "alpha"]);
const ASSET_RE = /^store_[a-z0-9]+_[a-z0-9]+(\.exe)?$/;

// Returns the hex sha256 for an asset as text/plain, read from the signed
// manifest envelope. The install scripts use this to verify downloads without
// needing jq or base64 JSON parsing in shell.
export async function GET(req, { params }) {
  const { channel, asset } = await params;
  if (!CHANNELS.has(channel) || !ASSET_RE.test(asset)) {
    return new Response("bad request", { status: 400 });
  }
  if (channel === "alpha") {
    const code = new URL(req.url).searchParams.get("code") || req.headers.get("x-console-code") || "";
    if (!checkAlphaCode(code).ok) return new Response("alpha is invite-only", { status: 403 });
  }
  const tag = await latestTag(channel);
  if (!tag) return new Response("no release", { status: 404 });

  const body = await fetchSignedManifest(tag);
  if (body === null) return new Response("manifest missing", { status: 502 });
  // VERIFY the ed25519 signature before trusting any field from the manifest —
  // a compromised GitHub asset (poisoned payload + checksum, without the signing
  // key) is caught here rather than served to the installer. verifyManifest
  // also absorbs malformed-JSON, returning null instead of throwing an uncaught
  // 500.
  const payload = verifyManifest(body);
  if (payload === null) return new Response("manifest verification failed", { status: 502 });
  // asset name → asset key: strip "store_" prefix and ".exe" suffix.
  const key = asset.replace(/^store_/, "").replace(/\.exe$/, "");
  const sum = payload.assets?.[key];
  if (!sum) return new Response("unknown asset", { status: 404 });
  return new Response(sum + "\n", {
    status: 200,
    headers: { "content-type": "text/plain", "cache-control": "no-store" },
  });
}
