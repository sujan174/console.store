"use client";

import { useEffect, useState } from "react";

const mono = '"JetBrains Mono", ui-monospace, monospace';

// PayClient renders the scan QR + UPI-app button, but ONLY inside the payment
// window. exp is the unix-millis deadline (Swiggy's 5-min maxTimeToPollForInMs,
// passed from the terminal). Past it, paying would just be refunded async — so
// the page hard-invalidates: it hides the QR and shows an "expired" notice, with
// a live countdown until then. exp <= 0 means "no deadline given" (never expires,
// e.g. a manually built link).
export default function PayClient({ svg, amt, upi, exp }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);

  const hasDeadline = exp > 0;
  const msLeft = hasDeadline ? exp - now : Infinity;
  const expired = hasDeadline && msLeft <= 0;
  const secLeft = Math.max(0, Math.floor(msLeft / 1000));
  const mm = Math.floor(secLeft / 60);
  const ss = String(secLeft % 60).padStart(2, "0");

  if (expired) {
    return (
      <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: "16px", maxWidth: "420px" }}>
        <div style={{ fontSize: "40px" }}>⌛</div>
        <div style={{ color: "#e9ebf7", fontSize: "20px", fontWeight: 600 }}>this payment link expired</div>
        <div style={{ fontSize: "13.5px", color: "#b6bce0", lineHeight: 1.7 }}>
          for your safety, paying an expired link is disabled — it would be refunded.
          go back to the consolestore terminal and place the order again for a fresh QR.
        </div>
      </div>
    );
  }

  return (
    <>
      <div style={{ color: "#e9ebf7", fontSize: "22px", fontWeight: 600 }}>
        scan to pay{amt ? ` ₹${amt}` : ""}
      </div>
      {hasDeadline && (
        <div style={{ fontSize: "13px", color: "#eab560" }}>expires in {mm}:{ss}</div>
      )}
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
