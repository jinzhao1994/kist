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
type Binding = { row: Row; path: string };
type Snapshot = Row & { binding?: Binding };
function writeValue(
  fileID: string,
  blobKey: string,
  snapshot: Snapshot,
  value: string,
  remove = false,
) {
  const before = snapshot.binding?.row || snapshot;
  return createWrite("PUT", `/files/${fileID}/metadata`, {
    blob_key: blobKey,
    key: before.key,
    before_value: before.value,
    before_source: before.source,
    before_type: before.type,
    path: snapshot.binding?.path,
    value,
    ...(remove ? { remove: true } : {}),
  });
}

export function MetadataValue({
  fileID,
  blobKey,
  row,
  label,
  children,
  editor,
  binding,
  editable = true,
}: {
  fileID: string;
  blobKey: string;
  row: Row;
  label: string;
  children: ReactNode;
  editor?: (row: Row, close: () => void) => ReactNode;
  binding?: Binding;
  editable?: boolean;
}) {
  const { t } = useI18n();
  // Retain the displayed before-state even if background extraction refreshes.
  const [editing, setEditing] = useState<Snapshot>();
  const trigger = useRef<HTMLButtonElement>(null);
  const removeTrigger = useRef<HTMLButtonElement>(null);
  const [removing, setRemoving] = useState<{
    row: Snapshot;
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
    const content = element.querySelector<HTMLElement>(
      ".metadata-value-content",
    )!;
    const actions = element.querySelector<HTMLElement>(
      ".metadata-value-transient-actions",
    )!;
    const copy = element.querySelector<HTMLElement>(
      ".copy-value span, .copy-text-action",
    );
    if (!copy) {
      setCopySpace(0);
      return;
    }
    // Copy follows the text rather than sitting at the inline end. Compare
    // its actual position with the toolbar's unshifted position, not its width.
    const measure = () => {
      const box = content.getBoundingClientRect();
      const target = copy.getBoundingClientRect();
      const toolbar = actions.getBoundingClientRect();
      const rtl = getComputedStyle(element).direction === "rtl";
      const left = rtl ? box.left : box.right - toolbar.width;
      const right = left + toolbar.width;
      const gap = 8;
      const overlaps =
        target.top < toolbar.bottom &&
        target.bottom > toolbar.top &&
        target.left - gap < right &&
        target.right + gap > left;
      setCopySpace(
        overlaps
          ? rtl
            ? target.right + gap - box.left
            : box.right - target.left + gap
          : 0,
      );
    };
    const observer = new ResizeObserver(measure);
    observer.observe(content);
    observer.observe(copy);
    // Clipping and the copied label can move the inline action without
    // changing either element's size, so size observation alone is insufficient.
    const mutations = new MutationObserver(measure);
    mutations.observe(content, {
      childList: true,
      characterData: true,
      subtree: true,
    });
    measure();
    return () => {
      observer.disconnect();
      mutations.disconnect();
    };
  }, [editing, row.value, row.source, t]);
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
              write={(value) => writeValue(fileID, blobKey, editing, value)}
            />
          )
        ) : (
          children
        )}
      </div>
      <div className="metadata-value-actions">
        <div className="metadata-value-transient-actions">
          {editable && (
            <ActionIcon
              ref={trigger}
              variant="subtle"
              size="xs"
              className="metadata-edit-action"
              aria-label={editLabel}
              disabled={!!editing || !!removing}
              onClick={() => setEditing({ ...row, binding })}
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
          )}
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
                row: { ...row, binding },
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
            writeValue(
              removing.fileID,
              removing.blobKey,
              removing.row,
              "",
              true,
            )
          }
        />
      )}
    </div>
  );
}
