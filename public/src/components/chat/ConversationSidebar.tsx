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
 * The conversation list. Each row is a full-width rectangular button with a
 * sheared plate behind it. The active row carries the ember marker, one of
 * the three ember placements in the workspace.
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
      <ChatButton tone="red" shadow="ember" aria-label="Start a new chat" className="chat-new" onClick={onNewChat}>
        <Plus aria-hidden="true" />
        New chat
      </ChatButton>
      <ul className="chat-nav-list chat-scroll">
        {conversations.map((conversation) => {
          const active = conversation.id === activeId;
          return (
            <li key={conversation.id}>
              <ChatButton
                tone={active ? "scale" : "ink"}
                aria-current={active ? "true" : undefined}
                className="chat-nav-item"
                onClick={() => onSelect(conversation.id)}
              >
                {active ? (
                  <Decor shape="plate" tone="ember" data-chat-active-marker="" className="chat-active-marker" />
                ) : null}
                <span className="chat-nav-title">{conversation.title}</span>
                {streamingIds.has(conversation.id) ? <span className="chat-nav-state chat-mono">replying</span> : null}
              </ChatButton>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
