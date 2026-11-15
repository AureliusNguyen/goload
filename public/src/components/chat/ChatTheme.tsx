import * as React from "react";
import { cn } from "@/lib/utils";
import "./chat-theme.css";

export const CHAT_THEME_CLASS = "chat-theme";

/**
 * The scoping wrapper for the chat workspace. Every rule in chat-theme.css is
 * prefixed with this class, so the comic theme exists only inside this element.
 * Anything rendered through a portal must carry the class itself.
 */
export const ChatTheme = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => <div ref={ref} className={cn(CHAT_THEME_CLASS, className)} {...props} />,
);
ChatTheme.displayName = "ChatTheme";
