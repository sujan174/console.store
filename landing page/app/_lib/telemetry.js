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

// clientIP extracts the trustworthy client IP from the request headers. The
// platform edge proxy (Railway) APPENDS the real client IP to X-Forwarded-For,
// so the RIGHT-MOST entry is the one it set; the left-most values are
// client-supplied and spoofable. Keying the rate limiter on the left-most value
// let an attacker mint a fresh bucket per request (rotate XFF) and defeat the
// cap entirely. Falls back to a platform header, then "unknown".
export function clientIP(headers) {
  const xff = headers.get("x-forwarded-for");
  if (xff) {
    const parts = xff.split(",").map((s) => s.trim()).filter(Boolean);
    if (parts.length) return parts[parts.length - 1];
  }
  return headers.get("x-real-ip")?.trim() || "unknown";
}

// In-memory per-IP token bucket: 30 requests / 60s window. Single-instance only.
const WINDOW_MS = 60_000;
const CAP = 30;
// MAX_BUCKETS bounds memory: once exceeded, expired buckets are swept and, if
// still over, the map is cleared. A spoofable key made this map grow without
// bound before; even keyed on the real IP a busy instance shouldn't grow it
// unboundedly.
const MAX_BUCKETS = 10_000;
const buckets = new Map(); // ip -> {count, resetAt}

function sweep(now) {
  for (const [k, b] of buckets) {
    if (now >= b.resetAt) buckets.delete(k);
  }
  if (buckets.size > MAX_BUCKETS) buckets.clear(); // hard backstop
}

export function rateLimited(ip, now = Date.now()) {
  if (buckets.size >= MAX_BUCKETS) sweep(now);
  const b = buckets.get(ip);
  if (!b || now >= b.resetAt) {
    buckets.set(ip, { count: 1, resetAt: now + WINDOW_MS });
    return false;
  }
  b.count += 1;
  return b.count > CAP;
}

// MAX_BODY_BYTES caps the request body an event endpoint will parse. App-Router
// handlers don't apply the Pages-API bodyParser size limit, so without this the
// whole body is buffered before validation.
export const MAX_BODY_BYTES = 4096;

// tooLarge reports whether the request's declared body exceeds the cap. Reject
// BEFORE calling req.json() so an oversized/absent-length body is never buffered.
export function tooLarge(headers) {
  const len = Number(headers.get("content-length"));
  return !Number.isFinite(len) || len > MAX_BODY_BYTES;
}

// shapeStats normalizes the SQL aggregate into the public /stats JSON shape.
export function shapeStats(agg) {
  const series = (agg.series || []).map((r) => ({
    week: r.week,
    installs: Number(r.installs) || 0,
    orders: Number(r.orders) || 0,
  }));
  return {
    orders: Number(agg.orders) || 0,
    installs: Number(agg.installs) || 0,
    active_installs: Number(agg.active_installs) || 0,
    orders_week: Number(agg.orders_week) || 0,
    installs_week: Number(agg.installs_week) || 0,
    series,
  };
}
