import { ImageResponse } from "next/og";

export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

// Shared Tokyo-night social card used by both opengraph-image and twitter-image.
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
            "radial-gradient(900px 500px at 80% -10%, rgba(122,162,247,0.22), transparent 60%), radial-gradient(700px 460px at 0% 110%, rgba(187,154,247,0.18), transparent 60%), #07070c",
          fontFamily: "monospace",
          color: "#c0caf5"
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 18, fontSize: 30, color: "#7aa2f7" }}>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              width: 56,
              height: 56,
              borderRadius: 14,
              border: "2px solid #2a2e47",
              background: "#0e0f18",
              color: "#c0caf5",
              fontSize: 34
            }}
          >
            ›
          </div>
          <span style={{ color: "#8b93b8" }}>// terminal-native ordering</span>
        </div>

        <div
          style={{
            display: "flex",
            fontSize: 120,
            fontWeight: 700,
            letterSpacing: -4,
            marginTop: 28,
            color: "#c0caf5"
          }}
        >
          console<span style={{ color: "#7aa2f7" }}>.store</span>
        </div>

        <div style={{ display: "flex", fontSize: 40, color: "#8b93b8", marginTop: 18 }}>
          dinner, piped through your terminal.
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 14, marginTop: 44, fontSize: 28 }}>
          <span style={{ color: "#565f89" }}>$</span>
          <span style={{ color: "#9ece6a" }}>curl -fsSL consolestore.in/install | sh</span>
        </div>
      </div>
    ),
    { ...size }
  );
}
