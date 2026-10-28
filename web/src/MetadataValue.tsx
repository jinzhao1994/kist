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
import { ConfirmPopover } from "./ConfirmPopover";

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
  const removeTrigger = useRef<HTMLButtonElement>(null);
  const [removing, setRemoving] = useState<{
    row: Row;
    fileID: string;
    blobKey: string;
    x: number;
    y: number;
  }>();
  const root = useRef<HTMLDivElement>(null);
  const wasRemoving = useRef(false);
  useLayoutEffect(() => {
    if (wasRemoving.current && !removing) removeTrigger.current?.focus();
    wasRemoving.current = !!removing;
  }, [removing]);
  const [copySpace, setCopySpace] = useState(0);
  useLayoutEffect(() => {
    const element = root.current;
    if (!element) return;
    const copy = element.querySelector<HTMLElement>(
      ".copy-value span, .copy-text-action",
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
      data-confirming={!!removing || undefined}
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
        <div className="metadata-value-transient-actions">
          <ActionIcon
            ref={trigger}
            variant="subtle"
            size="xs"
            className="metadata-edit-action"
            aria-label={editLabel}
            disabled={!!editing || !!removing}
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
          <ActionIcon
            ref={removeTrigger}
            variant="subtle"
            size="xs"
            className="metadata-edit-action"
            aria-label={t("removeMetadata", { field: label })}
            disabled={!!editing || !!removing}
            onClick={(event) => {
              const rect = event.currentTarget.getBoundingClientRect();
              setRemoving({
                row: { ...row },
                fileID,
                blobKey,
                x: event.detail ? event.clientX : rect.left,
                y: event.detail ? event.clientY : rect.bottom,
              });
            }}
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
              <path d="M3 5h14M7 5V2h6v3M5 5l1 13h8l1-13M8 8v7m4-7v7" />
            </svg>
          </ActionIcon>
        </div>
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
      {removing && (
        <ConfirmPopover
          anchor={removing}
          title={t("removeMetadataQuestion", { field: label })}
          description={t("removeMetadataHint")}
          confirmLabel={t("confirmRemoveMetadata")}
          close={() => {
            setRemoving(undefined);
          }}
          write={() =>
            createWrite("PUT", `/files/${removing.fileID}/metadata`, {
              blob_key: removing.blobKey,
              key: removing.row.key,
              before_value: removing.row.value,
              before_source: removing.row.source,
              before_type: removing.row.type,
              value: "",
              remove: true,
            })
          }
        />
      )}
    </div>
  );
}
