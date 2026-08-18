<script lang="ts">
    import { page } from "$app/state";
    import directionCaret from "$lib/assets/svgs/caret-right-solid.svg";
    import RoutePlaybackControl from "$lib/components/trail/route_playback_control.svelte";
    import GPX from "$lib/models/gpx/gpx";
    import type { Trail } from "$lib/models/trail";
    import type { Waypoint } from "$lib/models/waypoint";
    import { theme } from "$lib/stores/theme_store";
    import { formatDistance, formatTimeHHMM } from "$lib/util/format_util";
    import { findStartAndEndPoints } from "$lib/util/geojson_util";
    import {
        createMarkerFromWaypoint,
        createPopupFromTrail,
        FontawesomeMarker,
    } from "$lib/util/maplibre_util";
    import type { RoutePlaybackState } from "$lib/util/route_playback";
    import { RoutePlayback } from "$lib/util/route_playback";
    import {
        routeGeometryFromGeoJson,
        buildSpeedColoredRoute,
        distanceAlongRouteForPoint,
        type RouteGeometryState,
    } from "$lib/util/route_geometry";
    import {
        WAYPOINT_FOCUS_EVENT,
        getWaypointPopupMedia,
        type WaypointPopupMedia,
        type WaypointFocusDetail,
    } from "$lib/util/waypoint_map_util";
    import { decodePolyline } from "$lib/util/polyline_util";
    import type { ElevationProfileControl } from "$lib/vendor/maplibre-elevation-profile/elevationprofile-control";
    import { FullscreenControl } from "$lib/vendor/maplibre-fullscreen/fullscreen-control";
    import MaplibreGraticule from "$lib/vendor/maplibre-graticule/maplibre-graticule";
    import { CaretLayer } from "$lib/vendor/maplibre-layer-manager/caret-layer";
    import { ClusterLayer } from "$lib/vendor/maplibre-layer-manager/cluster-layer";
    import { baseMapStyles } from "$lib/vendor/maplibre-layer-manager/layers";
    import { LayerManager } from "$lib/vendor/maplibre-layer-manager/maplibre-layer-manager";
    import type { OverpassPopupActionFactory } from "$lib/vendor/maplibre-layer-manager/overpass-layer";
    import { PreviewLayer } from "$lib/vendor/maplibre-layer-manager/preview-layer";
    import { RouteProgressLayer } from "$lib/vendor/maplibre-layer-manager/route-progress-layer";
    import { TerrainLayer } from "$lib/vendor/maplibre-layer-manager/terrain-layer";
    import { TrailLayer } from "$lib/vendor/maplibre-layer-manager/trail-layer";
    import { StyleSwitcherControl } from "$lib/vendor/maplibre-style-switcher/style-switcher-control";
    import type { Feature, FeatureCollection, GeoJSON } from "geojson";
    import * as M from "maplibre-gl";
    import "maplibre-gl/dist/maplibre-gl.css";
    import { onDestroy, onMount, tick, untrack } from "svelte";

    interface Props {
        trails?: Trail[];
        serverClusters?: GeoJSON.FeatureCollection;
        gpx?: GPX;
        waypoints?: Waypoint[];
        markers?: M.Marker[];
        map?: M.Map | null;
        drawing?: boolean;
        displayWaypoints?: boolean;
        showElevation?: boolean;
        showInfoPopup?: boolean;
        showGrid?: boolean;
        showStyleSwitcher?: boolean;
        showFullscreen?: boolean;
        showTerrain?: boolean;
        fitBounds?: "animate" | "instant" | "off";
        onmarkerdragend?:
            | ((marker: M.Marker, wpId?: string) => void)
            | undefined;
        elevationProfileContainer?: string | HTMLDivElement | undefined;
        mapOptions?: Partial<M.MapOptions> | undefined;
        activeTrail?: number | null;
        clusterTrails?: boolean;
        onsegmentdragend?: (data: {
            segment: number;
            event: M.MapMouseEvent;
        }) => void;
        onsegmentclick?: (data: {
            segment: number;
            event: M.MapMouseEvent;
        }) => void;
        onselect?: (trail: Trail) => void;
        onunselect?: (trail: Trail) => void;
        onfullscreen?: () => void;
        onmoveend?: (map: M.Map) => void;
        onzoom?: (map: M.Map) => void;
        onclick?: (event: M.MapMouseEvent & Object) => void;
        oncontextmenu?: (event: M.MapMouseEvent & Object) => void;
        onUnclusteredClick?: (
            event: M.MapMouseEvent & Object,
            trail: Trail,
        ) => void;
        oninit?: (map: M.Map) => void;
        autoGeolocateOnDrawing?: boolean;
        enableRoutePlayback?: boolean;
        buildPoiAnchorAction?: OverpassPopupActionFactory;
    }

    let {
        trails = [],
        serverClusters = undefined,
        waypoints = [],
        markers = $bindable([]),
        map = $bindable(),
        drawing = false,
        displayWaypoints = true,
        showElevation = true,
        showInfoPopup = false,
        showGrid = false,
        showStyleSwitcher = true,
        showFullscreen = false,
        showTerrain = false,
        fitBounds = "instant",
        elevationProfileContainer = undefined,
        mapOptions = undefined,
        activeTrail = $bindable(0),
        clusterTrails = false,
        onmarkerdragend,
        onsegmentdragend,
        onsegmentclick,
        onselect,
        onunselect,
        onfullscreen,
        onmoveend,
        onzoom,
        onclick,
        oncontextmenu,
        onUnclusteredClick,
        oninit,
        autoGeolocateOnDrawing = false,
        enableRoutePlayback = false,
        buildPoiAnchorAction = undefined,
    }: Props = $props();

    let mapContainer: HTMLDivElement;
    let epc: ElevationProfileControl;
    let graticule: MaplibreGraticule;

    let layerManager: LayerManager;

    let elevationMarker: FontawesomeMarker;
    let playbackMarker: FontawesomeMarker | null = null;
    let playback: RoutePlayback | null = null;
    let playbackState: RoutePlaybackState | null = $state(null);
    let followPlaybackCamera: boolean = $state(true);
    let enablePlayback3d: boolean = $state(true);
    let lastPlaybackTerrainExaggeration: number | null = null;
    let lastPlaybackCameraUpdate = 0;
    let playbackLayersInitialized = false;
    const playbackFollowZoomMax = 14.75;
    const playbackFollowZoomBoost = 0.4;
    let playbackMediaDisplayMs = $state(1500);
    const playbackMediaDurationOptions = [1000, 1500, 2000, 3000];
    const playbackMediaCrossfadeMs = 400;
    let playbackCameraBearing: number | null = $state(null);
    let playbackTrailId: string | null = $state(null);
    let playbackRouteGeometry: RouteGeometryState | null = null;
    let playbackReady = $state(false);
    let playbackOverlayLayers: Array<{
        id: string;
        waypointName: string;
        media: WaypointPopupMedia;
        displayUrl: string;
        visible: boolean;
    }> = $state([]);
    type PlaybackWaypointMediaScheduleItem = {
        waypointId: string;
        waypointName: string;
        media: WaypointPopupMedia;
        distance: number;
    };
    let playbackWaypointMediaSchedule: PlaybackWaypointMediaScheduleItem[] = [];
    let activePlaybackMediaUrl: string | null = null;
    let playbackMediaBlobCache = new Map<string, Promise<string>>();
    let playbackMediaReadyUrls = new Map<string, string>();
    let playbackMediaHideTimeout: ReturnType<typeof setTimeout> | null = null;

    let draggingSegment: number | null = null;

    let hoveringTrail: boolean = false;

    let mapLoaded: boolean = $state(false);
    let terrainEnabled: boolean | null = null;
    let elevationProfileVisibilityPreference: boolean | null = null;

    const playbackLayerId = "route-playback-progress";

    const trailColors = [
        "#3549bb", // blue
        "#ff7f0e", // orange
        "#2ca02c", // green
        "#d62728", // red
        "#9467bd", // purple
        "#8c564b", // brown
        "#e377c2", // pink
        "#373642", // gray
        "#fae455", // yellow
        "#17becf", // teal
    ];

    let clusterPopup: M.Popup | null = null;

    let mapData = $derived(getData(trails, serverClusters));
    let gpxDataMap = $derived(mapData[0]);
    let clusterData = $derived(mapData[1]);
    let previewData = $derived(mapData[2]);

    $effect(() => {
        // Track dependencies for Svelte 5
        mapData;

        if (map && mapLoaded) {
            untrack(() => initMap(map?.loaded() ?? false));
        }
    });
    $effect(() => {
        adjustTrailFocus(activeTrail);
    });
    $effect(() => {
        toggleEpcTheme();
    });
    $effect(() => {
        if (drawing && map && layerManager) {
            untrack(() => startDrawing());
        } else if (map && layerManager) {
            untrack(() => stopDrawing());
        }
    });
    $effect(() => {
        if (showGrid) {
            if (!graticule) {
                graticule = new MaplibreGraticule({
                    minZoom: 0,
                    maxZoom: 20,
                    showLabels: true,
                    labelType: "hdms",
                    labelSize: 10,
                    labelColor: "#858585",
                    longitudePosition: "top",
                    latitudePosition: "right",
                    paint: {
                        "line-opacity": 0.8,
                        "line-color": "rgba(0,0,0,0.2)",
                    },
                });
            }
            map?.addControl(graticule);
        } else {
            if (graticule) {
                map?.removeControl(graticule);
            }
        }
    });
    $effect(() => {
        waypoints;
        displayWaypoints;
        untrack(() => {
            syncWaypointMarkers();
            refreshElevationProfile();
            preloadAllWaypointMedia();
        });
    });
    $effect(() => {
        if (playbackRouteGeometry) {
            untrack(() => {
                buildPlaybackWaypointMediaSchedule(playbackRouteGeometry!);
                preloadPlaybackWaypointMedia();
            });
        }
    });
    $effect(() => {
        mapData;
        activeTrail;
        enableRoutePlayback;
        drawing;
        clusterTrails;
        if (!mapLoaded || !map) {
            return;
        }
        untrack(() => {
            syncRoutePlayback();
        });
    });

    function getData(
        trails: Trail[],
        serverClusters?: GeoJSON.FeatureCollection
    ): [
        Record<string, FeatureCollection>,
        FeatureCollection,
        FeatureCollection,
    ] {
        let clusterData: FeatureCollection = serverClusters ?? {
            type: "FeatureCollection",
            features: [],
        };
        let previewData: FeatureCollection = {
            type: "FeatureCollection",
            features: [],
        };
        let gpxDataMap: Record<string, FeatureCollection> = {};

        trails.forEach((t) => {
            if (t.id) {
                let fc: FeatureCollection | null = null;
                if (t.expand?.gpx) {
                    fc = t.expand.gpx.toGeoJSON();
                } else if (t.expand?.gpx_data) {
                    fc = GPX.parse(t.expand.gpx_data).toGeoJSON();
                }

                if (fc) {
                    fc.features.forEach((f) => {
                        if (f.properties) {
                            f.properties.bounding_box_diagonal =
                                t.bounding_box_diagonal;
                        }
                    });
                    gpxDataMap[t.id] = fc;
                }
            }

            if (clusterTrails) {
                if (!serverClusters && t.lat !== undefined && t.lon !== undefined) {
                    clusterData.features.push({
                        id: t.id,
                        type: "Feature",
                        properties: {
                            trail: t.id,
                            bounding_box_diagonal: t.bounding_box_diagonal,
                        },
                        geometry: {
                            type: "Point",
                            coordinates: [t.lon ?? 0, t.lat ?? 0],
                        },
                    } as Feature);
                }

                if (t.polyline) {
                    previewData.features.push({
                        id: t.id,
                        type: "Feature",
                        properties: {
                            trail: t.id,
                            bounding_box_diagonal: t.bounding_box_diagonal,
                            color: trailColors[
                                hashStringToIndex(
                                    t.id ?? "",
                                    trailColors.length,
                                )
                            ],
                        },
                        geometry: {
                            type: "LineString",
                            coordinates: decodePolyline(t.polyline, 5),
                        },
                    });
                }
            }
        });

        return [gpxDataMap, clusterData, previewData];
    }

    function initMap(mapLoaded: boolean) {
        if (!map || !layerManager) {
            return;
        }

        refreshElevationProfile();
        syncElevationProfileVisibility();

        trails.forEach((t) => {
            const layerId = t.id!;
            addTrailLayer(t, layerId, 0, gpxDataMap[layerId]);
        });

        Object.entries(layerManager.layers).forEach(([id, layer]) => {
            if (!(layer instanceof TrailLayer)) {
                return;
            }
            const isStillVisible = trails.some((t) => t.id === id);
            if (!isStillVisible) {
                removeCaretLayer();
                removeTrailLayer(id);
            }
        });

        if (clusterTrails) {
            addPreviewLayer(previewData);
            addClusterLayer(clusterData);
        }

        if (!drawing && fitBounds !== "off") {
            const currentBboxes = Object.values(gpxDataMap)
                .map((d) => d.bbox)
                .filter((b) => b !== undefined);

            if (
                activeTrail !== null &&
                trails[activeTrail] &&
                mapLoaded &&
                gpxDataMap[trails[activeTrail].id!]
            ) {
                focusTrail(trails[activeTrail]);
            } else if (currentBboxes.length > 0) {
                flyToBounds();
            }
        } else if (drawing && activeTrail !== null && mapLoaded) {
            const activeId = trails[activeTrail]?.id;
            if (activeId && gpxDataMap[activeId]) {
                addCaretLayer(gpxDataMap[activeId]);
            }
        }
    }

    function syncHillshadingVisibility() {
        if (!map?.getLayer("hillshading")) {
            terrainEnabled = null;
            return;
        }

        const isTerrainEnabled = Boolean(map.getTerrain());
        if (isTerrainEnabled === terrainEnabled) {
            return;
        }

        map.setLayoutProperty(
            "hillshading",
            "visibility",
            isTerrainEnabled ? "visible" : "none",
        );
        terrainEnabled = isTerrainEnabled;
    }

    export function refreshElevationProfile() {
        const activeId = activeTrail !== null ? trails[activeTrail]?.id : null;
        if (activeId && gpxDataMap[activeId]) {
            epc?.setData(gpxDataMap[activeId]!, waypoints);
        }
    }

    function getActiveTrailColor() {
        const index = activeTrail ?? 0;
        const trailId = trails[index]?.id ?? "";
        return trailColors[
            clusterTrails
                ? hashStringToIndex(trailId, trailColors.length)
                : index % trailColors.length
        ];
    }

    function createPlaybackMarker() {
        if (playbackMarker || !map) {
            return;
        }
        playbackMarker = new FontawesomeMarker(
            {
                id: "playback-marker",
                icon: "fa-solid fa-person-hiking",
                width: 8,
                backgroundColor: "bg-primary",
                fontColor: "white",
            },
            {},
        );
        playbackMarker.setLngLat([0, 0]).addTo(map);
        playbackMarker.setOpacity("0");
    }

    function removePlaybackMarker() {
        playbackMarker?.remove();
        playbackMarker = null;
    }

    function isPlayback3dAllowed() {
        return Boolean(map);
    }

    function clearPlaybackWaypointMedia() {
        if (playbackMediaHideTimeout !== null) {
            clearTimeout(playbackMediaHideTimeout);
            playbackMediaHideTimeout = null;
        }
        playbackOverlayLayers = [];
        activePlaybackMediaUrl = null;
        playbackWaypointMediaSchedule = [];
        playbackMediaReadyUrls.clear();
    }

    function revokePlaybackMediaBlobCache() {
        for (const promise of playbackMediaBlobCache.values()) {
            void promise.then((blobUrl) => {
                if (blobUrl.startsWith("blob:")) {
                    URL.revokeObjectURL(blobUrl);
                }
            });
        }
        playbackMediaBlobCache.clear();
        playbackMediaReadyUrls.clear();
    }

    function decodeMediaUrl(url: string, video: boolean): Promise<void> {
        if (video) {
            return new Promise((resolve) => {
                const videoEl = document.createElement("video");
                videoEl.preload = "auto";
                videoEl.muted = true;
                videoEl.playsInline = true;
                const finish = () => resolve();
                videoEl.addEventListener("canplaythrough", finish, { once: true });
                videoEl.addEventListener("loadeddata", finish, { once: true });
                videoEl.addEventListener("error", finish, { once: true });
                videoEl.src = url;
                videoEl.load();
                window.setTimeout(finish, 8000);
            });
        }

        return new Promise((resolve) => {
            const image = new Image();
            image.decoding = "async";
            const finish = () => resolve();
            image.onload = () => {
                void image.decode().then(finish).catch(finish);
            };
            image.onerror = finish;
            image.src = url;
            if (image.complete && image.naturalWidth > 0) {
                void image.decode().then(finish).catch(finish);
            }
            window.setTimeout(finish, 8000);
        });
    }

    function prefetchPlaybackMedia(media: WaypointPopupMedia): Promise<string> {
        const ready = playbackMediaReadyUrls.get(media.url);
        if (ready) {
            return Promise.resolve(ready);
        }

        const cached = playbackMediaBlobCache.get(media.url);
        if (cached) {
            return cached;
        }

        const promise = decodeMediaUrl(media.url, media.video).then(() => {
            playbackMediaReadyUrls.set(media.url, media.url);
            return media.url;
        });

        playbackMediaBlobCache.set(media.url, promise);
        return promise;
    }

    function preloadAllWaypointMedia() {
        for (const waypoint of waypoints) {
            for (const media of getWaypointPopupMedia(waypoint)) {
                void prefetchPlaybackMedia(media);
            }
        }
    }

    function buildPlaybackWaypointMediaSchedule(geometry: RouteGeometryState) {
        playbackWaypointMediaSchedule = waypoints.flatMap((waypoint) => {
            const media = getWaypointPopupMedia(waypoint)[0];
            if (!media || !waypoint.id) {
                return [];
            }
            const distance = distanceAlongRouteForPoint(
                geometry,
                waypoint.lat,
                waypoint.lon,
            );
            if (distance === null) {
                return [];
            }
            return [
                {
                    waypointId: waypoint.id,
                    waypointName: waypoint.name ?? "",
                    media,
                    distance,
                },
            ];
        });
        preloadAllWaypointMedia();
        playback?.setPhotoDistances(
            playbackWaypointMediaSchedule.map((item) => item.distance),
        );
    }

    function preloadPlaybackWaypointMedia() {
        preloadAllWaypointMedia();
        for (const item of playbackWaypointMediaSchedule) {
            void prefetchPlaybackMedia(item.media);
        }
    }

    function findActivePlaybackMediaScheduleItem(state: RoutePlaybackState) {
        if (state.activePhotoDistance === null) {
            return null;
        }

        let best: PlaybackWaypointMediaScheduleItem | null = null;
        let bestDelta = Infinity;
        for (const item of playbackWaypointMediaSchedule) {
            const delta = Math.abs(item.distance - state.activePhotoDistance);
            if (delta >= bestDelta) {
                continue;
            }
            best = item;
            bestDelta = delta;
        }
        return best;
    }

    function hidePlaybackWaypointMedia() {
        if (!playbackOverlayLayers.length && !activePlaybackMediaUrl) {
            return;
        }
        activePlaybackMediaUrl = null;
        playbackOverlayLayers = playbackOverlayLayers.map((layer) => ({
            ...layer,
            visible: false,
        }));
        if (playbackMediaHideTimeout !== null) {
            clearTimeout(playbackMediaHideTimeout);
        }
        playbackMediaHideTimeout = setTimeout(() => {
            playbackMediaHideTimeout = null;
            if (!activePlaybackMediaUrl) {
                playbackOverlayLayers = [];
            }
        }, playbackMediaCrossfadeMs);
    }

    async function showPlaybackWaypointMedia(item: PlaybackWaypointMediaScheduleItem) {
        const displayUrl = await prefetchPlaybackMedia(item.media);
        const currentState = playback?.getState();
        if (
            !currentState ||
            findActivePlaybackMediaScheduleItem(currentState)?.media.url !== item.media.url
        ) {
            return;
        }

        if (playbackMediaHideTimeout !== null) {
            clearTimeout(playbackMediaHideTimeout);
            playbackMediaHideTimeout = null;
        }

        if (activePlaybackMediaUrl === item.media.url) {
            playbackOverlayLayers = playbackOverlayLayers.map((layer) => ({
                ...layer,
                visible: layer.media.url === item.media.url,
            }));
            return;
        }

        activePlaybackMediaUrl = item.media.url;
        const existing = playbackOverlayLayers.some(
            (layer) => layer.media.url === item.media.url,
        );
        if (!existing) {
            playbackOverlayLayers = [
                ...playbackOverlayLayers,
                {
                    id: item.media.url,
                    waypointName: item.waypointName,
                    media: item.media,
                    displayUrl,
                    visible: false,
                },
            ];
            await tick();
            await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
        }

        if (
            findActivePlaybackMediaScheduleItem(playback?.getState() ?? currentState)
                ?.media.url !== item.media.url
        ) {
            return;
        }

        playbackOverlayLayers = playbackOverlayLayers.map((layer) => ({
            ...layer,
            visible: layer.media.url === item.media.url,
        }));

        window.setTimeout(() => {
            if (activePlaybackMediaUrl !== item.media.url) {
                return;
            }
            playbackOverlayLayers = playbackOverlayLayers.filter(
                (layer) => layer.media.url === item.media.url || layer.visible,
            );
        }, playbackMediaCrossfadeMs);
    }

    function syncPlaybackWaypointMedia(state: RoutePlaybackState) {
        if (!playbackWaypointMediaSchedule.length) {
            hidePlaybackWaypointMedia();
            return;
        }

        const active = findActivePlaybackMediaScheduleItem(state);
        if (!active) {
            hidePlaybackWaypointMedia();
            return;
        }

        showPlaybackWaypointMedia(active);
    }

    function syncPlaybackTerrain() {
        if (!map?.getSource("terrain")) {
            return;
        }
        const exaggeration = enablePlayback3d ? 1.35 : 1;
        if (lastPlaybackTerrainExaggeration === exaggeration && map.getTerrain()) {
            return;
        }
        lastPlaybackTerrainExaggeration = exaggeration;
        map.setTerrain({
            source: "terrain",
            exaggeration,
        });
    }

    function resetPlaybackCamera() {
        if (!map) {
            return;
        }
        playbackCameraBearing = null;
        syncPlaybackTerrain();
        map.easeTo({
            pitch: 0,
            bearing: 0,
            duration: 350,
        });
    }

    function normalizeBearing(bearing: number) {
        return ((bearing % 360) + 360) % 360;
    }

    function shortestBearingDelta(from: number, to: number) {
        return ((to - from + 540) % 360) - 180;
    }

    function smoothPlaybackBearing(nextBearing: number) {
        const normalized = normalizeBearing(nextBearing);
        if (playbackCameraBearing === null) {
            playbackCameraBearing = normalized;
            return normalized;
        }
        const delta = shortestBearingDelta(playbackCameraBearing, normalized);
        playbackCameraBearing = normalizeBearing(playbackCameraBearing + delta * 0.08);
        return playbackCameraBearing;
    }

    function updatePlaybackCamera(state: RoutePlaybackState) {
        if (!map || !followPlaybackCamera || state.status !== "playing") {
            return;
        }
        const now = performance.now();
        if (now - lastPlaybackCameraUpdate < 240) {
            return;
        }
        lastPlaybackCameraUpdate = now;
        const smoothedBearing = smoothPlaybackBearing(state.bearing);
        const targetZoom = Math.min(
            playbackFollowZoomMax,
            map.getZoom() + playbackFollowZoomBoost,
        );
        map.easeTo({
            center: [state.position[0], state.position[1]],
            bearing: smoothedBearing,
            pitch: enablePlayback3d && isPlayback3dAllowed() ? 42 : 0,
            zoom: targetZoom,
            duration: 900,
            easing: (t) => 1 - Math.pow(1 - t, 3),
        });
    }

    function removePlaybackLayerArtifacts() {
        if (!map) {
            return;
        }
        playbackLayersInitialized = false;
        layerManager?.removeLayer(playbackLayerId);
        for (const layerId of [
            `${playbackLayerId}-line`,
            `${playbackLayerId}-remaining`,
            `${playbackLayerId}-completed`,
        ]) {
            if (map.getLayer(layerId)) {
                map.removeLayer(layerId);
            }
        }
        for (const sourceId of [
            `${playbackLayerId}-source`,
            `${playbackLayerId}-remaining-source`,
            `${playbackLayerId}-completed-source`,
        ]) {
            if (map.getSource(sourceId)) {
                map.removeSource(sourceId);
            }
        }
        delete layerManager?.layers[playbackLayerId];
        setTrailPlaybackOpacity(1);
    }

    function setTrailPlaybackOpacity(opacity: number) {
        const id = activeTrail !== null ? trails[activeTrail]?.id : null;
        if (!id || !map?.getLayer(id)) {
            return;
        }
        map.setPaintProperty(id, "line-opacity", opacity);
    }

    function ensureSpeedColoredRoute() {
        if (!playbackRouteGeometry || !map || playbackLayersInitialized) {
            return;
        }

        layerManager.addLayer(
            playbackLayerId,
            new RouteProgressLayer(
                playbackLayerId,
                buildSpeedColoredRoute(playbackRouteGeometry),
                getActiveTrailColor(),
            ),
        );

        const coloredLayerId = `${playbackLayerId}-line`;
        if (map.getLayer(coloredLayerId)) {
            map.moveLayer(coloredLayerId);
            setTrailPlaybackOpacity(0);
            playbackLayersInitialized = true;
        }
    }

    function handlePlaybackUpdate(state: RoutePlaybackState) {
        playbackState = state;
        playbackMarker?.setLngLat([state.position[0], state.position[1]]);
        playbackMarker?.setOpacity("1");
        ensureSpeedColoredRoute();
        epc?.seekToProgress(state.progress);
        syncPlaybackWaypointMedia(state);
        if (state.progress >= 1) {
            resetPlaybackCamera();
            return;
        }
        updatePlaybackCamera(state);
    }

    function destroyRoutePlayback() {
        playback?.destroy();
        playback = null;
        playbackState = null;
        playbackCameraBearing = null;
        lastPlaybackTerrainExaggeration = null;
        playbackReady = false;
        playbackTrailId = null;
        playbackRouteGeometry = null;
        revokePlaybackMediaBlobCache();
        clearPlaybackWaypointMedia();
        syncPlaybackTerrain();
        removePlaybackMarker();
        removePlaybackLayerArtifacts();
    }

    function syncRoutePlayback() {
        const id = activeTrail !== null ? trails[activeTrail]?.id : null;
        const geojson = id ? gpxDataMap[id] : null;
        const shouldEnable =
            enableRoutePlayback &&
            !clusterTrails &&
            !drawing &&
            trails.length === 1 &&
            Boolean(id && geojson && map);

        if (!shouldEnable || !id || !geojson) {
            destroyRoutePlayback();
            return;
        }

        if (playbackTrailId === id && playback) {
            playbackReady = true;
            return;
        }

        destroyRoutePlayback();

        const geometry = routeGeometryFromGeoJson(geojson);
        playbackTrailId = id;
        playbackRouteGeometry = geometry;
        buildPlaybackWaypointMediaSchedule(geometry);
        playbackReady = true;
        if (geometry.positions.length < 2) {
            // Show the UI even if playback can't interpolate properly.
            // This helps diagnosing missing controls / geometry issues.
            return;
        }

        createPlaybackMarker();
        preloadPlaybackWaypointMedia();
        playback = new RoutePlayback({
            geometry,
            constantDurationMs: Math.max(45_000, geometry.totalDistance * 15),
            photoDistances: playbackWaypointMediaSchedule.map((item) => item.distance),
            photoDwellMs: playbackMediaDisplayMs,
            onUpdate: handlePlaybackUpdate,
        });
        enablePlayback3d = isPlayback3dAllowed();
        playback.seek(0);
    }

    function togglePlayback() {
        if (!playback) {
            return;
        }
        const state = playback.getState();
        if (state?.status === "playing") {
            playback.pause();
            resetPlaybackCamera();
            return;
        }
        if ((state?.progress ?? 0) >= 1) {
            playback.seek(0);
        }
        playback.play();
        syncPlaybackTerrain();
    }

    function handlePlaybackProgressChange(progress: number) {
        if (!playback) {
            return;
        }
        playback.seek(progress);
        if (playback.getState()?.status !== "playing") {
            updatePlaybackCamera(playback.getState()!);
        }
    }

    function handlePlaybackPhotoDurationChange(durationMs: number) {
        if (!playbackMediaDurationOptions.includes(durationMs)) {
            return;
        }
        playbackMediaDisplayMs = durationMs;
        playback?.setPhotoDwellMs(durationMs);
    }

    function syncElevationProfileVisibility() {
        if (
            showElevation &&
            Object.keys(gpxDataMap).length &&
            activeTrail !== null &&
            elevationProfileVisibilityPreference !== false
        ) {
            epc?.showProfile();
        } else {
            epc?.hideProfile();
        }
    }

    function getBounds() {
        let minX = Infinity,
            minY = Infinity,
            maxX = -Infinity,
            maxY = -Infinity;

        for (const [xMin, yMin, xMax, yMax] of Object.values(gpxDataMap)
            .filter((d) => d.bbox !== undefined)
            .map((d) => d.bbox!)) {
            minX = Math.min(minX, xMin);
            minY = Math.min(minY, yMin);
            maxX = Math.max(maxX, xMax);
            maxY = Math.max(maxY, yMax);
        }

        if (
            minX < Infinity &&
            minY < Infinity &&
            maxX > -Infinity &&
            maxY > -Infinity
        ) {
            return new M.LngLatBounds([minX, minY, maxX, maxY]);
        } else {
            return new M.LngLatBounds([0, 0, 0, 0]);
        }
    }

    export function fitToBounds(bounds?: M.LngLatBoundsLike) {
        const activeId = activeTrail !== null ? trails[activeTrail]?.id : null;
        const boundsToFit =
            bounds ??
            (activeId && gpxDataMap[activeId]
                ? (gpxDataMap[activeId].bbox as M.LngLatBoundsLike)
                : getBounds());

        if (!boundsToFit || !map) {
            return;
        }

        map!.fitBounds(boundsToFit, {
            animate: fitBounds == "animate",
            padding: {
                top: 16,
                left: 16,
                right: 16,
                bottom:
                    16 +
                    (epc?.isProfileShown && !elevationProfileContainer
                        ? map!.getContainer().clientHeight * 0.3
                        : 0),
            },
        });
    }

    function flyToBounds() {
        fitToBounds();
    }

    function removeTrailLayer(id: string) {
        layerManager.removeLayer(id);
    }

    function addTrailLayer(
        trail: Trail,
        id: string,
        index: number,
        geojson: GeoJSON.FeatureCollection | null | undefined,
    ) {
        if (!geojson || !map) {
            return;
        }
        const trailLayer = new TrailLayer(
            id,
            geojson,
            trailColors[
                clusterTrails
                    ? hashStringToIndex(id ?? "", trailColors.length)
                    : index % trailColors.length
            ],
            {
                listeners: {
                    onEnter: (e) =>
                        highlightTrail(id, trails[activeTrail ?? -1]?.id == id),

                    onLeave: (e) => unHighlightTrail(id),
                    onMouseUp: (e) => {
                        activeTrail = trails.findIndex((t) => t.id == trail.id);
                    },
                    onMouseMove: moveCrosshairToCursorPosition,
                    onMouseDown: (e) => handleDragStart(e, id),
                },
            },
        );

        layerManager.addLayer(id, trailLayer);

        if (!drawing && !clusterTrails) {
            addStartEndMarkers(trail, id, geojson);
        }
    }

    function addClusterLayer(geojson: FeatureCollection) {
        if (!geojson || !map || !map.style) {
            return;
        }
        layerManager.addLayer(
            "clusters",
            new ClusterLayer(map, geojson, {
                "unclustered-point": {
                    onEnter: (e) => {
                        if (map) map.getCanvas().style.cursor = "pointer";
                        const id = (e as any).features[0].properties.id;
                        const trail = trails.find((t) => t.id === id);
                        if (!hasTrailDetails(trail)) return;
                        highlightCluster(trail, e.lngLat);
                    },
                },
            }),
        );
    }

    function addPreviewLayer(geojson: FeatureCollection) {
        if (!geojson || !map || !map.style) {
            return;
        }
        layerManager.addLayer(
            "preview",
            new PreviewLayer(map, geojson, {
                showStartMarker: page.data.settings?.behavior?.showTrailStartMarker ?? false,
                listeners: {
                    preview: {
                        onEnter: (e) => {
                            if (map) map.getCanvas().style.cursor = "pointer";
                            const trail = trails.find(
                                (t) =>
                                    t.id ===
                                    (e as any).features[0].properties.trail,
                            );
                            if (!hasTrailDetails(trail)) return;
                            highlightCluster(trail, e.lngLat);
                        },
                    },
                },
            }),
        );
    }

    function moveCrosshairToCursorPosition(e: M.MapMouseEvent) {
        if (playbackState?.status === "playing") {
            return;
        }
        epc?.moveCrosshair(e.lngLat.lat, e.lngLat.lng);
        moveElevationMarkerToCursorPosition(e);
    }

    function moveElevationMarkerToCursorPosition(e: M.MapMouseEvent) {
        elevationMarker.setLngLat(e.lngLat);
    }

    function handleDragStart(e: M.MapMouseEvent, id: string) {
        if (
            !drawing ||
            (e.originalEvent.target as HTMLElement | null)?.classList.contains(
                "route-anchor",
            )
        ) {
            return;
        }
        e.preventDefault();

        const features = map?.queryRenderedFeatures(e.point, {
            layers: [id],
        });
        const segmentId = features?.at(0)?.properties.segmentId;
        if (segmentId !== null) {
            draggingSegment = segmentId;
        }

        map?.on("mousemove", moveElevationMarkerToCursorPosition);
        map?.once("mouseup", (e2) => handleDragEnd(e2, e));
    }

    function handleDragEnd(end: M.MapMouseEvent, start: M.MapMouseEvent) {
        map?.off("mousemove", moveElevationMarkerToCursorPosition);
        epc?.hideCrosshair();
        const distanceDragged = Math.sqrt(
            Math.pow(end.originalEvent.x - start.originalEvent.x, 2) +
                Math.pow(end.originalEvent.y - start.originalEvent.y, 2),
        );
        if (distanceDragged < 0.5) {
            onsegmentclick?.({ segment: draggingSegment!, event: end });
        } else {
            onsegmentdragend?.({ segment: draggingSegment!, event: end });
        }
        draggingSegment = null;
    }

    function addCaretLayer(geojson: GeoJSON) {
        if (!map) {
            return;
        }
        if (map.getLayer("direction-carets")) {
            removeCaretLayer();
        }
        layerManager.addLayer(
            "direction-carets",
            new CaretLayer({ type: "geojson", data: geojson }),
        );
    }

    function removeCaretLayer() {
        layerManager?.removeLayer("direction-carets");
    }

    export function highlightTrail(
        id: string,
        showElevationMarker: boolean = false,
    ) {
        if (!id) {
            return;
        }
        if (showElevationMarker) {
            elevationMarker.setOpacity("1");
        }
        map?.setPaintProperty(id, "line-width", 7);
        if (map?.getLayer(id)) {
            hoveringTrail = true;
        }
        // map?.setPaintProperty(id, "line-color", "#2766e3");
    }

    export function unHighlightTrail(id: string | undefined) {
        if (!id || draggingSegment !== null) {
            return;
        }
        if (playbackState?.status === "playing") {
            return;
        }
        elevationMarker.setOpacity("0");
        epc?.hideCrosshair();
        hoveringTrail = false;
        if (map?.getLayer(id)) {
            map?.setPaintProperty(id, "line-width", 5);
        }
        // map?.setPaintProperty(id, "line-color", "#648ad5");
    }

    function hasTrailDetails(trail: Trail | undefined): trail is Trail {
        return Boolean(trail?.name?.trim());
    }

    export async function highlightCluster(
        trail: Trail,
        lnglat?: M.LngLatLike,
    ) {
        if (!map || !map.style || !hasTrailDetails(trail)) {
            return;
        }
        clusterPopup?.remove();
        clusterPopup = createPopupFromTrail(trail);
        clusterPopup.setLngLat(lnglat ?? [trail.lon!, trail.lat!]).addTo(map);
        clusterPopup.on("close", () => {
            unHighlightCluster(false);
        });
        map.on("mousemove", unHighlightClusterDistanceNotifier);
    }

    function unHighlightClusterDistanceNotifier(e: M.MapMouseEvent) {
        if (!clusterPopup || !map) {
            return;
        }
        if (
            map.project(clusterPopup.getLngLat()).dist(map.project(e.lngLat)) >
            60
        ) {
            clusterPopup.remove();
            map.off("mousemove", unHighlightClusterDistanceNotifier);
        }
    }

    export async function unHighlightCluster(closePopup: boolean = true) {
        if (!map || !map.style) {
            return;
        }
        layerManager.removeLayer("cluster-highlight");
        if (closePopup) {
            clusterPopup?.remove();
        }
    }

    function adjustTrailFocus(activeTrail: number | null) {
        if (activeTrail !== null && trails[activeTrail] !== undefined) {
            if (
                !drawing &&
                fitBounds !== "off" &&
                Object.values(gpxDataMap).some((d) => d.bbox !== undefined)
            ) {
                untrack(() => focusTrail(trails[activeTrail]));
            }
        } else if (activeTrail === null && trails.length) {
            untrack(() => unFocusTrail());
        }
    }

    function focusTrail(trail: Trail) {
        activeTrail = trails.findIndex((t) => t.id == trail.id);
        if (activeTrail < 0) {
            activeTrail = null;
            return;
        }
        onselect?.(trail);

        try {
            refreshElevationProfile();
            syncElevationProfileVisibility();
            syncWaypointMarkers();
            if (trail.id && gpxDataMap[trail.id]) {
                addCaretLayer(gpxDataMap[trail.id]);
            }
            flyToBounds();
        } catch (e) {
            console.warn(e);
        }
    }

    function unFocusTrail(trail?: Trail) {
        if (trail) {
            onunselect?.(trail);
            unHighlightTrail(trail.id!);
        }

        activeTrail = null;
        flyToBounds();

        if (showElevation) {
            epc?.hideProfile();
        }
        hideWaypoints();
        removeCaretLayer();
    }

    function startDrawing() {
        if (!map) {
            return;
        }
        activeTrail ??= 0;
        map.getCanvas().style.cursor = "crosshair";
        if (trails[activeTrail]) {
            removeStartEndMarkers(trails[activeTrail].id);
        }

        if (autoGeolocateOnDrawing) {
            geolocate();
        }
    }

    function stopDrawing() {
        if (!map) {
            return;
        }
        syncWaypointMarkers();
        map.getCanvas().style.cursor = "inherit";

        if (activeTrail !== null && trails[activeTrail] && !clusterTrails) {
            const activeId = trails[activeTrail].id;
            addStartEndMarkers(
                trails[activeTrail],
                activeId,
                activeId ? gpxDataMap[activeId] : null,
            );
        }
    }

    function addStartEndMarkers(
        trail: Trail,
        id: string | undefined,
        geojson: GeoJSON | null | undefined,
    ) {
        if (
            !map ||
            !trail ||
            !id ||
            !(layerManager.layers[id] instanceof TrailLayer)
        ) {
            return;
        }

        const startMarker = new FontawesomeMarker(
            { icon: "fa fa-bullseye" },
            {},
        );
        layerManager.layers[id].markers.start = startMarker;

        if (!geojson) {
            if (trail.lon && trail.lat) {
                startMarker.setLngLat([trail.lon, trail.lat]).addTo(map);
            }
            return;
        }

        const startEndPoint = findStartAndEndPoints(geojson);

        if (!startEndPoint.length) {
            return;
        }

        const endMarker = new FontawesomeMarker(
            { icon: "fa fa-flag-checkered" },
            {},
        );
        layerManager.layers[id].markers.end = endMarker;

        startMarker.setLngLat(startEndPoint[0] as M.LngLatLike);
        endMarker.setLngLat(
            startEndPoint[startEndPoint.length - 1] as M.LngLatLike,
        );

        if (showInfoPopup) {
            const popup = createPopupFromTrail(trail);
            startMarker.setPopup(popup);
            endMarker.setPopup(popup);
        }

        startMarker.addTo(map);
        if (!clusterTrails) {
            endMarker.addTo(map);
        }
    }

    function removeStartEndMarkers(id: string | undefined) {
        if (!id) {
            return;
        }
        layerManager.layers[id].markers?.start?.remove();
        layerManager.layers[id].markers?.end?.remove();
    }

    function showWaypoints() {
        if (!map) {
            return;
        }

        hideWaypoints();
        for (const waypoint of waypoints) {
            if (!markers.find((m) => m._element.id == waypoint.id)) {
                const marker = createMarkerFromWaypoint(
                    waypoint,
                    onmarkerdragend,
                );
                marker.addTo(map);
                markers.push(marker);
            }
        }
        markers = markers.filter((marker) => {
            if (!waypoints.find((w) => w.id == marker._element.id)) {
                marker.remove();
                return false;
            }
            return true;
        });
    }

    function syncWaypointMarkers() {
        if (displayWaypoints) {
            showWaypoints();
        } else {
            hideWaypoints();
        }
    }

    function hideWaypoints() {
        if (!map) {
            return;
        }
        for (const m of markers) {
            m.remove();
        }
        markers = [];
    }

    function handleWaypointFocus(event: Event) {
        const detail = (event as CustomEvent<WaypointFocusDetail>).detail;
        if (!detail?.waypointId) {
            return;
        }

        if (detail.source === "profile") {
            const marker = markers.find(
                (item) => item.getElement().id === detail.waypointId,
            );
            if (marker && !marker.getPopup()?.isOpen()) {
                marker.togglePopup();
            }
        }

        if (Number.isFinite(detail.lat) && Number.isFinite(detail.lon)) {
            epc?.moveCrosshair(detail.lat, detail.lon);
        }
    }

    function toggleEpcTheme() {
        if ($theme == "dark") {
            epc?.toggleTheme({
                profileBackgroundColor: "#191b24",
                elevationGridColor: "#ddd2",
                labelColor: "#ddd8",
                crosshairColor: "#fff5",
                tooltipBackgroundColor: "#242734",
                tooltipTextColor: "#fff",
            });
        } else {
            epc?.toggleTheme({
                profileBackgroundColor: "#242734",
                elevationGridColor: "#0002",
                labelColor: "#0009",
                crosshairColor: "#0005",
                tooltipBackgroundColor: "#fff",
                tooltipTextColor: "#000",
            });
        }
    }

    let geolocateControl: M.GeolocateControl;

    onMount(async () => {
        document.addEventListener(WAYPOINT_FOCUS_EVENT, handleWaypointFocus);
        const initialState = {
            lng: 0,
            lat: 0,
            zoom: 1,
        };
        const ElevationProfileControl = (
            await import(
                "$lib/vendor/maplibre-elevation-profile/elevationprofile-control"
            )
        ).ElevationProfileControl;

        if (!mapContainer) {
            return;
        }

        for (const tileset of page.data.settings?.tilesets ?? []) {
            baseMapStyles[tileset.name] = tileset.url;
        }

        const finalMapOptions: M.MapOptions = {
            ...{
                container: mapContainer,
                center: [initialState.lng, initialState.lat],
                zoom: initialState.zoom,
            },
            ...mapOptions,
        };
        map = new M.Map(finalMapOptions);

        layerManager = new LayerManager(map, { overpassActionFactory: buildPoiAnchorAction });

        elevationMarker = new FontawesomeMarker(
            {
                id: "elevation-marker",
                icon: "fa-regular fa-circle",
                fontSize: "xs",
                width: 4,
                backgroundColor: "bg-primary",
                fontColor: "white",
            },
            {},
        );
        elevationMarker.setLngLat([0, 0]).addTo(map);
        elevationMarker.setOpacity("0");

        let img = new Image(20, 20);
        img.onload = () => map!.addImage("direction-caret", img);
        img.src = directionCaret;

        const switcherControl = new StyleSwitcherControl({
            styles: baseMapStyles,
            onchange: (state) => {
                layerManager.update(JSON.parse(JSON.stringify(state)));
            },
            state: JSON.parse(JSON.stringify(layerManager.state)),
        });

        map.addControl(
            new M.NavigationControl({ visualizePitch: showTerrain }),
        );
        map.addControl(
            new M.ScaleControl({
                maxWidth: 120,
                unit: page.data.settings?.unit ?? "metric",
            }),
            "top-left",
        );

        geolocateControl = new M.GeolocateControl({
            positionOptions: {
                enableHighAccuracy: true,
            },
            fitBoundsOptions: {
                animate: fitBounds == "animate",
            },
            trackUserLocation: true,
        });
        map.addControl(geolocateControl);

        if (showStyleSwitcher) {
            map.addControl(switcherControl);
        }

        if (showElevation) {
            epc = new ElevationProfileControl({
                visible: false,
                profileBackgroundColor:
                    $theme == "light" ? "#242734" : "#191b24",
                backgroundColor: "bg-menu-background/90",
                unit: page.data.settings?.unit ?? "metric",
                profileLineWidth: 3,
                displayDistanceGrid: true,
                tooltipDisplayDPlus: false,
                tooltipBackgroundColor: $theme == "light" ? "#fff" : "#242734",
                tooltipTextColor: $theme == "light" ? "#000" : "#fff",
                zoom: false,
                container: elevationProfileContainer,
                onEnter: () => {
                    elevationMarker.setOpacity("1");
                },
                onLeave: () => {
                    elevationMarker.setOpacity("0");
                },
                onToggle: (visible) => {
                    elevationProfileVisibilityPreference = visible;
                },
                onMove: (data) => {
                    if (!hoveringTrail && playbackState?.status !== "playing") {
                        elevationMarker.setLngLat(
                            data.position as M.LngLatLike,
                        );
                    }
                },
            });
            toggleEpcTheme();
            map.addControl(epc);
        }

        if (showFullscreen) {
            map.addControl(
                new FullscreenControl(() => {
                    onfullscreen?.();
                }),
                "bottom-right",
            );
        }

        if (showTerrain && page.data.settings?.terrain?.terrain) {
            map!.addControl(
                new M.TerrainControl({
                    source: "terrain",
                }),
            );
        }

        map.on("styledata", (e) => {
            if (showTerrain && page.data.settings?.terrain?.terrain) {
                layerManager.addLayer(
                    "terrain",
                    new TerrainLayer(
                        page.data.settings.terrain.terrain,
                        page.data.settings?.terrain?.hillshading,
                    ),
                );
                syncHillshadingVisibility();
            }
        });

        map.on("render", () => {
            syncHillshadingVisibility();
        });

        map.on("moveend", (e) => {
            onmoveend?.(e.target);
        });

        map.on("zoom", (e) => {
            onzoom?.(e.target);
        });

        map.on("click", (e) => {
            if (hoveringTrail && drawing) {
                return;
            }
            onclick?.(e);
        });

        map.on("contextmenu", (e) => {
            oncontextmenu?.(e);
        });

        map.on("load", () => {
            layerManager.init();
            initMap(true);
            oninit?.(map!);
            mapLoaded = true;
        });

        syncWaypointMarkers();
    });

    function geolocate() {
        if (!page.data.settings?.behavior) return;

        if (page.data.settings.behavior.allowAutoGeolocate === true) {
            if (geolocateControl._watchState === "OFF") {
                geolocateControl.options.trackUserLocation = true;
                geolocateControl.trigger();
            }
        }
    }

    onDestroy(() => {
        destroyRoutePlayback();
        clearPlaybackWaypointMedia();
        if (typeof document !== "undefined") {
            document.removeEventListener(WAYPOINT_FOCUS_EVENT, handleWaypointFocus);
        }
        map?.remove();
    });

    function handleKeydown(e: KeyboardEvent) {
        const target = e.target as HTMLElement;

        const isInputField =
            target.tagName === "INPUT" ||
            target.tagName === "TEXTAREA" ||
            target.isContentEditable;

        if (isInputField) {
            return;
        }

        if (e.key == "m") {
            if (trails.length === 1) {
                removeCaretLayer();
                removeTrailLayer(trails[0].id!);
            }
        }
    }

    function handleKeyup(e: KeyboardEvent) {
        const target = e.target as HTMLElement;

        const isInputField =
            target.tagName === "INPUT" ||
            target.tagName === "TEXTAREA" ||
            target.isContentEditable;

        if (isInputField) {
            return;
        }

        if (e.key == "m") {
            if (trails.length === 1) {
                const trailId = trails[0].id!;
                addTrailLayer(trails[0], trailId, 0, gpxDataMap[trailId]);
                addCaretLayer(gpxDataMap[trailId]);
            }
        } else if (e.key == "p") {
            if (showElevation) {
                epc?.toggleProfile();
            }
        }
    }

    function hashStringToIndex(str: string, max: number) {
        let hash = 0;
        for (let i = 0; i < str.length; i++) {
            hash = (hash << 5) - hash + str.charCodeAt(i);
            hash |= 0;
        }
        return Math.abs(hash) % max;
    }
