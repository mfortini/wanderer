export interface Asset {
    id: string;
    collectionId: string;
    collectionName: string;
    type: "photo";
    file?: string;
    storage_mode?: "copy" | "link_private" | "link_public";
    remote_status?: "available" | "missing" | "inaccessible";
    remote_checked_at?: string;
    remote_missing_since?: string;
    remote_error?: string;
    author: string;
    trail?: string;
    waypoint?: string;
    summit_log?: string;
    external_provider?: string;
    external_id?: string;
    taken_at?: string;
    lat?: number;
    lon?: number;
    metadata?: Record<string, unknown>;
}
