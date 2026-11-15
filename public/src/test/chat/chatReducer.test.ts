import { describe, expect, it } from "vitest";
import {
  canSend,
  chatReducer,
  deriveTitle,
  isConversationStreaming,
  NEW_CHAT_TITLE,
} from "@/lib/chat/chatReducer";
import { createSeedState } from "@/lib/chat/seed";
import type { ChatMessage, ChatState } from "@/lib/chat/types";

const message = (overrides: Partial<ChatMessage> = {}): ChatMessage => ({
  id: "m-1",
  role: "user",
  content: "hi",
  status: "complete",
  createdAt: 1,
  ...overrides,
});

const twoChats = (): ChatState => ({
  activeId: "a",
  conversations: [
    { id: "a", title: NEW_CHAT_TITLE, createdAt: 2, messages: [] },
    { id: "b", title: "Older chat", createdAt: 1, messages: [message({ id: "b-1" })] },
  ],
});

const find = (state: ChatState, id: string) => state.conversations.find((conversation) => conversation.id === id)!;

// A user turn plus an empty assistant turn that is still streaming.
function streamingIn(conversationId: string): ChatState {
  let state = twoChats();
  state = chatReducer(state, {
    type: "message/append",
    conversationId,
    message: message({ id: "u-1", content: "question" }),
  });
  state = chatReducer(state, {
    type: "message/append",
    conversationId,
    message: message({ id: "r-1", role: "assistant", content: "", status: "streaming" }),
  });
  return state;
}

describe("chatReducer", () => {
  it("selects an existing conversation and ignores unknown ids", () => {
    const state = twoChats();
    expect(chatReducer(state, { type: "conversation/select", id: "b" }).activeId).toBe("b");
    expect(chatReducer(state, { type: "conversation/select", id: "missing" })).toBe(state);
  });

  it("reuses the empty conversation instead of piling up new chats", () => {
    const onOlder = chatReducer(twoChats(), { type: "conversation/select", id: "b" });
    const next = chatReducer(onOlder, { type: "conversation/create", id: "c", createdAt: 3 });
    expect(next.conversations).toHaveLength(2);
    expect(next.activeId).toBe("a");
  });

  it("creates a new active conversation at the top when none is empty", () => {
    const filled = chatReducer(twoChats(), {
      type: "message/append",
      conversationId: "a",
      message: message({ id: "a-1" }),
    });
    const next = chatReducer(filled, { type: "conversation/create", id: "c", createdAt: 3 });
    expect(next.conversations.map((conversation) => conversation.id)).toEqual(["c", "a", "b"]);
    expect(next.activeId).toBe("c");
    expect(find(next, "c")).toMatchObject({ title: NEW_CHAT_TITLE, messages: [] });
  });

  it("titles a conversation from its first user message only", () => {
    let state = chatReducer(twoChats(), {
      type: "message/append",
      conversationId: "a",
      message: message({ id: "a-1", content: "  mirror   this\nplease " }),
    });
    expect(find(state, "a").title).toBe("mirror this please");
    state = chatReducer(state, {
      type: "message/append",
      conversationId: "a",
      message: message({ id: "a-2", content: "second" }),
    });
    expect(find(state, "a").title).toBe("mirror this please");
    expect(find(state, "b").title).toBe("Older chat");
  });

  it("keeps a streaming reply attached to its conversation while another one is active", () => {
    let state = streamingIn("a");
    state = chatReducer(state, { type: "conversation/select", id: "b" });
    state = chatReducer(state, { type: "message/delta", conversationId: "a", messageId: "r-1", delta: "Hello " });
    state = chatReducer(state, { type: "message/delta", conversationId: "a", messageId: "r-1", delta: "there" });
    expect(state.activeId).toBe("b");
    expect(find(state, "a").messages[1].content).toBe("Hello there");
    expect(find(state, "b").messages).toHaveLength(1);
  });

  it("completes a reply and ignores late or misaddressed deltas", () => {
    let state = streamingIn("a");
    state = chatReducer(state, { type: "message/delta", conversationId: "a", messageId: "r-1", delta: "done" });
    state = chatReducer(state, { type: "message/complete", conversationId: "a", messageId: "r-1" });
    expect(find(state, "a").messages[1]).toMatchObject({ content: "done", status: "complete" });
    expect(isConversationStreaming(find(state, "a"))).toBe(false);
    expect(chatReducer(state, { type: "message/delta", conversationId: "a", messageId: "r-1", delta: "late" })).toBe(state);
    expect(chatReducer(state, { type: "message/delta", conversationId: "b", messageId: "r-1", delta: "x" })).toBe(state);
    expect(chatReducer(state, { type: "message/delta", conversationId: "missing", messageId: "r-1", delta: "x" })).toBe(state);
  });

  it("allows sending only non-blank text to an idle, existing conversation", () => {
    const idle = twoChats();
    expect(canSend(idle, "a", "hello")).toBe(true);
    expect(canSend(idle, "a", "   \n\t ")).toBe(false);
    expect(canSend(idle, "missing", "hello")).toBe(false);
    const busy = streamingIn("a");
    expect(isConversationStreaming(find(busy, "a"))).toBe(true);
    expect(canSend(busy, "a", "hello")).toBe(false);
    expect(canSend(busy, "b", "hello")).toBe(true);
  });
});

describe("deriveTitle", () => {
  it("collapses whitespace and cuts long text at 40 characters", () => {
    expect(deriveTitle("  one\n\ttwo  ")).toBe("one two");
    expect(deriveTitle("a".repeat(50))).toBe(`${"a".repeat(40)}...`);
    expect(deriveTitle("   ")).toBe(NEW_CHAT_TITLE);
  });
});

describe("createSeedState", () => {
  it("opens on an empty chat and labels every seeded conversation as a sample", () => {
    const seed = createSeedState(0);
    expect(find(seed, seed.activeId).messages).toEqual([]);
    const samples = seed.conversations.filter((conversation) => conversation.id !== seed.activeId);
    expect(samples.length).toBeGreaterThanOrEqual(2);
    for (const sample of samples) {
      expect(sample.title).toMatch(/^Sample: /);
      expect(sample.messages.every((item) => item.status === "complete")).toBe(true);
      const replies = sample.messages.filter((item) => item.role === "assistant");
      expect(replies.length).toBeGreaterThan(0);
      for (const reply of replies) expect(reply.content).toMatch(/^Sample conversation: /);
    }
  });
});
