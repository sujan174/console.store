import { ImageResponse } from "next/og";

// Brand favicon (terminal-prompt mark), rendered to PNG so Google's favicon
// crawler reliably picks it up.
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
          gap: 5,
          background: "#0e0f18",
          borderRadius: 16,
          border: "2px solid #2a2e47"
        }}
      >
        <span style={{ color: "#c0caf5", fontSize: 40, fontWeight: 700, marginTop: -4 }}>
          {">"}
        </span>
        <div style={{ width: 14, height: 6, borderRadius: 3, background: "#7aa2f7", marginTop: 14 }} />
      </div>
    ),
    { ...size }
  );
}
