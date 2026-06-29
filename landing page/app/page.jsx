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
        <h1>consolestore — terminal-native food ordering</h1>
        <p>
          consolestore (also written consolestore.in) is a terminal-native CLI
          and TUI for ordering food through Swiggy. Browse restaurants, reorder a
          saved preset, and place real orders from the command line in a few
          keystrokes — no browser, no mouse.
        </p>
      </header>
      <div ref={hostRef} dangerouslySetInnerHTML={{ __html: MARKUP }} />
    </>
  );
}
