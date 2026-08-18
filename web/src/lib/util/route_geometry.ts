import { haversineDistance } from "$lib/models/gpx/utils";
import type {
    Feature,
    FeatureCollection,
    GeoJsonObject,
    GeometryObject,
    LineString,
    MultiLineString,
    Position,
} from "geojson";

export type RouteGeometryPoint = {
    position: Position;
    distance: number;
    progress: number;
    timestamp?: Date;
};

export type RouteGeometryState = {
    positions: Position[];
    timestamps: Array<Date | undefined>;
    cumulatedDistance: number[];
    totalDistance: number;
    segmentSpeeds: Array<number | undefined>;
};

export type RouteInterpolation = {
    position: Position;
    distance: number;
    progress: number;
    bearing: number;
    index: number;
};

function extractLineStrings(
    geoJson: GeoJsonObject,
): { lineStrings: Array<LineString | MultiLineString>; times: Date[] } {
    const lineStrings: Array<LineString | MultiLineString> = [];
    const times: Date[] = [];

    function extractFromGeometry(geometry: GeometryObject) {
        if (geometry.type === "LineString" || geometry.type === "MultiLineString") {
            lineStrings.push(geometry as LineString | MultiLineString);
        }
    }

    function extractFromFeature(feature: Feature) {
        if (feature.geometry) {
            extractFromGeometry(feature.geometry);
        }
        const coordinateTimes = feature.properties?.coordinateProperties?.times;
        if (Array.isArray(coordinateTimes)) {
            times.push(...coordinateTimes.map((time: string) => new Date(time)));
        }
    }

    function extractFromFeatureCollection(collection: FeatureCollection) {
        for (const feature of collection.features) {
            if (feature.type === "Feature") {
                extractFromFeature(feature);
            } else if (feature.type === "FeatureCollection") {
                extractFromFeatureCollection(feature as unknown as FeatureCollection);
            }
        }
    }

    if (geoJson.type === "Feature") {
        extractFromFeature(geoJson as Feature);
    } else if (geoJson.type === "FeatureCollection") {
        extractFromFeatureCollection(geoJson as FeatureCollection);
    } else {
        extractFromGeometry(geoJson as GeometryObject);
    }

    return { lineStrings, times };
}

export function routeGeometryFromGeoJson(geoJson: GeoJsonObject): RouteGeometryState {
    const { lineStrings, times } = extractLineStrings(geoJson);
    const positions = lineStrings.flatMap((line) =>
        line.type === "LineString" ? line.coordinates : line.coordinates.flat(),
    );

    if (positions.length === 0) {
        return {
            positions: [],
            timestamps: [],
            cumulatedDistance: [],
            totalDistance: 0,
            segmentSpeeds: [],
        };
    }

    const cumulatedDistance = [0];
    const segmentSpeeds: Array<number | undefined> = [];
    let totalDistance = 0;
    for (let i = 1; i < positions.length; i += 1) {
        const previous = positions[i - 1];
        const current = positions[i];
        const segmentDistance = haversineDistance(
            previous[1],
            previous[0],
            current[1],
            current[0],
        );
        totalDistance += segmentDistance;
        cumulatedDistance.push(totalDistance);

        const previousTime = times[i - 1];
        const currentTime = times[i];
        if (previousTime && currentTime) {
            const deltaSeconds =
                (currentTime.getTime() - previousTime.getTime()) / 1000;
            segmentSpeeds.push(deltaSeconds > 0 ? segmentDistance / deltaSeconds : undefined);
        } else {
            segmentSpeeds.push(undefined);
        }
    }

    const timestamps = positions.map((_, index) => times[index]);

    return {
        positions,
        timestamps,
        cumulatedDistance,
        totalDistance,
        segmentSpeeds,
    };
}

export function interpolatePosition(a: Position, b: Position, ratio: number): Position {
    const z1 = typeof a[2] === "number" ? a[2] : 0;
    const z2 = typeof b[2] === "number" ? b[2] : z1;
    return [
        a[0] + (b[0] - a[0]) * ratio,
        a[1] + (b[1] - a[1]) * ratio,
        z1 + (z2 - z1) * ratio,
    ];
}

export function bearingBetween(a: Position, b: Position): number {
    const lon1 = (a[0] * Math.PI) / 180;
    const lon2 = (b[0] * Math.PI) / 180;
    const lat1 = (a[1] * Math.PI) / 180;
    const lat2 = (b[1] * Math.PI) / 180;
    const y = Math.sin(lon2 - lon1) * Math.cos(lat2);
    const x =
        Math.cos(lat1) * Math.sin(lat2) -
        Math.sin(lat1) * Math.cos(lat2) * Math.cos(lon2 - lon1);
    return ((Math.atan2(y, x) * 180) / Math.PI + 360) % 360;
}

