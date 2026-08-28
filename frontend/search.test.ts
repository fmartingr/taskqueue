import { describe, expect, test } from "bun:test";
import type { ExcludedTerm, Filters, TextTerm } from "./board";
import {
  applyCompletion,
  canonicalValues,
  completeQuery,
  equalFilters,
  parseQuery,
  queryFromURL,
  sameFilters,
  tokenize,
  urlWithQuery,
  NO_FILTERS,
  type Sources,
} from "./search";

const filters = (overrides: Partial<Filters>): Filters => ({
  ...NO_FILTERS,
  text: [],
  excluded: [],
  ...overrides,
});

/** One free-text term the query has to find, and one it must not. */
const word = (value: string): TextTerm => ({ value, negated: false });
const not = (value: string): TextTerm => ({ value, negated: true });

const without = (key: ExcludedTerm["key"], value: string): ExcludedTerm => ({ key, value });

/** Stands in for a project's vocabularies and the assignees its tasks carry. */
const SOURCES: Sources = {
  status: [
    { value: "inbox", display: "Inbox" },
    { value: "todo", display: "To do" },
    { value: "in-progress", display: "In Progress" },
    { value: "done", display: "Done" },
  ],
  priority: [
    { value: "urgent", display: "Urgent" },
    { value: "high", display: "High" },
    { value: "normal", display: "Normal" },
  ],
  label: [
    { value: "bug", display: "Bug" },
    { value: "component/api", display: "Component | API" },
  ],
  assignee: [{ value: "agent-api" }, { value: "agent ui" }],
};

describe("tokenize", () => {
  test("splits on whitespace and records where each term is", () => {
    const tokens = tokenize("auth status=todo");
    expect(tokens.map((token) => [token.start, token.end, token.key, token.value])).toEqual([
      [0, 4, "", "auth"],
      [5, 16, "status", "todo"],
    ]);
  });

  test("a quoted value keeps its spaces", () => {
    expect(tokenize('assignee="agent ui"')).toEqual([
      {
        start: 0,
        end: 19,
        raw: 'assignee="agent ui"',
        negated: false,
        key: "assignee",
        value: "agent ui",
      },
    ]);
  });

  test("an unbalanced quote runs to the end, so a half-typed value is one term", () => {
    const tokens = tokenize('assignee="agent u');
    expect(tokens).toHaveLength(1);
    expect(tokens[0]!.value).toBe("agent u");
  });

  test("a leading dash is the negation, and a quoted one is a character", () => {
    expect(tokenize("-done").map((token) => [token.negated, token.key, token.value])).toEqual([
      [true, "", "done"],
    ]);
    expect(tokenize("-priority=low").map((token) => [token.negated, token.key, token.value])).toEqual([
      [true, "priority", "low"],
    ]);
    expect(tokenize('"-done"').map((token) => [token.negated, token.key, token.value])).toEqual([
      [false, "", "-done"],
    ]);
  });

  test("runs of whitespace are not terms", () => {
    expect(tokenize("   auth   ").map((token) => token.value)).toEqual(["auth"]);
    expect(tokenize("")).toEqual([]);
  });
});

