import type { MapMouseEvent, StyleSpecification } from "maplibre-gl";
import type { BaseLayer } from "./layers";
import * as M from "maplibre-gl";

type LayerListeners = Record<string, {
    onMouseUp?: (e: MapMouseEvent) => void;
    onMouseDown?: (e: MapMouseEvent) => void;
    onEnter?: (e: MapMouseEvent) => void;
    onLeave?: (e: MapMouseEvent) => void;
    onMouseMove?: (e: MapMouseEvent) => void;
}>;

function lineEndpoint(
    feature: GeoJSON.Feature,
    which: "start" | "end",
): GeoJSON.Position | null {
    const geometry = feature.geometry as GeoJSON.LineString | GeoJSON.MultiLineString | undefined;
    if (!geometry?.coordinates?.length) {
        return null;
    }
    if (geometry.type === "MultiLineString") {
        const line = which === "start"
            ? geometry.coordinates[0]
            : geometry.coordinates[geometry.coordinates.length - 1];
        if (!line?.length) {
            return null;
        }
        return which === "start" ? line[0] : line[line.length - 1];
    }
    const coords = geometry.coordinates;
    return which === "start" ? coords[0] : coords[coords.length - 1];
}

function pointsFromLines(
    geojson: GeoJSON.FeatureCollection,
    which: "start" | "end",
): GeoJSON.FeatureCollection {
    return {
        type: "FeatureCollection",
        features: geojson.features.flatMap((f, i) => {
            const coordinates = lineEndpoint(f, which);
            if (!coordinates) {
                return [];
            }
            return [{
                type: "Feature" as const,
                properties: {
                    ...f.properties,
                    id: `${which}-${i}`,
                },
                geometry: {
                    type: "Point" as const,
                    coordinates,
                },
            }];
        }),
    };
}

export class PreviewLayer implements BaseLayer {

    private map: M.Map;

    spec: StyleSpecification;
    listeners: LayerListeners = {
        "preview": {
            onEnter: () => this.map!.getCanvas().style.cursor = "pointer",
            onLeave: () => this.map!.getCanvas().style.cursor = ""
        },
        "preview-hit": {
            onEnter: () => this.map!.getCanvas().style.cursor = "pointer",
            onLeave: () => this.map!.getCanvas().style.cursor = ""
        },
        "preview-start-points": {
            onEnter: () => this.map!.getCanvas().style.cursor = "pointer",
            onLeave: () => this.map!.getCanvas().style.cursor = ""
        },
        "preview-end-points": {
            onEnter: () => this.map!.getCanvas().style.cursor = "pointer",
            onLeave: () => this.map!.getCanvas().style.cursor = ""
        }
    };

    constructor(map: M.Map, geojson: GeoJSON.FeatureCollection, options?: {
        showStartMarker?: boolean;
        showEndMarker?: boolean;
        listeners?: LayerListeners;
    }) {

        this.map = map;
        const listeners = options?.listeners;
        this.listeners = {
            "preview": { ...this.listeners["preview"], ...listeners?.["preview"] },
            "preview-hit": { ...this.listeners["preview-hit"], ...listeners?.["preview"], ...listeners?.["preview-hit"] },
            "preview-start-points": { ...this.listeners["preview-start-points"], ...listeners?.["preview"], ...listeners?.["preview-start-points"] },
            "preview-end-points": { ...this.listeners["preview-end-points"], ...listeners?.["preview"], ...listeners?.["preview-end-points"] }
        }

        const startPoints = pointsFromLines(geojson, "start");
        const endPoints = pointsFromLines(geojson, "end");

        this.spec = {
            version: 8,
            name: "preview",
            glyphs: "https://tiles.openfreemap.org/fonts/{fontstack}/{range}.pbf",
            sources: {
                "preview": {
                    type: "geojson",
                    data: geojson,
                },
                "preview-start-points": {
                    type: "geojson",
                    data: startPoints,
                },
                "preview-end-points": {
                    type: "geojson",
                    data: endPoints,
                }
            },
            layers: [
                {
                    id: "preview-hit",
                    type: "line",
                    source: "preview",
                    paint: {
                        "line-color": "#000",
                        "line-width": 22,
                        "line-opacity": 0,
                    },
                },
                {
                    id: "preview",
                    type: "line",
                    source: "preview",
                    paint: {
                        "line-color": ["get", "color"],
                        "line-width": 5,
                    },
                },
                {
                    id: "preview-start-points",
                    type: "circle",
                    source: "preview-start-points",
                    filter: ["literal", options?.showStartMarker ?? false],
                    paint: {
                        "circle-color": "#242734",
                        "circle-radius": 6,
                        "circle-stroke-width": 2,
                        "circle-stroke-color": "#fff",
                    },
                },
                {
                    id: "preview-end-points",
                    type: "circle",
                    source: "preview-end-points",
                    filter: ["literal", options?.showEndMarker ?? false],
                    paint: {
                        "circle-color": "#fff",
                        "circle-radius": 6,
                        "circle-stroke-width": 3,
                        "circle-stroke-color": "#242734",
                    },
                },
                {
                    id: "preview-direction-carets",
                    type: "symbol",
                    source: "preview",
                    layout: {
                        "symbol-placement": "line",
                        "symbol-spacing": [
                            "interpolate",
                            ["exponential", 1.5],
                            ["zoom"],
                            0,
                            80,
                            18,
                            200,
                        ],
                        "icon-image": "direction-caret",
                        "icon-size": [
                            "interpolate",
                            ["exponential", 1.5],
                            ["zoom"],
                            0,
                            0.5,
                            18,
                            0.8,
                        ],
                    },
                }
            ]

        };
    }
}
