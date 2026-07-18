import { ensureSchema, query } from "../../_lib/db.js";
import { validEvent, rateLimited, clientIP, tooLarge } from "../../_lib/telemetry.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function POST(req) {
  if (tooLarge(req.headers)) return new Response("payload too large", { status: 413 });
  const ip = clientIP(req.headers);
  if (rateLimited(ip)) return new Response("rate limited", { status: 429 });

  let body;
  try {
    body = await req.json();
  } catch {
    return new Response("bad json", { status: 400 });
  }
  const v = validEvent(body, {});
  if (!v.ok) return new Response("bad event: " + v.error, { status: 400 });

  try {
    await ensureSchema();
    await query(
      `INSERT INTO installs (install_id, channel, version)
       VALUES ($1, $2, $3)
       ON CONFLICT (install_id)
       DO UPDATE SET last_seen = now(), channel = EXCLUDED.channel, version = EXCLUDED.version`,
      [v.value.install_id, v.value.channel, v.value.version],
    );
  } catch (e) {
    console.error("launch insert failed:", e.message);
    return new Response("db error", { status: 500 });
  }
  return new Response(null, { status: 204 });
}
