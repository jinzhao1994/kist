import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { Menu } from "@mantine/core";
import { BackgroundMenu } from "./BackgroundMenu";
import { ThemeProvider } from "./theme";

afterEach(cleanup);
it("opens on background, reuses actions, and preserves control menus", async () => {
  const action = vi.fn();
  render(
    <ThemeProvider env="test">
      <main data-testid="background">
        <BackgroundMenu>
          <Menu.Item onClick={action}>Create</Menu.Item>
        </BackgroundMenu>
        <button>Existing item</button>
        <input aria-label="Search" />
      </main>
    </ThemeProvider>,
  );
  for (const target of [
    screen.getByRole("button"),
    screen.getByRole("textbox"),
  ]) {
    expect(fireEvent.contextMenu(target)).toBe(true);
    expect(screen.queryByRole("menuitem")).not.toBeInTheDocument();
  }
  expect(
    fireEvent.contextMenu(screen.getByTestId("background"), {
      clientX: 200,
      clientY: 240,
    }),
  ).toBe(false);
  fireEvent.click(await screen.findByRole("menuitem", { name: "Create" }));
  expect(action).toHaveBeenCalledOnce();
  fireEvent.contextMenu(screen.getByTestId("background"));
  expect(await screen.findByRole("menuitem")).toBeVisible();
  fireEvent.scroll(screen.getByTestId("background"));
  expect(screen.queryByRole("menuitem")).not.toBeInTheDocument();
});
