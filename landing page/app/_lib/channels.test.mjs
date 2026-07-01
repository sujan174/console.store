import { test } from "node:test";
import assert from "node:assert/strict";
import { checkAlphaCode, ghAssetURL, pickLatestTag } from "./channels.js";

// GitHub's /releases order is unreliable: here the newest beta (beta.10) sorts
// BELOW beta.3, and beta.10 must still beat beta.9 numerically ("10" < "9" as a
// string). pickLatestTag must pick by version, not list position.
test("pickLatestTag picks the highest version, not the first match", () => {
  const tags = [
    "v0.1.0-beta.9",
    "v0.1.0-beta.8",
    "v0.1.0-beta.3",
    "v0.1.0-beta.10",
    "v0.1.0-alpha.27",
    "v0.1.0-alpha.26",
    "v0.1.0",
  ];
  assert.equal(pickLatestTag(tags, "beta"), "v0.1.0-beta.10");
  assert.equal(pickLatestTag(tags, "alpha"), "v0.1.0-alpha.27");
  assert.equal(pickLatestTag(tags, "stable"), "v0.1.0");
});

test("pickLatestTag prefers a higher base and stable over prerelease", () => {
  assert.equal(
    pickLatestTag(["v0.1.0-beta.10", "v0.2.0-beta.1"], "beta"),
    "v0.2.0-beta.1",
  );
  assert.equal(
    pickLatestTag(["v0.1.0-beta.2", "v0.1.0"], "stable"),
    "v0.1.0",
  );
  assert.equal(pickLatestTag(["v0.1.0-beta.1"], "stable"), null);
});

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
