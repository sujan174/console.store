import { Pool } from "pg";

// One pool per server process. DATABASE_URL is injected by Railway Postgres.
let _pool;
function pool() {
  if (!_pool) {
    const url = process.env.DATABASE_URL || "";
    // Railway's internal network (*.railway.internal) speaks plain TCP — forcing
    // SSL there fails ("server does not support SSL"). The public proxy host does
    // require SSL. Production uses the internal host, so this path is normally a
    // no-op; when a public-proxy/staging URL IS used, set DATABASE_CA (the
    // server's CA PEM) to verify the cert. Without it we fall back to accepting
    // the cert unverified — the documented Railway workaround, but a MITM window
    // on a public host, hence the CA override.
    const needsSSL = url !== "" && !url.includes(".railway.internal");
    const ca = process.env.DATABASE_CA || "";
    _pool = new Pool({
      connectionString: url,
      ssl: needsSSL ? (ca ? { ca } : { rejectUnauthorized: false }) : undefined,
      max: 4,
    });
  }
  return _pool;
}

export function query(text, params) {
  return pool().query(text, params);
}

let _schemaReady;
// ensureSchema creates the tables once per process (idempotent).
export function ensureSchema() {
  if (!_schemaReady) {
    _schemaReady = query(`
      CREATE TABLE IF NOT EXISTS installs (
        install_id TEXT PRIMARY KEY,
        channel    TEXT,
        version    TEXT,
        first_seen TIMESTAMPTZ DEFAULT now(),
        last_seen  TIMESTAMPTZ DEFAULT now()
      );
      CREATE TABLE IF NOT EXISTS orders (
        order_key  TEXT PRIMARY KEY,
        install_id TEXT,
        channel    TEXT,
        version    TEXT,
        placed_at  TIMESTAMPTZ DEFAULT now()
      );
      CREATE TABLE IF NOT EXISTS waitlist (
        email      TEXT PRIMARY KEY,
        source     TEXT,
        created_at TIMESTAMPTZ DEFAULT now()
      );
    `).catch((e) => {
      _schemaReady = undefined; // allow retry on next request
      throw e;
    });
  }
  return _schemaReady;
}
