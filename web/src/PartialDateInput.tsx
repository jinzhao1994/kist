import { Button, Popover, TextInput } from "@mantine/core";
import { DatePicker } from "@mantine/dates";
import { useRef, useState } from "react";
import dayjs from "dayjs";
import "dayjs/locale/zh-cn";
import { useI18n } from "./i18n";

type Level = "decade" | "year" | "month";
function levelOf(value: string): Level {
  return value.length === 4 ? "year" : "month";
}
// Normalization is only for calendar navigation, never for the stored value.
function calendarDate(value: string): string | null {
  if (!/^\d{4}(-\d{2}){0,2}$/.test(value) || value.startsWith("0000"))
    return null;
  const full =
    value.length === 4
      ? `${value}-01-01`
      : value.length === 7
        ? `${value}-01`
        : value;
  const parsed = dayjs(full);
  return parsed.isValid() && parsed.format("YYYY-MM-DD") === full ? full : null;
}
export function PartialDateInput({
  id,
  label,
  value,
  onChange,
  referenceDate,
  describedBy,
}: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  referenceDate?: string;
  describedBy?: string;
}) {
  const { t, prefs } = useI18n();
  const [opened, setOpened] = useState(false);
  const [level, setLevel] = useState<Level>(levelOf(value));
  const [date, setDate] = useState<string>(
    calendarDate(value) ||
      calendarDate(referenceDate || "") ||
      dayjs().format("YYYY-MM-DD"),
  );
  const input = useRef<HTMLInputElement>(null);
  const dropdown = useRef<HTMLDivElement>(null);
  const suppressFocus = useRef(false);
  const open = () => {
    if (suppressFocus.current) return;
    setLevel(levelOf(value));
    setDate(
      calendarDate(value) ||
        calendarDate(referenceDate || "") ||
        dayjs().format("YYYY-MM-DD"),
    );
    setOpened(true);
  };
  const choose = (selected: string | null) => {
    if (!selected) return;
    onChange(selected);
    setOpened(false);
    suppressFocus.current = true;
    input.current?.focus();
    suppressFocus.current = false;
  };
  const pickerProps = {
    value: value.length === 10 ? calendarDate(value) : null,
    date,
    onDateChange: setDate,
    onChange: choose,
    level,
    onLevelChange: setLevel,
    locale: prefs.language === "zh" ? "zh-cn" : "en",
    size: "sm" as const,
    ariaLabels: {
      nextMonth: t("nextMonth"),
      previousMonth: t("previousMonth"),
      nextYear: t("nextYear"),
      previousYear: t("previousYear"),
      nextDecade: t("nextDecade"),
      previousDecade: t("previousDecade"),
      monthLevelControl: t("chooseMonth"),
      yearLevelControl: t("chooseYear"),
    },
  };
  return (
    <div
      onBlur={(event) => {
        if (
          !event.currentTarget.contains(event.relatedTarget) &&
          !dropdown.current?.contains(event.relatedTarget)
        )
          setOpened(false);
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape" && opened) {
          event.stopPropagation();
          setOpened(false);
          suppressFocus.current = true;
          input.current?.focus();
          suppressFocus.current = false;
        }
      }}
    >
      <Popover
        opened={opened}
        onChange={setOpened}
        withinPortal
        zIndex={400}
        position="bottom-start"
        shadow="md"
        offset={6}
        returnFocus={false}
        closeOnEscape={false}
      >
        <Popover.Target>
          <TextInput
            ref={input}
            id={id}
            label={label}
            value={value}
            aria-describedby={describedBy}
            aria-expanded={opened}
            aria-haspopup="dialog"
            autoComplete="off"
            data-mantine-stop-propagation={opened ? "true" : undefined}
            onFocus={open}
            onClick={() => {
              if (!opened) open();
            }}
            onChange={(event) => {
              const next = event.target.value;
              onChange(next);
              const parsed = calendarDate(next);
              if (parsed) {
                setDate(parsed);
                setLevel(levelOf(next));
              }
            }}
            onKeyDown={(event) => {
              if (event.key === "ArrowDown") {
                event.preventDefault();
                open();
                requestAnimationFrame(() =>
                  dropdown.current
                    ?.querySelector<HTMLButtonElement>("button:not(:disabled)")
                    ?.focus(),
                );
              }
              if (event.key === "Enter" && opened) {
                event.preventDefault();
                setOpened(false);
              }
            }}
          />
        </Popover.Target>
        <Popover.Dropdown
          ref={dropdown}
          onFocusCapture={(event) => {
            // Modal handles Escape in window capture before React handlers run.
            event.target.setAttribute("data-mantine-stop-propagation", "true");
          }}
          className="partial-date-dropdown"
          role="dialog"
          aria-label={label}
          aria-labelledby={undefined}
        >
          <DatePicker {...pickerProps} />
          <div className="partial-date-actions">
            <Button
              variant="subtle"
              color="gray"
              size="xs"
              onClick={() => choose(date.slice(0, level === "month" ? 7 : 4))}
            >
              {level === "month"
                ? t("useDateMonth", {
                    year: date.slice(0, 4),
                    month: String(Number(date.slice(5, 7))),
                  })
                : t("useDateYear", { year: date.slice(0, 4) })}
            </Button>
            <Button
              variant="subtle"
              color="gray"
              size="xs"
              onClick={() => choose(dayjs().format("YYYY-MM-DD"))}
            >
              {t("today")}
            </Button>
          </div>
        </Popover.Dropdown>
      </Popover>
    </div>
  );
}
