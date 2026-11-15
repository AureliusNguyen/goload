/**
 * The mocked assistant for the first slice. Replies are canned text delivered
 * on timers so the streaming UI is exercised. Nothing here talks to a server,
 * and every reply says it is simulated: the UI must never imply that a
 * download happened.
 */
export const START_DELAY_MS = 350;
export const CHUNK_DELAY_MS = 40;

export interface StreamHandlers {
  onDelta: (delta: string) => void;
  onDone: () => void;
}

export interface StreamOptions {
  startDelayMs?: number;
  chunkDelayMs?: number;
  reducedMotion?: boolean;
}

const FENCE = "`".repeat(3);
const LINK_PATTERN = /https?:\/\/\S+|magnet:\?\S+/i;
const DOWNLOAD_PATTERN = /\b(download|torrent|magnet|queue|mirror)\b/i;
const COMMAND_PATTERN = /\b(code|command|script|snippet|checksum)\b/i;

export function pickMockReply(userText: string): string {
  if (LINK_PATTERN.test(userText) || DOWNLOAD_PATTERN.test(userText)) {
    return [
      "Simulated reply: this preview has no backend.",
      "In the real product I would hand that to goload as a download task and report its progress here.",
      "Right now nothing was fetched, queued, or saved.",
    ].join(" ");
  }
  if (COMMAND_PATTERN.test(userText)) {
    return [
      "Simulated reply: here is a sample command. Nothing ran on any machine.",
      "",
      `${FENCE}bash`,
      "# sample only",
      "sha256sum big-file.iso",
      FENCE,
      "",
      "Compare the printed hash with the one the publisher lists.",
    ].join("\n");
  }
  return [
    "Simulated reply: the assistant is not connected yet.",
    "This workspace previews the chat layout, so I cannot search, read files, or start downloads.",
    "Send a link or ask for a command to see the other canned replies.",
  ].join(" ");
}

/** Word-sized chunks that keep their whitespace: joining them restores the text. */
export function chunkReply(text: string): string[] {
  return text.match(/\S+\s*|\s+/g) ?? [];
}

export function prefersReducedMotion(): boolean {
  return (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  );
}

/**
 * Feeds `text` to `handlers` on timers and returns a cancel function. With
 * `reducedMotion` the reply arrives as a single piece: no typing effect.
 */
export function streamReply(text: string, handlers: StreamHandlers, options: StreamOptions = {}): () => void {
  const { startDelayMs = START_DELAY_MS, chunkDelayMs = CHUNK_DELAY_MS, reducedMotion = false } = options;
  const whole = text === "" ? [] : [text];
  const chunks = reducedMotion ? whole : chunkReply(text);
  let index = 0;
  let cancelled = false;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const step = () => {
    if (cancelled) return;
    if (index >= chunks.length) {
      handlers.onDone();
      return;
    }
    handlers.onDelta(chunks[index]);
    index += 1;
    if (index >= chunks.length) {
      handlers.onDone();
      return;
    }
    timer = setTimeout(step, chunkDelayMs);
  };

  timer = setTimeout(step, startDelayMs);

  return () => {
    cancelled = true;
    if (timer !== undefined) clearTimeout(timer);
  };
}
