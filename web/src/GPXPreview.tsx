import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Button } from "@mantine/core";
import { get } from "./api";
import type { components } from "./api-schema";
import { useI18n } from "./i18n";
import { mapSlot, prepareGPX, type GPXData } from "./gpx-map";
import "maplibre-gl/dist/maplibre-gl.css";
import "./gpx-map.css";

type Attribution = { text: string; href?: string };
type Snapshot = { image: string; attribution: Attribution[]; warning: boolean };
const snapshots = new Map<string, Snapshot>();

function attributionParts(root: Element | null): Attribution[] {
  if (!root) return [];
  // MapLibre has already sanitized this DOM. Preserve its required links after
  // releasing the WebGL map, without passing provider HTML through React.
  return Array.from(root.childNodes).flatMap((node): Attribution[] => {
    if (node instanceof HTMLAnchorElement)
      return [{ text: node.textContent || "", href: node.href }];
    if (node instanceof Element) return attributionParts(node);
    return [{ text: node.textContent || "" }];
  });
}

export default function GPXPreview({
  src,
  name,
  detail,
}: {
  src: string;
  name: string;
  detail: boolean;
}) {
  const { t } = useI18n();
  const container = useRef<HTMLDivElement>(null);
  const [snapshot, setSnapshot] = useState<Snapshot>();
  const [failed, setFailed] = useState(false);
  const [warning, setWarning] = useState(false);
  const [ready, setReady] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const data = useQuery({
    queryKey: ["gpx", src],
    queryFn: async ({ signal }) => {
      const response = await fetch(src, { signal });
      if (!response.ok) throw new Error("GPX unavailable");
      return prepareGPX((await response.json()) as GPXData);
    },
  });
  const provider = useQuery({
    queryKey: ["map-provider"],
    queryFn: ({ signal }) =>
      get<components["schemas"]["MapProvider"]>("/map-provider", signal),
  });
  useEffect(() => {
    if (!data.data || !provider.data || !container.current) return;
    const prepared = data.data;
    const config = provider.data;
    const target = container.current;
    const key = JSON.stringify([src, config]);
    setFailed(false);
    setWarning(false);
    setReady(false);
    setSnapshot(undefined);
    if (!detail && snapshots.has(key)) {
      setSnapshot(snapshots.get(key));
      return;
    }
    const abort = new AbortController();
    let destroy = () => {};
    void (async () => {
      const release = await mapSlot(abort.signal, detail);
      try {
        const [lib, worker] = await Promise.all([
          import("maplibre-gl"),
          import("maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url"),
        ]);
        if (abort.signal.aborted) {
          release();
          return;
        }
        lib.setWorkerUrl(worker.default);
        lib.setWorkerCount(1);
        const map = new lib.Map({
          container: target,
          style: config.style_url,
          bounds: prepared.bounds,
          fitBoundsOptions: {
            padding: detail ? 60 : 35,
            maxZoom: 15,
            duration: 0,
          },
          interactive: detail,
          pixelRatio: detail ? window.devicePixelRatio : 1,
          locale: {
            "NavigationControl.ZoomIn": t("mapZoomIn"),
            "NavigationControl.ZoomOut": t("mapZoomOut"),
            "AttributionControl.ToggleAttribution": t("mapAttribution"),
            "Map.Title": name,
          },
          attributionControl: { compact: detail },
          canvasContextAttributes: { preserveDrawingBuffer: !detail },
          fadeDuration: 0,
          // Keep dateline-crossing routes and their basemap continuous.
          renderWorldCopies: true,
          maxZoom: 18,
        });
        let stopped = false;
        let hadError = false;
        let timer: ReturnType<typeof setTimeout>;
        const resize = new ResizeObserver(() => map.resize());
        resize.observe(target);
        destroy = () => {
          if (stopped) return;
          stopped = true;
          clearTimeout(timer);
          resize.disconnect();
          map.remove();
          release();
        };
        timer = setTimeout(() => {
          if (!abort.signal.aborted) setFailed(true);
          destroy();
        }, 25000);
        map.on("error", () => {
          if (stopped || abort.signal.aborted) return;
          if (!map.isStyleLoaded()) {
            setFailed(true);
            destroy();
            return;
          }
          hadError = true;
          setWarning(true);
        });
        map.on("load", () => {
          try {
            if (config.terrain_tiles) {
              map.addSource("kist-terrain", {
                type: "raster-dem",
                tiles: [config.terrain_tiles],
                tileSize: 256,
                maxzoom: 15,
                encoding: "terrarium",
                attribution: config.terrain_attribution,
              });
              const firstLine = map
                .getStyle()
                .layers.find(
                  (layer) => layer.type === "line" || layer.type === "symbol",
                );
              map.addLayer(
                {
                  id: "kist-hillshade",
                  type: "hillshade",
                  source: "kist-terrain",
                  paint: { "hillshade-exaggeration": 0.35 },
                },
                firstLine?.id,
              );
            }
            map.addSource("kist-route", {
              type: "geojson",
              data: prepared.geojson,
            });
            map.addLayer({
              id: "kist-route-outline",
              type: "line",
              source: "kist-route",
              paint: { "line-color": "#ffffff", "line-width": 7 },
              layout: { "line-join": "round", "line-cap": "round" },
            });
            map.addLayer({
              id: "kist-route-line",
              type: "line",
              source: "kist-route",
              paint: { "line-color": "#d23e38", "line-width": 4 },
              layout: { "line-join": "round", "line-cap": "round" },
            });
            const endpoints = prepared.geojson.features.flatMap((f) => {
              const points =
                f.geometry.type === "Point"
                  ? [f.geometry.coordinates]
                  : [f.geometry.coordinates[0], f.geometry.coordinates.at(-1)!];
              return points.map((p, i) => ({
                type: "Feature" as const,
                properties: { color: i ? "#d23e38" : "#17876c" },
                geometry: { type: "Point" as const, coordinates: p },
              }));
            });
            map.addSource("kist-endpoints", {
              type: "geojson",
              data: { type: "FeatureCollection", features: endpoints },
            });
            map.addLayer({
              id: "kist-endpoints",
              type: "circle",
              source: "kist-endpoints",
              paint: {
                "circle-radius": 5,
                "circle-color": ["get", "color"],
                "circle-stroke-color": "#fff",
                "circle-stroke-width": 2,
              },
            });
            map.fitBounds(prepared.bounds, {
              padding: detail ? 60 : 35,
              maxZoom: 15,
              duration: 0,
            });
            if (detail)
              map.addControl(
                new lib.NavigationControl({ showCompass: false }),
                "top-right",
              );
            map.once("idle", () => {
              if (stopped || abort.signal.aborted) return;
              clearTimeout(timer);
              setReady(true);
              if (!detail) {
                try {
                  const result = {
                    image: map.getCanvas().toDataURL("image/png"),
                    attribution: attributionParts(
                      target.querySelector(".maplibregl-ctrl-attrib-inner"),
                    ),
                    warning: hadError,
                  };
                  // Failed backgrounds are retryable, never cached as complete maps.
                  if (!hadError) {
                    snapshots.set(key, result);
                    if (snapshots.size > 32)
                      snapshots.delete(snapshots.keys().next().value!);
                  }
                  setSnapshot(result);
                  destroy();
                } catch {
                  setFailed(true);
                  destroy();
                }
              }
            });
          } catch {
            setFailed(true);
            destroy();
          }
        });
      } catch (error) {
        destroy();
        release();
        throw error;
      }
    })().catch(() => {
      if (!abort.signal.aborted) setFailed(true);
    });
    return () => {
      abort.abort();
      destroy();
    };
  }, [data.data, provider.data, src, detail, attempt, t, name]);

  const unavailable = failed || data.isError || provider.isError;
  return (
    <div
      className={`gpx-preview${detail ? " gpx-detail" : ""}`}
      aria-label={name}
      onPointerDown={detail ? (e) => e.stopPropagation() : undefined}
      onKeyDown={
        detail
          ? (e) => {
              if (e.key !== "Escape") e.stopPropagation();
            }
          : undefined
      }
    >
      <div
        ref={container}
        className={detail ? "gpx-map" : "gpx-map gpx-render"}
        style={snapshot || unavailable ? { visibility: "hidden" } : undefined}
      />
      {snapshot && (
        <>
          <img className="file-image" src={snapshot.image} alt={name} />
          <div className="gpx-attribution" onClick={(e) => e.stopPropagation()}>
            {snapshot.attribution.map((a, i) =>
              a.href ? (
                <a key={i} href={a.href} target="_blank" rel="noreferrer">
                  {a.text}
                </a>
              ) : (
                <span key={i}>{a.text}</span>
              ),
            )}
          </div>
        </>
      )}
      {unavailable && data.data && (
        <RouteFallback data={data.data.geojson} bounds={data.data.bounds} />
      )}
      {((!ready && !snapshot) ||
        unavailable ||
        warning ||
        snapshot?.warning) && (
        <div className="gpx-status" role={detail ? "status" : undefined}>
          {t(
            data.isError
              ? "mediaRequestFailed"
              : unavailable || warning || snapshot?.warning
                ? "mapUnavailable"
                : "mapLoading",
          )}
          {detail && (unavailable || warning) && (
            <Button
              size="xs"
              onClick={() => {
                void data.refetch();
                void provider.refetch();
                setAttempt((a) => a + 1);
              }}
            >
              {t("retry")}
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

function RouteFallback({
  data,
  bounds,
}: {
  data: GPXData;
  bounds: [[number, number], [number, number]];
}) {
  const mercator = (lat: number) =>
    (Math.log(Math.tan(Math.PI / 4 + (lat * Math.PI) / 360)) * 180) / Math.PI;
  const north = mercator(bounds[1][1]),
    south = mercator(bounds[0][1]);
  const scale = Math.min(
    440 / Math.max(0.001, bounds[1][0] - bounds[0][0]),
    290 / Math.max(0.001, north - south),
  );
  const project = (p: number[]) => [
    256 + (p[0] - (bounds[0][0] + bounds[1][0]) / 2) * scale,
    192 - (mercator(p[1]) - (north + south) / 2) * scale,
  ];
  return (
    <svg className="gpx-fallback" viewBox="0 0 512 384" aria-hidden="true">
      {data.features.map((f, i) =>
        f.geometry.type === "Point" ? (
          <circle
            key={i}
            cx={project(f.geometry.coordinates)[0]}
            cy={project(f.geometry.coordinates)[1]}
            r="5"
            fill="#17876c"
          />
        ) : (
          <polyline
            key={i}
            points={f.geometry.coordinates
              .map((p) => project(p).join(","))
              .join(" ")}
            fill="none"
            stroke="#d23e38"
            strokeWidth="3"
          />
        ),
      )}
    </svg>
  );
}
