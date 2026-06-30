import { ensureSchema, query } from "../_lib/db.js";
import { shapeStats } from "../_lib/telemetry.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

export async function GET() {
  try {
    await ensureSchema();
    const totals = await query(
      `SELECT
         (SELECT count(*) FROM installs) AS installs,
         (SELECT count(*) FROM installs WHERE last_seen > now() - interval '7 days') AS active_installs,
         (SELECT count(*) FROM orders) AS orders`,
    );
    const byInstalls = await query(`SELECT channel, count(*) AS installs FROM installs GROUP BY channel`);
    const byOrders = await query(`SELECT channel, count(*) AS orders FROM orders GROUP BY channel`);

    const merged = {};
    for (const r of byInstalls.rows) merged[r.channel] = { channel: r.channel, installs: r.installs, orders: 0 };
    for (const r of byOrders.rows) {
      merged[r.channel] = merged[r.channel] || { channel: r.channel, installs: 0, orders: 0 };
      merged[r.channel].orders = r.orders;
    }
    const out = shapeStats({ ...totals.rows[0], by_channel: Object.values(merged) });

    return new Response(JSON.stringify(out), {
      status: 200,
      headers: { "content-type": "application/json", "cache-control": "public, max-age=60" },
    });
  } catch (e) {
    console.error("stats failed:", e.message);
    return new Response(JSON.stringify({ orders: 0, installs: 0, active_installs: 0, by_channel: {} }), {
      status: 503,
      headers: { "content-type": "application/json", "cache-control": "no-store" },
    });
  }
}
