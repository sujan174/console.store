import { ImageResponse } from "next/og";

// Favicon — plain, static (non-neon) mark: solid brand-blue bowl+steam on a
// white tile. Single colour + bold strokes so it stays legible at 16–32px in
// Google results and browser tabs.
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
          background: "#ffffff",
          borderRadius: 13,
        }}
      >
        <svg width="50" height="50" viewBox="0 0 64 64" fill="none">
          <path
            d="M15 25 L26 34 L15 43"
            stroke="#4a6fd4"
            strokeWidth="6.4"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <rect x="30" y="41" width="20" height="5" rx="2.5" fill="#4a6fd4" />
          <path d="M34 31 a6 6 0 0 0 12 0 Z" fill="#4a6fd4" />
          <path d="M38 28 Q40 25 38 22" stroke="#4a6fd4" strokeWidth="2" fill="none" strokeLinecap="round" />
          <path d="M43 28 Q45 25 43 22" stroke="#4a6fd4" strokeWidth="2" fill="none" strokeLinecap="round" />
        </svg>
      </div>
    ),
    { ...size }
  );
}
