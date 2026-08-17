import type { Waypoint } from "$lib/models/waypoint";
import { getFileURL, isVideoURL } from "./file_util";

export type WaypointPopupMedia = {
    url: string;
    video: boolean;
};

export function waypointAnchorId(waypointId?: string) {
    return waypointId ? `waypoint-${waypointId}` : undefined;
}

export function getWaypointPopupMedia(waypoint: Waypoint): WaypointPopupMedia[] {
    const urls: string[] = [];

    for (const photo of waypoint.photos ?? []) {
        const url = getFileURL(waypoint, photo);
        if (url) {
            urls.push(url);
        }
    }

    for (const file of waypoint._photos ?? []) {
        urls.push(URL.createObjectURL(file));
    }

    return urls.map((url) => ({
        url,
        video: isVideoURL(url),
    }));
}

export function scrollToWaypointAnchor(waypointId?: string) {
    const id = waypointAnchorId(waypointId);
    if (!id || typeof document === "undefined") {
        return null;
    }

    const element = document.getElementById(id);
    element?.scrollIntoView({ behavior: "smooth", block: "center" });
    return element ?? null;
}
