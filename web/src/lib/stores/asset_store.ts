import type { Asset } from "$lib/models/asset";
import { APIError } from "$lib/util/api_util";

interface AssetTarget {
    trail?: string;
    waypoint?: string;
    summit_log?: string;
    lat?: number;
    lon?: number;
}

export async function assets_create(
    files: File[],
    target: AssetTarget,
    f: (url: RequestInfo | URL, config?: RequestInit) => Promise<Response> = fetch,
): Promise<Asset[]> {
    if (!files.length) {
        return [];
    }

    const formData = new FormData();
    for (const file of files) {
        formData.append("files", file);
    }
    for (const [key, value] of Object.entries(target)) {
        if (value !== undefined && value !== null) {
            formData.append(key, value.toString());
        }
    }

    const r = await f("/api/v1/assets", {
        method: "PUT",
        body: formData,
    });
    if (!r.ok) {
        const response = await r.json();
        throw new APIError(r.status, response.message, response.detail);
    }

    return await r.json();
}

export async function assets_delete_removed(oldPhotos: string[] | undefined, newPhotos: string[] | undefined): Promise<void> {
    const removedAssetIds = (oldPhotos ?? [])
        .filter((oldPhoto) => !(newPhotos ?? []).find((newPhoto) => newPhoto === oldPhoto))
        .map(assetIdFromPhotoUrl)
        .filter((id): id is string => !!id);

    for (const assetId of removedAssetIds) {
        const r = await fetch(`/api/v1/assets/${assetId}`, { method: "DELETE" });
        if (!r.ok && r.status !== 404) {
            const response = await r.json();
            throw new APIError(r.status, response.message, response.detail);
        }
    }
}

export function asset_photo_url(asset: Asset): string {
    if (asset.file) {
        return `/api/v1/files/${asset.collectionId}/${asset.id}/${asset.file}`;
    }
    return `/api/v1/assets/${asset.id}/file`;
}

function assetIdFromPhotoUrl(photo: string): string | undefined {
    const remoteMatch = photo.match(/\/api\/v1\/assets\/([a-z0-9]{15})\/file/);
    if (remoteMatch) {
        return remoteMatch[1];
    }

    const fileMatch = photo.match(/\/api\/v1\/files\/[^/]+\/([a-z0-9]{15})\//);
    return fileMatch?.[1];
}
