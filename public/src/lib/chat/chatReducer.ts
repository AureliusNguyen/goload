import type { ChatMessage, ChatState, Conversation } from "./types";

export const NEW_CHAT_TITLE = "New chat";
export const TITLE_MAX_LENGTH = 40;

export type ChatAction =
  | { type: "conversation/create"; id: string; createdAt: number }
  | { type: "conversation/select"; id: string }
  | { type: "message/append"; conversationId: string; message: ChatMessage }
  | { type: "message/delta"; conversationId: string; messageId: string; delta: string }
  | { type: "message/complete"; conversationId: string; messageId: string };

export function deriveTitle(text: string): string {
  const flat = text.replace(/\s+/g, " ").trim();
  if (flat === "") return NEW_CHAT_TITLE;
  return flat.length > TITLE_MAX_LENGTH ? `${flat.slice(0, TITLE_MAX_LENGTH)}...` : flat;
}

export function findConversation(state: ChatState, id: string): Conversation | undefined {
  return state.conversations.find((conversation) => conversation.id === id);
}

export function isConversationStreaming(conversation: Conversation | undefined): boolean {
  return Boolean(conversation?.messages.some((message) => message.status === "streaming"));
}

/** The single choke point for "may this conversation send now". */
export function canSend(state: ChatState, conversationId: string, text: string): boolean {
  const conversation = findConversation(state, conversationId);
  if (!conversation) return false;
  if (text.trim() === "") return false;
  return !isConversationStreaming(conversation);
}

// Both helpers return the input object when nothing changed, so a no-op action
// keeps the same state reference.
function updateConversation(
  state: ChatState,
  id: string,
  update: (conversation: Conversation) => Conversation,
): ChatState {
  let changed = false;
  const conversations = state.conversations.map((conversation) => {
    if (conversation.id !== id) return conversation;
    const next = update(conversation);
    if (next !== conversation) changed = true;
    return next;
  });
  return changed ? { ...state, conversations } : state;
}

function updateMessage(
  conversation: Conversation,
  messageId: string,
  update: (message: ChatMessage) => ChatMessage,
): Conversation {
  let changed = false;
  const messages = conversation.messages.map((message) => {
    if (message.id !== messageId) return message;
    const next = update(message);
    if (next !== message) changed = true;
    return next;
  });
  return changed ? { ...conversation, messages } : conversation;
}

export function chatReducer(state: ChatState, action: ChatAction): ChatState {
  switch (action.type) {
    case "conversation/create": {
      // An unused empty chat is reused, so repeated clicks never pile up blanks.
      const reusable = state.conversations.find((conversation) => conversation.messages.length === 0);
      if (reusable) return reusable.id === state.activeId ? state : { ...state, activeId: reusable.id };
      if (findConversation(state, action.id)) return state;
      const conversation: Conversation = {
        id: action.id,
        title: NEW_CHAT_TITLE,
        createdAt: action.createdAt,
        messages: [],
      };
      return { conversations: [conversation, ...state.conversations], activeId: conversation.id };
    }
    case "conversation/select":
      if (action.id === state.activeId || !findConversation(state, action.id)) return state;
      return { ...state, activeId: action.id };
    case "message/append":
      return updateConversation(state, action.conversationId, (conversation) => {
        const firstUserTurn =
          action.message.role === "user" && !conversation.messages.some((message) => message.role === "user");
        return {
          ...conversation,
          title: firstUserTurn ? deriveTitle(action.message.content) : conversation.title,
          messages: [...conversation.messages, action.message],
        };
      });
    // Deltas and completion are addressed by conversation id and message id,
    // never by the active conversation: a reply stays where it was asked.
    case "message/delta":
      return updateConversation(state, action.conversationId, (conversation) =>
        updateMessage(conversation, action.messageId, (message) =>
          message.status === "streaming" ? { ...message, content: message.content + action.delta } : message,
        ),
      );
    case "message/complete":
      return updateConversation(state, action.conversationId, (conversation) =>
        updateMessage(conversation, action.messageId, (message) =>
          message.status === "streaming" ? { ...message, status: "complete" } : message,
        ),
      );
    default:
      return state;
  }
}
