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

test("shapeStats aggregates totals + weekly deltas + growth series", () => {
  const out = shapeStats({
    installs: 10,
    active_installs: 4,
    installs_week: 3,
    orders: 7,
    orders_week: 2,
    series: [
      { week: "2026-06-22", installs: "6", orders: "4" },
      { week: "2026-06-29", installs: "10", orders: "7" },
    ],
  });
  assert.equal(out.orders, 7);
  assert.equal(out.installs, 10);
  assert.equal(out.active_installs, 4);
  assert.equal(out.installs_week, 3);
  assert.equal(out.orders_week, 2);
  assert.equal(out.series.length, 2);
  assert.equal(out.series[1].installs, 10); // strings coerced to numbers
  assert.equal(out.series[0].orders, 4);
});
