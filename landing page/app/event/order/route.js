import { ensureSchema, query } from "../../_lib/db.js";
import { validEvent, rateLimited } from "../../_lib/telemetry.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function POST(req) {
  const ip = req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() || "unknown";
  if (rateLimited(ip)) return new Response("rate limited", { status: 429 });

  let body;
  try {
    body = await req.json();
  } catch {
    return new Response("bad json", { status: 400 });
  }
  const v = validEvent(body, { requireOrderKey: true });
  if (!v.ok) return new Response("bad event: " + v.error, { status: 400 });

  try {
    await ensureSchema();
    await query(
      `INSERT INTO orders (order_key, install_id, channel, version)
       VALUES ($1, $2, $3, $4)
       ON CONFLICT (order_key) DO NOTHING`,
      [v.value.order_key, v.value.install_id, v.value.channel, v.value.version],
    );
  } catch (e) {
    console.error("order insert failed:", e.message);
    return new Response("db error", { status: 500 });
  }
  return new Response(null, { status: 204 });
}
