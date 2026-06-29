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
          background: "#ffffff",
        }}
      >
        <svg width="132" height="132" viewBox="0 0 64 64" fill="none">
          <path
            d="M15 25 L26 34 L15 43"
            stroke="#4a6fd4"
            strokeWidth="6"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <rect x="30" y="41" width="20" height="4.6" rx="2.3" fill="#4a6fd4" />
          <path d="M34 31 a6 6 0 0 0 12 0 Z" fill="#4a6fd4" />
          <path d="M38 28 Q40 25 38 22" stroke="#4a6fd4" strokeWidth="1.9" fill="none" strokeLinecap="round" />
          <path d="M43 28 Q45 25 43 22" stroke="#4a6fd4" strokeWidth="1.9" fill="none" strokeLinecap="round" />
        </svg>
      </div>
    ),
    { ...size }
  );
}