describe("parseQuery", () => {
  test("an empty query constrains nothing", () => {
    expect(parseQuery("")).toEqual(NO_FILTERS);
    expect(parseQuery("   ")).toEqual(NO_FILTERS);
  });

  test("reads each structured key", () => {
    expect(parseQuery("status=todo priority=urgent label=bug assignee=agent-api")).toEqual(
      filters({ status: "todo", priority: "urgent", label: "bug", assignee: "agent-api" }),
    );
  });

  test("keys are case-insensitive; values are taken as typed", () => {
    // The board matches without case (see board.ts), so a value is not
    // corrected here — the line stays what was typed.
    expect(parseQuery("PRIORITY=Urgent")).toEqual(filters({ priority: "Urgent" }));
  });

  test("bare ready means ready, and ready=false means not", () => {
    expect(parseQuery("ready").ready).toBe(true);
    expect(parseQuery("ready=true").ready).toBe(true);
    expect(parseQuery("ready=yes").ready).toBe(true);
    expect(parseQuery("ready=").ready).toBe(true);
    expect(parseQuery("ready=false").ready).toBe(false);
    expect(parseQuery("ready=no").ready).toBe(false);
  });

  test("every bare word is a term of its own, and all of them have to match", () => {
    expect(parseQuery("global search bar").text).toEqual([word("global"), word("search"), word("bar")]);
  });

  test("a quoted run stays one phrase", () => {
    expect(parseQuery('"global search" bar').text).toEqual([word("global search"), word("bar")]);
  });

  test("an unknown key is free text, kept whole", () => {
    expect(parseQuery("owner=nobody")).toEqual(filters({ text: [word("owner=nobody")] }));
  });

  test("a key with no value is no constraint", () => {
    expect(parseQuery("priority=")).toEqual(NO_FILTERS);
  });

  test("a key repeated keeps the last one", () => {
    expect(parseQuery("priority=high priority=urgent").priority).toBe("urgent");
  });

  test("quotes hold a value with spaces", () => {
    expect(parseQuery('assignee="agent ui"').assignee).toBe("agent ui");
  });

  test("a quoted term is text, whatever it looks like", () => {
    expect(parseQuery('"priority=urgent"')).toEqual(filters({ text: [word("priority=urgent")] }));
    expect(parseQuery('"ready"')).toEqual(filters({ text: [word("ready")] }));
    expect(parseQuery('"-done"')).toEqual(filters({ text: [word("-done")] }));
  });

  test("text and terms mix in any order", () => {
    expect(parseQuery("status=todo oidc ready")).toEqual(
      filters({ status: "todo", text: [word("oidc")], ready: true }),
    );
  });

  describe("negation", () => {
    test("a word is excluded", () => {
      expect(parseQuery("auth -done")).toEqual(filters({ text: [word("auth"), not("done")] }));
    });

    test("a phrase is excluded whole", () => {
      expect(parseQuery('-"global search"')).toEqual(filters({ text: [not("global search")] }));
    });

    test("a key's value is excluded, and several can be", () => {
      expect(parseQuery("-priority=low -priority=normal")).toEqual(
        filters({ excluded: [without("priority", "low"), without("priority", "normal")] }),
      );
    });

    test("every key can be excluded", () => {
      expect(parseQuery('-status=done -label=bug -assignee="agent ui"').excluded).toEqual([
        without("status", "done"),
        without("label", "bug"),
        without("assignee", "agent ui"),
      ]);
    });

    test("an exclusion does not take the positive slot for its key", () => {
      expect(parseQuery("priority=high -priority=low")).toEqual(
        filters({ priority: "high", excluded: [without("priority", "low")] }),
      );
    });

    test("-ready is ready=false: readiness is a yes or a no, with no third state", () => {
      expect(parseQuery("-ready").ready).toBe(false);
      expect(parseQuery("ready -ready").ready).toBe(false);
      expect(parseQuery("-ready=false").ready).toBe(true);
    });

    test("an unknown key negated is negated free text", () => {
      expect(parseQuery("-owner=nobody")).toEqual(filters({ text: [not("owner=nobody")] }));
    });

    test("a lone dash is a negation being typed, not a term", () => {
      expect(parseQuery("-")).toEqual(NO_FILTERS);
      expect(parseQuery("auth -")).toEqual(filters({ text: [word("auth")] }));
    });
  });
});

describe("equalFilters", () => {
  test("compares every field", () => {
    expect(equalFilters(NO_FILTERS, { ...NO_FILTERS })).toBe(true);
    expect(equalFilters(NO_FILTERS, filters({ text: [word("a")] }))).toBe(false);
    expect(equalFilters(NO_FILTERS, filters({ ready: true }))).toBe(false);
    expect(equalFilters(NO_FILTERS, filters({ excluded: [without("label", "bug")] }))).toBe(false);
    expect(equalFilters(filters({ status: "todo" }), filters({ status: "done" }))).toBe(false);
  });

  test("free text is compared in order, sign included", () => {
    expect(equalFilters(filters({ text: [word("a"), word("b")] }), filters({ text: [word("a")] }))).toBe(
      false,
    );
    expect(equalFilters(filters({ text: [word("a")] }), filters({ text: [not("a")] }))).toBe(false);
    expect(equalFilters(filters({ text: [word("a")] }), filters({ text: [word("A")] }))).toBe(true);
  });

  test("case is not a difference, because nothing matches on it", () => {
    expect(equalFilters(filters({ status: "todo" }), filters({ status: "TODO" }))).toBe(true);
    expect(
      equalFilters(
        filters({ excluded: [without("label", "Bug")] }),
        filters({ excluded: [without("label", "bug")] }),
      ),
    ).toBe(true);
  });
});

