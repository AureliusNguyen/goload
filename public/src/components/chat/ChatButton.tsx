import * as React from "react";
import { Decor, type DecorTone } from "@/components/chat/Decor";
import { Button, type ButtonProps } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface ChatButtonProps extends Omit<ButtonProps, "variant" | "asChild"> {
  tone?: DecorTone;
}

/**
 * The shadcn Button, restyled for the chat scope. The button element stays a
 * plain rectangle (full hit area, visible focus outline); the plate is a child
 * layer behind the label. Only amber and red plates carry the tilt.
 */
export const ChatButton = React.forwardRef<HTMLButtonElement, ChatButtonProps>(
  ({ tone = "ink", className, children, type = "button", ...props }, ref) => (
    <Button
      ref={ref}
      type={type}
      variant="ghost"
      data-tone={tone}
      className={cn("chat-surface chat-btn h-auto focus-visible:ring-0 focus-visible:ring-offset-0", className)}
      {...props}
    >
      <Decor shape="plate" tone={tone} />
      {children}
    </Button>
  ),
);
ChatButton.displayName = "ChatButton";
