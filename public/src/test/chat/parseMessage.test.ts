import { describe, expect, it } from "vitest";
import { parseMessageContent } from "@/lib/chat/parseMessage";

const FENCE = "`".repeat(3);

describe("parseMessageContent", () => {
  it("returns one text segment for plain text", () => {
    expect(parseMessageContent("hello\nworld")).toEqual([{ kind: "text", text: "hello\nworld" }]);
  });

  it("splits text and a fenced block and reads the language", () => {
    const content = ["Intro line.", "", `${FENCE}bash`, "sha256sum big-file.iso", FENCE, "", "Outro line."].join("\n");
    expect(parseMessageContent(content)).toEqual([
      { kind: "text", text: "Intro line." },
      { kind: "code", code: "sha256sum big-file.iso", lang: "bash", closed: true },
      { kind: "text", text: "Outro line." },
    ]);
  });

  it("treats an unterminated fence as a code block in progress", () => {
    expect(parseMessageContent(`Intro.\n${FENCE}bash\nsha256`)).toEqual([
      { kind: "text", text: "Intro." },
      { kind: "code", code: "sha256", lang: "bash", closed: false },
    ]);
    // The fence line itself is still arriving: nothing is shown as raw backticks.
    const partial = parseMessageContent(`Intro.\n${FENCE}ba`);
    expect(partial.map((segment) => segment.kind)).toEqual(["text", "code"]);
    expect(JSON.stringify(partial)).not.toContain(FENCE);
  });

  it("keeps the text around a one-line fence", () => {
    expect(parseMessageContent(`use ${FENCE}x${FENCE} here`)).toEqual([
      { kind: "text", text: "use " },
      { kind: "code", code: "x", lang: null, closed: true },
      { kind: "text", text: " here" },
    ]);
  });

  it("keeps a non-language info line as code instead of dropping it", () => {
    expect(parseMessageContent(`${FENCE}not a lang\ncode line\n${FENCE}`)).toEqual([
      { kind: "code", code: "not a lang\ncode line", lang: null, closed: true },
    ]);
    expect(parseMessageContent(`${FENCE}\ncode\n${FENCE}`)).toEqual([
      { kind: "code", code: "code", lang: null, closed: true },
    ]);
  });

  it("returns nothing for empty or blank content", () => {
    expect(parseMessageContent("")).toEqual([]);
    expect(parseMessageContent("  \n ")).toEqual([]);
  });
});