export function interpolateAtDistance(
    geometry: RouteGeometryState,
    targetDistance: number,
): RouteInterpolation | null {
    if (geometry.positions.length === 0) {
        return null;
    }

    if (geometry.positions.length === 1 || geometry.totalDistance <= 0) {
        return {
            position: geometry.positions[0],
            distance: 0,
            progress: 0,
            bearing: 0,
            index: 0,
        };
    }

    const distance = Math.max(0, Math.min(targetDistance, geometry.totalDistance));
    let index = geometry.cumulatedDistance.findIndex((item) => item >= distance);
    if (index <= 0) {
        index = 1;
    }
    if (index < 0) {
        index = geometry.cumulatedDistance.length - 1;
    }

    const previousDistance = geometry.cumulatedDistance[index - 1];
    const nextDistance = geometry.cumulatedDistance[index];
    const segmentDistance = nextDistance - previousDistance;
    const ratio = segmentDistance > 0 ? (distance - previousDistance) / segmentDistance : 0;
    const start = geometry.positions[index - 1];
    const end = geometry.positions[index];

    return {
        position: interpolatePosition(start, end, ratio),
        distance,
        progress: geometry.totalDistance > 0 ? distance / geometry.totalDistance : 0,
        bearing: bearingBetween(start, end),
        index,
    };
}

function closestPointOnSegment(
    lon: number,
    lat: number,
    start: Position,
    end: Position,
): { point: Position; t: number } {
    const dx = end[0] - start[0];
    const dy = end[1] - start[1];
    if (dx === 0 && dy === 0) {
        return { point: start, t: 0 };
    }
    const t = Math.max(
        0,
        Math.min(
            1,
            ((lon - start[0]) * dx + (lat - start[1]) * dy) / (dx * dx + dy * dy),
        ),
    );
    return {
        point: interpolatePosition(start, end, t),
        t,
    };
}

export function distanceAlongRouteForPoint(
    geometry: RouteGeometryState,
    lat: number,
    lon: number,
): number | null {
    if (geometry.positions.length === 0) {
        return null;
    }
    if (geometry.positions.length === 1) {
        return 0;
    }

    let bestDistance = Infinity;
    let bestRouteDistance = 0;

    for (let i = 1; i < geometry.positions.length; i += 1) {
        const start = geometry.positions[i - 1];
        const end = geometry.positions[i];
        const closest = closestPointOnSegment(lon, lat, start, end);
        const distanceToPoint = haversineDistance(
            lat,
            lon,
            closest.point[1],
            closest.point[0],
        );
        if (distanceToPoint >= bestDistance) {
            continue;
        }
        bestDistance = distanceToPoint;
        const segmentStart = geometry.cumulatedDistance[i - 1];
        const segmentEnd = geometry.cumulatedDistance[i];
        bestRouteDistance =
            segmentStart + closest.t * (segmentEnd - segmentStart);
    }

    return bestRouteDistance;
}

function createSpeedColorizer(geometry: RouteGeometryState) {
    const validSpeeds = geometry.segmentSpeeds.filter(
        (speed): speed is number => typeof speed === "number" && Number.isFinite(speed),
    );
    const minSpeed = validSpeeds.length ? Math.min(...validSpeeds) : 0;
    const maxSpeed = validSpeeds.length ? Math.max(...validSpeeds) : 0;

    return (speed?: number) => {
        if (
            speed === undefined ||
            !Number.isFinite(speed) ||
            minSpeed === maxSpeed
        ) {
            return "#9ca3af";
        }
        const normalized = Math.max(
            0,
            Math.min(1, (speed - minSpeed) / (maxSpeed - minSpeed)),
        );
        const step = Math.round(normalized * 11);
        const quantized = step / 11;
        const red = Math.round(255 * quantized);
        const green = Math.round(200 * (1 - quantized) + 55);
        const blue = 60;
        return `#${red.toString(16).padStart(2, "0")}${green.toString(16).padStart(2, "0")}${blue.toString(16).padStart(2, "0")}`;
    };
}

type ColoredLineBuffer = {
    color: string;
    coordinates: Position[];
};

function appendToColoredLineBuffer(
    buffer: ColoredLineBuffer | null,
    coordinates: Position[],
    color: string,
): ColoredLineBuffer {
    if (coordinates.length < 2) {
        return buffer ?? { color, coordinates: [] };
    }
    if (buffer && buffer.color === color && buffer.coordinates.length > 0) {
        buffer.coordinates.push(...coordinates.slice(1));
        return buffer;
    }
    return { color, coordinates: [...coordinates] };
}

