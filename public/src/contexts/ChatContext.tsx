import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef } from "react";
import { canSend, chatReducer, findConversation, isConversationStreaming } from "@/lib/chat/chatReducer";
import { pickMockReply, prefersReducedMotion, streamReply, type StreamOptions } from "@/lib/chat/mockAssistant";
import { createSeedState } from "@/lib/chat/seed";
import type { ChatState, Conversation } from "@/lib/chat/types";

export interface ChatContextValue {
  conversations: Conversation[];
  activeId: string;
  activeConversation: Conversation;
  /** Ids of the conversations whose reply is still arriving. */
  streamingIds: ReadonlySet<string>;
  newConversation: () => void;
  selectConversation: (id: string) => void;
  /** Returns false, and changes nothing, when the message is refused. */
  sendMessage: (conversationId: string, text: string) => boolean;
}

export interface ChatProviderProps {
  children: React.ReactNode;
  initialState?: ChatState;
  streamOptions?: StreamOptions;
}

const ChatContext = createContext<ChatContextValue | undefined>(undefined);

function initState(initial: ChatState | undefined): ChatState {
  const state = initial && initial.conversations.length > 0 ? initial : createSeedState();
  return findConversation(state, state.activeId) ? state : { ...state, activeId: state.conversations[0].id };
}

/**
 * Session-only chat state plus the mock assistant. Nothing is persisted and
 * nothing leaves the browser. Swapping this provider for one backed by an API
 * is the seam for the later backend slice.
 */
export const ChatProvider = ({ children, initialState, streamOptions }: ChatProviderProps) => {
  const [state, dispatch] = useReducer(chatReducer, initialState, initState);
  const stateRef = useRef(state);
  stateRef.current = state;
  const streamOptionsRef = useRef(streamOptions);
  streamOptionsRef.current = streamOptions;
  const sequence = useRef(0);
  // conversation id -> cancel function of its running stream.
  const activeStreams = useRef(new Map<string, () => void>());

  useEffect(() => {
    const streams = activeStreams.current;
    return () => {
      streams.forEach((cancel) => cancel());
      streams.clear();
    };
  }, []);

  const nextId = useCallback((prefix: string) => {
    sequence.current += 1;
    return `${prefix}-${sequence.current}`;
  }, []);

  const newConversation = useCallback(() => {
    dispatch({ type: "conversation/create", id: nextId("chat"), createdAt: Date.now() });
  }, [nextId]);

  const selectConversation = useCallback((id: string) => {
    dispatch({ type: "conversation/select", id });
  }, []);

  const sendMessage = useCallback(
    (conversationId: string, text: string): boolean => {
      // The map is checked first: it is updated synchronously, so a second call
      // in the same tick is refused before React has re-rendered.
      if (activeStreams.current.has(conversationId)) return false;
      if (!canSend(stateRef.current, conversationId, text)) return false;

      const now = Date.now();
      const userId = nextId("msg");
      const replyId = nextId("msg");
      dispatch({
        type: "message/append",
        conversationId,
        message: { id: userId, role: "user", content: text.trim(), status: "complete", createdAt: now },
      });
      dispatch({
        type: "message/append",
        conversationId,
        message: { id: replyId, role: "assistant", content: "", status: "streaming", createdAt: now },
      });

      // The handlers close over the conversation that asked, not the active one.
      const cancel = streamReply(
        pickMockReply(text),
        {
          onDelta: (delta) => dispatch({ type: "message/delta", conversationId, messageId: replyId, delta }),
          onDone: () => {
            activeStreams.current.delete(conversationId);
            dispatch({ type: "message/complete", conversationId, messageId: replyId });
          },
        },
        { ...streamOptionsRef.current, reducedMotion: prefersReducedMotion() },
      );
      activeStreams.current.set(conversationId, cancel);
      return true;
    },
    [nextId],
  );

  const streamingIds = useMemo(
    () =>
      new Set(
        state.conversations
          .filter((conversation) => isConversationStreaming(conversation))
          .map((conversation) => conversation.id),
      ),
    [state.conversations],
  );
  const activeConversation = findConversation(state, state.activeId) ?? state.conversations[0];

  const value = useMemo<ChatContextValue>(
    () => ({
      conversations: state.conversations,
      activeId: state.activeId,
      activeConversation,
      streamingIds,
      newConversation,
      selectConversation,
      sendMessage,
    }),
    [state.conversations, state.activeId, activeConversation, streamingIds, newConversation, selectConversation, sendMessage],
  );

  return <ChatContext.Provider value={value}>{children}</ChatContext.Provider>;
};

// eslint-disable-next-line react-refresh/only-export-components
export const useChat = () => {
  const context = useContext(ChatContext);
  if (!context) throw new Error("useChat must be used within a ChatProvider");
  return context;
};
