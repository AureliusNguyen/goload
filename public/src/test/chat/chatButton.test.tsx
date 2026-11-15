import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ChatButton } from "@/components/chat/ChatButton";
import { ChatTheme } from "@/components/chat/ChatTheme";

describe("ChatButton", () => {
  it("keeps the angled plate on a child layer so the button box stays the hit target", () => {
    render(
      <ChatTheme>
        <ChatButton tone="ember" aria-label="Send message">
          Go
        </ChatButton>
      </ChatTheme>,
    );
    const button = screen.getByRole("button", { name: "Send message" });
    expect(button).toHaveAttribute("type", "button");
    expect(button).toHaveAttribute("data-tone", "ember");
    expect(button.closest("[data-chat-decor]")).toBeNull();

    const plate = button.querySelector("[data-chat-decor]");
    expect(plate).toHaveAttribute("data-chat-decor", "plate");
    expect(plate).toHaveAttribute("data-tone", "ember");
    expect(plate).toHaveAttribute("aria-hidden", "true");
    expect(plate?.textContent).toBe("");
  });

  it("applies the scope class through ChatTheme", () => {
    render(
      <ChatTheme data-testid="scope">
        <span>inside</span>
      </ChatTheme>,
    );
    expect(screen.getByTestId("scope")).toHaveClass("chat-theme");
  });
});
