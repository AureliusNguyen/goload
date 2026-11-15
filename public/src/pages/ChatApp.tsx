import { Link } from "react-router-dom";
import { ChatTheme } from "@/components/chat/ChatTheme";
import { Decor } from "@/components/chat/Decor";

const ChatApp = () => (
  <ChatTheme className="chat-root flex flex-col">
    <header className="chat-header">
      <Link to="/" aria-label="GoLoad home" className="chat-surface chat-brand chat-display">
        <Decor shape="spike" tone="red" />
        GOLOAD
      </Link>
      <h1 className="chat-title">Chat workspace</h1>
    </header>
  </ChatTheme>
);

export default ChatApp;
