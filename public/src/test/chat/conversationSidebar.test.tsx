import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ConversationSidebar, type ConversationSidebarProps } from "@/components/chat/ConversationSidebar";
import type { Conversation } from "@/lib/chat/types";

const LONG_TITLE = "A very long conversation title that will need truncating in the list";

const conversations: Conversation[] = [
  { id: "a", title: "New chat", createdAt: 3, messages: [] },
  { id: "b", title: "Sample: verify a checksum", createdAt: 2, messages: [] },
  { id: "c", title: LONG_TITLE, createdAt: 1, messages: [] },
];

function setup(overrides: Partial<ConversationSidebarProps> = {}) {
  const props: ConversationSidebarProps = {
    conversations,
    activeId: "a",
    streamingIds: new Set<string>(),
    onSelect: vi.fn(),
    onNewChat: vi.fn(),
    ...overrides,
  };
  const view = render(<ConversationSidebar {...props} />);
  const nav = screen.getByRole("navigation", { name: "Conversations" });
  const update = (next: Partial<ConversationSidebarProps>) => view.rerender(<ConversationSidebar {...props} {...next} />);
  return { props, nav, update };
}

const item = (nav: HTMLElement, name: string | RegExp) => within(nav).getByRole("button", { name });

describe("ConversationSidebar", () => {
  it("lists every conversation in order", () => {
    const { nav } = setup();
    const rows = within(nav).getAllByRole("listitem");
    expect(rows.map((row) => row.textContent)).toEqual(["New chat", "Sample: verify a checksum", LONG_TITLE]);
  });

  it("marks only the active conversation and moves the marker with it", () => {
    const { nav, update } = setup();
    expect(item(nav, "New chat")).toHaveAttribute("aria-current", "true");
    expect(item(nav, "New chat").querySelector("[data-chat-active-marker]")).not.toBeNull();
    expect(item(nav, "Sample: verify a checksum")).not.toHaveAttribute("aria-current");
    expect(nav.querySelectorAll("[data-chat-active-marker]")).toHaveLength(1);

    update({ activeId: "b" });
    expect(item(nav, "Sample: verify a checksum")).toHaveAttribute("aria-current", "true");
    expect(item(nav, "New chat")).not.toHaveAttribute("aria-current");
    expect(nav.querySelectorAll("[data-chat-active-marker]")).toHaveLength(1);
    expect(item(nav, "Sample: verify a checksum").querySelector("[data-chat-active-marker]")).not.toBeNull();
  });

  it("selects a conversation on click", () => {
    const { nav, props } = setup();
    fireEvent.click(item(nav, LONG_TITLE));
    expect(props.onSelect).toHaveBeenCalledWith("c");
  });

  it("starts a new chat from an ember-accented, not ember-filled, button", () => {
    const { nav, props } = setup();
    const button = item(nav, "Start a new chat");
    expect(button).toHaveTextContent("New chat");
    const plate = button.querySelector('[data-chat-decor="plate"]');
    expect(plate).toHaveAttribute("data-tone", "red");
    expect(plate).toHaveAttribute("data-shadow", "ember");
    fireEvent.click(button);
    expect(props.onNewChat).toHaveBeenCalledTimes(1);
  });

  it("shows which conversations are still replying", () => {
    const { nav } = setup({ streamingIds: new Set(["b"]) });
    expect(item(nav, /Sample: verify a checksum/)).toHaveTextContent("replying");
    expect(item(nav, "New chat")).not.toHaveTextContent("replying");
  });

  it("uses no spike and keeps every button out of decoration layers", () => {
    const { nav } = setup();
    expect(nav.querySelector('[data-chat-decor="spike"]')).toBeNull();
    const buttons = within(nav).getAllByRole("button");
    expect(buttons).toHaveLength(conversations.length + 1);
    buttons.forEach((button) => expect(button.closest("[data-chat-decor]")).toBeNull());
  });
});
