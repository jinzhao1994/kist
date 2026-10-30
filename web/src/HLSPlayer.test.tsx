import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ThemeProvider } from "./theme";
import { I18nProvider } from "./i18n";
import HLSPlayer from "./HLSPlayer";

const engines = vi.hoisted(
  () =>
    [] as {
      config: Record<string, unknown>;
      listeners: Record<string, (...args: any[]) => void>;
      startLoad: ReturnType<typeof vi.fn>;
      stopLoad: ReturnType<typeof vi.fn>;
      destroy: ReturnType<typeof vi.fn>;
      loadSource: ReturnType<typeof vi.fn>;
    }[],
);
vi.mock("hls.js", () => ({
  default: class {
    static isSupported = () => true;
    static Events = {
      ERROR: "error",
      MEDIA_ATTACHED: "attached",
      MANIFEST_PARSED: "parsed",
    };
    listeners: Record<string, (...args: any[]) => void> = {};
    startLoad = vi.fn();
    stopLoad = vi.fn();
    destroy = vi.fn();
    loadSource = vi.fn();
    constructor(public config: Record<string, unknown>) {
      engines.push(this);
    }
    on(event: string, callback: (...args: any[]) => void) {
      this.listeners[event] = callback;
    }
    attachMedia() {
      this.listeners.attached();
      this.listeners.parsed();
    }
  },
}));
afterEach(() => {
  engines.length = 0;
  vi.restoreAllMocks();
});
function mount() {
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
  vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => {});
  return render(
    <ThemeProvider env="test">
      <I18nProvider>
        <HLSPlayer
          src="/synthetic/index.m3u8"
          name="Synthetic video"
          tracks={[
            {
              src: "/subtitle.vtt",
              label: "English",
              language: "en",
              default: true,
            },
          ]}
        />
      </I18nProvider>
    </ThemeProvider>,
  );
}
it("loads a playlist without encoding fragments until play and stops reads on pause or unmount", async () => {
  const view = mount();
  await waitFor(() => expect(engines).toHaveLength(1));
  const engine = engines[0];
  expect(engine.config).toMatchObject({
    autoStartLoad: false,
    maxMaxBufferLength: 12,
    backBufferLength: 6,
  });
  expect(engine.loadSource).toHaveBeenCalledWith("/synthetic/index.m3u8");
  expect(engine.startLoad).not.toHaveBeenCalled();
  const video = screen.getByLabelText("Synthetic video") as HTMLVideoElement;
  expect(video.querySelector("track")).toHaveAttribute("default");
  fireEvent.play(video);
  expect(engine.startLoad).toHaveBeenLastCalledWith(0);
  fireEvent.pause(video);
  expect(engine.stopLoad).toHaveBeenCalledOnce();
  video.currentTime = 75;
  fireEvent.play(video);
  expect(engine.startLoad).toHaveBeenLastCalledWith(75);
  view.unmount();
  expect(engine.destroy).toHaveBeenCalledOnce();
  expect(video).not.toHaveAttribute("src");
});
it("shows a fatal playback failure and creates a fresh engine on explicit retry", async () => {
  mount();
  await waitFor(() => expect(engines).toHaveLength(1));
  act(() => engines[0].listeners.error(null, { fatal: true }));
  expect(screen.getByRole("status")).toBeVisible();
  expect(engines[0].destroy).toHaveBeenCalledOnce();
  fireEvent.click(screen.getByRole("button", { name: "Try again" }));
  await waitFor(() => expect(engines).toHaveLength(2));
  expect(screen.getByLabelText("Synthetic video")).toBeVisible();
});
