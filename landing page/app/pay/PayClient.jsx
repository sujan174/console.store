"use client";

import { useEffect, useState } from "react";

const mono = '"JetBrains Mono", ui-monospace, monospace';

// MAX_WINDOW_MS bounds how far in the future a deadline may sit for the branded
// scan-to-pay page to render. A legitimate link from the consolestore terminal
// carries Swiggy's ~5-min window; anything much larger (or missing) is treated
// as an untrusted/hand-crafted link and refused, so the real origin's brand
// can't wrap an arbitrary UPI intent for phishing.
const MAX_WINDOW_MS = 60 * 60 * 1000; // 60 minutes

// PayClient renders the scan QR + UPI-app button, but ONLY for a link with a
// valid, near-future deadline (exp, unix millis). A missing/expired/too-distant
// deadline hides the QR and the ₹ amount and shows an invalid notice. The payee
// VPA is always displayed so the user can eyeball the recipient.
export default function PayClient({ svg, amt, upi, exp, payee }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const hasDeadline = exp > 0;
  const msLeft = hasDeadline ? exp - now : 0;
  // A trusted link has a deadline that is in the future AND within the expected
  // window. No deadline, an expired one, or an implausibly distant one is refused.
  const trusted = hasDeadline && msLeft > 0 && msLeft <= MAX_WINDOW_MS;
  const secLeft = Math.max(0, Math.floor(msLeft / 1000));
  const mm = Math.floor(secLeft / 60);
  const ss = String(secLeft % 60).padStart(2, "0");

  if (!trusted) {
    const expired = hasDeadline && msLeft <= 0;
    return (
      <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: "16px", maxWidth: "420px" }}>
        <div style={{ fontSize: "40px" }}>⌛</div>
        <div style={{ color: "#e9ebf7", fontSize: "20px", fontWeight: 600 }}>
          {expired ? "this payment link expired" : "this payment link isn't valid"}
        </div>
        <div style={{ fontSize: "13.5px", color: "#b6bce0", lineHeight: 1.7 }}>
          for your safety, this page only shows a QR for a fresh link opened from the
          consolestore terminal's payment screen. go back to the terminal and place the
          order again for a scannable QR.
        </div>
      </div>
    );
  }

  return (
    <>
      <div style={{ color: "#e9ebf7", fontSize: "22px", fontWeight: 600 }}>
        scan to pay{amt ? ` ₹${amt}` : ""}
      </div>
      {payee && (
        <div style={{ fontSize: "13px", color: "#b6bce0" }}>
          to <span style={{ color: "#e9ebf7" }}>{payee}</span>
        </div>
      )}
      <div style={{ fontSize: "13px", color: "#eab560" }}>expires in {mm}:{ss}</div>
      <div
        style={{
          background: "#ffffff",
          padding: "18px",
          borderRadius: "16px",
          boxShadow: "0 0 40px rgba(147,168,255,.15)",
          width: "min(340px, 82vw)",
          lineHeight: 0
        }}
        dangerouslySetInnerHTML={{ __html: svg }}
      />
      <div style={{ fontSize: "13.5px", color: "#b6bce0", lineHeight: 1.7 }}>
        scan with any UPI app · GPay · PhonePe · Paytm · BHIM
      </div>
      <a
        href={upi}
        style={{
          marginTop: "4px",
          display: "inline-block",
          padding: "12px 22px",
          borderRadius: "10px",
          background: "#eab560",
          color: "#030307",
          fontWeight: 700,
          fontFamily: mono,
          textDecoration: "none"
        }}
      >
        open in your UPI app ↗
      </a>
      <div style={{ fontSize: "12px", color: "#5c608a", maxWidth: "360px", lineHeight: 1.6 }}>
        on your phone the button opens your UPI app directly. on a computer, scan the
        code above. pay before the timer runs out.
      </div>
    </>
  );
}
