import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it } from "vitest";
import { FloatingTooltip } from "./FloatingTooltip";
import { ThemeProvider } from "./theme";

it.each(["outside", "scroll", "escape", "blur"] as const)(
  "dismisses a floating tooltip on %s without needing target mouseleave",
  async (action) => {
    render(
      <ThemeProvider>
        <FloatingTooltip label="Full value" role="tooltip">
          <span>Preview</span>
        </FloatingTooltip>
      </ThemeProvider>,
    );
    const target = screen.getByText("Preview");
    await userEvent.hover(target);
    expect(screen.getByRole("tooltip")).toBeVisible();
    fireEvent.mouseMove(target);
    expect(screen.getByRole("tooltip")).toBeVisible();
    if (action === "outside") fireEvent.mouseMove(document.body);
    if (action === "scroll") fireEvent.scroll(document);
    if (action === "escape") fireEvent.keyDown(document, { key: "Escape" });
    if (action === "blur") fireEvent.blur(window);
    await waitFor(() => expect(screen.queryByRole("tooltip")).toBeNull());
    await userEvent.unhover(target);
    await userEvent.hover(target);
    expect(screen.getByRole("tooltip")).toBeVisible();
  },
);
