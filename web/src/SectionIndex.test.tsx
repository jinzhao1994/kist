import { act, fireEvent, render, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { SectionIndex } from "./SectionIndex";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";

it("waits for the sticky header measurement before restoring the fragment", async () => {
  window.history.replaceState(null, "", "/settings#settings-service");
  const scroll = vi.spyOn(Element.prototype, "scrollIntoView");
  function View({ ready }: { ready: boolean }) {
    return (
      <ThemeProvider env="test">
        <I18nProvider>
          <SectionIndex
            ready={ready}
            items={[{ id: "settings-service", label: "Service" }]}
          >
            <section id="settings-service">Service</section>
          </SectionIndex>
        </I18nProvider>
      </ThemeProvider>
    );
  }
  const view = render(<View ready={false} />);
  await new Promise(requestAnimationFrame);
  expect(scroll).not.toHaveBeenCalled();
  view.rerender(<View ready />);
  await waitFor(() => expect(scroll).toHaveBeenCalledTimes(1));
  expect(scroll.mock.instances[0]).toBe(
    document.getElementById("settings-service"),
  );
  view.rerender(<View ready />);
  await new Promise(requestAnimationFrame);
  expect(scroll).toHaveBeenCalledTimes(1);
  window.history.replaceState(null, "", "/");
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  window.history.replaceState(null, "", "/");
});
it("follows late layout changes until the user scrolls and resumes on index navigation", async () => {
  window.history.replaceState(null, "", "/settings#settings-location");
  let resized: (() => void) | undefined;
  const observe = vi.fn(),
    disconnect = vi.fn();
  vi.stubGlobal(
    "ResizeObserver",
    class {
      constructor(callback: () => void) {
        resized = callback;
      }
      observe = observe;
      disconnect = disconnect;
    },
  );
  const scroll = vi.spyOn(Element.prototype, "scrollIntoView");
  const view = render(
    <ThemeProvider env="test">
      <I18nProvider>
        <SectionIndex items={[{ id: "settings-location", label: "GPS" }]}>
          <section>Loading AI settings</section>
          <section id="settings-location">GPS settings</section>
        </SectionIndex>
      </I18nProvider>
    </ThemeProvider>,
  );
  await waitFor(() => expect(scroll).toHaveBeenCalledTimes(1));
  expect(resized).toBeTypeOf("function");
  act(() => resized!());
  await waitFor(() => expect(scroll).toHaveBeenCalledTimes(2));
  expect(scroll.mock.instances[1]).toBe(
    document.getElementById("settings-location"),
  );
  fireEvent.wheel(document.body);
  act(() => resized!());
  await act(async () => {
    await new Promise(requestAnimationFrame);
  });
  expect(scroll).toHaveBeenCalledTimes(2);
  fireEvent.click(view.getByRole("link", { name: "GPS" }));
  await waitFor(() => expect(scroll).toHaveBeenCalledTimes(3));
  act(() => resized!());
  await waitFor(() => expect(scroll).toHaveBeenCalledTimes(4));
  view.unmount();
  expect(disconnect).toHaveBeenCalled();
});
