import { ensureSchema, query } from "../_lib/db.js";
import { shapeStats } from "../_lib/telemetry.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// demoStats is served ONLY in local dev when no Postgres is wired
// (DATABASE_URL unset) so the live-stats chip + growth chart are developable
// without a database. Production always has DATABASE_URL, so this never fires
// there. Shape matches the real /stats payload exactly.
function demoStats() {
  const s = [
    { week: "w-8", installs: 41, orders: 12 },
    { week: "w-7", installs: 58, orders: 19 },
    { week: "w-6", installs: 90, orders: 31 },
    { week: "w-5", installs: 128, orders: 44 },
    { week: "w-4", installs: 171, orders: 63 },
    { week: "w-3", installs: 219, orders: 88 },
    { week: "w-2", installs: 264, orders: 111 },
    { week: "w-1", installs: 312, orders: 140 },
  ];
  return shapeStats({
    installs: 312, active_installs: 47, installs_week: 48,
    orders: 140, orders_week: 29, series: s,
  });
}

export async function GET() {
  if (!process.env.DATABASE_URL) {
    return new Response(JSON.stringify(demoStats()), {
      status: 200,
      headers: { "content-type": "application/json", "cache-control": "no-store" },
    });
  }
  try {
    await ensureSchema();
    const totals = await query(
      `SELECT
         (SELECT count(*) FROM installs) AS installs,
         (SELECT count(*) FROM installs WHERE last_seen  > now() - interval '7 days') AS active_installs,
         (SELECT count(*) FROM installs WHERE first_seen > now() - interval '7 days') AS installs_week,
         (SELECT count(*) FROM orders)  AS orders,
         (SELECT count(*) FROM orders   WHERE placed_at  > now() - interval '7 days') AS orders_week`,
    );

    // Cumulative install/order totals at the end of each of the last 8 weeks —
    // a growth curve ending at "now" that the drawer renders as a chart.
    const series = await query(
      `WITH weeks AS (
         SELECT generate_series(
           date_trunc('week', now()) - interval '7 weeks',
           date_trunc('week', now()),
           interval '1 week'
         ) AS wk
       )
       SELECT to_char(w.wk, 'YYYY-MM-DD') AS week,
         (SELECT count(*) FROM installs WHERE first_seen < w.wk + interval '1 week') AS installs,
         (SELECT count(*) FROM orders   WHERE placed_at  < w.wk + interval '1 week') AS orders
       FROM weeks w ORDER BY w.wk`,
    );

    const out = shapeStats({ ...totals.rows[0], series: series.rows });

    return new Response(JSON.stringify(out), {
      status: 200,
      headers: { "content-type": "application/json", "cache-control": "public, max-age=60" },
    });
  } catch (e) {
    console.error("stats failed:", e.message);
    return new Response(JSON.stringify({ orders: 0, installs: 0, active_installs: 0, orders_week: 0, installs_week: 0, series: [] }), {
      status: 503,
      headers: { "content-type": "application/json", "cache-control": "no-store" },
    });
  }
}
