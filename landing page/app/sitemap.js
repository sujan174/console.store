const SITE = "https://consolestore.in";

export default function sitemap() {
  return [
    {
      url: SITE,
      changeFrequency: "weekly",
      priority: 1
    },
    {
      url: SITE + "/how-to",
      changeFrequency: "monthly",
      priority: 0.8
    }
  ];
}
