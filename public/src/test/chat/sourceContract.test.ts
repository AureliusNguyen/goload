import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";

// Lint-like guards for rules a stylesheet-less jsdom cannot observe.
// Paths that do not exist yet are skipped, so this file grows with the slice.
const root = process.cwd();
const SLICE_PATHS = [
  "src/components/chat",
  "src/lib/chat",
  "src/test/chat",
  "src/contexts/ChatContext.tsx",
  "src/hooks/use-visual-viewport.ts",
  "src/pages/ChatApp.tsx",
];

function listFiles(target: string): string[] {
  const absolute = path.resolve(root, target);
  if (!existsSync(absolute)) return [];
  if (statSync(absolute).isFile()) return [target];
  return readdirSync(absolute).flatMap((entry) => listFiles(`${target}/${entry}`));
}

const sources = SLICE_PATHS.flatMap(listFiles).map((file) => ({
  file,
  text: readFileSync(path.resolve(root, file), "utf8"),
}));
const productCode = sources.filter(
  (source) => /\.tsx?$/.test(source.file) && !source.file.startsWith("src/test/"),
);

describe("chat slice source contract", () => {
  it("finds the slice files", () => {
    expect(sources.length).toBeGreaterThan(0);
    expect(productCode.length).toBeGreaterThan(0);
  });

  it("is ASCII only", () => {
    const offenders = sources.filter((source) => /[^\t\n\r\x20-\x7E]/.test(source.text)).map((source) => source.file);
    expect(offenders).toEqual([]);
  });

  it("keeps every angle in chat-theme.css", () => {
    const angled = /\d(deg|rad|turn)\b|\b(rotate|skew-[xy])-\[?-?\d|clip-path|clipPath|skew[XY]?\(|rotate[XYZ]?\(/;
    expect(productCode.filter((source) => angled.test(source.text)).map((source) => source.file)).toEqual([]);
  });

  it("reads no file contents, stores nothing and makes no network calls", () => {
    const forbidden = /FileReader|\.arrayBuffer\(|fetch\(|XMLHttpRequest|axios|localStorage|sessionStorage|indexedDB/;
    expect(productCode.filter((source) => forbidden.test(source.text)).map((source) => source.file)).toEqual([]);
  });
});