</script>

<svelte:window on:keydown={handleKeydown} on:keyup={handleKeyup} />
<div id="map-wrapper">
    <div id="map" bind:this={mapContainer}></div>
    {#if playbackOverlayLayers.length}
        <div class="playback-waypoint-media">
            {#each playbackOverlayLayers as layer (layer.id)}
                {#if layer.media.video}
                    <video
                        class="playback-waypoint-media-frame"
                        class:is-ready={layer.visible}
                        src={layer.displayUrl}
                        autoplay
                        muted
                        playsinline
                        loop
                    ></video>
                {:else}
                    <img
                        class="playback-waypoint-media-frame"
                        class:is-ready={layer.visible}
                        src={layer.displayUrl}
                        alt={layer.waypointName}
                    />
                {/if}
            {/each}
            {#if playbackOverlayLayers.find((layer) => layer.visible)?.waypointName}
                <div class="absolute left-0 right-0 bottom-0 z-10 px-3 py-2 text-sm font-medium text-white bg-black/55">
                    {playbackOverlayLayers.find((layer) => layer.visible)?.waypointName}
                </div>
            {/if}
        </div>
    {/if}
    <RoutePlaybackControl
        visible={playbackReady}
        playing={playbackState?.status === "playing"}
        progress={playbackState?.progress ?? 0}
        mode={playbackState?.mode ?? "constant"}
        photoDurationMs={playbackMediaDisplayMs}
        followCamera={followPlaybackCamera}
        enable3d={enablePlayback3d}
        allow3d={isPlayback3dAllowed()}
        distanceLabel={`${formatDistance(playbackState?.distance)} / ${formatDistance(playbackState?.totalDistance)}`}
        durationLabel={formatTimeHHMM((playbackState?.durationMs ?? 0) / 1000)}
        onplaypause={togglePlayback}
        onprogresschange={handlePlaybackProgressChange}
        onphotodurationchange={handlePlaybackPhotoDurationChange}
        onfollowcamerachange={(enabled) => (followPlaybackCamera = enabled)}
        onenable3dchange={(enabled) => {
            enablePlayback3d = enabled;
            lastPlaybackTerrainExaggeration = null;
            syncPlaybackTerrain();
            if (!enabled) {
                resetPlaybackCamera();
            }
        }}
    />
</div>

<style lang="postcss">
    @reference "tailwindcss";
    @reference "../../../css/app.css";

    #map-wrapper,
    #map {
        width: 100%;
        height: 100%;
    }

    #map-wrapper {
        position: relative;
    }

    .playback-waypoint-media {
        position: absolute;
        top: 16px;
        right: 16px;
        width: min(320px, calc(100% - 32px));
        aspect-ratio: 4 / 3;
        z-index: 24;
        overflow: visible;
        pointer-events: none;
        background: transparent;
    }

    .playback-waypoint-media-frame {
        position: absolute;
        inset: 0;
        width: 100%;
        height: 100%;
        object-fit: cover;
        opacity: 0;
        border-radius: 0.75rem;
        box-shadow: none;
        transition: opacity 400ms ease;
    }

    .playback-waypoint-media-frame.is-ready {
        opacity: 1;
        box-shadow: 0 10px 15px -3px rgb(0 0 0 / 0.2);
    }

    :global(.maplibregl-popup-content) {
        @apply bg-background rounded-md shadow-xl p-0 overflow-hidden pr-5;
    }

    :global(.waypoint-popup) {
        @apply cursor-pointer;
    }

    :global(.waypoint-popup-photos) {
        @apply mb-2 grid grid-cols-1 gap-1;
    }

    :global(.waypoint-popup-photos.multiple) {
        @apply grid-cols-2;
    }

    :global(.waypoint-popup-media) {
        @apply h-24 w-full rounded-md object-cover;
    }

    :global(.maplibregl-popup-close-button) {
        top: 4px;
        right: 4px;
        line-height: 0;
        padding-bottom: 2.5px;
        @apply bg-menu-item-background-focus w-3 aspect-square rounded-full;
    }

    :global(
            .maplibregl-user-location-accuracy-circle,
            .maplibregl-user-location-dot
        ) {
        pointer-events: none;
    }
</style>
