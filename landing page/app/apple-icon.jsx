import { ImageResponse } from "next/og";

// Apple touch icon — same pixel chevron prompt as the favicon, full-bleed black
// (iOS applies its own corner mask), scaled up via the shared 64-unit viewBox.
export const size = { width: 180, height: 180 };
export const contentType = "image/png";

export default function AppleIcon() {
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
        }}
      >
        <svg width="132" height="132" viewBox="0 0 64 64" fill="none" shapeRendering="crispEdges">
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
          <rect x="30" y="48" width="18" height="5" fill="#eab560" />
        </svg>
      </div>
    ),
    { ...size }
  );
}
