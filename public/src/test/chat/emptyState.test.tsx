import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SUGGESTIONS } from "@/components/chat/ChatThread";
import { ChatWorkspace } from "@/components/chat/ChatWorkspace";
import { ChatProvider } from "@/contexts/ChatContext";
import type { ChatState } from "@/lib/chat/types";

const STREAM = { startDelayMs: 100, chunkDelayMs: 10 };

const initialState = (): ChatState => ({
  activeId: "a",
  conversations: [
    { id: "a", title: "New chat", createdAt: 2, messages: [] },
    {
      id: "b",
      title: "Older chat",
      createdAt: 1,
      messages: [
        { id: "b-1", role: "user", content: "old question", status: "complete", createdAt: 1 },
        { id: "b-2", role: "assistant", content: "old answer", status: "complete", createdAt: 1 },
      ],
    },
  ],
});

function renderWorkspace() {
  return render(
    <MemoryRouter initialEntries={["/app"]}>
      <ChatProvider initialState={initialState()} streamOptions={STREAM}>
        <ChatWorkspace />
      </ChatProvider>
    </MemoryRouter>,
  );
}

const composer = () => screen.getByRole("form", { name: "Message composer" });
const startGroup = () => screen.queryByRole("group", { name: "Start a conversation" });

describe("empty state: centered composer with suggestion chips", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("centers the composer under the heading, with the three chips after it", () => {
    renderWorkspace();
    const group = startGroup();
    expect(group).not.toBeNull();
    const inside = within(group as HTMLElement);
    expect(inside.getByRole("heading", { level: 2 })).toBeInTheDocument();
    expect(inside.getByRole("form", { name: "Message composer" })).toBe(composer());
    expect(composer()).toHaveAttribute("data-layout", "centered");

    const chips = SUGGESTIONS.map((text) => inside.getByRole("button", { name: text }));
    expect(chips).toHaveLength(3);
    chips.forEach((chip) => {
      // The composer comes first in reading order, then the chips.
      expect(composer().compareDocumentPosition(chip) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    });
  });

  it("sends the clicked chip and then docks the composer below the thread", () => {
    renderWorkspace();
    fireEvent.click(screen.getByRole("button", { name: SUGGESTIONS[2] }));
    act(() => {
      vi.runAllTimers();
    });
    expect(within(screen.getByRole("article", { name: "You said" })).getByText(SUGGESTIONS[2])).toBeInTheDocument();
    expect(startGroup()).toBeNull();
    expect(composer()).toHaveAttribute("data-layout", "docked");
    const thread = screen.getByRole("region", { name: "Chat thread" });
    expect(thread.contains(composer())).toBe(false);
    SUGGESTIONS.forEach((text) => expect(within(thread).queryByRole("button", { name: text })).toBeNull());
  });

  it("docks the composer in a conversation that already has messages", () => {
    renderWorkspace();
    fireEvent.click(screen.getByRole("button", { name: /Older chat/ }));
    expect(startGroup()).toBeNull();
    expect(composer()).toHaveAttribute("data-layout", "docked");
  });
});
