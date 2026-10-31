import { PasswordInput, type PasswordInputProps } from "@mantine/core";
import { useState } from "react";
import { useI18n } from "./i18n";

type Props = Omit<
  PasswordInputProps,
  | "visible"
  | "defaultVisible"
  | "onVisibilityChange"
  | "visibilityToggleIcon"
  | "autoComplete"
>;

function VisibilityIcon({ visible }: { visible: boolean }) {
  return (
    <svg
      className="secret-visibility-icon"
      viewBox="0 0 256 256"
      fill="none"
      stroke="currentColor"
      strokeWidth="16"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {visible ? (
        <>
          <path d="M48 40l160 176M154.91 157.6a40 40 0 01-53.82-59.2M135.53 88.71a40 40 0 0132.3 35.53" />
          <path d="M208.61 169.1C230.41 149.58 240 128 240 128s-32-72-112-72a126 126 0 00-20.68 1.68M74 68.6C33.23 89.24 16 128 16 128s32 72 112 72a118.05 118.05 0 0054-12.6" />
        </>
      ) : (
        <>
          <path d="M128 56c-80 0-112 72-112 72s32 72 112 72 112-72 112-72-32-72-112-72z" />
          <circle cx="128" cy="128" r="40" />
        </>
      )}
    </svg>
  );
}

// Keep the existing secret-field UI without advertising an API key as a login
// password. Unsupported engines keep native masking instead of exposing a key.
export function SecretInput({
  className,
  mod,
  visibilityToggleButtonProps,
  ...props
}: Props) {
  const { t } = useI18n();
  const [visible, setVisible] = useState(false);
  const maskedText =
    typeof CSS !== "undefined" &&
    !!CSS.supports?.("-webkit-text-security", "disc");
  return (
    <PasswordInput
      {...props}
      className={["secret-input", className].filter(Boolean).join(" ")}
      mod={[mod, { "masked-secret": maskedText && !visible }]}
      autoComplete="off"
      spellCheck={false}
      autoCapitalize="none"
      autoCorrect="off"
      visible={maskedText || visible}
      onVisibilityChange={() => setVisible((value) => !value)}
      visibilityToggleIcon={() => <VisibilityIcon visible={visible} />}
      visibilityToggleFocusable
      visibilityToggleButtonProps={{
        ...visibilityToggleButtonProps,
        "aria-label": t(visible ? "hideSecret" : "showSecret"),
        "aria-pressed": visible,
      }}
    />
  );
}
