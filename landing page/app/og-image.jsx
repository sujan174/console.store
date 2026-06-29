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
              width: 64,
              height: 64,
              borderRadius: 14,
              border: "2px solid #2a2e47",
              background: "#0e0f18"
            }}
          >
            <svg width="44" height="44" viewBox="0 0 64 64" fill="none">
              <path d="M15 25 L26 34 L15 43" stroke="#7aa2f7" strokeWidth="6" strokeLinecap="round" strokeLinejoin="round" />
              <rect x="30" y="41" width="20" height="4.4" rx="2.2" fill="#7aa2f7" />
              <path d="M34 31 a6 6 0 0 0 12 0 Z" fill="#ffffff" />
              <path d="M38 28 Q40 25 38 22" stroke="#ffffff" strokeWidth="1.8" fill="none" strokeLinecap="round" />
              <path d="M43 28 Q45 25 43 22" stroke="#ffffff" strokeWidth="1.8" fill="none" strokeLinecap="round" />
            </svg>
          </div>
          <span style={{ color: "#8b93b8" }}>// terminal-native ordering</span>
        </div>

        <div
          style={{
            display: "flex",
            fontSize: 104,
            fontWeight: 700,
            letterSpacing: -4,
            marginTop: 28,
            color: "#c0caf5"
          }}
        >
          consolestore<span style={{ color: "#7aa2f7" }}>.in</span>
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
