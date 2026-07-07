import { App } from "@modelcontextprotocol/ext-apps";

const root = document.getElementById("app")!;
const app = new App({ name: "consolestore order", version: "0.1.0" });
app.connect();

// The open_store tool result is pushed here on first render.
app.ontoolresult = (result) => {
  const sc = (result as any).structuredContent ?? {};
  const name = sc?.restaurant?.name ?? "unknown";
  const count = Array.isArray(sc?.menu?.items) ? sc.menu.items.length : 0;
  root.textContent = `Loaded ${name} — ${count} items.`;
};
