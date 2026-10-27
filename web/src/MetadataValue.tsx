import { ActionIcon, Tooltip } from "@mantine/core";
import {
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";
import { createWrite, type FileDetail } from "./api";
import { InlineValueEdit } from "./InlineValueEdit";
import { FloatingTooltip } from "./FloatingTooltip";
import { useI18n } from "./i18n";

type Row = NonNullable<FileDetail["metadata"]>[number];

export function MetadataValue({
  fileID,
  blobKey,
  row,
  label,
  children,
  editor,
}: {
  fileID: string;
  blobKey: string;
  row: Row;
  label: string;
  children: ReactNode;
  editor?: (row: Row, close: () => void) => ReactNode;
}) {
  const { t } = useI18n();
  // Retain the displayed before-state even if background extraction refreshes.
  const [editing, setEditing] = useState<Row>();
  const trigger = useRef<HTMLButtonElement>(null);
  const root = useRef<HTMLDivElement>(null);
  const [copySpace, setCopySpace] = useState(0);
  useLayoutEffect(() => {
    const element = root.current;
    if (!element) return;
    const copy = element.querySelector<HTMLElement>(
      ".copy-value span, .copy-text-preview button",
    );
    if (!copy) {
      setCopySpace(0);
      return;
    }
    // A floating edit action must not cover a copy action on the first line.
    const measure = () => {
      const box = element.getBoundingClientRect();
      const action = copy.getBoundingClientRect();
      const line = parseFloat(getComputedStyle(element).lineHeight);
      setCopySpace(
        action.top < box.top + line && action.bottom > box.top
          ? action.width + 8
          : 0,
      );
    };
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    observer.observe(copy);
    measure();
    return () => observer.disconnect();
  }, [editing, row.value, t]);
  const editLabel = t("editMetadata", { field: label });
  const close = () => {
    setEditing(undefined);
    trigger.current?.focus();
  };
  return (
    <div
      ref={root}
      className="metadata-value"
      style={{ "--metadata-copy-space": `${copySpace}px` } as CSSProperties}
      data-user={row.source === "user" || undefined}
      data-editing={!!editing || undefined}
    >
      <div className="metadata-value-content">
        {editing ? (
          editor ? (
            editor(editing, close)
          ) : (
            <InlineValueEdit
              initialValue={editing.value}
              label={editLabel}
              multiline={
                editing.key === "ocr:text" ||
                editing.key === "desc:text" ||
                /[\r\n]/u.test(editing.value) ||
                editing.value.length > 160
              }
              close={close}
              write={(value) =>
                createWrite("PUT", `/files/${fileID}/metadata`, {
                  blob_key: blobKey,
                  key: editing.key,
                  before_value: editing.value,
                  before_source: editing.source,
                  before_type: editing.type,
                  value,
                })
              }
            />
          )
        ) : (
          children
        )}
      </div>
      <div className="metadata-value-actions">
        <ActionIcon
          ref={trigger}
          variant="subtle"
          size="xs"
          className="metadata-edit-action"
          aria-label={editLabel}
          disabled={!!editing}
          onClick={() => setEditing({ ...row })}
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 20 20"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.4"
            aria-hidden="true"
          >
            <path d="m12 4 4 4M3 17l1-5L14 2l4 4L8 16z" />
          </svg>
        </ActionIcon>
        {row.source === "user" && (
          <FloatingTooltip
            label={t("metadataUserEdited")}
            position="bottom-start"
            offset={16}
            role="tooltip"
            classNames={{ tooltip: "metadata-tooltip" }}
          >
            <Tooltip
              label={t("metadataUserEdited")}
              position="bottom-start"
              offset={8}
              classNames={{ tooltip: "metadata-tooltip" }}
              openDelay={0}
              transitionProps={{ duration: 0 }}
              events={{ hover: false, focus: true, touch: true }}
            >
              <span
                className="metadata-user-source"
                tabIndex={0}
                role="img"
                aria-label={t("metadataUserEdited")}
              >
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 20 20"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.4"
                  aria-hidden="true"
                >
                  <circle cx="10" cy="6" r="3" />
                  <path d="M4 18v-2a6 6 0 0 1 12 0v2" />
                </svg>
              </span>
            </Tooltip>
          </FloatingTooltip>
        )}
      </div>
    </div>
  );
}
