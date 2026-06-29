import { readFileSync } from "node:fs";
import { join } from "node:path";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

// The installer is bundled into the Next app's public/ dir so it is always
// present in the deployed image regardless of Railway's build root. public/
// is kept in sync with the canonical scripts/install.sh (repo root); the
// ../scripts path is a fallback for running from a full repo checkout.
const CANDIDATES = [
  join(process.cwd(), "public", "install.sh"),
  join(process.cwd(), "..", "scripts", "install.sh"),
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
