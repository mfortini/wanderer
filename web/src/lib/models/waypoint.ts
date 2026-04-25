import type { icons } from "$lib/util/icon_util";
import * as M from "maplibre-gl";
import type { Asset } from "./asset";

class Waypoint {
    id?: string;
    name?: string;
    description?: string;
    lat: number;
    lon: number;
    distance_from_start?: number;
    icon?: typeof icons[number];
    marker?: M.Marker;
    photos: string[];
    _photos?: File[];
    _immichCandidates?: { assetId: string; lat: number; lon: number; originalFileName?: string }[];
    author: string;
    trail?: string;
    expand?: {
        assets_via_waypoint?: Asset[];
    }

    constructor(lat: number, lon: number, params?: {
        id?: string, name?: string, description?: string, icon?: typeof icons[number], marker?: M.Marker, photos?: string[], trail?: string
    }) {
        this.trail = params?.trail;
        this.id = params?.id;
        this.name = params?.name ?? "";
        this.description = params?.description ?? "";
        this.lat = lat;
        this.lon = lon;
        this.icon = params?.icon ?? "circle";
        this.marker = params?.marker;
        this.photos = params?.photos ?? []
        this._photos = [];
        this.author = "000000000000000"
    }
}

export { Waypoint };
