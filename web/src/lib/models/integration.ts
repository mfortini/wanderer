
export interface BaseIntegration {
    active: boolean
}

export interface StravaIntegration extends BaseIntegration {
    clientId: string | number;
    clientSecret?: string;
    routes: boolean;
    activities: boolean;
    accessToken?: string;
    refreshToken?: string;
    expiresAt?: number;
    after?: string
    privacy: "original" | "settings"
}

export interface KomootIntegration extends BaseIntegration {
    email: string,
    password: string,
    completed: boolean,
    planned: boolean
    privacy: "original" | "settings"
}

export interface HammerheadIntegration extends BaseIntegration {
    email: string,
    password: string,
    completed: boolean,
    planned: boolean,
    after?: string
}

export interface ImmichIntegration extends BaseIntegration {
    url: string;
    apiKey: string;
    timeWindowMinutes: number;
    maxDistanceMeters: number;
    maxWaypoints: number;
    photoMode: "copy" | "link_private" | "link_public";
    providers: ("strava" | "komoot" | "hammerhead" | "upload")[];
}


export class Integration {
    id?: string;
    user: string;
    strava?: StravaIntegration | null;
    komoot?: KomootIntegration | null;
    hammerhead?: HammerheadIntegration | null;
    immich?: ImmichIntegration | null;

    constructor(user: string, strava?: StravaIntegration, komoot?: KomootIntegration, hammerhead?: HammerheadIntegration, immich?: ImmichIntegration) {
        this.user = user;
        this.strava = strava;
        this.komoot = komoot;
        this.hammerhead = hammerhead;
        this.immich = immich;
    }
}
