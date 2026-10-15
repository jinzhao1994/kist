import { cleanup, render } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { ContextMenu } from "./ContextMenu";
import { ThemeProvider } from "./theme";

afterEach(cleanup);

it("retains the last trigger position while closing and updates it on reopening", () => {
  const page = (anchor: { x: number; y: number } | null) => (
    <ThemeProvider env="test">
      <ContextMenu anchor={anchor} close={() => {}}>
        Action
      </ContextMenu>
    </ThemeProvider>
  );
  const { rerender } = render(page({ x: 320, y: 240 }));
  const trigger = () =>
    document.querySelector<HTMLElement>(
      'span[aria-hidden="true"][style*="position: fixed"]',
    )!;
  expect(trigger()).toHaveStyle({ left: "320px", top: "240px" });
  rerender(page(null));
  expect(trigger()).toHaveStyle({ left: "320px", top: "240px" });
  rerender(page({ x: 480, y: 360 }));
  expect(trigger()).toHaveStyle({ left: "480px", top: "360px" });
});
