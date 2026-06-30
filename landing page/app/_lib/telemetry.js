const CHANNELS = new Set(["alpha", "beta", "stable"]);
const MAXLEN = 80;

function str(v) {
  return typeof v === "string" ? v : "";
}

// validEvent validates a telemetry payload shape. Returns {ok, value} or {ok:false, error}.
export function validEvent(body, { requireOrderKey } = {}) {
  const install_id = str(body?.install_id);
  const channel = str(body?.channel);
  const version = str(body?.version);
  const order_key = str(body?.order_key);
  if (install_id.length < 8 || install_id.length > MAXLEN) return { ok: false, error: "install_id" };
  if (!CHANNELS.has(channel)) return { ok: false, error: "channel" };
  if (version.length < 1 || version.length > MAXLEN) return { ok: false, error: "version" };
  if (requireOrderKey && (order_key.length < 8 || order_key.length > MAXLEN)) return { ok: false, error: "order_key" };
  return { ok: true, value: { install_id, channel, version, order_key } };
}

// In-memory per-IP token bucket: 30 requests / 60s window. Single-instance only.
const WINDOW_MS = 60_000;
const CAP = 30;
const buckets = new Map(); // ip -> {count, resetAt}

export function rateLimited(ip, now = Date.now()) {
  const b = buckets.get(ip);
  if (!b || now >= b.resetAt) {
    buckets.set(ip, { count: 1, resetAt: now + WINDOW_MS });
    return false;
  }
  b.count += 1;
  return b.count > CAP;
}

// shapeStats normalizes the SQL aggregate into the public /stats JSON shape.
export function shapeStats(agg) {
  const by_channel = {};
  for (const row of agg.by_channel || []) {
    by_channel[row.channel] = { installs: Number(row.installs) || 0, orders: Number(row.orders) || 0 };
  }
  return {
    orders: Number(agg.orders) || 0,
    installs: Number(agg.installs) || 0,
    active_installs: Number(agg.active_installs) || 0,
    by_channel,
  };
}
