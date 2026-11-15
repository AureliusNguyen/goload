import { render } from "@testing-library/react";
import { useRef } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { useVisualViewportVars } from "@/hooks/use-visual-viewport";

function Probe() {
  const ref = useRef<HTMLDivElement>(null);
  useVisualViewportVars(ref);
  return <div ref={ref} data-testid="probe" />;
}

// jsdom has no visualViewport: install a minimal stand-in.
function installVisualViewport(height: number) {
  const listeners = new Map<string, Set<() => void>>();
  const viewport = {
    height,
    offsetTop: 0,
    scale: 1,
    addEventListener: (type: string, listener: () => void) => {
      if (!listeners.has(type)) listeners.set(type, new Set());
      listeners.get(type)!.add(listener);
    },
    removeEventListener: (type: string, listener: () => void) => {
      listeners.get(type)?.delete(listener);
    },
  };
  Object.defineProperty(window, "visualViewport", { configurable: true, value: viewport });
  return {
    viewport,
    emit: (type: "resize" | "scroll") => listeners.get(type)?.forEach((listener) => listener()),
    listenerCount: () => [...listeners.values()].reduce((total, set) => total + set.size, 0),
  };
}

afterEach(() => {
  delete (window as { visualViewport?: unknown }).visualViewport;
});

describe("useVisualViewportVars", () => {
  it("sizes the element to the visual viewport and follows the keyboard", () => {
    const { viewport, emit } = installVisualViewport(844);
    const probe = render(<Probe />).getByTestId("probe");
    expect(probe.style.getPropertyValue("--chat-vvh")).toBe("844px");
    expect(probe.style.getPropertyValue("--chat-vv-top")).toBe("0px");

    // The keyboard opens: the visible area shrinks and is offset.
    viewport.height = 420.4;
    viewport.offsetTop = 12;
    emit("resize");
    expect(probe.style.getPropertyValue("--chat-vvh")).toBe("420px");
    expect(probe.style.getPropertyValue("--chat-vv-top")).toBe("12px");
  });

  it("ignores pinch zoom", () => {
    const { viewport, emit } = installVisualViewport(844);
    const probe = render(<Probe />).getByTestId("probe");
    viewport.scale = 2;
    viewport.height = 422;
    emit("resize");
    expect(probe.style.getPropertyValue("--chat-vvh")).toBe("844px");
  });

  it("removes its listeners and variables on unmount", () => {
    const { listenerCount } = installVisualViewport(844);
    const view = render(<Probe />);
    const probe = view.getByTestId("probe");
    expect(listenerCount()).toBe(2);
    view.unmount();
    expect(listenerCount()).toBe(0);
    expect(probe.style.getPropertyValue("--chat-vvh")).toBe("");
    expect(probe.style.getPropertyValue("--chat-vv-top")).toBe("");
  });

  it("does nothing when the browser has no visual viewport", () => {
    const probe = render(<Probe />).getByTestId("probe");
    expect(probe.style.getPropertyValue("--chat-vvh")).toBe("");
  });
});
