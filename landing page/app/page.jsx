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
        <h1>consolestore — order food from your terminal, or from Claude</h1>
        <p>
          consolestore (also written consolestore.in) is a terminal-native CLI
          and TUI for ordering food through Swiggy — and an MCP server that lets
          Claude (Claude Desktop and Claude Code) place real orders for you.
          Browse restaurants, reorder a saved preset, or just tell Claude “order
          my usual” — real orders from the command line or your AI agent, no
          browser, no mouse.
        </p>
      </header>
      <div ref={hostRef} dangerouslySetInnerHTML={{ __html: MARKUP }} />
    </>
  );
}
