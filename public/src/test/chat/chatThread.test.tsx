import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ChatThread, SUGGESTIONS } from "@/components/chat/ChatThread";
import { isNearBottom, NEAR_BOTTOM_PX } from "@/lib/chat/scroll";
import type { ChatMessage } from "@/lib/chat/types";

const msg = (
  id: string,
  role: ChatMessage["role"],
  content: string,
  status: ChatMessage["status"] = "complete",
): ChatMessage => ({ id, role, content, status, createdAt: 1 });

const history = [msg("u-1", "user", "first question"), msg("r-1", "assistant", "first answer")];

// jsdom does no layout: give the pane the numbers a browser would report.
function setMetrics(element: HTMLElement, metrics: { scrollHeight: number; clientHeight: number }) {
  Object.defineProperty(element, "scrollHeight", { configurable: true, value: metrics.scrollHeight });
  Object.defineProperty(element, "clientHeight", { configurable: true, value: metrics.clientHeight });
}

function readerScrollsTo(element: HTMLElement, scrollTop: number) {
  element.scrollTop = scrollTop;
  fireEvent.scroll(element);
}

function renderThread(messages: ChatMessage[] = history, conversationId = "a") {
  const onSuggestion = vi.fn();
  const view = render(<ChatThread conversationId={conversationId} messages={messages} onSuggestion={onSuggestion} />);
  const pane = screen.getByTestId("chat-scroll");
  const update = (next: ChatMessage[], nextId = conversationId) =>
    view.rerender(<ChatThread conversationId={nextId} messages={next} onSuggestion={onSuggestion} />);
  return { pane, onSuggestion, update };
}

const jumpButton = () => screen.queryByRole("button", { name: "Jump to latest" });

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("isNearBottom", () => {
  it("treats the reader as near the bottom within the threshold", () => {
    expect(NEAR_BOTTOM_PX).toBe(80);
    expect(isNearBottom({ scrollTop: 520, scrollHeight: 1000, clientHeight: 400 })).toBe(true);
    expect(isNearBottom({ scrollTop: 519, scrollHeight: 1000, clientHeight: 400 })).toBe(false);
    expect(isNearBottom({ scrollTop: 0, scrollHeight: 300, clientHeight: 400 })).toBe(true);
    expect(isNearBottom({ scrollTop: 0, scrollHeight: 1000, clientHeight: 400 }, 600)).toBe(true);
  });
});

describe("ChatThread", () => {
  it("shows the intro state with suggestions when there are no messages", () => {
    const { onSuggestion } = renderThread([]);
    expect(screen.getByRole("heading", { level: 2 })).toBeInTheDocument();
    expect(screen.queryByRole("log")).toBeNull();
    expect(jumpButton()).toBeNull();
    expect(screen.getAllByRole("button")).toHaveLength(SUGGESTIONS.length);
    fireEvent.click(screen.getByRole("button", { name: SUGGESTIONS[0] }));
    expect(onSuggestion).toHaveBeenCalledWith(SUGGESTIONS[0]);
  });

  it("renders one panel per message inside the log", () => {
    renderThread();
    const log = screen.getByRole("log", { name: "Messages" });
    expect(within(log).getAllByRole("article")).toHaveLength(2);
    expect(screen.queryByRole("heading", { level: 2 })).toBeNull();
  });

  it("follows new content while the reader is at the bottom", () => {
    const { pane, update } = renderThread();
    setMetrics(pane, { scrollHeight: 1000, clientHeight: 400 });
    readerScrollsTo(pane, 600);
    setMetrics(pane, { scrollHeight: 1400, clientHeight: 400 });
    update([...history, msg("r-2", "assistant", "more", "streaming")]);
    expect(pane.scrollTop).toBe(1400);
    expect(jumpButton()).toBeNull();
  });

  it("does not yank a reader who scrolled up and offers a jump", () => {
    const { pane, update } = renderThread();
    setMetrics(pane, { scrollHeight: 1000, clientHeight: 400 });
    readerScrollsTo(pane, 100);
    expect(jumpButton()).not.toBeNull();

    setMetrics(pane, { scrollHeight: 1400, clientHeight: 400 });
    update([...history, msg("r-2", "assistant", "streaming text", "streaming")]);
    expect(pane.scrollTop).toBe(100);
    update([...history, msg("r-2", "assistant", "streaming text that keeps growing", "streaming")]);
    expect(pane.scrollTop).toBe(100);

    fireEvent.click(jumpButton()!);
    expect(pane.scrollTop).toBe(1400);
    expect(jumpButton()).toBeNull();
  });

  it("returns to the latest message after the reader sends", () => {
    const { pane, update } = renderThread();
    setMetrics(pane, { scrollHeight: 1000, clientHeight: 400 });
    readerScrollsTo(pane, 100);
    setMetrics(pane, { scrollHeight: 1400, clientHeight: 400 });
    update([...history, msg("u-2", "user", "second question"), msg("r-2", "assistant", "", "streaming")]);
    expect(pane.scrollTop).toBe(1400);
    expect(jumpButton()).toBeNull();
  });

  it("starts at the latest message when the conversation changes", () => {
    const { pane, update } = renderThread();
    setMetrics(pane, { scrollHeight: 1000, clientHeight: 400 });
    readerScrollsTo(pane, 100);
    expect(jumpButton()).not.toBeNull();
    setMetrics(pane, { scrollHeight: 700, clientHeight: 400 });
    update([msg("u-9", "user", "other chat"), msg("r-9", "assistant", "other answer")], "b");
    expect(pane.scrollTop).toBe(700);
    expect(jumpButton()).toBeNull();
  });

  it("keeps the latest message in view when the pane resizes", () => {
    const callbacks: Array<() => void> = [];
    vi.stubGlobal(
      "ResizeObserver",
      class {
        constructor(callback: () => void) {
          callbacks.push(callback);
        }
        observe() {}
        disconnect() {}
      },
    );
    const { pane } = renderThread();
    expect(callbacks).toHaveLength(1);

    // The phone keyboard opens: the pane gets shorter while the reader is at the bottom.
    setMetrics(pane, { scrollHeight: 1000, clientHeight: 200 });
    callbacks[0]();
    expect(pane.scrollTop).toBe(1000);

    // A reader who scrolled up is left where they are.
    readerScrollsTo(pane, 100);
    setMetrics(pane, { scrollHeight: 1000, clientHeight: 150 });
    callbacks[0]();
    expect(pane.scrollTop).toBe(100);
  });
});
