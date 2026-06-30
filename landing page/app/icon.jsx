import { ImageResponse } from "next/og";

// Favicon — pixel-art chevron prompt drawn from the 8-bit logo: a ">" that
// fades blue→violet (the CONSOLE gradient) over a gold "_" prompt (STORE),
// on a black rounded square, with a small gold sparkle glint. Discrete colour
// bands (no gradient defs) so Satori renders it crisply at any size.
export const size = { width: 64, height: 64 };
export const contentType = "image/png";

export default function Icon() {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: "#000000",
          borderRadius: 14,
        }}
      >
        <svg width="50" height="50" viewBox="0 0 64 64" fill="none" shapeRendering="crispEdges">
          {/* chevron — top rows blue → violet (the CONSOLE gradient) */}
          <rect x="20" y="18" width="6" height="6" fill="#93a8ff" />
          <rect x="26" y="18" width="6" height="6" fill="#93a8ff" />
          <rect x="26" y="24" width="6" height="6" fill="#9c9af4" />
          <rect x="32" y="24" width="6" height="6" fill="#9c9af4" />
          <rect x="32" y="30" width="6" height="6" fill="#b08cf5" />
          <rect x="38" y="30" width="6" height="6" fill="#b08cf5" />
          <rect x="26" y="36" width="6" height="6" fill="#b08cf5" />
          <rect x="32" y="36" width="6" height="6" fill="#b08cf5" />
          <rect x="20" y="42" width="6" height="6" fill="#b08cf5" />
          <rect x="26" y="42" width="6" height="6" fill="#b08cf5" />
          {/* gold prompt underscore (STORE) */}
          <rect x="30" y="48" width="18" height="5" fill="#eab560" />
        </svg>
      </div>
    ),
    { ...size }
  );
}
