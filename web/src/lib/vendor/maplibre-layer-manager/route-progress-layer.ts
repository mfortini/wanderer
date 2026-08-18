import type { StyleSpecification } from "maplibre-gl";
import type { FeatureCollection } from "geojson";
import type { BaseLayer } from "./layers";

export class RouteProgressLayer implements BaseLayer {
    spec: StyleSpecification;

    constructor(
        id: string,
        route: FeatureCollection,
        color: string,
    ) {
        const source = `${id}-source`;

        this.spec = {
            version: 8,
            name: id,
            sources: {
                [source]: {
                    type: "geojson",
                    data: route,
                },
            },
            layers: [
                {
                    id: `${id}-line`,
                    type: "line",
                    source,
                    layout: {
                        "line-cap": "round",
                        "line-join": "round",
                    },
                    paint: {
                        "line-color": ["coalesce", ["get", "color"], color],
                        "line-opacity": 1,
                        "line-width": 6,
                    },
                },
            ],
        };
    }
}
