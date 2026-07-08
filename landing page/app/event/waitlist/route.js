import { ensureSchema, query } from "../../_lib/db.js";
import { rateLimited } from "../../_lib/telemetry.js";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// Minimal email shape check — the DB is the source of truth (email is the PK,
// so duplicates are a no-op). We only guard against obvious junk + oversize.
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export async function POST(req) {
  const ip = req.headers.get("x-forwarded-for")?.split(",")[0]?.trim() || "unknown";
  if (rateLimited(ip)) return new Response("rate limited", { status: 429 });

  let body;
  try {
    body = await req.json();
  } catch {
    return new Response("bad json", { status: 400 });
  }

  const email = (body?.email || "").toString().trim().toLowerCase();
  if (!EMAIL.test(email) || email.length > 254) {
    return new Response("bad email", { status: 400 });
  }
  const source = (body?.source || "web").toString().slice(0, 40);

  try {
    await ensureSchema();
    await query(
      `INSERT INTO waitlist (email, source)
       VALUES ($1, $2)
       ON CONFLICT (email) DO NOTHING`,
      [email, source],
    );
  } catch (e) {
    console.error("waitlist insert failed:", e.message);
    return new Response("db error", { status: 500 });
  }
  return new Response(null, { status: 204 });
}
