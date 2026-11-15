import * as React from "react";

export type DecorShape = "plate" | "panel" | "spike" | "halftone" | "speedlines";
export type DecorTone = "ink" | "card" | "red" | "scale" | "ember" | "ash";
export type DecorShadow = "red" | "ember";

export interface DecorProps extends Omit<React.HTMLAttributes<HTMLSpanElement>, "children"> {
  shape: DecorShape;
  tone?: DecorTone;
  shadow?: DecorShadow;
}

/**
 * A decoration layer: the only kind of element the theme may shear or cut.
 * It sits behind the content of a .chat-surface parent, holds no text and is
 * never interactive, so reading stays horizontal and hit areas stay whole.
 */
export function Decor({ shape, tone = "ink", shadow, ...props }: DecorProps) {
  return <span {...props} aria-hidden="true" data-chat-decor={shape} data-tone={tone} data-shadow={shadow} />;
}
