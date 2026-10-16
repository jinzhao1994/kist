import {
  cleanup,
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { useState } from "react";
import { Dialog, DialogPresence } from "./Dialog";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";

afterEach(cleanup);

it.each(["cancel", "saved", "close"])(
  "retains dialog content through exit after %s",
  async (path) => {
    function Example() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button onClick={() => setOpen(true)}>launch</button>
          <DialogPresence>
            {open && (
              <Dialog open title="Example" onOpenChange={setOpen}>
                <input aria-label="draft" defaultValue="unsaved draft" />
                <button onClick={() => setOpen(false)}>cancel</button>
                <button onClick={() => setOpen(false)}>saved</button>
              </Dialog>
            )}
          </DialogPresence>
        </>
      );
    }
    // Production Mantine transitions are required: test mode removes content immediately.
    render(
      <ThemeProvider>
        <I18nProvider>
          <Example />
        </I18nProvider>
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByText("launch"));
    const input = await screen.findByLabelText("draft");
    await waitFor(() => expect(input).toBeVisible());
    if (path === "close")
      fireEvent.click(document.querySelector(".mantine-Modal-close")!);
    else fireEvent.click(screen.getByText(path));
    expect(input).toBeInTheDocument();
    expect(input).toHaveValue("unsaved draft");
    await waitFor(() => expect(screen.queryByLabelText("draft")).toBeNull());
    fireEvent.click(screen.getByText("launch"));
    const reopened = await screen.findByLabelText("draft");
    await waitFor(() => expect(reopened).toBeVisible());
  },
);

it("enters through a transition when an asynchronous result mounts an open dialog", async () => {
  let nextFrame: FrameRequestCallback | undefined;
  const frames = vi
    .spyOn(window, "requestAnimationFrame")
    .mockImplementation((callback) => {
      nextFrame = callback;
      return 1;
    });
  try {
    render(
      <ThemeProvider>
        <I18nProvider>
          <DialogPresence>
            <Dialog open title="Upload conflict" onOpenChange={() => {}}>
              <input aria-label="conflict choice" />
            </Dialog>
          </DialogPresence>
        </I18nProvider>
      </ThemeProvider>,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    frames.mockRestore();
    await act(async () => {
      nextFrame?.(0);
    });
    const input = await screen.findByLabelText("conflict choice");
    const dialog = input.closest('[role="dialog"]')!;
    expect(dialog).toHaveStyle({ opacity: "0" });
    await waitFor(() => expect(dialog).toHaveStyle({ opacity: "1" }));
    expect((dialog as HTMLElement).style.transitionProperty).toContain(
      "transform",
    );
  } finally {
    frames.mockRestore();
  }
});
