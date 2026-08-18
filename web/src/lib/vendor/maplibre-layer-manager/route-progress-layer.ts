import type {
    FillExtrusionLayerSpecification,
    LineLayerSpecification,
    StyleSpecification,
} from "maplibre-gl";
import type { FeatureCollection } from "geojson";
import type { BaseLayer } from "./layers";
import { buildRouteRibbonCollection } from "$lib/util/route_geometry";

export class RouteProgressLayer implements BaseLayer {
    spec: StyleSpecification;

    constructor(
        id: string,
        route: FeatureCollection,
        color: string,
    ) {
        const lineSource = `${id}-source`;
        const ribbonSource = `${id}-ribbon-source`;
        const ribbon = buildRouteRibbonCollection(route);

        const ribbonLayer: FillExtrusionLayerSpecification = {
            id: `${id}-ribbon`,
            type: "fill-extrusion",
            source: ribbonSource,
            paint: {
                "fill-extrusion-color": ["coalesce", ["get", "color"], color],
                "fill-extrusion-opacity": 0.94,
                "fill-extrusion-height": 5.5,
                "fill-extrusion-base": 0,
                "fill-extrusion-vertical-gradient": true,
            },
        };

        const caseLayer: LineLayerSpecification = {
            id: `${id}-line-case`,
            type: "line",
            source: lineSource,
            layout: {
                "line-cap": "round",
                "line-join": "round",
            },
            paint: {
                "line-color": "#111827",
                "line-opacity": 0.72,
                "line-width": 12,
            },
        };

        const lineLayer: LineLayerSpecification = {
            id: `${id}-line`,
            type: "line",
            source: lineSource,
            layout: {
                "line-cap": "round",
                "line-join": "round",
            },
            paint: {
                "line-color": ["coalesce", ["get", "color"], color],
                "line-opacity": 1,
                "line-width": 6,
            },
        };

        this.spec = {
            version: 8,
            name: id,
            sources: {
                [lineSource]: {
                    type: "geojson",
                    data: route,
                },
                [ribbonSource]: {
                    type: "geojson",
                    data: ribbon,
                },
            },
            layers: [ribbonLayer, caseLayer, lineLayer],
        };
    }
}
