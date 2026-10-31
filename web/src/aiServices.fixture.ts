import type { components } from "./api-schema";

type View = components["schemas"]["AiServicesView"];

// Synthetic public API fixture. No real credentials or files belong here.
export function aiServicesFixture({
  model = "gpt-6-luna",
  connected = true,
  ...overrides
}: Partial<View> & { model?: string; connected?: boolean } = {}): View {
  const names = ["orientation", "ocr", "transcription", "desc", "facts"];
  return {
    revision: "one",
    proxies: [],
    accounts: [
      {
        number: 1,
        id: "Test account",
        type: "openai",
        proxy: 0,
        connected,
        email: "",
      },
    ],
    features: Object.fromEntries(
      names.map((name) => [
        name,
        {
          account: 1,
          model: name === "transcription" && model ? "gpt-transcribe" : model,
          reasoning_effort: name === "transcription" ? "" : "low",
          image_detail: name === "transcription" ? "" : "original",
          automatic: false,
        },
      ]),
    ),
    definitions: names.map(
      (name): NonNullable<View["definitions"]>[number] => ({
        name,
        ...(name === "transcription"
          ? {
              default_model: "gpt-transcribe",
              models: [
                "gpt-transcribe",
                "gpt-4o-transcribe",
                "gpt-4o-mini-transcribe",
                "gpt-4o-mini-transcribe-2025-12-15",
                "whisper-1",
              ],
            }
          : {}),
        options:
          name === "transcription"
            ? {}
            : {
                reasoning_effort: [
                  "",
                  "none",
                  "minimal",
                  "low",
                  "medium",
                  "high",
                  "xhigh",
                  "max",
                ],
                image_detail: ["", "auto", "low", "high", "original"],
              },
        parameters:
          name === "transcription" ? [] : ["reasoning_effort", "image_detail"],
        account_types:
          name === "transcription" ? ["openai"] : ["chatgpt", "openai"],
      }),
    ),
    pending_account: 0,
    error_account: 0,
    error: "",
    ...overrides,
  };
}
