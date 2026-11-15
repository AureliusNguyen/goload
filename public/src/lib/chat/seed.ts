import { NEW_CHAT_TITLE } from "./chatReducer";
import type { ChatState } from "./types";

const FENCE = "`".repeat(3);
const HOUR_MS = 60 * 60 * 1000;

/**
 * The mocked "past conversations" the workspace opens with. Session-only.
 * Every seeded title starts with "Sample: " and every seeded reply with
 * "Sample conversation: ", so nothing here reads as real history.
 */
export function createSeedState(now: number = Date.now()): ChatState {
  const checksumAt = now - 2 * HOUR_MS;
  const queueAt = now - 26 * HOUR_MS;
  return {
    activeId: "chat-seed-new",
    conversations: [
      { id: "chat-seed-new", title: NEW_CHAT_TITLE, createdAt: now, messages: [] },
      {
        id: "chat-seed-checksum",
        title: "Sample: verify a checksum",
        createdAt: checksumAt,
        messages: [
          {
            id: "seed-checksum-1",
            role: "user",
            content: "How do I check that an ISO arrived intact?",
            status: "complete",
            createdAt: checksumAt,
          },
          {
            id: "seed-checksum-2",
            role: "assistant",
            content: [
              "Sample conversation: nothing ran on any machine.",
              "",
              `${FENCE}bash`,
              "# sample only",
              "sha256sum big-file.iso",
              FENCE,
              "",
              "Compare the printed hash with the one the publisher lists.",
            ].join("\n"),
            status: "complete",
            createdAt: checksumAt,
          },
        ],
      },
      {
        id: "chat-seed-queue",
        title: "Sample: queue a big ISO",
        createdAt: queueAt,
        messages: [
          {
            id: "seed-queue-1",
            role: "user",
            content: "Queue https://example.com/big-file.iso for tonight.",
            status: "complete",
            createdAt: queueAt,
          },
          {
            id: "seed-queue-2",
            role: "assistant",
            content:
              "Sample conversation: this preview has no backend. In the real product I would hand that link to goload as a download task. Here nothing was fetched, queued, or saved.",
            status: "complete",
            createdAt: queueAt,
          },
        ],
      },
    ],
  };
}
