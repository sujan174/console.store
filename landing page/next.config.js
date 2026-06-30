/** @type {import('next').NextConfig} */
const nextConfig = {
  poweredByHeader: false,
  // The landing page is a faithful 1:1 port of a single-mount Claude Design
  // doc: logic.js runs imperative canvas/animation code once in a useEffect and
  // tears it down on unmount. StrictMode's dev-only double-invoke (mount →
  // unmount → mount) races the fonts-gated hero wordmark animation — the
  // synthetic unmount kills the rAF loop mid-sweep. Disabling it makes dev
  // match production (which never double-invokes), so the hero renders.
  reactStrictMode: false
};

module.exports = nextConfig;
