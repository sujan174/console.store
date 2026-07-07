// Inline-SVG icon helper. Replaces the Tabler <i class="ti ti-*"> icons that
// rendered blank in the sandbox (no icon font is bundled — see the redesign
// spec at .superpowers/sdd/order-app-redesign-spec.md). Every icon is a small
// hand-written Feather/Tabler-style line glyph, stroke=currentColor, so it
// inherits the surrounding text color and themes for free.

export type IconName =
  | "plus"
  | "minus"
  | "arrow-left"
  | "map-pin"
  | "check-circle"
  | "alert-triangle"
  | "alert-circle"
  | "lock"
  | "loader";

const PATHS: Record<IconName, string> = {
  plus: `<path d="M12 5v14M5 12h14"/>`,
  minus: `<path d="M5 12h14"/>`,
  "arrow-left": `<path d="M19 12H5M12 19l-7-7 7-7"/>`,
  "map-pin": `<path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0Z"/><circle cx="12" cy="10" r="3"/>`,
  "check-circle": `<path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><path d="m9 11 3 3L22 4"/>`,
  "alert-triangle": `<path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"/><path d="M12 9v4M12 17h.01"/>`,
  "alert-circle": `<circle cx="12" cy="12" r="10"/><path d="M12 8v4M12 16h.01"/>`,
  lock: `<rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>`,
  loader: `<path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83"/>`,
};

// icon renders an inline <svg> for `name` at `size` px. `loader` gets the
// `.spin` class (styles.ts defines the keyframes, reduced-motion guarded).
export function icon(name: IconName, size = 16): string {
  const cls = name === "loader" ? ` class="spin"` : "";
  return (
    `<svg width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" ` +
    `stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"${cls}>${PATHS[name]}</svg>`
  );
}
