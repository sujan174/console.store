import { readFileSync } from "node:fs";
import { join } from "node:path";

export const runtime = "nodejs";
export const dynamic = "force-static";

// scripts/install.ps1 is the source of truth, committed at the repo root. The
// landing app reads it at build/start time (one dir up from "landing page").
export function GET() {
  const path = join(process.cwd(), "..", "scripts", "install.ps1");
  let body;
  try {
    body = readFileSync(path, "utf8");
  } catch {
    return new Response("installer unavailable", { status: 500 });
  }
  return new Response(body, {
    status: 200,
    headers: { "content-type": "text/plain; charset=utf-8", "cache-control": "no-store" },
  });
}
