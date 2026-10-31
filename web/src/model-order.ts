const modelIDs = new Intl.Collator("en", {
  numeric: true,
  sensitivity: "base",
});

// IDs are identifiers, so their order is independent of the UI language and
// display names. Break numeric/case ties by the exact ID, not server order.
export function compareModelIDs(a: string, b: string): number {
  return modelIDs.compare(a, b) || (a < b ? -1 : a > b ? 1 : 0);
}
