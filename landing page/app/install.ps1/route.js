import { readFileSync } from "node:fs";
import { join } from "node:path";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// Bundled into public/ so it ships in the deployed image regardless of
// Railway's build root; ../scripts is a fallback for a full repo checkout.
const CANDIDATES = [
  join(process.cwd(), "public", "install.ps1"),
  join(process.cwd(), "..", "scripts", "install.ps1"),
];

export function GET() {
  for (const path of CANDIDATES) {
    try {
      const body = readFileSync(path, "utf8");
      return new Response(body, {
        status: 200,
        headers: { "content-type": "text/plain; charset=utf-8", "cache-control": "no-store" },
      });
    } catch {
      // try next candidate
    }
  }
  return new Response("installer unavailable", { status: 500 });
}
