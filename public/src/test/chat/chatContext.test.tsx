import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ChatProvider, useChat } from "@/contexts/ChatContext";
import { pickMockReply } from "@/lib/chat/mockAssistant";
import type { ChatState, Conversation } from "@/lib/chat/types";

const STREAM = { startDelayMs: 100, chunkDelayMs: 10 };

const twoChats = (): ChatState => ({
  activeId: "a",
  conversations: [
    { id: "a", title: "New chat", createdAt: 2, messages: [] },
    {
      id: "b",
      title: "Older chat",
      createdAt: 1,
      messages: [{ id: "b-1", role: "user", content: "old question", status: "complete", createdAt: 1 }],
    },
  ],
});

// Pass null to let the provider fall back to its own seed. (Passing undefined
// would trigger the default parameter instead.)
function setup(initialState: ChatState | null = twoChats()) {
  const wrapper = ({ children }: { children: ReactNode }) => (
    <ChatProvider initialState={initialState ?? undefined} streamOptions={STREAM}>
      {children}
    </ChatProvider>
  );
  return renderHook(() => useChat(), { wrapper });
}

const find = (conversations: Conversation[], id: string) =>
  conversations.find((conversation) => conversation.id === id)!;

describe("ChatProvider", () => {
  const originalMatchMedia = window.matchMedia;

  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    window.matchMedia = originalMatchMedia;
  });

  it("streams a reply into the conversation that asked, then completes it", () => {
    const { result } = setup();
    let accepted = false;
    act(() => {
      accepted = result.current.sendMessage("a", "  hello there  ");
    });
    expect(accepted).toBe(true);
    expect(result.current.streamingIds.has("a")).toBe(true);
    expect(find(result.current.conversations, "a").messages).toMatchObject([
      { role: "user", content: "hello there", status: "complete" },
      { role: "assistant", content: "", status: "streaming" },
    ]);

    act(() => {
      vi.runAllTimers();
    });
    const reply = find(result.current.conversations, "a").messages[1];
    expect(reply).toMatchObject({ status: "complete", content: pickMockReply("hello there") });
    expect(result.current.streamingIds.size).toBe(0);
  });

  it("blocks a second send to a conversation that is still streaming", () => {
    const { result } = setup();
    act(() => {
      result.current.sendMessage("a", "first");
    });
    let second = true;
    act(() => {
      second = result.current.sendMessage("a", "second");
    });
    expect(second).toBe(false);
    expect(find(result.current.conversations, "a").messages.filter((item) => item.role === "user")).toHaveLength(1);

    act(() => {
      vi.runAllTimers();
    });
    let third = false;
    act(() => {
      third = result.current.sendMessage("a", "third");
    });
    expect(third).toBe(true);
  });

  it("refuses a second send in the same tick", () => {
    const { result } = setup();
    let first = false;
    let second = true;
    act(() => {
      first = result.current.sendMessage("a", "one");
      second = result.current.sendMessage("a", "two");
    });
    expect([first, second]).toEqual([true, false]);
    expect(find(result.current.conversations, "a").messages).toHaveLength(2);
  });

  it("keeps the reply attached when the active conversation changes mid-stream", () => {
    const { result } = setup();
    act(() => {
      result.current.sendMessage("a", "hello there");
    });
    act(() => {
      vi.advanceTimersByTime(STREAM.startDelayMs);
    });
    act(() => {
      result.current.selectConversation("b");
    });
    expect(result.current.activeId).toBe("b");
    expect(result.current.activeConversation.id).toBe("b");

    act(() => {
      vi.runAllTimers();
    });
    expect(find(result.current.conversations, "b").messages).toHaveLength(1);
    expect(find(result.current.conversations, "a").messages[1]).toMatchObject({
      status: "complete",
      content: pickMockReply("hello there"),
    });
  });

  it("lets another conversation send while one is streaming", () => {
    const { result } = setup();
    let other = false;
    act(() => {
      result.current.sendMessage("a", "hello");
      other = result.current.sendMessage("b", "meanwhile");
    });
    expect(other).toBe(true);
    expect([...result.current.streamingIds].sort()).toEqual(["a", "b"]);
  });

  it("rejects blank text and unknown conversations", () => {
    const { result } = setup();
    let blank = true;
    let unknown = true;
    act(() => {
      blank = result.current.sendMessage("a", "   \n ");
      unknown = result.current.sendMessage("missing", "hello");
    });
    expect([blank, unknown]).toEqual([false, false]);
    expect(find(result.current.conversations, "a").messages).toEqual([]);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("delivers the reply whole when the user prefers reduced motion", () => {
    window.matchMedia = ((query: string) => ({
      ...originalMatchMedia(query),
      matches: query === "(prefers-reduced-motion: reduce)",
    })) as unknown as typeof window.matchMedia;
    const { result } = setup();
    act(() => {
      result.current.sendMessage("a", "hello there");
    });
    act(() => {
      vi.advanceTimersByTime(STREAM.startDelayMs);
    });
    expect(find(result.current.conversations, "a").messages[1]).toMatchObject({
      status: "complete",
      content: pickMockReply("hello there"),
    });
  });

  it("cancels running streams on unmount", () => {
    const { result, unmount } = setup();
    act(() => {
      result.current.sendMessage("a", "hello");
    });
    expect(vi.getTimerCount()).toBeGreaterThan(0);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("starts a new chat only when no empty one exists", () => {
    const { result } = setup();
    act(() => {
      result.current.newConversation();
    });
    expect(result.current.conversations).toHaveLength(2);
    expect(result.current.activeId).toBe("a");

    act(() => {
      result.current.sendMessage("a", "hello");
    });
    act(() => {
      vi.runAllTimers();
    });
    act(() => {
      result.current.newConversation();
    });
    expect(result.current.conversations).toHaveLength(3);
    expect(result.current.activeConversation).toMatchObject({ title: "New chat", messages: [] });
    expect(result.current.conversations[0].id).toBe(result.current.activeId);
  });

  it("falls back to the seed or the first conversation when the initial state is unusable", () => {
    const seeded = setup(null);
    expect(seeded.result.current.conversations.length).toBeGreaterThanOrEqual(3);
    expect(seeded.result.current.activeConversation.messages).toEqual([]);

    const empty = setup({ activeId: "x", conversations: [] });
    expect(empty.result.current.conversations.length).toBeGreaterThanOrEqual(3);

    const dangling = setup({ ...twoChats(), activeId: "missing" });
    expect(dangling.result.current.activeId).toBe("a");
  });

  it("throws a clear error outside the provider", () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => renderHook(() => useChat())).toThrow("useChat must be used within a ChatProvider");
  });
});
