import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Composer, STATUS_BUSY, STATUS_FILES, STATUS_IDLE } from "@/components/chat/Composer";

function setup(options: { busy?: boolean; accept?: boolean } = {}) {
  const onSend = vi.fn((_text: string) => options.accept ?? true);
  const view = render(<Composer busy={options.busy ?? false} onSend={onSend} />);
  const textarea = screen.getByRole("textbox", { name: "Message GoLoad" }) as HTMLTextAreaElement;
  const sendButton = screen.getByRole("button", { name: "Send message" });
  const setBusy = (busy: boolean) => view.rerender(<Composer busy={busy} onSend={onSend} />);
  return { onSend, textarea, sendButton, setBusy };
}

const type = (textarea: HTMLTextAreaElement, text: string) => fireEvent.change(textarea, { target: { value: text } });

// fireEvent returns false when the handler called preventDefault.
const pressEnter = (textarea: HTMLTextAreaElement, init: KeyboardEventInit = {}) =>
  fireEvent.keyDown(textarea, { key: "Enter", ...init });

function chooseFiles(...names: string[]) {
  fireEvent.change(screen.getByLabelText("Choose files to attach"), {
    target: { files: names.map((name) => new File(["x"], name)) },
  });
}

const chipList = () => screen.queryByRole("list", { name: "Selected files" });
const form = () => screen.getByRole("form", { name: "Message composer" });

describe("Composer", () => {
  it("sends on Enter and clears the draft", () => {
    const { onSend, textarea } = setup();
    type(textarea, "hello");
    expect(pressEnter(textarea)).toBe(false);
    expect(onSend.mock.calls).toEqual([["hello"]]);
    expect(textarea).toHaveValue("");
  });

  it("keeps Shift+Enter for a new line", () => {
    const { onSend, textarea } = setup();
    type(textarea, "line one");
    expect(pressEnter(textarea, { shiftKey: true })).toBe(true);
    expect(onSend).not.toHaveBeenCalled();
    expect(textarea).toHaveValue("line one");
  });

  it("blocks empty and whitespace-only messages", () => {
    const { onSend, textarea, sendButton } = setup();
    expect(sendButton).toBeDisabled();
    pressEnter(textarea);
    type(textarea, "   \n\t ");
    expect(sendButton).toBeDisabled();
    pressEnter(textarea);
    expect(onSend).not.toHaveBeenCalled();
  });

  it("does not send while an IME composition is open", () => {
    const { onSend, textarea } = setup();
    type(textarea, "nihon");
    fireEvent.compositionStart(textarea);
    pressEnter(textarea);
    expect(onSend).not.toHaveBeenCalled();
    expect(textarea).toHaveValue("nihon");

    fireEvent.compositionEnd(textarea);
    pressEnter(textarea);
    expect(onSend).toHaveBeenCalledTimes(1);
  });

  it("does not send when the keydown reports isComposing", () => {
    const { onSend, textarea } = setup();
    type(textarea, "nihon");
    expect(pressEnter(textarea, { isComposing: true })).toBe(true);
    expect(onSend).not.toHaveBeenCalled();
  });

  it("does not send on keyCode 229", () => {
    const { onSend, textarea } = setup();
    type(textarea, "nihon");
    expect(pressEnter(textarea, { keyCode: 229 })).toBe(true);
    expect(onSend).not.toHaveBeenCalled();
  });

  it("sends from the send button and returns focus to the textarea", () => {
    const { onSend, textarea, sendButton } = setup();
    type(textarea, "hello");
    expect(sendButton).toBeEnabled();
    fireEvent.click(sendButton);
    expect(onSend.mock.calls).toEqual([["hello"]]);
    expect(textarea).toHaveFocus();
  });

  it("blocks sending while busy and keeps the draft", () => {
    const { onSend, textarea, sendButton, setBusy } = setup({ busy: true });
    type(textarea, "next question");
    expect(textarea).toBeEnabled();
    expect(sendButton).toBeDisabled();
    expect(pressEnter(textarea)).toBe(false);
    expect(onSend).not.toHaveBeenCalled();
    expect(textarea).toHaveValue("next question");
    expect(screen.getByRole("status")).toHaveTextContent(STATUS_BUSY);
    expect(form()).toHaveAttribute("aria-busy", "true");

    setBusy(false);
    expect(screen.getByRole("status")).toHaveTextContent(STATUS_IDLE);
    expect(form()).not.toHaveAttribute("aria-busy");
    pressEnter(textarea);
    expect(onSend.mock.calls).toEqual([["next question"]]);
  });

  it("keeps the draft when the store refuses the message", () => {
    const { onSend, textarea } = setup({ accept: false });
    chooseFiles("notes.pdf");
    type(textarea, "hello");
    pressEnter(textarea);
    expect(onSend).toHaveBeenCalledTimes(1);
    expect(textarea).toHaveValue("hello");
    expect(screen.getByText("notes.pdf")).toBeInTheDocument();
  });

  it("opens the file picker from the attach button", () => {
    setup();
    const input = screen.getByLabelText("Choose files to attach") as HTMLInputElement;
    const click = vi.spyOn(input, "click").mockImplementation(() => {});
    fireEvent.click(screen.getByRole("button", { name: "Attach files" }));
    expect(click).toHaveBeenCalledTimes(1);
  });

  it("shows a Not uploaded chip for each chosen file", () => {
    setup();
    expect(chipList()).toBeNull();
    chooseFiles("notes.pdf", "cover.png");
    const chips = within(chipList()!).getAllByRole("listitem");
    expect(chips).toHaveLength(2);
    expect(chips[0]).toHaveTextContent("notes.pdf");
    expect(chips[0]).toHaveTextContent("Not uploaded");
    expect(chips[1]).toHaveTextContent("cover.png");
    expect(chips[1]).toHaveTextContent("Not uploaded");
    expect(screen.getByRole("status")).toHaveTextContent(STATUS_FILES);
  });

  it("removes a chip", () => {
    setup();
    chooseFiles("notes.pdf", "cover.png");
    fireEvent.click(screen.getByRole("button", { name: "Remove notes.pdf" }));
    expect(screen.queryByText("notes.pdf")).toBeNull();
    expect(screen.getByText("cover.png")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Remove cover.png" }));
    expect(chipList()).toBeNull();
  });

  it("clears chips on send and sends text only", () => {
    const { onSend, textarea } = setup();
    chooseFiles("notes.pdf");
    type(textarea, "see attached");
    pressEnter(textarea);
    expect(onSend.mock.calls).toEqual([["see attached"]]);
    expect(chipList()).toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent(STATUS_IDLE);
  });
});
