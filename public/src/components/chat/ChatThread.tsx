import { useCallback, useLayoutEffect, useRef, useState } from "react";
import { ArrowDown } from "lucide-react";
import { ChatButton } from "@/components/chat/ChatButton";
import { MessagePanel } from "@/components/chat/MessagePanel";
import { isNearBottom } from "@/lib/chat/scroll";
import type { ChatMessage } from "@/lib/chat/types";

// eslint-disable-next-line react-refresh/only-export-components
export const SUGGESTIONS = [
  "Download https://example.com/big-file.iso",
  "Show me a command to verify a checksum",
  "What can you do in this preview?",
] as const;

export interface ChatThreadProps {
  conversationId: string;
  messages: ChatMessage[];
  onSuggestion: (text: string) => void;
}

/**
 * The scrolling pane of one conversation. New content follows the bottom only
 * while the reader is already near it; a reader who scrolled up is left alone
 * and gets a "Jump to latest" button instead.
 */
export function ChatThread({ conversationId, messages, onSuggestion }: ChatThreadProps) {
  const paneRef = useRef<HTMLDivElement>(null);
  // The ref is the truth for effects; the state only drives the jump button.
  const pinnedRef = useRef(true);
  const [pinned, setPinned] = useState(true);

  const setPin = useCallback((next: boolean) => {
    pinnedRef.current = next;
    setPinned(next);
  }, []);

  const scrollToLatest = useCallback(() => {
    const pane = paneRef.current;
    // Instant on purpose: no smooth scrolling to fight reduced-motion settings.
    if (pane) pane.scrollTop = pane.scrollHeight;
    setPin(true);
  }, [setPin]);

  const last = messages[messages.length - 1];
  const contentKey = `${messages.length}:${last ? last.content.length : 0}`;
  const userTurns = messages.filter((message) => message.role === "user").length;

  // Opening a conversation, or sending in it, always shows the latest message.
  useLayoutEffect(() => {
    scrollToLatest();
  }, [conversationId, userTurns, scrollToLatest]);

  // Streaming growth follows the bottom only while the reader is pinned there.
  useLayoutEffect(() => {
    const pane = paneRef.current;
    if (pane && pinnedRef.current) pane.scrollTop = pane.scrollHeight;
  }, [contentKey]);

  // The pane shrinks when the phone keyboard opens: keep a pinned reader pinned.
  useLayoutEffect(() => {
    const pane = paneRef.current;
    if (!pane || typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(() => {
      if (pinnedRef.current) pane.scrollTop = pane.scrollHeight;
    });
    observer.observe(pane);
    return () => observer.disconnect();
  }, []);

  const handleScroll = () => {
    const pane = paneRef.current;
    if (pane) setPin(isNearBottom(pane));
  };

  return (
    <section aria-label="Chat thread" className="chat-thread relative flex min-h-0 flex-1 flex-col">
      <div
        ref={paneRef}
        data-testid="chat-scroll"
        tabIndex={0}
        onScroll={handleScroll}
        className="chat-scroll min-h-0 flex-1 px-4 py-6 md:px-8"
      >
        {messages.length === 0 ? (
          <div className="chat-empty mx-auto flex min-h-full w-full max-w-2xl flex-col justify-center gap-6">
            <p className="chat-kicker chat-mono">PREVIEW / SIMULATED REPLIES</p>
            <h2 className="chat-empty-title chat-display">Name it. I will fetch it.</h2>
            <p className="chat-empty-lede">
              Not yet, though: replies here are canned. Nothing is searched, downloaded or uploaded in this preview.
            </p>
            <ul className="m-0 flex list-none flex-col gap-3 p-0 sm:flex-row sm:flex-wrap">
              {SUGGESTIONS.map((suggestion) => (
                <li key={suggestion}>
                  <ChatButton className="chat-suggestion" onClick={() => onSuggestion(suggestion)}>
                    {suggestion}
                  </ChatButton>
                </li>
              ))}
            </ul>
          </div>
        ) : (
          <div role="log" aria-label="Messages" className="mx-auto flex w-full max-w-3xl flex-col gap-8">
            {messages.map((message) => (
              <MessagePanel key={message.id} message={message} />
            ))}
          </div>
        )}
      </div>
      {messages.length > 0 && !pinned ? (
        <ChatButton shadow="ember" className="chat-jump" onClick={scrollToLatest}>
          <ArrowDown aria-hidden="true" />
          Jump to latest
        </ChatButton>
      ) : null}
    </section>
  );
}
