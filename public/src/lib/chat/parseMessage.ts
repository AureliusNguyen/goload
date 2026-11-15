export type MessageSegment =
  | { kind: "text"; text: string }
  | { kind: "code"; code: string; lang: string | null; closed: boolean };

const FENCE = "`".repeat(3);

function cleanLang(info: string): string | null {
  const lang = info.trim();
  return /^[A-Za-z0-9_+-]{1,20}$/.test(lang) ? lang : null;
}

function pushText(segments: MessageSegment[], raw: string): void {
  const text = raw.replace(/^\n+|\n+$/g, "");
  if (text.trim() !== "") segments.push({ kind: "text", text });
}

/**
 * Splits message content into text and fenced code segments. This is not a
 * markdown renderer: only fences are special. A fence that has not closed yet
 * (a reply cut off mid-stream) is still a code segment, with closed=false.
 * The only characters ever dropped are the fences and a recognized language
 * tag on the opening line.
 */
export function parseMessageContent(content: string): MessageSegment[] {
  const segments: MessageSegment[] = [];
  let rest = content;
  while (rest.length > 0) {
    const open = rest.indexOf(FENCE);
    if (open === -1) {
      pushText(segments, rest);
      break;
    }
    pushText(segments, rest.slice(0, open));
    const afterOpen = rest.slice(open + FENCE.length);
    const close = afterOpen.indexOf(FENCE);
    const inner = close === -1 ? afterOpen : afterOpen.slice(0, close);
    const lineEnd = inner.indexOf("\n");
    const lang = lineEnd === -1 ? null : cleanLang(inner.slice(0, lineEnd));
    const code = lang === null ? inner : inner.slice(lineEnd + 1);
    segments.push({ kind: "code", code: code.replace(/^\n|\n$/g, ""), lang, closed: close !== -1 });
    if (close === -1) break;
    rest = afterOpen.slice(close + FENCE.length);
  }
  return segments;
}
