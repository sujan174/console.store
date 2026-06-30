import { test } from "node:test";
import assert from "node:assert/strict";
import { validEvent, rateLimited, shapeStats } from "./telemetry.js";

test("validEvent accepts a good launch event", () => {
  const r = validEvent({ install_id: "a".repeat(36), channel: "alpha", version: "v0.1.0" }, {});
  assert.equal(r.ok, true);
  assert.equal(r.value.channel, "alpha");
});

test("validEvent rejects missing install_id", () => {
  assert.equal(validEvent({ channel: "alpha", version: "v1" }, {}).ok, false);
});

test("validEvent rejects bad channel", () => {
  assert.equal(validEvent({ install_id: "x".repeat(10), channel: "prod", version: "v1" }, {}).ok, false);
});

test("validEvent requires order_key when asked", () => {
  const base = { install_id: "x".repeat(10), channel: "beta", version: "v1" };
  assert.equal(validEvent(base, { requireOrderKey: true }).ok, false);
  assert.equal(validEvent({ ...base, order_key: "k".repeat(10) }, { requireOrderKey: true }).ok, true);
});

test("validEvent rejects oversized fields", () => {
  assert.equal(validEvent({ install_id: "x".repeat(200), channel: "beta", version: "v1" }, {}).ok, false);
});

test("rateLimited trips after the cap", () => {
  const ip = "1.2.3.4";
  const now = 1000;
  let tripped = false;
  for (let i = 0; i < 35; i++) tripped = rateLimited(ip, now) || tripped;
  assert.equal(tripped, true);
  assert.equal(rateLimited("9.9.9.9", now), false); // other IP unaffected
});

test("shapeStats aggregates totals + per-channel", () => {
  const out = shapeStats({
    installs: 10,
    active_installs: 4,
    orders: 7,
    by_channel: [
      { channel: "alpha", installs: 6, orders: 5 },
      { channel: "stable", installs: 4, orders: 2 },
    ],
  });
  assert.equal(out.orders, 7);
  assert.equal(out.installs, 10);
  assert.equal(out.active_installs, 4);
  assert.equal(out.by_channel.alpha.orders, 5);
  assert.equal(out.by_channel.stable.installs, 4);
});
