import { ColorSwatch, DEFAULT_THEME, Radio } from "@mantine/core";
import { useI18n } from "./i18n";
import { colors } from "./theme";

export function ColorChoiceGroup({
  id,
  label,
  value,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  const { t } = useI18n();
  return (
    <Radio.Group id={id} aria-label={label} value={value} onChange={onChange}>
      <div className="color-choices">
        {colors.map((color) => (
          <Radio.Card
            key={color}
            value={color}
            disabled={disabled}
            tabIndex={value === color ? 0 : -1}
            className="color-choice"
          >
            <ColorSwatch
              className="color-choice-swatch"
              color={DEFAULT_THEME.colors[color][6]}
              withShadow={false}
              radius="xs"
              aria-hidden="true"
            />
            {t(color)}
          </Radio.Card>
        ))}
      </div>
    </Radio.Group>
  );
}
