import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import App from "@/App";

// AuthProvider signs in over the network on mount. Replace it so these tests
// stay offline; every page under test reads auth through this same module.
vi.mock("@/contexts/AuthContext", () => ({
  AuthProvider: ({ children }: { children: unknown }) => children,
  useAuth: () => ({
    token: null,
    account: null,
    isAuthenticated: false,
    signIn: vi.fn(),
    signUp: vi.fn(),
    signOut: vi.fn(),
  }),
}));

function renderAt(route: string) {
  window.history.pushState({}, "", route);
  return render(<App />);
}

const chatBrand = () => screen.findByRole("link", { name: "GoLoad home" });
const landingHeading = () => screen.findByRole("heading", { level: 1, name: /Your files/i });

// The route is React.lazy. Load the page module once up front so each test's
// findBy query is not racing a cold transform on a slow filesystem.
beforeAll(async () => {
  await import("@/pages/ChatApp");
}, 30_000);

afterEach(() => {
  vi.restoreAllMocks();
});

describe("routing and theme scope", () => {
  it("renders /app inside the scoped theme", async () => {
    renderAt("/app");
    expect((await chatBrand()).closest(".chat-theme")).not.toBeNull();
  });

  it("renders the landing page without the chat theme", async () => {
    renderAt("/");
    expect(await landingHeading()).toBeInTheDocument();
    expect(document.querySelector(".chat-theme")).toBeNull();
  });

  it("renders the 404 page without the chat theme", async () => {
    vi.spyOn(console, "error").mockImplementation(() => {});
    renderAt("/no-such-page");
    expect(await screen.findByText("Oops! Page not found")).toBeInTheDocument();
    expect(document.querySelector(".chat-theme")).toBeNull();
  });

  it("sends an unauthenticated /dashboard visit to the landing page without the chat theme", async () => {
    renderAt("/dashboard");
    expect(await landingHeading()).toBeInTheDocument();
    expect(document.querySelector(".chat-theme")).toBeNull();
  });

  it("removes the chat scope after navigating from /app back to /", async () => {
    renderAt("/app");
    fireEvent.click(await chatBrand());
    expect(await landingHeading()).toBeInTheDocument();
    expect(document.querySelector(".chat-theme")).toBeNull();
  });

  it("leaves html and body untouched after /app unmounts", async () => {
    const before = {
      htmlClass: document.documentElement.className,
      htmlStyle: document.documentElement.getAttribute("style"),
      bodyClass: document.body.className,
      bodyStyle: document.body.getAttribute("style"),
    };
    const view = renderAt("/app");
    await chatBrand();
    view.unmount();
    expect({
      htmlClass: document.documentElement.className,
      htmlStyle: document.documentElement.getAttribute("style"),
      bodyClass: document.body.className,
      bodyStyle: document.body.getAttribute("style"),
    }).toEqual(before);
    expect(document.querySelector(".chat-theme")).toBeNull();
  });
});
