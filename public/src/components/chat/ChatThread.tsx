import type { ReactNode } from "react";
import { useCallback, useLayoutEffect, useRef, useState } from "react";
import { ArrowDown } from "lucide-react";
import { ChatButton } from "@/components/chat/ChatButton";
import { MessagePanel } from "@/components/chat/MessagePanel";
import { isNearBottom } from "@/lib/chat/scroll";
import type { ChatMessage } from "@/lib/chat/types";

export const SUGGESTIONS = [
  "Download https://example.com/big-file.iso",
  "Show me a command to verify a checksum",
  "What can you do in this preview?",
] as const;

export interface ChatThreadProps {
  conversationId: string;
  messages: ChatMessage[];
  onSuggestion: (text: string) => void;
  /** Rendered between the heading and the chips when there are no messages. */
  emptySlot?: ReactNode;
}

/**
 * The scrolling pane of one conversation. New content follows the bottom only
 * while the reader is already near it; a reader who scrolled up is left alone
 * and gets a "Jump to latest" button instead.
 */
export function ChatThread({ conversationId, messages, onSuggestion, emptySlot }: ChatThreadProps) {
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
          <div role="group" aria-label="Start a conversation" className="chat-empty chat-column flex min-h-full flex-col justify-center gap-6">
            <h2 className="chat-empty-title">What should I fetch?</h2>
            <p className="chat-empty-lede">
              Replies here are simulated. Nothing is searched, downloaded or uploaded in this preview.
            </p>
            {emptySlot}
            <ul className="chat-chips-row">
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
          <div role="log" aria-label="Messages" className="chat-column flex flex-col gap-8">
            {messages.map((message) => (
              <MessagePanel key={message.id} message={message} />
            ))}
          </div>
        )}
      </div>
      {messages.length > 0 && !pinned ? (
        <ChatButton className="chat-jump" onClick={scrollToLatest}>
          <ArrowDown aria-hidden="true" />
          Jump to latest
        </ChatButton>
      ) : null}
    </section>
  );
}
