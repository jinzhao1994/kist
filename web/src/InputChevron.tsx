// Numeric steppers and selection inputs share the same masked vector asset.
export function InputChevron({ error = false }: { error?: boolean }) {
  return (
    <span
      className="input-chevron"
      data-error={error || undefined}
      aria-hidden="true"
    />
  );
}
