import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { BackgroundTasks } from "./BackgroundTasks";
import { I18nProvider, storageKey } from "./i18n";
import { ThemeProvider } from "./theme";

afterEach(() => vi.unstubAllGlobals());

function mount() {
  localStorage.setItem(
    storageKey,
    JSON.stringify({ language: "en", timezone: "UTC", formatLocale: "en-US" }),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  const view = render(
    <QueryClientProvider client={client}>
      <ThemeProvider env="test">
        <I18nProvider>
          <BackgroundTasks />
        </I18nProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  return { client, ...view };
}

const pending = {
  paused: true,
  scanning: false,
  queued: 1,
  failed: 1,
  running: "",
  error: "",
};
const items = [
  {
    request_id: "request-1",
    file_id: "file-1",
    name: "Certificate.pdf",
    state: "queued",
    file_available: true,
    box: { name: "Synthetic records", start_at: "2026-03" },
  },
  {
    request_id: "request-2",
    file_id: "file-2",
    name: "Scan.png",
    state: "failed",
    error: "context canceled",
    file_available: true,
    box: { name: "Trip", start_at: "2026-03-12T14:30" },
  },
];

function base(path: string) {
  if (path === "/api/v1/ai") return { supported: true, connected: true };
  if (path.endsWith("/settings")) return { model: "test" };
  return pending;
}

it("loads mixed queue details only when expanded and links filenames with the card box context", async () => {
  const reads: string[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      reads.push(path);
      return Response.json(
        path.includes("/jobs?") ? { items, next_cursor: "" } : base(path),
      );
    }),
  );
  const { client, unmount } = mount();
  const row = (await screen.findByRole("rowheader", { name: /^OCR/ })).closest(
    "tr",
  )!;
  expect(reads.some((p) => p.includes("/jobs"))).toBe(false);
  const toggle = within(row).getByRole("button", { name: "Expand OCR tasks" });
  await userEvent.click(toggle);
  const table = await screen.findByRole("table", { name: "OCR tasks" });
  expect(row).toContainElement(table);
  expect(document.querySelectorAll(".task-table > tbody > tr")).toHaveLength(6);
  expect(
    within(table).getByRole("link", { name: "Certificate.pdf" }),
  ).toHaveAttribute("href", "/files?file=file-1");
  expect(within(table).getByText("Synthetic records")).toBeInTheDocument();
  expect(
    within(table).getByText(/2026-03$/, { selector: "time" }),
  ).toBeInTheDocument();
  expect(
    within(table).getByText(/2026-03-12 14:30/, { selector: "time" }),
  ).toBeInTheDocument();
  expect(within(table).getByText("Queued")).toBeInTheDocument();
  expect(within(table).getByText("Failed")).toBeInTheDocument();
  expect(
    within(table).getByText("Task canceled.", { selector: "p" }),
  ).toBeInTheDocument();
  expect(
    within(table).queryByRole("button", { name: /view file/i }),
  ).not.toBeInTheDocument();
  expect(toggle).toHaveAttribute("aria-expanded", "true");
  await userEvent.click(toggle);
  expect(
    screen.queryByRole("table", { name: "OCR tasks" }),
  ).not.toBeInTheDocument();
  expect(
    client
      .getQueryCache()
      .find({ queryKey: ["ocr-queue-jobs", ""] })
      ?.getObserversCount(),
  ).toBe(0);
  unmount();
  client.clear();
});

it("pages through all task states and keeps unavailable files as plain text", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) =>
      Response.json(
        path.includes("/jobs?")
          ? path.includes("cursor=next")
            ? {
                items: [
                  {
                    ...items[1],
                    name: "Removed.png",
                    file_available: false,
                    box: undefined,
                  },
                ],
                next_cursor: "",
              }
            : { items: [items[0]], next_cursor: "next" }
          : base(path),
      ),
    ),
  );
  const { client, unmount } = mount();
  await userEvent.click(
    await screen.findByRole("button", { name: "Expand OCR tasks" }),
  );
  const next = await screen.findByRole("button", { name: "Next page" });
  await waitFor(() => expect(next).toBeEnabled());
  await userEvent.click(next);
  expect(
    await screen.findByText("Removed.png", { selector: "p" }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("link", { name: "Removed.png" }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Next page" })).toBeDisabled();
  await userEvent.click(screen.getByRole("button", { name: "Previous page" }));
  expect(
    await screen.findByRole("link", { name: "Certificate.pdf" }),
  ).toBeInTheDocument();
  unmount();
  client.clear();
});

it("shows loading, a retryable read error, and the empty state inside the expanded row", async () => {
  let rejectRead: (reason: Error) => void = () => undefined;
  let calls = 0;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (path: string) => {
      if (!path.includes("/jobs?")) return Response.json(base(path));
      if (++calls === 1)
        return new Promise<Response>((_, reject) => {
          rejectRead = reject;
        });
      return Response.json({ items: [], next_cursor: "" });
    }),
  );
  const { client, unmount } = mount();
  await userEvent.click(
    await screen.findByRole("button", { name: "Expand OCR tasks" }),
  );
  const details = document.getElementById("background-task-ocr-jobs")!;
  expect(within(details).getByRole("status")).toHaveTextContent("Loading");
  rejectRead(new Error("Synthetic connection failure"));
  expect(await within(details).findByRole("alert")).toHaveTextContent(
    "Something went wrong. Please try again.",
  );
  await userEvent.click(
    within(details).getByRole("button", { name: "Try again" }),
  );
  expect(
    await within(details).findByText("No queued or failed tasks"),
  ).toBeInTheDocument();
  unmount();
  client.clear();
});
