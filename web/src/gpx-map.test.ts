import { describe, expect, it } from "vitest";
import { mapSlot, prepareGPX, type GPXData } from "./gpx-map";

const route = (coordinates: number[][]): GPXData => ({
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      properties: {},
      geometry: { type: "LineString", coordinates },
    },
  ],
});

describe("GPX geographic framing", () => {
  it("fits dateline crossings without spanning the world", () => {
    const result = prepareGPX(
      route([
        [179, 10],
        [-179, 11],
      ]),
    );
    expect(result.bounds[1][0] - result.bounds[0][0]).toBe(2);
    expect(result.geojson.features[0].geometry.coordinates).toEqual([
      [179, 10],
      [181, 11],
    ]);
  });
  it("preserves ordinary western longitudes and clamps polar projection", () => {
    expect(
      prepareGPX(
        route([
          [-122, 90],
          [-121, -90],
        ]),
      ).bounds,
    ).toEqual([
      [-122, -85.051129],
      [-121, 85.051129],
    ]);
  });
  it("keeps segment boundaries and single points", () => {
    const data = route([
      [10, 20],
      [11, 21],
    ]);
    data.features.push({
      type: "Feature",
      properties: {},
      geometry: { type: "Point", coordinates: [12, 22] },
    });
    expect(prepareGPX(data).geojson.features).toHaveLength(2);
    expect(prepareGPX(route([[1, 2]])).bounds).toEqual([
      [1, 2],
      [1, 2],
    ]);
  });
});

it("bounds WebGL contexts and removes cancelled queued work", async () => {
  const first = await mapSlot(new AbortController().signal);
  const second = await mapSlot(new AbortController().signal);
  const cancelled = new AbortController();
  const pending = mapSlot(cancelled.signal);
  cancelled.abort();
  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  let started = false;
  const next = mapSlot(new AbortController().signal).then((release) => {
    started = true;
    return release;
  });
  await Promise.resolve();
  expect(started).toBe(false);
  first();
  first();
  const third = await next;
  expect(started).toBe(true);
  second();
  third();
});
