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
        <h1>console.store — terminal-native food ordering (consolestore)</h1>
        <p>
          console.store, also written consolestore or consolestore.in, is a
          terminal-native CLI food ordering shop. Browse restaurants, rebuild
          your usual cart, and place real Swiggy orders from the command line in
          three keystrokes — no browser, no tabs, no mouse.
        </p>
      </header>
      <div ref={hostRef} dangerouslySetInnerHTML={{ __html: MARKUP }} />
    </>
  );
}