function flushColoredLineBuffer(target: Feature[], buffer: ColoredLineBuffer | null) {
    if (!buffer || buffer.coordinates.length < 2) {
        return;
    }
    target.push({
        type: "Feature",
        properties: { color: buffer.color },
        geometry: {
            type: "LineString",
            coordinates: buffer.coordinates,
        },
    });
}

export function buildSpeedColoredRoute(
    geometry: RouteGeometryState,
): FeatureCollection {
    const speedToColor = createSpeedColorizer(geometry);
    const empty = (): FeatureCollection => ({
        type: "FeatureCollection",
        features: [],
    });

    if (geometry.positions.length < 2) {
        return empty();
    }

    const features: Feature[] = [];
    let buffer: ColoredLineBuffer | null = null;

    for (let i = 1; i < geometry.positions.length; i += 1) {
        const color = speedToColor(geometry.segmentSpeeds[i - 1]);
        buffer = appendToColoredLineBuffer(
            buffer,
            [geometry.positions[i - 1], geometry.positions[i]],
            color,
        );
        const nextColor = i < geometry.positions.length - 1
            ? speedToColor(geometry.segmentSpeeds[i])
            : null;
        if (nextColor !== color) {
            flushColoredLineBuffer(features, buffer);
            buffer = null;
        }
    }

    flushColoredLineBuffer(features, buffer);

    return {
        type: "FeatureCollection",
        features,
    };
}

export function splitLineAtDistance(
    geometry: RouteGeometryState,
    targetDistance: number,
): { completed: FeatureCollection; remaining: FeatureCollection } {
    const speedToColor = createSpeedColorizer(geometry);

    const empty = (): FeatureCollection => ({
        type: "FeatureCollection",
        features: [],
    });

    if (geometry.positions.length === 0) {
        return { completed: empty(), remaining: empty() };
    }

    if (geometry.positions.length === 1 || geometry.totalDistance <= 0) {
        const feature = {
            type: "Feature" as const,
            properties: {},
            geometry: {
                type: "LineString" as const,
                coordinates: geometry.positions,
            },
        };
        return { completed: empty(), remaining: { type: "FeatureCollection", features: [feature] } };
    }

    const interpolation = interpolateAtDistance(geometry, targetDistance);
    if (!interpolation) {
        return { completed: empty(), remaining: empty() };
    }

    const splitPoint = interpolation.position;
    const completedFeatures: Feature[] = [];
    const remainingFeatures: Feature[] = [];
    let completedBuffer: ColoredLineBuffer | null = null;
    let remainingBuffer: ColoredLineBuffer | null = null;

    for (let i = 1; i < geometry.positions.length; i += 1) {
        const start = geometry.positions[i - 1];
        const end = geometry.positions[i];
        const startDistance = geometry.cumulatedDistance[i - 1];
        const endDistance = geometry.cumulatedDistance[i];
        const color = speedToColor(geometry.segmentSpeeds[i - 1]);

        if (endDistance <= targetDistance) {
            completedBuffer = appendToColoredLineBuffer(completedBuffer, [start, end], color);
            continue;
        }
        if (startDistance >= targetDistance) {
            flushColoredLineBuffer(completedFeatures, completedBuffer);
            completedBuffer = null;
            remainingBuffer = appendToColoredLineBuffer(remainingBuffer, [start, end], color);
            continue;
        }

        completedBuffer = appendToColoredLineBuffer(completedBuffer, [start, splitPoint], color);
        flushColoredLineBuffer(completedFeatures, completedBuffer);
        completedBuffer = null;
        remainingBuffer = appendToColoredLineBuffer(remainingBuffer, [splitPoint, end], color);
    }

    flushColoredLineBuffer(completedFeatures, completedBuffer);
    flushColoredLineBuffer(remainingFeatures, remainingBuffer);

    return {
        completed: {
            type: "FeatureCollection",
            features: completedFeatures,
        },
        remaining: {
            type: "FeatureCollection",
            features: remainingFeatures,
        },
    };
}

export function pointAtProgress(
    geometry: RouteGeometryState,
    progress: number,
): RouteGeometryPoint | null {
    const interpolation = interpolateAtDistance(
        geometry,
        geometry.totalDistance * Math.max(0, Math.min(progress, 1)),
    );
    if (!interpolation) {
        return null;
    }
    return {
        position: interpolation.position,
        distance: interpolation.distance,
        progress: interpolation.progress,
        timestamp: geometry.timestamps[interpolation.index],
    };
}
