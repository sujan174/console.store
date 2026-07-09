import QRCode from "qrcode";

export const metadata = {
  title: "pay — consolestore",
  description: "Scan to pay for your consolestore order via UPI.",
  robots: { index: false }
};

// The terminal opens /pay?upi=<url-encoded upi:// intent> when it can't render a
// scannable QR itself (transparent terminals, small windows). This page renders
// a big, reliable QR of that intent — scan it with any UPI app — plus a direct
// "open in your UPI app" link that deep-links the app on a phone. Everything is
// derived from the query string; no data is stored or sent anywhere.

function amountFromUPI(upi) {
  try {
    const u = new URL(upi);
    const am = u.searchParams.get("am") || u.searchParams.get("amount");
    if (am) return Math.round(parseFloat(am));
  } catch {}
  return 0;
}

export default async function Pay({ searchParams }) {
  const sp = await searchParams;
  const raw = sp?.upi;
  const upi = Array.isArray(raw) ? raw[0] : raw || "";
  const valid = upi.startsWith("upi://");
  const amt = valid ? amountFromUPI(upi) : 0;

  let svg = "";
  if (valid) {
    svg = await QRCode.toString(upi, {
      type: "svg",
      margin: 2,
      width: 320,
      color: { dark: "#0b0b12", light: "#ffffff" }
    });
  }

  const mono = '"JetBrains Mono", ui-monospace, monospace';

  return (
    <main
      style={{
        minHeight: "100vh",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        gap: "22px",
        padding: "40px 20px",
        background: "#030307",
        color: "#8a8fb4",
        fontFamily: mono,
        textAlign: "center"
      }}
    >
      <div style={{ fontSize: "15px", letterSpacing: ".02em" }}>
        <span style={{ color: "#eab560" }}>~ %</span>{" "}
        <span style={{ color: "#e9ebf7" }}>consolestore</span>
      </div>

      {valid ? (
        <>
          <div style={{ color: "#e9ebf7", fontSize: "22px", fontWeight: 600 }}>
            scan to pay{amt ? ` ₹${amt}` : ""}
          </div>
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
            on your phone the button opens your UPI app directly. on a computer,
            scan the code above. this order stays pending until you pay.
          </div>
        </>
      ) : (
        <div style={{ color: "#e9ebf7", fontSize: "16px", maxWidth: "420px", lineHeight: 1.7 }}>
          no payment to show — open this from the consolestore terminal's payment
          screen, or place an order first.
        </div>
      )}
    </main>
  );
}
