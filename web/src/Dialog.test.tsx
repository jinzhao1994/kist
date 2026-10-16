import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
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
    if (path === "close")
      fireEvent.click(document.querySelector(".mantine-Modal-close")!);
    else fireEvent.click(screen.getByText(path));
    expect(input).toBeInTheDocument();
    expect(input).toHaveValue("unsaved draft");
    await waitFor(() => expect(screen.queryByLabelText("draft")).toBeNull());
    fireEvent.click(screen.getByText("launch"));
    expect(await screen.findByLabelText("draft")).toBeVisible();
  },
);