describe("NO_FILTERS", () => {
  test("is frozen, arrays included, so a copy that aliases it cannot be pushed into", () => {
    expect(Object.isFrozen(NO_FILTERS)).toBe(true);
    expect(Object.isFrozen(NO_FILTERS.text)).toBe(true);
    expect(Object.isFrozen(NO_FILTERS.excluded)).toBe(true);
    expect(() => ({ ...NO_FILTERS }).text.push(word("a"))).toThrow();
  });

  test("parseQuery hands back arrays of its own", () => {
    const parsed = parseQuery("");
    expect(parsed.text).not.toBe(NO_FILTERS.text);
    expect(parsed.excluded).not.toBe(NO_FILTERS.excluded);
    expect(Object.isFrozen(parsed.text)).toBe(false);
  });
});

describe("sameFilters", () => {
  // The other question `equalFilters` does not answer: not "do these two hide
  // the same cards" but "is this the same set down to its spelling", which is
  // what decides whether a correction is written through.
  test("a difference of case in a canonicalised field is a difference", () => {
    const typed = filters({ status: "TODO" });
    const canonical = filters({ status: "todo" });
    expect(equalFilters(typed, canonical)).toBe(true);
    expect(sameFilters(typed, canonical)).toBe(false);
  });

  test("every field canonicalValues rewrites is compared exactly", () => {
    expect(sameFilters(filters({ priority: "Urgent" }), filters({ priority: "urgent" }))).toBe(false);
    expect(sameFilters(filters({ label: "BUG" }), filters({ label: "bug" }))).toBe(false);
  });

  test("the assignee is not, because nothing canonicalises it", () => {
    expect(sameFilters(filters({ assignee: "AGENT" }), filters({ assignee: "agent" }))).toBe(true);
  });

  test("anything equalFilters calls different is different here too", () => {
    expect(sameFilters(NO_FILTERS, filters({ ready: true }))).toBe(false);
    expect(sameFilters(filters({ text: [word("a")] }), filters({ text: [not("a")] }))).toBe(false);
    expect(sameFilters(NO_FILTERS, filters({}))).toBe(true);
  });

  test("a correction the vocabularies make is seen, which is the whole point", () => {
    const typed = parseQuery("status=INBOX label=BUG");
    const corrected = canonicalValues(typed, SOURCES);
    expect(equalFilters(corrected, typed)).toBe(true);
    expect(sameFilters(corrected, typed)).toBe(false);
    expect([corrected.status, corrected.label]).toEqual(["inbox", "bug"]);
  });
});

describe("canonicalValues", () => {
  test("replaces a mis-cased value with the project's own spelling", () => {
    const corrected = canonicalValues(parseQuery("status=TODO priority=Urgent label=BUG"), SOURCES);
    expect([corrected.status, corrected.priority, corrected.label]).toEqual(["todo", "urgent", "bug"]);
  });

  test("leaves a value the project does not declare alone", () => {
    expect(canonicalValues(parseQuery("status=Elsewhere"), SOURCES).status).toBe("Elsewhere");
  });

  test("leaves the assignee alone: it is a substring of a freeform name", () => {
    expect(canonicalValues(parseQuery("assignee=AGENT"), SOURCES).assignee).toBe("AGENT");
  });

  test("what it corrects hides no different cards, but is still a correction", () => {
    const typed = parseQuery("status=TODO");
    const corrected = canonicalValues(typed, SOURCES);
    // The two narrow the board identically…
    expect(equalFilters(corrected, typed)).toBe(true);
    // …and the correction is still written through, which is `sameFilters`' job.
    expect(sameFilters(corrected, typed)).toBe(false);
  });
});

