import "./styles.css";

const SITE = "https://consolestore.in";

export const metadata = {
  metadataBase: new URL(SITE),
  title: {
    default: "console.store — order food from your terminal | consolestore.in",
    template: "%s | consolestore.in"
  },
  description:
    "console.store (consolestore.in) is a terminal-native CLI food ordering shop. Browse restaurants, rebuild your usual cart, and check out real Swiggy orders in three keystrokes — no browser, no tabs, no mouse.",
  applicationName: "console.store",
  authors: [{ name: "console.store" }],
  creator: "console.store",
  publisher: "console.store",
  category: "technology",
  keywords: [
    "console store",
    "consolestore",
    "console.store",
    "consolestore.in",
    "terminal food ordering",
    "CLI food ordering",
    "terminal-native ordering",
    "order food from terminal",
    "Swiggy CLI",
    "command line ordering",
    "TUI food app",
    "developer tools"
  ],
  openGraph: {
    title: "console.store — order food from your terminal",
    description:
      "A Tokyo Night terminal shop for hungry builders. Search, cart, checkout, and track real Swiggy orders from the command line.",
    url: SITE,
    siteName: "console.store",
    type: "website",
    locale: "en_US"
  },
  twitter: {
    card: "summary_large_image",
    title: "console.store — order food from your terminal",
    description:
      "Terminal-native food ordering. Search, cart, checkout, and track from the command line — no browser required."
  },
  alternates: {
    canonical: "/"
  },
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      "max-image-preview": "large",
      "max-snippet": -1,
      "max-video-preview": -1
    }
  },
  // Set GOOGLE_SITE_VERIFICATION in Railway to emit the Search Console tag.
  verification: process.env.GOOGLE_SITE_VERIFICATION
    ? { google: process.env.GOOGLE_SITE_VERIFICATION }
    : undefined
};

export const viewport = {
  themeColor: "#07070c",
  colorScheme: "dark",
  width: "device-width",
  initialScale: 1
};

const jsonLd = {
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      "@id": `${SITE}/#org`,
      name: "console.store",
      alternateName: ["consolestore", "consolestore.in", "console store"],
      url: SITE,
      description:
        "Terminal-native CLI food ordering shop. Order real food from your terminal."
    },
    {
      "@type": "WebSite",
      "@id": `${SITE}/#website`,
      url: SITE,
      name: "console.store",
      alternateName: ["consolestore", "console store", "consolestore.in"],
      publisher: { "@id": `${SITE}/#org` },
      inLanguage: "en"
    },
    {
      "@type": "SoftwareApplication",
      name: "console.store",
      alternateName: "consolestore",
      applicationCategory: "DeveloperApplication",
      operatingSystem: "macOS, Linux, Windows",
      url: SITE,
      description:
        "A terminal-native CLI that brokers real Swiggy food orders. Browse, search, cart, checkout, and track delivery without leaving the shell.",
      offers: { "@type": "Offer", price: "0", priceCurrency: "USD" }
    }
  ]
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="" />
        <link
          href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;600;700&display=swap"
          rel="stylesheet"
        />
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
        />
      </head>
      <body>{children}</body>
    </html>
  );
}
