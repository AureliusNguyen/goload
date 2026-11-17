import { useRef, useState } from "react";
import { Link } from "react-router-dom";
import { Menu, PanelLeft } from "lucide-react";
import { ChatButton } from "@/components/chat/ChatButton";
import { ChatTheme } from "@/components/chat/ChatTheme";
import { ChatThread } from "@/components/chat/ChatThread";
import { Composer } from "@/components/chat/Composer";
import { ConversationSidebar } from "@/components/chat/ConversationSidebar";
import { Decor } from "@/components/chat/Decor";
import { Sheet, SheetClose, SheetContent, SheetDescription, SheetTitle } from "@/components/ui/sheet";
import { useChat } from "@/contexts/ChatContext";
import { useIsMobile } from "@/hooks/use-mobile";
import { useVisualViewportVars } from "@/hooks/use-visual-viewport";

/**
 * The two-pane chat workspace. Below 768px the sidebar becomes a drawer.
 * The drawer is a Radix portal under body, outside the scope wrapper, so its
 * content carries the chat-theme class itself.
 */
export function ChatWorkspace() {
  const { conversations, activeId, activeConversation, streamingIds, newConversation, selectConversation, sendMessage } =
    useChat();
  const isMobile = useIsMobile();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [railOpen, setRailOpen] = useState(true);
  // The composer remounts when the thread flips between empty and non-empty
  // (it moves between the intro slot and the dock), so the draft lives here.
  const [draft, setDraft] = useState("");
  const rootRef = useRef<HTMLDivElement>(null);
  useVisualViewportVars(rootRef);

  const renderSidebar = (afterAction?: () => void) => (
    <ConversationSidebar
      conversations={conversations}
      activeId={activeId}
      streamingIds={streamingIds}
      onSelect={(id) => {
        selectConversation(id);
        afterAction?.();
      }}
      onNewChat={() => {
        newConversation();
        afterAction?.();
      }}
    />
  );

  const empty = activeConversation.messages.length === 0;
  const composer = (
    <Composer
      layout={empty ? "centered" : "docked"}
      draft={draft}
      onDraftChange={setDraft}
      busy={streamingIds.has(activeId)}
      onSend={(text) => sendMessage(activeId, text)}
    />
  );

  return (
    <ChatTheme ref={rootRef} className="chat-root flex flex-col">
      <header className="chat-header">
        {isMobile ? (
          <ChatButton
            size="icon"
            aria-label="Open conversations"
            aria-expanded={drawerOpen}
            onClick={() => setDrawerOpen(true)}
          >
            <Menu aria-hidden="true" />
          </ChatButton>
        ) : (
          <ChatButton
            size="icon"
            aria-label={railOpen ? "Hide conversations" : "Show conversations"}
            aria-expanded={railOpen}
            onClick={() => setRailOpen((open) => !open)}
          >
            <PanelLeft aria-hidden="true" />
          </ChatButton>
        )}
        <Link to="/" aria-label="GoLoad home" className="chat-surface chat-brand chat-display">
          <Decor shape="plate" tone="red" />
          GOLOAD
        </Link>
        <h1 className="chat-title">{activeConversation.title}</h1>
        <p className="chat-preview-note">
          PREVIEW<span className="hidden sm:inline">: simulated replies, nothing is downloaded or uploaded</span>
        </p>
      </header>

      <div className="chat-body flex min-h-0 flex-1">
        {/* The class "hidden md:flex" hides this before useIsMobile has measured. */}
        {isMobile || !railOpen ? null : <aside className="chat-sidebar hidden md:flex">{renderSidebar()}</aside>}
        <main className="chat-main flex min-w-0 flex-1 flex-col">
          <ChatThread
            conversationId={activeId}
            messages={activeConversation.messages}
            onSuggestion={(text) => {
              sendMessage(activeId, text);
            }}
            emptySlot={empty ? composer : undefined}
          />
          {empty ? null : composer}
        </main>
      </div>

      {isMobile ? (
        <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
          <SheetContent side="left" className="chat-theme chat-drawer [&>button.absolute]:hidden">
            <SheetTitle className="sr-only">Conversations</SheetTitle>
            <SheetDescription className="sr-only">Pick a conversation or start a new chat.</SheetDescription>
            {renderSidebar(() => setDrawerOpen(false))}
            <SheetClose asChild>
              <ChatButton>Close</ChatButton>
            </SheetClose>
          </SheetContent>
        </Sheet>
      ) : null}
    </ChatTheme>
  );
}
