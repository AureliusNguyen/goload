import { useRef, useState } from "react";
import { ArrowUp, Paperclip, X } from "lucide-react";
import { ChatButton } from "@/components/chat/ChatButton";
import { Decor } from "@/components/chat/Decor";
import { Textarea } from "@/components/ui/textarea";
import type { AttachmentStub } from "@/lib/chat/types";

export interface ComposerProps {
  /** True while the active conversation's reply is still streaming. */
  busy: boolean;
  /** Receives the raw draft. Returns false when the store refused it. */
  onSend: (text: string) => boolean;
  /** "centered" sits in the empty state under the heading; "docked" is below the thread. */
  layout?: "centered" | "docked";
}

export const STATUS_BUSY = "GoLoad is replying. Sending is paused until this reply finishes.";
export const STATUS_FILES = "Files are not uploaded in this preview.";
export const STATUS_IDLE = "Enter to send. Shift+Enter for a new line.";

/**
 * The message composer. While `busy`, sending is blocked (not queued): the
 * send button is disabled and Enter does nothing. The textarea itself stays
 * editable so focus and the phone keyboard are not lost on every turn.
 *
 * The attach control is a stub: it lists the chosen file names as chips marked
 * "Not uploaded". File contents are never read and nothing is sent with the
 * message.
 */
export function Composer({ busy, onSend, layout = "docked" }: ComposerProps) {
  const [value, setValue] = useState("");
  const [attachments, setAttachments] = useState<AttachmentStub[]>([]);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const composing = useRef(false);
  const attachmentSequence = useRef(0);

  const canSubmit = !busy && value.trim() !== "";

  const submit = () => {
    if (!canSubmit) return;
    if (!onSend(value)) return;
    setValue("");
    setAttachments([]);
    textareaRef.current?.focus();
  };

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== "Enter" || event.shiftKey) return;
    // During IME composition Enter confirms a candidate and must not send.
    // keyCode 229 covers Safari, which reports isComposing=false on that key.
    if (composing.current || event.nativeEvent.isComposing || event.keyCode === 229) return;
    event.preventDefault();
    submit();
  };

  const handleFiles = (event: React.ChangeEvent<HTMLInputElement>) => {
    // Copy the names first: resetting the value empties the live FileList.
    const picked = Array.from(event.target.files ?? []);
    // Reset so choosing the same file again still fires a change event.
    event.target.value = "";
    if (picked.length === 0) return;
    const stubs = picked.map((file) => {
      attachmentSequence.current += 1;
      return { id: `file-${attachmentSequence.current}`, name: file.name, size: file.size };
    });
    setAttachments((current) => [...current, ...stubs]);
  };

  const removeAttachment = (id: string) => {
    setAttachments((current) => current.filter((item) => item.id !== id));
  };

  const status = busy ? STATUS_BUSY : attachments.length > 0 ? STATUS_FILES : STATUS_IDLE;

  return (
    <form
      aria-label="Message composer"
      aria-busy={busy ? true : undefined}
      data-busy={busy ? "true" : undefined}
      data-layout={layout}
      className="chat-composer"
      onSubmit={(event) => {
        event.preventDefault();
        submit();
      }}
    >
      {attachments.length > 0 ? (
        <ul aria-label="Selected files" className="chat-chips">
          {attachments.map((file) => (
            <li key={file.id} className="chat-surface chat-chip">
              <Decor shape="panel" tone="card" />
              <span className="chat-chip-name" title={file.name}>
                {file.name}
              </span>
              <span className="chat-chip-state chat-mono">Not uploaded</span>
              <ChatButton size="icon" aria-label={`Remove ${file.name}`} onClick={() => removeAttachment(file.id)}>
                <X aria-hidden="true" />
              </ChatButton>
            </li>
          ))}
        </ul>
      ) : null}
      <div className="chat-surface chat-composer-box">
        <Decor shape="panel" tone="card" />
        <Textarea
          ref={textareaRef}
          aria-label="Message GoLoad"
          placeholder="Ask GoLoad anything..."
          rows={2}
          value={value}
          onChange={(event) => setValue(event.target.value)}
          onKeyDown={handleKeyDown}
          onCompositionStart={() => {
            composing.current = true;
          }}
          onCompositionEnd={() => {
            composing.current = false;
          }}
          className="chat-textarea min-w-0 flex-1 focus-visible:ring-0 focus-visible:ring-offset-0"
        />
        <div className="chat-composer-actions">
          <input
            ref={fileInputRef}
            type="file"
            multiple
            tabIndex={-1}
            aria-label="Choose files to attach"
            className="hidden"
            onChange={handleFiles}
          />
          <ChatButton size="icon" aria-label="Attach files" onClick={() => fileInputRef.current?.click()}>
            <Paperclip aria-hidden="true" />
          </ChatButton>
          <ChatButton type="submit" size="icon" tone="ember" aria-label="Send message" disabled={!canSubmit}>
            <ArrowUp aria-hidden="true" />
          </ChatButton>
        </div>
      </div>
      <p role="status" className="chat-composer-status chat-mono">
        {status}
      </p>
    </form>
  );
}
