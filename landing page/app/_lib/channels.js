export const GITHUB_REPO = "sujan174/console.store";

export function ghAssetURL(tag, asset) {
  return `https://github.com/${GITHUB_REPO}/releases/download/${tag}/${asset}`;
}

// latestTag returns the newest release tag for a channel. stable = a tag with
// no -alpha/-beta suffix; beta/alpha = the matching prerelease suffix.
export async function latestTag(channel) {
  const res = await fetch(`https://api.github.com/repos/${GITHUB_REPO}/releases?per_page=30`, {
    headers: { Accept: "application/vnd.github+json", "User-Agent": "consolestore-landing" },
    next: { revalidate: 60 },
  });
  if (!res.ok) return null;
  const releases = await res.json();
  const match = (tag) => {
    const isAlpha = tag.includes("-alpha");
    const isBeta = tag.includes("-beta");
    if (channel === "alpha") return isAlpha;
    if (channel === "beta") return isBeta;
    return !isAlpha && !isBeta; // stable
  };
  for (const r of releases) {
    if (r.draft) continue;
    if (match(r.tag_name)) return r.tag_name;
  }
  return null;
}

export function checkAlphaCode(code) {
  if (!code) return { ok: false, label: null };
  const raw = process.env.CONSOLE_ALPHA_CODES || "";
  for (const pair of raw.split(",")) {
    const [label, value] = pair.split(":");
    if (value && value === code) return { ok: true, label: label || "unknown" };
  }
  return { ok: false, label: null };
}

export function logAlphaGrant({ label, asset, version, ip, ua }) {
  console.log(
    `alpha-grant who=${label} asset=${asset || "-"} version=${version || "-"} ip=${ip || "-"} ua=${JSON.stringify(ua || "-")}`,
  );
}

export async function ghReleaseBody(tag) {
  const res = await fetch(`https://api.github.com/repos/${GITHUB_REPO}/releases/tags/${tag}`, {
    headers: { "User-Agent": "consolestore-landing" },
  });
  if (!res.ok) return null;
  const data = await res.json();
  const body = data.body;
  if (!body || !body.trim()) return null;
  return body;
}
