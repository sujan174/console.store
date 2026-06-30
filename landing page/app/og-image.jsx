import { ImageResponse } from "next/og";

export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

// Shared social card (opengraph + twitter) — pixel-arcade skin matching the
// 8-bit CONSOLE (blue→violet) / STORE (gold) logo on pure black.
export function OgImage() {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          padding: "80px",
          background:
            "radial-gradient(900px 520px at 82% -12%, rgba(147,168,255,0.22), transparent 60%), radial-gradient(760px 480px at 0% 112%, rgba(176,140,245,0.18), transparent 60%), radial-gradient(620px 420px at 100% 110%, rgba(234,181,96,0.12), transparent 60%), #030307",
          fontFamily: "monospace",
          color: "#e9ebf7"
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 18, fontSize: 30 }}>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              width: 76,
              height: 76,
              borderRadius: 16,
              border: "2px solid rgba(147,168,255,0.25)",
              background: "#000000"
            }}
          >
            <svg width="56" height="56" viewBox="0 0 64 64" fill="none" shapeRendering="crispEdges">
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
          <span style={{ color: "#93a8ff" }}>// terminal-native ordering</span>
        </div>

        <div
          style={{
            display: "flex",
            fontSize: 116,
            fontWeight: 700,
            letterSpacing: 2,
            marginTop: 30,
            textTransform: "uppercase"
          }}
        >
          <span style={{ color: "#a6b8ff" }}>console</span>
          <span style={{ color: "#eab560" }}>store</span>
        </div>

        <div style={{ display: "flex", fontSize: 40, color: "#8a8fb4", marginTop: 20 }}>
          dinner, piped through your terminal.
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 14, marginTop: 44, fontSize: 28 }}>
          <span style={{ color: "#565b80" }}>$</span>
          <span style={{ color: "#8ee08a" }}>curl -fsSL consolestore.in/install | sh</span>
        </div>
      </div>
    ),
    { ...size }
  );
}
