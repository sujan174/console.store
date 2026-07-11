import "./styles.css";

const SITE = "https://consolestore.in";

export const metadata = {
  metadataBase: new URL(SITE),
  title: {
    default: "consolestore — order food from your terminal | consolestore.in",
    template: "%s | consolestore.in"
  },
  description:
    "consolestore (consolestore.in) is a terminal-native CLI and TUI for ordering food through Swiggy. Browse restaurants, reorder a saved preset, and check out real orders straight from your shell — no browser, no mouse.",
  applicationName: "consolestore",
  authors: [{ name: "consolestore" }],
  creator: "consolestore",
  publisher: "consolestore",
  category: "technology",
  keywords: [
    "console store",
    "consolestore",
    "consolestore.in",
    "terminal food ordering",
    "CLI food ordering",
    "terminal-native ordering",
    "order food from terminal",
    "unofficial Swiggy ordering CLI",
    "command line ordering",
    "TUI food app",
    "developer tools"
  ],
  openGraph: {
    title: "consolestore — order food from your terminal",
    description:
      "A Tokyo Night terminal shop for hungry builders. Search, cart, checkout, and track real Swiggy orders from the command line.",
    url: SITE,
    siteName: "consolestore",
    type: "website",
    locale: "en_US"
  },
  twitter: {
    card: "summary_large_image",
    title: "consolestore — order food from your terminal",
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
  themeColor: "#030307",
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
      name: "consolestore",
      alternateName: ["consolestore", "consolestore.in", "console store"],
      url: SITE,
      description:
        "Terminal-native CLI food ordering shop. Order real food from your terminal."
    },
    {
      "@type": "WebSite",
      "@id": `${SITE}/#website`,
      url: SITE,
      name: "consolestore",
      alternateName: ["consolestore", "console store", "consolestore.in"],
      publisher: { "@id": `${SITE}/#org` },
      inLanguage: "en"
    },
    {
      "@type": "SoftwareApplication",
      name: "consolestore",
      alternateName: "consolestore",
      applicationCategory: "DeveloperApplication",
      operatingSystem: "macOS, Linux, Windows",
      url: SITE,
      description:
        "An independent, unofficial terminal-native CLI that places orders through Swiggy's MCP APIs (not affiliated with Swiggy). Browse, search, cart, checkout, and track delivery without leaving the shell.",
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
          href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:ital,wght@0,400;0,500;0,600;0,700;0,800;1,400&display=swap"
          rel="stylesheet"
        />
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
        />
        {/* Always open/reload at the top of the page. Disabling the browser's
            scroll restoration means a refresh (or a fresh visit) never lands
            mid-page; shared #anchor links still work because we only force the
            top when there's no hash. Runs on every route (home, features, how-to). */}
        <script
          dangerouslySetInnerHTML={{
            __html:
              "try{if('scrollRestoration' in history){history.scrollRestoration='manual';}window.addEventListener('load',function(){if(!location.hash){window.scrollTo(0,0);}});}catch(e){}"
          }}
        />
      </head>
      <body>{children}</body>
    </html>
  );
}
