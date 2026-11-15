export type ChatRole = "user" | "assistant";
export type MessageStatus = "streaming" | "complete";

export interface ChatMessage {
  id: string;
  role: ChatRole;
  content: string;
  status: MessageStatus;
  createdAt: number;
}

export interface Conversation {
  id: string;
  title: string;
  createdAt: number;
  messages: ChatMessage[];
}

export interface ChatState {
  conversations: Conversation[];
  activeId: string;
}

/** A file the user picked. Name and size only: nothing is read or uploaded. */
export interface AttachmentStub {
  id: string;
  name: string;
  size: number;
}
