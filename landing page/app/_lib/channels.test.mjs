import { test } from "node:test";
import assert from "node:assert/strict";
import { checkAlphaCode, ghAssetURL } from "./channels.js";

test("checkAlphaCode validates per-person codes", () => {
  process.env.CONSOLE_ALPHA_CODES = "alice:a1b2,bob:c3d4";
  assert.deepEqual(checkAlphaCode("c3d4"), { ok: true, label: "bob" });
  assert.deepEqual(checkAlphaCode("nope"), { ok: false, label: null });
  assert.deepEqual(checkAlphaCode(""), { ok: false, label: null });
});

test("ghAssetURL builds a release download URL", () => {
  assert.equal(
    ghAssetURL("v0.1.0", "store_linux_amd64"),
    "https://github.com/sujan174/console.store/releases/download/v0.1.0/store_linux_amd64",
  );
});
