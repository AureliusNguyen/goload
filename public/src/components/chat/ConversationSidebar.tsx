import { Plus } from "lucide-react";
import { ChatButton } from "@/components/chat/ChatButton";
import { Decor } from "@/components/chat/Decor";
import type { Conversation } from "@/lib/chat/types";

export interface ConversationSidebarProps {
  conversations: Conversation[];
  activeId: string;
  /** Ids of the conversations whose reply is still arriving. */
  streamingIds: ReadonlySet<string>;
  onSelect: (id: string) => void;
  onNewChat: () => void;
}

/**
 * The conversation list: a straight rail of plain rows. The active row carries
 * a small angled amber marker, which is where the tilt lives in this pane.
 */
export function ConversationSidebar({
  conversations,
  activeId,
  streamingIds,
  onSelect,
  onNewChat,
}: ConversationSidebarProps) {
  return (
    <nav aria-label="Conversations" className="chat-nav">
      <ChatButton aria-label="Start a new chat" className="chat-new" onClick={onNewChat}>
        <Plus aria-hidden="true" />
        New chat
      </ChatButton>
      <ul className="chat-nav-list chat-scroll">
        {conversations.map((conversation) => {
          const active = conversation.id === activeId;
          return (
            <li key={conversation.id}>
              <button
                type="button"
                aria-current={active ? "true" : undefined}
                className="chat-surface chat-nav-item"
                onClick={() => onSelect(conversation.id)}
              >
                {active ? <Decor shape="plate" tone="ember" data-chat-active-marker="" className="chat-active-marker" /> : null}
                <span className="chat-nav-title">{conversation.title}</span>
                {streamingIds.has(conversation.id) ? <span className="chat-nav-state">replying</span> : null}
              </button>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
