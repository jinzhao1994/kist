import { Button, Table } from "@mantine/core";
import { useCallback, useRef, type ReactNode } from "react";
import { EmptyHint } from "./EmptyHint";
import { HelpTooltip } from "./HelpTooltip";
import { InlineReveal } from "./InlineReveal";
import { useI18n } from "./i18n";

// New objects are edited above the table; existing objects replace their row.
export function SettingsCollection<T extends { number: number }>({
  id,
  title,
  help,
  addLabel,
  emptyLabel,
  items,
  creating,
  editing,
  disabled,
  onCreate,
  editor,
  headers,
  minWidth,
  renderRow,
}: {
  id: string;
  title: string;
  help: string;
  addLabel: string;
  emptyLabel: string;
  items: T[];
  creating?: number;
  editing?: number;
  disabled: boolean;
  onCreate: () => void;
  editor: ReactNode;
  headers: string[];
  minWidth: number;
  renderRow: (item: T) => ReactNode;
}) {
  const { t } = useI18n();
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const entered = useCallback(() => {
    if (
      document.activeElement === document.body ||
      document.activeElement === trigger.current
    )
      panel.current
        ?.querySelector<HTMLElement>('input:not([disabled]), [role="combobox"]')
        ?.focus({ preventScroll: true });
  }, []);
  const exited = useCallback(() => {
    if (
      document.activeElement === document.body ||
      panel.current?.contains(document.activeElement)
    )
      trigger.current?.focus({ preventScroll: true });
  }, []);
  const rows = items.filter((item) => item.number !== creating);
  return (
    <section
      id={id}
      className="settings-fields settings-subsection"
      aria-labelledby={`${id}-title`}
    >
      <div className="settings-section-heading">
        <h3 id={`${id}-title`} className="settings-subsection-heading">
          {title}
        </h3>
        <HelpTooltip label={title}>{help}</HelpTooltip>
        <div
          className="setting-field-action settings-create-trigger"
          data-open={creating !== undefined || undefined}
          aria-hidden={creating !== undefined}
          inert={creating !== undefined}
        >
          <Button
            ref={trigger}
            aria-expanded={creating !== undefined}
            aria-controls={`${id}-create`}
            disabled={
              disabled || editing !== undefined || creating !== undefined
            }
            onClick={onCreate}
          >
            {addLabel}
          </Button>
        </div>
      </div>
      <div>
        <InlineReveal onEntered={entered} onExited={exited}>
          {creating !== undefined && (
            <div
              id={`${id}-create`}
              ref={panel}
              className="settings-reveal-content"
            >
              {editor}
            </div>
          )}
        </InlineReveal>
        {!rows.length && creating === undefined && (
          <EmptyHint>{emptyLabel}</EmptyHint>
        )}
        {!!rows.length && (
          <Table.ScrollContainer minWidth={minWidth}>
            <Table
              highlightOnHover
              className="settings-table settings-table--compact"
              aria-label={title}
            >
              <Table.Thead>
                <Table.Tr>
                  {headers.map((header, index) => (
                    <Table.Th
                      key={header}
                      className={
                        index === 0 ? "settings-number-column" : undefined
                      }
                    >
                      {header}
                    </Table.Th>
                  ))}
                  <Table.Th>
                    <span className="sr-only">
                      {t("settingCredentialActions")}
                    </span>
                  </Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {rows.map((item) => (
                  <Table.Tr key={item.number}>
                    {item.number === editing ? (
                      <Table.Td colSpan={headers.length + 1}>{editor}</Table.Td>
                    ) : (
                      renderRow(item)
                    )}
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
        )}
      </div>
    </section>
  );
}
