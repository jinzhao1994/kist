import { useState } from "react";
import { render, screen, fireEvent, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import { Dialog } from "./Dialog";
import { PartialDateInput } from "./PartialDateInput";
function Example({ initial = "", referenceDate = "2026-10" }) {
  const [value, setValue] = useState(initial);
  return (
    <ThemeProvider env="test">
      <I18nProvider>
        <PartialDateInput
          id="date"
          label="Date"
          value={value}
          onChange={setValue}
          referenceDate={referenceDate}
        />
        <button>Outside</button>
      </I18nProvider>
    </ThemeProvider>
  );
}
it("preserves year and month precision when picking, and returns focus without reopening", async () => {
  const user = userEvent.setup();
  render(<Example initial="2026" />);
  const input = screen.getByRole("textbox", { name: "Date" });
  await user.click(input);
  await user.click(screen.getByRole("button", { name: "Next year" }));
  expect(input).toHaveValue("2026");
  await user.click(screen.getByRole("button", { name: "Use 2027" }));
  expect(input).toHaveValue("2027");
  expect(input).toHaveFocus();
  expect(input).toHaveAttribute("aria-expanded", "false");
  await user.click(input);
  await user.click(screen.getByRole("button", { name: "Feb" }));
  expect(input).toHaveValue("2027");
  await user.click(screen.getByRole("button", { name: "Use 2027-2" }));
  expect(input).toHaveValue("2027-02");
});
it("uses the reference month for an empty field and selects a full date", async () => {
  const user = userEvent.setup();
  render(<Example />);
  const input = screen.getByRole("textbox", { name: "Date" });
  await user.click(input);
  expect(
    screen.getByRole("button", { name: "Choose month" }),
  ).toHaveTextContent("October 2026");
  await user.click(
    within(screen.getByRole("dialog")).getByRole("button", {
      name: /17 October 2026/,
    }),
  );
  expect(input).toHaveValue("2026-10-17");
});
it("leaves typed partial or invalid text intact and closes on Escape and blur", async () => {
  const user = userEvent.setup();
  render(<Example />);
  const input = screen.getByRole("textbox", { name: "Date" });
  await user.click(input);
  fireEvent.change(input, { target: { value: "2026-02-31" } });
  await user.keyboard("{Escape}");
  expect(input).toHaveValue("2026-02-31");
  expect(input).toHaveAttribute("aria-expanded", "false");
  await user.click(input);
  fireEvent.change(input, { target: { value: "2026-10" } });
  await user.click(screen.getByRole("button", { name: "Outside" }));
  expect(input).toHaveValue("2026-10");
  expect(input).toHaveAttribute("aria-expanded", "false");
});

it("closes only the calendar on Escape inside a modal", async () => {
  const user = userEvent.setup();
  const close = vi.fn();
  render(
    <ThemeProvider env="test">
      <I18nProvider>
        <Dialog open title="Editor" onOpenChange={close}>
          <PartialDateInput
            id="modal-date"
            label="Date"
            value="2026"
            onChange={() => {}}
          />
        </Dialog>
      </I18nProvider>
    </ThemeProvider>,
  );
  const input = screen.getByRole("textbox", { name: "Date" });
  await user.click(input);
  await user.keyboard("{Escape}");
  expect(close).not.toHaveBeenCalled();
  expect(input).toHaveAttribute("aria-expanded", "false");
  await user.click(input);
  await user.click(screen.getByRole("button", { name: "Next year" }));
  await user.keyboard("{Escape}");
  expect(close).not.toHaveBeenCalled();
  expect(input).toHaveFocus();
  await user.keyboard("{Escape}");
  expect(close).toHaveBeenCalledWith(false);
});

it("does not highlight an invented day for a month-only value", async () => {
  const user = userEvent.setup();
  render(<Example initial="2026-10" />);
  await user.click(screen.getByRole("textbox", { name: "Date" }));
  expect(
    screen.getByRole("button", { name: "1 October 2026" }),
  ).not.toHaveAttribute("data-selected", "true");
});
