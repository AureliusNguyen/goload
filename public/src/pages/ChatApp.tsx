import { ChatWorkspace } from "@/components/chat/ChatWorkspace";
import { ChatProvider } from "@/contexts/ChatContext";

// The /app page. ChatProvider is the seam for the later backend slice: replace
// it with an API-backed provider and the workspace stays as it is.
const ChatApp = () => (
  <ChatProvider>
    <ChatWorkspace />
  </ChatProvider>
);

export default ChatApp;
