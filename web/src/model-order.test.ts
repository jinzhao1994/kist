import { expect, it } from "vitest";
import { compareModelIDs } from "./model-order";
import catalog from "./testdata/openai-model-ids-2026-11-01.json";

it("compares numeric runs instead of their spelling", () => {
  expect(
    ["gpt-6.10", "gpt-10", "gpt-6.2", "gpt-2"].toSorted(compareModelIDs),
  ).toEqual(["gpt-2", "gpt-6.2", "gpt-6.10", "gpt-10"]);
});

it("breaks case and leading-zero ties without relying on server order", () => {
  const ids = ["model-1", "model-01", "MODEL-1"];
  const expected = ["MODEL-1", "model-01", "model-1"];
  expect(ids.toSorted(compareModelIDs)).toEqual(expected);
  expect(ids.toReversed().toSorted(compareModelIDs)).toEqual(expected);
});

it("orders the captured 130-model catalog consistently without changing its contents", () => {
  expect(catalog.ids).toHaveLength(130);
  expect(new Set(catalog.ids).size).toBe(130);
  expect(catalog.ids.toSorted(compareModelIDs)).toEqual(catalog.expected_ids);
  expect(catalog.ids.toReversed().toSorted(compareModelIDs)).toEqual(
    catalog.expected_ids,
  );
});
