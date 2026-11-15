import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MessagePanel } from "@/components/chat/MessagePanel";
import type { ChatMessage } from "@/lib/chat/types";

const FENCE = "`".repeat(3);

const message = (overrides: Partial<ChatMessage>): ChatMessage => ({
  id: "m-1",
  role: "assistant",
  content: "hello",
  status: "complete",
  createdAt: 1,
  ...overrides,
});

describe("MessagePanel", () => {
  it("labels a user turn", () => {
    render(<MessagePanel message={message({ role: "user", content: "mirror this for me" })} />);
    const panel = screen.getByRole("article", { name: "You said" });
    expect(panel).toHaveAttribute("data-role", "user");
    expect(within(panel).getByText("YOU")).toBeInTheDocument();
    expect(within(panel).getByText("mirror this for me")).toBeInTheDocument();
  });

  it("labels an assistant turn", () => {
    render(<MessagePanel message={message({ content: "Simulated reply: hello." })} />);
    const panel = screen.getByRole("article", { name: "GoLoad said" });
    expect(panel).toHaveAttribute("data-role", "assistant");
    expect(within(panel).getByText("GOLOAD")).toBeInTheDocument();
    expect(within(panel).getByText("Simulated reply: hello.")).toBeInTheDocument();
  });

  it("renders fenced code in a mono block without the fence characters", () => {
    const content = ["Run this:", "", `${FENCE}bash`, "sha256sum big-file.iso", FENCE].join("\n");
    render(<MessagePanel message={message({ content })} />);
    const panel = screen.getByRole("article", { name: "GoLoad said" });
    const block = within(panel).getByRole("region", { name: "bash code block" });
    expect(block.tagName).toBe("PRE");
    expect(block).toHaveTextContent("sha256sum big-file.iso");
    expect(block).toHaveAttribute("tabindex", "0");
    expect(within(panel).getByText("Run this:")).toBeInTheDocument();
    expect(panel.textContent).not.toContain(FENCE);
  });

  it("renders markup in a message as plain text", () => {
    const content = '<img src="x" onerror="alert(1)"> <b>bold</b>';
    render(<MessagePanel message={message({ role: "user", content })} />);
    const panel = screen.getByRole("article", { name: "You said" });
    expect(panel.querySelector("img, b")).toBeNull();
    expect(within(panel).getByText(content)).toBeInTheDocument();
  });

  it("marks a streaming reply as busy and shows the caret", () => {
    const { rerender } = render(<MessagePanel message={message({ content: "", status: "streaming" })} />);
    const panel = screen.getByRole("article", { name: "GoLoad said" });
    expect(panel).toHaveAttribute("aria-busy", "true");
    expect(panel).toHaveAttribute("data-status", "streaming");
    expect(panel.querySelectorAll("[data-chat-caret]")).toHaveLength(1);
    expect(within(panel).getByText("GoLoad is replying")).toBeInTheDocument();

    rerender(<MessagePanel message={message({ content: "Simulated reply: par", status: "streaming" })} />);
    expect(panel.querySelectorAll("[data-chat-caret]")).toHaveLength(1);

    rerender(<MessagePanel message={message({ content: "Simulated reply: partial.", status: "complete" })} />);
    expect(panel).not.toHaveAttribute("aria-busy");
    expect(panel.querySelector("[data-chat-caret]")).toBeNull();
    expect(within(panel).queryByText("GoLoad is replying")).toBeNull();
  });

  it("keeps message text out of decoration layers", () => {
    const content = ["plain words", "", `${FENCE}bash`, "echo hi", FENCE].join("\n");
    render(<MessagePanel message={message({ content })} />);
    const panel = screen.getByRole("article", { name: "GoLoad said" });
    expect(panel.closest("[data-chat-decor]")).toBeNull();
    expect(within(panel).getByText("plain words").closest("[data-chat-decor]")).toBeNull();
    expect(within(panel).getByRole("region", { name: "bash code block" }).closest("[data-chat-decor]")).toBeNull();

    const layers = panel.querySelectorAll("[data-chat-decor]");
    expect(layers.length).toBeGreaterThan(0);
    layers.forEach((layer) => {
      expect(layer).toHaveAttribute("aria-hidden", "true");
      expect(layer.textContent).toBe("");
      expect(layer).not.toHaveAttribute("data-chat-decor", "spike");
    });
  });
});
