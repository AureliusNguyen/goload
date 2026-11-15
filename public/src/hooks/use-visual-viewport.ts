import { useEffect, type RefObject } from "react";

/**
 * Keeps two CSS variables on the element in step with window.visualViewport:
 * --chat-vvh (visible height) and --chat-vv-top (offset inside the layout
 * viewport). The chat root is sized with them, so when a phone keyboard opens
 * the workspace shrinks to the visible area and the composer stays in view.
 * Without the API the variables stay unset and the stylesheet falls back to
 * the dynamic viewport height.
 */
export function useVisualViewportVars(ref: RefObject<HTMLElement>): void {
  useEffect(() => {
    const element = ref.current;
    const viewport = window.visualViewport;
    if (!element || !viewport) return;

    const apply = () => {
      // Pinch zoom shrinks the visual viewport too. Resizing the layout then
      // would fight the zoom, so keep the last unzoomed values.
      if (Math.abs(viewport.scale - 1) > 0.01) return;
      element.style.setProperty("--chat-vvh", `${Math.round(viewport.height)}px`);
      element.style.setProperty("--chat-vv-top", `${Math.round(viewport.offsetTop)}px`);
    };

    apply();
    viewport.addEventListener("resize", apply);
    viewport.addEventListener("scroll", apply);
    return () => {
      viewport.removeEventListener("resize", apply);
      viewport.removeEventListener("scroll", apply);
      element.style.removeProperty("--chat-vvh");
      element.style.removeProperty("--chat-vv-top");
    };
  }, [ref]);
}
