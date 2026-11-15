import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  chunkReply,
  CHUNK_DELAY_MS,
  pickMockReply,
  prefersReducedMotion,
  START_DELAY_MS,
  streamReply,
} from "@/lib/chat/mockAssistant";

const FENCE = "`".repeat(3);
const PROMPTS = [
  "Download https://example.com/big-file.iso",
  "magnet:?xt=urn:btih:abc",
  "queue this torrent",
  "show me a command",
  "what can you do?",
  "",
];

describe("pickMockReply", () => {
  it("labels every canned reply as simulated and never claims a download happened", () => {
    const claims =
      /download (has )?(started|finished|completed?)|downloaded to|saved to|added to (the |your )?queue|is downloading/i;
    for (const prompt of PROMPTS) {
      const reply = pickMockReply(prompt);
      expect(reply).toMatch(/^Simulated reply: /);
      expect(reply).not.toMatch(claims);
    }
  });

  it("answers a link or a download request with an explicit nothing-happened line", () => {
    expect(pickMockReply("Download https://example.com/big-file.iso")).toContain(
      "nothing was fetched, queued, or saved",
    );
    expect(pickMockReply("queue this torrent")).toContain("nothing was fetched, queued, or saved");
  });

  it("answers a command request with one fenced code block", () => {
    const reply = pickMockReply("show me a command");
    expect(reply).toContain(`${FENCE}bash\n`);
    expect(reply.split(FENCE)).toHaveLength(3);
  });

  it("is deterministic", () => {
    expect(pickMockReply("what can you do?")).toBe(pickMockReply("what can you do?"));
  });
});

describe("chunkReply", () => {
  it("splits into word chunks that join back to the exact text", () => {
    const text = "  one two\n\nthree  ";
    const chunks = chunkReply(text);
    expect(chunks.length).toBeGreaterThan(2);
    expect(chunks.join("")).toBe(text);
    expect(chunkReply("")).toEqual([]);
  });
});

describe("streamReply", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("emits nothing before the start delay, then every chunk in order, then done once", () => {
    const deltas: string[] = [];
    const onDone = vi.fn();
    streamReply("one two three", { onDelta: (delta) => deltas.push(delta), onDone });

    vi.advanceTimersByTime(START_DELAY_MS - 1);
    expect(deltas).toEqual([]);

    vi.advanceTimersByTime(1);
    expect(deltas).toEqual(["one "]);
    expect(onDone).not.toHaveBeenCalled();

    vi.advanceTimersByTime(CHUNK_DELAY_MS * 2);
    expect(deltas.join("")).toBe("one two three");
    expect(onDone).toHaveBeenCalledTimes(1);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("stops emitting once cancelled", () => {
    const onDelta = vi.fn();
    const onDone = vi.fn();
    const cancel = streamReply("one two three", { onDelta, onDone });
    vi.advanceTimersByTime(START_DELAY_MS);
    expect(onDelta).toHaveBeenCalledTimes(1);

    cancel();
    vi.runAllTimers();
    expect(onDelta).toHaveBeenCalledTimes(1);
    expect(onDone).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("delivers the whole reply in one piece when motion is reduced", () => {
    const onDelta = vi.fn();
    const onDone = vi.fn();
    streamReply("one two three", { onDelta, onDone }, { reducedMotion: true });
    vi.advanceTimersByTime(START_DELAY_MS);
    expect(onDelta.mock.calls).toEqual([["one two three"]]);
    expect(onDone).toHaveBeenCalledTimes(1);
  });

  it("honors custom delays and completes an empty reply without a delta", () => {
    const onDelta = vi.fn();
    const onDone = vi.fn();
    streamReply("", { onDelta, onDone }, { startDelayMs: 10, chunkDelayMs: 5 });
    vi.advanceTimersByTime(9);
    expect(onDone).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(onDelta).not.toHaveBeenCalled();
    expect(onDone).toHaveBeenCalledTimes(1);
  });
});

describe("prefersReducedMotion", () => {
  const original = window.matchMedia;
  afterEach(() => {
    window.matchMedia = original;
  });

  it("reads the reduced-motion media query", () => {
    expect(prefersReducedMotion()).toBe(false);
    window.matchMedia = ((query: string) => ({
      ...original(query),
      matches: query === "(prefers-reduced-motion: reduce)",
    })) as unknown as typeof window.matchMedia;
    expect(prefersReducedMotion()).toBe(true);
  });
});