describe("completeQuery", () => {
  const complete = (query: string, caret = query.length) => completeQuery(query, caret, SOURCES);
  const labels = (query: string, caret?: number) =>
    complete(query, caret).suggestions.map((suggestion) => suggestion.label);

  test("an empty query offers every key", () => {
    expect(labels("")).toEqual(["status=", "priority=", "label=", "assignee=", "ready"]);
  });

  test("a prefix narrows the keys", () => {
    expect(labels("pri")).toEqual(["priority="]);
    expect(labels("re")).toEqual(["ready"]);
  });

  test("free text that matches no key suggests nothing", () => {
    expect(labels("oidc")).toEqual([]);
  });

  test("a fixed key offers its values, from the project", () => {
    expect(labels("status=")).toEqual(["inbox", "todo", "in-progress", "done"]);
    expect(labels("priority=")).toEqual(["urgent", "high", "normal"]);
    expect(labels("label=")).toEqual(["bug", "component/api"]);
    expect(labels("assignee=")).toEqual(["agent-api", "agent ui"]);
    expect(labels("ready=")).toEqual(["true", "false"]);
  });

  test("values are matched on the value or its display name, prefixes first", () => {
    // "done" and "todo" both carry "do"; the one that starts with it comes first.
    expect(labels("status=do")).toEqual(["done", "todo"]);
    expect(labels("label=API")).toEqual(["component/api"]);
  });

  test("a value with spaces is suggested quoted", () => {
    const suggestions = complete("assignee=agent").suggestions;
    expect(suggestions.map((suggestion) => suggestion.insert)).toEqual([
      "assignee=agent-api",
      'assignee="agent ui"',
    ]);
  });

  test("a negation carries its dash into what it suggests", () => {
    expect(labels("-")).toEqual(["-status=", "-priority=", "-label=", "-assignee=", "-ready"]);
    expect(labels("-pri")).toEqual(["-priority="]);
    expect(complete("-priority=ur").suggestions.map((suggestion) => suggestion.insert)).toEqual([
      "-priority=urgent",
    ]);
  });

  test("the span it replaces is the whole term the caret is in", () => {
    const completion = complete("oidc status=to ready", 14);
    expect([completion.start, completion.end]).toEqual([5, 14]);
    expect(completion.suggestions.map((suggestion) => suggestion.label)).toEqual(["todo"]);
  });

  test("a caret in whitespace starts a new term", () => {
    const completion = complete("oidc ", 5);
    expect([completion.start, completion.end]).toEqual([5, 5]);
    expect(completion.suggestions).toHaveLength(5);
  });
});

describe("applyCompletion", () => {
  const accept = (query: string, caret = query.length, at = 0) => {
    const completion = completeQuery(query, caret, SOURCES);
    return applyCompletion(query, completion, completion.suggestions[at]!);
  };

  test("a key stops at its equals, so the values come up next", () => {
    expect(accept("pri")).toEqual({ query: "priority=", caret: 9 });
  });

  test("a value finishes the term and leaves a space to type in", () => {
    expect(accept("priority=urg")).toEqual({ query: "priority=urgent ", caret: 16 });
  });

  test("it replaces only its own term", () => {
    expect(accept("oidc status=to ready", 14)).toEqual({
      query: "oidc status=todo ready",
      caret: 16,
    });
  });

  test("bare ready is accepted whole", () => {
    expect(accept("rea")).toEqual({ query: "ready ", caret: 6 });
  });

  test("a negation keeps its dash all the way through", () => {
    expect(accept("-pri")).toEqual({ query: "-priority=", caret: 10 });
    expect(accept("-priority=nor")).toEqual({ query: "-priority=normal ", caret: 17 });
  });
});

describe("the address bar", () => {
  test("reads the query out of a URL", () => {
    expect(queryFromURL("http://127.0.0.1:7331/?q=priority%3Durgent")).toBe("priority=urgent");
    expect(queryFromURL("http://127.0.0.1:7331/")).toBe("");
    expect(queryFromURL("not a url")).toBe("");
  });

  test("writes it back as a relative reference, keeping whatever else is there", () => {
    expect(urlWithQuery("http://127.0.0.1:7331/", "status=todo")).toBe("/?q=status%3Dtodo");
    expect(urlWithQuery("http://127.0.0.1:7331/?tab=2", "oidc")).toBe("/?tab=2&q=oidc");
  });

  test("an empty query drops the parameter rather than leaving ?q= behind", () => {
    expect(urlWithQuery("http://127.0.0.1:7331/?q=oidc", "")).toBe("/");
    expect(urlWithQuery("http://127.0.0.1:7331/?q=oidc&tab=2", "  ")).toBe("/?tab=2");
  });

  test("round-trips a query with quotes and dashes in it", () => {
    const query = '-"two words" priority=urgent';
    expect(queryFromURL("http://x" + urlWithQuery("http://x/", query))).toBe(query);
  });
});
