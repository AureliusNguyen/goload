import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SUGGESTIONS } from "@/components/chat/ChatThread";
import { ChatWorkspace } from "@/components/chat/ChatWorkspace";
import { ChatProvider } from "@/contexts/ChatContext";
import { pickMockReply } from "@/lib/chat/mockAssistant";
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

const textbox = () => screen.getByRole("textbox", { name: "Message GoLoad" });
const sendButton = () => screen.getByRole("button", { name: "Send message" });
const nav = () => screen.getByRole("navigation", { name: "Conversations" });
const log = () => screen.getByRole("log", { name: "Messages" });

function send(text: string) {
  fireEvent.change(textbox(), { target: { value: text } });
  fireEvent.keyDown(textbox(), { key: "Enter" });
}

const finishStreams = () =>
  act(() => {
    vi.runAllTimers();
  });

describe("ChatWorkspace", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("sends a message, streams the reply, then re-enables sending", () => {
    renderWorkspace();
    send("hello there");
    expect(within(screen.getByRole("article", { name: "You said" })).getByText("hello there")).toBeInTheDocument();
    expect(screen.getByRole("article", { name: "GoLoad said" })).toHaveAttribute("aria-busy", "true");
    expect(screen.getByRole("status")).toHaveTextContent("Sending is paused");

    finishStreams();
    const reply = screen.getByRole("article", { name: "GoLoad said" });
    expect(reply).toHaveTextContent(pickMockReply("hello there"));
    expect(reply).not.toHaveAttribute("aria-busy");
    expect(screen.getByRole("status")).toHaveTextContent("Enter to send");
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("hello there");
  });

  it("blocks a second send while the reply is streaming and keeps the draft", () => {
    renderWorkspace();
    send("first");
    send("second");
    expect(screen.getAllByRole("article", { name: "You said" })).toHaveLength(1);
    expect(textbox()).toHaveValue("second");
    expect(sendButton()).toBeDisabled();

    finishStreams();
    expect(sendButton()).toBeEnabled();
    fireEvent.keyDown(textbox(), { key: "Enter" });
    expect(screen.getAllByRole("article", { name: "You said" })).toHaveLength(2);
  });

  it("keeps a streaming reply attached to the conversation that asked", () => {
    renderWorkspace();
    send("hello there");
    act(() => {
      vi.advanceTimersByTime(STREAM.startDelayMs);
    });

    // Switch away while the reply to "hello there" is still arriving.
    fireEvent.click(within(nav()).getByRole("button", { name: /Older chat/ }));
    expect(within(log()).getByText("old answer")).toBeInTheDocument();
    expect(within(log()).queryByText("hello there")).toBeNull();
    expect(within(nav()).getByRole("button", { name: /hello there/ })).toHaveTextContent("replying");

    // The conversation on screen is idle, so it may send.
    fireEvent.change(textbox(), { target: { value: "unrelated" } });
    expect(sendButton()).toBeEnabled();
    fireEvent.change(textbox(), { target: { value: "" } });

    finishStreams();
    expect(within(log()).getAllByRole("article")).toHaveLength(2);
    expect(within(nav()).getByRole("button", { name: /hello there/ })).not.toHaveTextContent("replying");

    // Back in the conversation that asked: the whole reply is there.
    fireEvent.click(within(nav()).getByRole("button", { name: /hello there/ }));
    const reply = screen.getByRole("article", { name: "GoLoad said" });
    expect(reply).toHaveTextContent(pickMockReply("hello there"));
    expect(reply).not.toHaveAttribute("aria-busy");
  });

  it("starts a new chat with the intro state and moves the active marker", () => {
    renderWorkspace();
    fireEvent.click(within(nav()).getByRole("button", { name: /Older chat/ }));
    expect(within(nav()).getByRole("button", { name: /Older chat/ })).toHaveAttribute("aria-current", "true");

    fireEvent.click(within(nav()).getByRole("button", { name: "Start a new chat" }));
    fireEvent.click(within(nav()).getByRole("button", { name: "Start a new chat" }));
    expect(screen.getByRole("heading", { level: 2 })).toBeInTheDocument();
    expect(within(nav()).getByRole("button", { name: "New chat" })).toHaveAttribute("aria-current", "true");
    expect(within(nav()).getAllByRole("listitem")).toHaveLength(2);
  });

  it("sends a suggestion from the intro state", () => {
    renderWorkspace();
    fireEvent.click(screen.getByRole("button", { name: SUGGESTIONS[1] }));
    expect(within(screen.getByRole("article", { name: "You said" })).getByText(SUGGESTIONS[1])).toBeInTheDocument();
    finishStreams();
    expect(screen.getByRole("region", { name: "bash code block" })).toHaveTextContent("sha256sum big-file.iso");
  });

  it("keeps decoration off interactive elements and off the reading surface", () => {
    const { container } = renderWorkspace();
    send("show me a command");
    finishStreams();

    const interactive = container.querySelectorAll("a, button, textarea, input, [tabindex]");
    expect(interactive.length).toBeGreaterThan(5);
    interactive.forEach((element) => expect(element.closest("[data-chat-decor]")).toBeNull());

    container.querySelectorAll("[data-chat-decor]").forEach((layer) => {
      expect(layer).toHaveAttribute("aria-hidden", "true");
      expect(layer.textContent).toBe("");
    });

    // The loud edge is rare: one at most, never in the thread or the list.
    expect(container.querySelectorAll('[data-chat-decor="spike"]').length).toBeLessThanOrEqual(1);
    expect(log().querySelector('[data-chat-decor="spike"]')).toBeNull();
    expect(nav().querySelector('[data-chat-decor="spike"]')).toBeNull();

    // Ember fills: the send button plate and the active marker, nothing else.
    const emberPlates = [...container.querySelectorAll('[data-chat-decor="plate"][data-tone="ember"]')];
    expect(emberPlates).toHaveLength(2);
    expect(emberPlates.filter((plate) => plate.hasAttribute("data-chat-active-marker"))).toHaveLength(1);
    expect(emberPlates.filter((plate) => plate.closest("button") === sendButton())).toHaveLength(1);
    // Ember texture: exactly one restrained tint.
    expect(container.querySelectorAll('[data-chat-decor="halftone"][data-tone="ember"]')).toHaveLength(1);
  });
});

describe("ChatWorkspace on a phone", () => {
  const originalWidth = window.innerWidth;
  beforeEach(() => {
    window.innerWidth = 390;
  });
  afterEach(() => {
    window.innerWidth = originalWidth;
  });

  it("collapses the sidebar into a themed drawer that closes on selection", async () => {
    renderWorkspace();
    expect(screen.queryByRole("navigation", { name: "Conversations" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Open conversations" }));
    const drawer = await screen.findByRole("dialog");
    // Rendered in a portal under body: it must carry the scope class itself.
    expect(drawer).toHaveClass("chat-theme");
    expect(within(drawer).getByRole("navigation", { name: "Conversations" })).toBeInTheDocument();

    fireEvent.click(within(drawer).getByRole("button", { name: /Older chat/ }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(within(log()).getByText("old answer")).toBeInTheDocument();
  });
});
