import { describe, expect, test } from "bun:test";
import { formatTime, splitList } from "./format";

describe("splitList", () => {
  test("splits on commas and trims each entry", () => {
    expect(splitList("backend, auth")).toEqual(["backend", "auth"]);
  });

  test("surrounding and repeated whitespace is dropped", () => {
    expect(splitList("  backend ,   auth  ")).toEqual(["backend", "auth"]);
  });

  test("empty entries are dropped rather than sent as empty labels", () => {
    expect(splitList("backend,,auth,")).toEqual(["backend", "auth"]);
  });

  test("a blank field is no entries at all, not one empty one", () => {
    expect(splitList("")).toEqual([]);
    expect(splitList("   ")).toEqual([]);
    expect(splitList(",")).toEqual([]);
  });

  test("a single entry needs no comma", () => {
    expect(splitList("component/backend")).toEqual(["component/backend"]);
  });

  test("spaces inside an entry are kept: only the comma separates", () => {
    expect(splitList("needs review, in progress")).toEqual(["needs review", "in progress"]);
  });
});

describe("formatTime", () => {
  test("an RFC 3339 stamp is shown as a local time, not as the stored string", () => {
    const stamp = "2026-08-25T09:42:00+02:00";
    const shown = formatTime(stamp);
    expect(shown).not.toBe(stamp);
    expect(shown).not.toContain("T");
    expect(shown).toContain("2026");
  });

  test("what is not a time is passed through untouched", () => {
    expect(formatTime("not a time")).toBe("not a time");
    expect(formatTime("")).toBe("");
  });
});
