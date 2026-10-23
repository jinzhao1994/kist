import { render, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
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
