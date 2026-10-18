import type { FeatureCollection, LineString, Point } from "geojson";

export type GPXData = FeatureCollection<LineString | Point>;

// Choose the smallest longitude interval, including routes across the dateline.
export function prepareGPX(data: GPXData) {
  const coordinates = data.features.flatMap((feature) =>
    feature.geometry.type === "Point"
      ? [feature.geometry.coordinates]
      : feature.geometry.coordinates,
  );
  if (!coordinates.length) throw new Error("Empty GPX");
  const sorted = coordinates
    .map((p) => (p[0] + 360) % 360)
    .sort((a, b) => a - b);
  let gap = -1;
  let start = sorted[0];
  for (let i = 0; i < sorted.length; i++) {
    const next =
      sorted[(i + 1) % sorted.length] + (i === sorted.length - 1 ? 360 : 0);
    if (next - sorted[i] > gap) {
      gap = next - sorted[i];
      start = next % 360;
    }
  }
  const shift = (p: number[]) => {
    let x = (p[0] + 360) % 360;
    if (x < start) x += 360;
    return [
      x - (start > 180 ? 360 : 0),
      Math.max(-85.051129, Math.min(85.051129, p[1])),
    ];
  };
  const points = coordinates.map(shift);
  const bounds: [[number, number], [number, number]] = [
    [Infinity, Infinity],
    [-Infinity, -Infinity],
  ];
  for (const p of points) {
    bounds[0][0] = Math.min(bounds[0][0], p[0]);
    bounds[0][1] = Math.min(bounds[0][1], p[1]);
    bounds[1][0] = Math.max(bounds[1][0], p[0]);
    bounds[1][1] = Math.max(bounds[1][1], p[1]);
  }
  const geojson: GPXData = {
    type: "FeatureCollection",
    features: data.features.map((f) => ({
      ...f,
      geometry:
        f.geometry.type === "Point"
          ? { type: "Point", coordinates: shift(f.geometry.coordinates) }
          : {
              type: "LineString",
              coordinates: f.geometry.coordinates.map(shift),
            },
    })),
  };
  return { geojson, bounds };
}

// Only two WebGL contexts at once; queued thumbnails disappear on unmount.
let running = 0;
const waiting: (() => void)[] = [];
export function mapSlot(
  signal: AbortSignal,
  priority = false,
): Promise<() => void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(new DOMException("Aborted", "AbortError"));
      return;
    }
    const cancel = () => {
      const i = waiting.indexOf(start);
      if (i >= 0) waiting.splice(i, 1);
      reject(new DOMException("Aborted", "AbortError"));
    };
    const start = () => {
      signal.removeEventListener("abort", cancel);
      if (signal.aborted) {
        cancel();
        return;
      }
      running++;
      let released = false;
      resolve(() => {
        if (released) return;
        released = true;
        running--;
        waiting.shift()?.();
      });
    };
    signal.addEventListener("abort", cancel, { once: true });
    if (running < 2) start();
    else if (priority) waiting.unshift(start);
    else waiting.push(start);
  });
}
