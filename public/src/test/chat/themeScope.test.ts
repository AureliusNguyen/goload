import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

// vitest runs with public/ as the working directory.
const read = (relative: string) => readFileSync(path.resolve(process.cwd(), relative), "utf8");

interface StyleRule {
  selectors: string[];
  body: string;
}

function splitSelectors(prelude: string): string[] {
  const selectors: string[] = [];
  let depth = 0;
  let current = "";
  for (const char of prelude) {
    if (char === "(") depth += 1;
    if (char === ")") depth -= 1;
    if (char === "," && depth === 0) {
      selectors.push(current.trim());
      current = "";
    } else {
      current += char;
    }
  }
  if (current.trim() !== "") selectors.push(current.trim());
  return selectors;
}

// Minimal reader for plain CSS without nesting: returns every style rule,
// descends into @media and @supports, skips @keyframes and other at-rules.
function collectRules(css: string): StyleRule[] {
  const rules: StyleRule[] = [];
  let index = 0;
  while (index < css.length) {
    const open = css.indexOf("{", index);
    if (open === -1) break;
    const statementEnd = css.indexOf(";", index);
    if (statementEnd !== -1 && statementEnd < open) {
      index = statementEnd + 1;
      continue;
    }
    let depth = 1;
    let close = open + 1;
    while (close < css.length && depth > 0) {
      if (css[close] === "{") depth += 1;
      if (css[close] === "}") depth -= 1;
      close += 1;
    }
    const prelude = css.slice(index, open).trim();
    const body = css.slice(open + 1, close - 1);
    if (/^@(media|supports)\b/.test(prelude)) {
      rules.push(...collectRules(body));
    } else if (!prelude.startsWith("@")) {
      rules.push({ selectors: splitSelectors(prelude), body });
    }
    index = close;
  }
  return rules;
}

const css = read("src/components/chat/chat-theme.css").replace(/\/\*[\s\S]*?\*\//g, "");
const rules = collectRules(css);
const selectors = rules.flatMap((rule) => rule.selectors);

describe("chat theme scope", () => {
  it("prefixes every selector with .chat-theme", () => {
    expect(rules.length).toBeGreaterThan(10);
    expect(selectors.filter((selector) => !/^\.chat-theme(?![\w-])/.test(selector))).toEqual([]);
  });

  it("never targets html, body or :root", () => {
    expect(selectors.filter((selector) => /:root|(^|[\s>+~(,])(html|body)(?![\w-])/.test(selector))).toEqual([]);
  });

  it("leaves the shared stylesheet and tailwind config free of chat tokens", () => {
    for (const file of ["src/index.css", "tailwind.config.ts", "index.html"]) {
      expect(read(file)).not.toMatch(/chat-theme|--chat-|Dela Gothic|Geist|JetBrains/);
    }
  });

  it("zeroes the radius for the whole subtree", () => {
    expect(css).toMatch(/--radius:\s*0px/);
    const reset = rules.find((rule) => /border-radius:\s*0\s*!important/.test(rule.body));
    expect(reset?.selectors).toEqual(expect.arrayContaining([".chat-theme", ".chat-theme *"]));
  });

  it("declares the graphite and amber palette and fonts verbatim", () => {
    const lower = css.toLowerCase();
    for (const value of [
      "#151618",
      "#202226",
      "#EEEDE9",
      "#F2BE54",
      "#D7261E",
      "Dela Gothic One",
      "Geist",
      "JetBrains Mono",
    ]) {
      expect(lower).toContain(value.toLowerCase());
    }
  });

  it("derives every angle from the single --chat-tilt", () => {
    expect(css.match(/--chat-tilt:\s*-?[\d.]+deg/g)).toHaveLength(1);
    const withoutSource = css.replace(/--chat-tilt:\s*-?[\d.]+deg/, "").replace(/\b90deg\b/g, "");
    expect(withoutSource).not.toMatch(/[\d.]+(deg|rad|grad|turn)\b/);
  });

  it("keeps skew, rotate and clip-path on decoration layers only", () => {
    const shaped = rules.filter((rule) =>
      /clip-path\s*:|skew[XY]?\(|rotate[XYZ3d]*\(|(^|[;\s])rotate\s*:/.test(rule.body),
    );
    expect(shaped.length).toBeGreaterThan(0);
    const offenders = shaped
      .flatMap((rule) => rule.selectors)
      .filter((selector) => !selector.includes("[data-chat-decor"));
    expect(offenders).toEqual([]);
  });

  it("puts ink text on amber buttons", () => {
    const ember = rules.filter((rule) =>
      rule.selectors.some((selector) => selector.includes('.chat-btn[data-tone="ember"]')),
    );
    expect(ember.length).toBeGreaterThan(0);
    expect(ember.some((rule) => /(^|[;\s])color:\s*var\(--chat-ink\)/.test(rule.body))).toBe(true);
  });

  it("gives focus an outline that box-shadows and clips cannot remove", () => {
    const focus = rules.find(
      (rule) =>
        rule.selectors.some((selector) => selector.includes(":focus-visible")) &&
        /outline:\s*3px solid/.test(rule.body),
    );
    expect(focus).toBeDefined();
  });

  it("turns animation and transition off for reduced motion", () => {
    const block = css.match(/@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{([\s\S]*?\})\s*\}/);
    expect(block?.[1]).toMatch(/animation:\s*none\s*!important/);
    expect(block?.[1]).toMatch(/transition:\s*none\s*!important/);
  });
});
