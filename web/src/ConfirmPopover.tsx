import { Button, Group, Popover, Portal, Stack, Text } from "@mantine/core";
import { useId } from "react";
import { useI18n } from "./i18n";
import { useWrites, WriteFeedback, type Write } from "./mutations";

// A compact, explicit confirmation at the pointer or keyboard trigger.
export function ConfirmPopover({
  anchor,
  title,
  description,
  confirmLabel,
  write,
  close,
}: {
  anchor: { x: number; y: number };
  title: string;
  description: string;
  confirmLabel: string;
  write: () => Write;
  close: () => void;
}) {
  const { t } = useI18n();
  const titleID = useId();
  const state = useWrites(close);
  return (
    <Popover
      opened
      withinPortal
      trapFocus
      returnFocus={false}
      position="top-end"
      floatingStrategy="fixed"
      offset={8}
      width="var(--confirm-popover-width)"
      transitionProps={{ transition: "fade", duration: 100, exitDuration: 0 }}
      onChange={(opened) => {
        if (!opened && !state.busy) close();
      }}
    >
      <Portal>
        <Popover.Target>
          <span
            className="pointer-anchor"
            aria-hidden="true"
            style={{ left: anchor.x, top: anchor.y }}
          />
        </Popover.Target>
      </Portal>
      <Popover.Dropdown
        className="confirm-popover"
        role="dialog"
        aria-labelledby={titleID}
        onClick={(event) => event.stopPropagation()}
        onKeyDown={(event) => {
          // Mantine's focus trap handles Tab at document level.
          if (event.key !== "Tab") event.stopPropagation();
        }}
      >
        <Stack gap="xs">
          <Text id={titleID} size="xs">
            {title}
          </Text>
          <Text size="xs" c="dimmed">
            {description}
          </Text>
          <Group gap="xs" justify="flex-end">
            <Button
              autoFocus
              data-autofocus
              variant="subtle"
              size="compact-xs"
              disabled={state.busy}
              onClick={close}
            >
              {t("cancel")}
            </Button>
            <Button
              color="red"
              variant="subtle"
              size="compact-xs"
              loading={state.busy}
              disabled={state.busy || state.locked}
              onClick={() => void state.run([write()])}
            >
              {confirmLabel}
            </Button>
          </Group>
          <WriteFeedback state={state} />
        </Stack>
      </Popover.Dropdown>
    </Popover>
  );
}
