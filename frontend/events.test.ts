import { describe, expect, test } from "bun:test";
import { backoffDelay, RECONNECT_MAX_MS, RECONNECT_MIN_MS } from "./events";

describe("backoffDelay", () => {
  test("the first retry is quick, because most drops are momentary", () => {
    expect(backoffDelay(0)).toBe(RECONNECT_MIN_MS);
  });

  test("it doubles", () => {
    expect(backoffDelay(1)).toBe(RECONNECT_MIN_MS * 2);
    expect(backoffDelay(2)).toBe(RECONNECT_MIN_MS * 4);
    expect(backoffDelay(3)).toBe(RECONNECT_MIN_MS * 8);
  });

  test("and stops doubling, so a server that is gone is asked about calmly", () => {
    expect(backoffDelay(20)).toBe(RECONNECT_MAX_MS);
  });

  test("an attempt count that overflows the double still gives a number", () => {
    // 2 ** 1100 is Infinity, and Math.min(Infinity, cap) is the cap — but only
    // because that is checked for; the naive version returns NaN for -Infinity.
    expect(backoffDelay(1100)).toBe(RECONNECT_MAX_MS);
    expect(Number.isFinite(backoffDelay(1100))).toBe(true);
  });

  test("a negative attempt is treated as the first", () => {
    expect(backoffDelay(-1)).toBe(RECONNECT_MIN_MS);
  });
});
