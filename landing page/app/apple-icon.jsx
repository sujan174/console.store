import { ImageResponse } from "next/og";

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
          gap: 14,
          background: "#0e0f18"
        }}
      >
        <span style={{ color: "#c0caf5", fontSize: 110, fontWeight: 700, marginTop: -10 }}>
          {">"}
        </span>
        <div style={{ width: 40, height: 16, borderRadius: 8, background: "#7aa2f7", marginTop: 38 }} />
      </div>
    ),
    { ...size }
  );
}
