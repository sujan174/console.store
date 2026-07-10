"use client";

import { useEffect, useRef } from "react";
import { MARKUP } from "./markup";
import { mount } from "./logic";

export default function Home() {
  const hostRef = useRef(null);

  useEffect(() => {
    const cleanup = mount(hostRef.current);
    return cleanup;
  }, []);

  return (
    <>
      {/* Crawlable brand/keyword text. The visual wordmark is a <canvas>, so
          this gives search engines and screen readers real, indexable copy. */}
      <header className="sr-only">
        <h1>consolestore — order food from your terminal, or just ask Claude</h1>
        <p>
          consolestore (also written consolestore.in) is a terminal-native CLI
          and TUI for ordering food and Instamart groceries through Swiggy —
          and an MCP server with a full ordering app that runs inside Claude
          (Claude Desktop and Claude Code). Browse restaurants, reorder a saved
          preset in one line (console order dinner), or tell Claude what you
          want to eat — real Swiggy orders with a real bill and your explicit
          confirmation, from the command line or your AI agent. No browser, no
          mouse.
        </p>
      </header>
      <div ref={hostRef} dangerouslySetInnerHTML={{ __html: MARKUP }} />
    </>
  );
}
