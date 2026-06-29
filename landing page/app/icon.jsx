import { ImageResponse } from "next/og";

// Favicon — pure black circle, large white bowl+steam mark, no glow, centered.
// Tight viewBox crops to the mark so it fills the circle.
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
          borderRadius: 32,
        }}
      >
        <svg width="54" height="54" viewBox="10 20 42 28" fill="none">
          <path
            d="M15 25 L26 34 L15 43"
            stroke="#ffffff"
            strokeWidth="6"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <rect x="30" y="41" width="20" height="5" rx="2.5" fill="#ffffff" />
          <path d="M34 31 a6 6 0 0 0 12 0 Z" fill="#ffffff" />
          <path d="M38 28 Q40 25 38 22" stroke="#ffffff" strokeWidth="2" fill="none" strokeLinecap="round" />
          <path d="M43 28 Q45 25 43 22" stroke="#ffffff" strokeWidth="2" fill="none" strokeLinecap="round" />
        </svg>
      </div>
    ),
    { ...size }
  );
}
