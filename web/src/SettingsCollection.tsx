import { Button, Table } from "@mantine/core";
import type { ReactNode } from "react";
import { EmptyHint } from "./EmptyHint";
import { HelpTooltip } from "./HelpTooltip";
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
        {creating === undefined && (
          <div className="setting-field-action">
            <Button
              disabled={disabled || editing !== undefined}
              onClick={onCreate}
            >
              {addLabel}
            </Button>
          </div>
        )}
      </div>
      {creating !== undefined && editor}
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
                {headers.map((header) => (
                  <Table.Th key={header}>{header}</Table.Th>
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
    </section>
  );
}
