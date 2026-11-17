import { memo } from "react";
import { Decor } from "@/components/chat/Decor";
import { parseMessageContent } from "@/lib/chat/parseMessage";
import type { ChatMessage } from "@/lib/chat/types";

export interface MessagePanelProps {
  message: ChatMessage;
}

const SPEAKERS = {
  user: { tab: "YOU", label: "You said" },
  assistant: { tab: "GOLOAD", label: "GoLoad said" },
} as const;

/**
 * One turn of the conversation. Assistant replies sit directly on the
 * background; user turns get a quiet filled surface. Only the code block is a
 * bounded panel (its clipped corner is a decoration layer, so the text, the
 * code and the caret are never transformed). Content is rendered as text
 * nodes only, so markup typed by the user is shown literally.
 */
export const MessagePanel = memo(function MessagePanel({ message }: MessagePanelProps) {
  const speaker = SPEAKERS[message.role];
  const streaming = message.status === "streaming";
  const segments = parseMessageContent(message.content);
  const lastIndex = segments.length - 1;
  const caretInText = streaming && segments[lastIndex]?.kind === "text";
  const caret = <span aria-hidden="true" data-chat-caret="" />;

  return (
    <article
      aria-label={speaker.label}
      aria-busy={streaming ? true : undefined}
      data-role={message.role}
      data-status={message.status}
      className="chat-panel"
    >
      <p className="chat-panel-tab">{speaker.tab}</p>
      <div className="chat-panel-body">
        {segments.map((segment, index) =>
          segment.kind === "code" ? (
            <figure key={index} className="chat-surface chat-code">
              <Decor shape="panel" tone="card" />
              <figcaption className="chat-mono">{segment.lang ?? "code"}</figcaption>
              <pre role="region" aria-label={`${segment.lang ?? "code"} code block`} tabIndex={0}>
                <code>{segment.code}</code>
              </pre>
            </figure>
          ) : (
            <p key={index} className="chat-text">
              {segment.text}
              {caretInText && index === lastIndex ? caret : null}
            </p>
          ),
        )}
        {streaming && !caretInText ? <p className="chat-text">{caret}</p> : null}
        {streaming ? <span className="sr-only">GoLoad is replying</span> : null}
      </div>
    </article>
  );
});
