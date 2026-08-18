import type { RouteGeometryState } from "./route_geometry";
import { interpolateAtDistance } from "./route_geometry";

export type RoutePlaybackMode = "realtime" | "constant";
export type RoutePlaybackStatus = "playing" | "paused";

export type RoutePlaybackState = {
    status: RoutePlaybackStatus;
    progress: number;
    distance: number;
    totalDistance: number;
    bearing: number;
    position: [number, number, number?];
    mode: RoutePlaybackMode;
    speed: number;
    elapsedMs: number;
    durationMs: number;
    activePhotoDistance: number | null;
};

type RoutePlaybackOptions = {
    geometry: RouteGeometryState;
    constantDurationMs?: number;
    maxDurationMs?: number;
    photoDistances?: number[];
    photoDwellMs?: number;
    onUpdate?: (state: RoutePlaybackState) => void;
};

const DEFAULT_CONSTANT_DURATION_MS = 90_000;
const DEFAULT_MAX_DURATION_MS = 60_000;
const DEFAULT_PHOTO_DWELL_MS = 1500;
const PHOTO_APPROACH_MS = 450;
const PHOTO_DEPART_MS = 550;
const PHOTO_HOLD_METERS = 3.5;
const PHOTO_APPROACH_MIN_RATIO = 0.22;

function clamp(value: number, min: number, max: number) {
    return Math.min(max, Math.max(min, value));
}

function clamp01(value: number) {
    return clamp(value, 0, 1);
}

function easeOutExpo(value: number) {
    const t = clamp01(value);
    if (t === 0) {
        return 0;
    }
    if (t === 1) {
        return 1;
    }
    return 1 - Math.pow(2, -10 * t);
}

function computeRealtimeDurationMs(geometry: RouteGeometryState) {
    const validTimes = geometry.timestamps.filter(
        (timestamp): timestamp is Date => timestamp instanceof Date && !Number.isNaN(timestamp.getTime()),
    );
    if (validTimes.length < 2) {
        return null;
    }
    const durationMs = validTimes[validTimes.length - 1].getTime() - validTimes[0].getTime();
    return durationMs > 0 ? durationMs : null;
}

export class RoutePlayback {
    private geometry: RouteGeometryState;
    private onUpdate?: (state: RoutePlaybackState) => void;
    private frameId: number | null = null;
    private lastTickMs: number | null = null;
    private elapsedMs = 0;
    private progress = 0;
    private status: RoutePlaybackStatus = "paused";
    private speed = 1;
    private mode: RoutePlaybackMode;
    private durationMs: number;
    private photoDistances: number[] = [];
    private photoDwellMs: number;
    private visitedPhotoIndexes = new Set<number>();
    private dwellPhotoIndex: number | null = null;
    private dwellRemainingMs = 0;
    private departElapsedMs = PHOTO_DEPART_MS;

    constructor(options: RoutePlaybackOptions) {
        this.geometry = options.geometry;
        this.onUpdate = options.onUpdate;
        this.photoDwellMs = options.photoDwellMs ?? DEFAULT_PHOTO_DWELL_MS;
        this.setPhotoDistances(options.photoDistances ?? []);
        const realtimeDurationMs = computeRealtimeDurationMs(options.geometry);
        this.mode = realtimeDurationMs ? "realtime" : "constant";
        const baseDurationMs =
            realtimeDurationMs ??
            options.constantDurationMs ??
            DEFAULT_CONSTANT_DURATION_MS;
        this.durationMs = Math.min(
            baseDurationMs,
            options.maxDurationMs ?? DEFAULT_MAX_DURATION_MS,
        );
        this.emitUpdate();
    }

    setPhotoDistances(distances: number[]) {
        this.photoDistances = distances
            .filter((distance) => Number.isFinite(distance))
            .sort((a, b) => a - b);
        this.syncVisitedPhotos();
    }

    setPhotoDwellMs(ms: number) {
        this.photoDwellMs = Math.max(500, ms);
        if (this.dwellPhotoIndex !== null) {
            this.dwellRemainingMs = Math.min(this.dwellRemainingMs, this.photoDwellMs);
        }
    }

    private syncVisitedPhotos() {
        const currentDistance = this.progress * this.geometry.totalDistance;
        this.visitedPhotoIndexes = new Set(
            this.photoDistances.flatMap((distance, index) =>
                distance < currentDistance - 0.5 ? [index] : [],
            ),
        );
        if (
            this.dwellPhotoIndex !== null &&
            !this.photoDistances[this.dwellPhotoIndex]
        ) {
            this.clearDwell();
        }
    }

    private clearDwell() {
        this.dwellPhotoIndex = null;
        this.dwellRemainingMs = 0;
        this.departElapsedMs = PHOTO_DEPART_MS;
    }

    private nextPhotoIndex(distance: number, excludeIndex: number | null = null) {
        return this.photoDistances.findIndex(
            (photoDistance, index) =>
                index !== excludeIndex &&
                !this.visitedPhotoIndexes.has(index) &&
                photoDistance >= distance - 0.5,
        );
    }

    private reachedPhotoIndex(distance: number, excludeIndex: number | null = null) {
        return this.photoDistances.findIndex(
            (photoDistance, index) =>
                index !== excludeIndex &&
                !this.visitedPhotoIndexes.has(index) &&
                distance >= photoDistance,
        );
    }

    private approachMeters() {
        return Math.max(12, this.metersPerMs() * PHOTO_APPROACH_MS);
    }

    private isPhotoClose(distance: number, photoIndex: number) {
        const remaining = this.photoDistances[photoIndex] - distance;
        return remaining <= this.approachMeters();
    }

    private metersPerMs() {
        if (this.durationMs <= 0 || this.geometry.totalDistance <= 0) {
            return 0;
        }
        return (this.geometry.totalDistance / this.durationMs) * this.speed;
    }

    private approachRatio() {
        return PHOTO_APPROACH_MIN_RATIO;
    }

    private holdRatio() {
        const cruise = this.metersPerMs();
        if (cruise <= 0) {
            return 0.04;
        }
        return clamp(
            PHOTO_HOLD_METERS / this.photoDwellMs / cruise,
            0.02,
            0.08,
        );
    }

    private clampToPhotoHold(distance: number) {
        if (this.dwellPhotoIndex === null || this.geometry.totalDistance <= 0) {
            return;
        }
        const photoDistance = this.photoDistances[this.dwellPhotoIndex];
        const maxDistance = photoDistance + PHOTO_HOLD_METERS;
        if (distance <= maxDistance) {
            return;
        }
        this.progress = clamp01(maxDistance / this.geometry.totalDistance);
        this.elapsedMs = this.progress * this.durationMs;
    }

    private speedMultiplier(distance: number) {
        const approach = this.approachRatio();
        if (this.dwellPhotoIndex !== null) {
            return this.holdRatio();
        }

        const nextIndex = this.nextPhotoIndex(distance);
        const approaching =
            nextIndex >= 0 && this.isPhotoClose(distance, nextIndex);

        if (approaching) {
            const remaining = this.photoDistances[nextIndex] - distance;
            if (remaining <= 0) {
                return this.holdRatio();
            }
            const t = remaining / this.approachMeters();
            return approach + (1 - approach) * easeOutExpo(t);
        }

        if (this.departElapsedMs < PHOTO_DEPART_MS) {
            const t = this.departElapsedMs / PHOTO_DEPART_MS;
            return approach + (1 - approach) * easeOutExpo(t);
        }

        return 1;
    }

    private buildState(): RoutePlaybackState | null {
        const interpolation = interpolateAtDistance(
            this.geometry,
            this.geometry.totalDistance * this.progress,
        );
        if (!interpolation) {
            return null;
        }
        return {
            status: this.status,
            progress: this.progress,
            distance: interpolation.distance,
            totalDistance: this.geometry.totalDistance,
            bearing: interpolation.bearing,
            position: interpolation.position as [number, number, number?],
            mode: this.mode,
            speed: this.speed,
            elapsedMs: this.elapsedMs,
            durationMs: this.durationMs,
            activePhotoDistance:
                this.dwellPhotoIndex !== null
                    ? this.photoDistances[this.dwellPhotoIndex] ?? null
                    : null,
        };
    }

    private emitUpdate() {
        const state = this.buildState();
        if (state) {
            this.onUpdate?.(state);
        }
    }

    private startDwell(index: number) {
        this.dwellPhotoIndex = index;
        this.dwellRemainingMs = this.photoDwellMs;
        this.departElapsedMs = 0;
        const photoDistance = this.photoDistances[index];
        if (this.geometry.totalDistance > 0) {
            this.progress = clamp01(photoDistance / this.geometry.totalDistance);
            this.elapsedMs = this.progress * this.durationMs;
        }
    }

    private tick = (now: number) => {
        if (this.status !== "playing") {
            return;
        }

        if (this.lastTickMs === null) {
            this.lastTickMs = now;
        }

        const dt = Math.min(48, Math.max(0, now - this.lastTickMs));
        this.lastTickMs = now;

        const currentDistance = this.progress * this.geometry.totalDistance;
        const localSpeed = this.speedMultiplier(currentDistance);

        this.elapsedMs = Math.max(0, this.elapsedMs + dt * this.speed * localSpeed);
        this.progress = clamp01(this.durationMs > 0 ? this.elapsedMs / this.durationMs : 1);
        this.clampToPhotoHold(this.progress * this.geometry.totalDistance);
        const nextDistance = this.progress * this.geometry.totalDistance;

        const arrivedIndex = this.reachedPhotoIndex(
            nextDistance,
            this.dwellPhotoIndex,
        );
        if (this.dwellPhotoIndex === null && arrivedIndex >= 0) {
            this.startDwell(arrivedIndex);
            this.departElapsedMs = PHOTO_DEPART_MS;
        }

        if (this.dwellPhotoIndex !== null) {
            this.dwellRemainingMs -= dt;
            if (this.dwellRemainingMs <= 0) {
                this.visitedPhotoIndexes.add(this.dwellPhotoIndex);
                const finishedIndex = this.dwellPhotoIndex;
                this.dwellPhotoIndex = null;
                this.dwellRemainingMs = 0;

                const chainedIndex = this.reachedPhotoIndex(nextDistance, finishedIndex);
                const upcomingIndex = this.nextPhotoIndex(nextDistance, finishedIndex);
                if (chainedIndex >= 0) {
                    this.startDwell(chainedIndex);
                } else if (
                    upcomingIndex >= 0 &&
                    this.isPhotoClose(nextDistance, upcomingIndex)
                ) {
                    this.departElapsedMs = PHOTO_DEPART_MS;
                } else {
                    this.departElapsedMs = 0;
                }
            }
        } else if (this.departElapsedMs < PHOTO_DEPART_MS) {
            const upcomingIndex = this.nextPhotoIndex(nextDistance);
            if (upcomingIndex >= 0 && this.isPhotoClose(nextDistance, upcomingIndex)) {
                this.departElapsedMs = PHOTO_DEPART_MS;
            } else {
                this.departElapsedMs = Math.min(
                    PHOTO_DEPART_MS,
                    this.departElapsedMs + dt,
                );
            }
        }

        this.emitUpdate();

        if (this.progress >= 1) {
            this.pause();
            return;
        }

        this.frameId = window.requestAnimationFrame(this.tick);
    };

    play() {
        if (this.status === "playing") {
            return;
        }
        this.status = "playing";
        this.lastTickMs = null;
        this.frameId = window.requestAnimationFrame(this.tick);
        this.emitUpdate();
    }

    pause() {
        this.status = "paused";
        this.lastTickMs = null;
        if (this.frameId !== null) {
            window.cancelAnimationFrame(this.frameId);
            this.frameId = null;
        }
        this.emitUpdate();
    }

    seek(progress: number) {
        this.progress = clamp01(progress);
        this.elapsedMs = this.progress * this.durationMs;
        this.lastTickMs = null;
        this.clearDwell();
        this.syncVisitedPhotos();
        this.emitUpdate();
    }

    setSpeed(speed: number) {
        const nextSpeed = Number.isFinite(speed) && speed > 0 ? speed : 1;
        const isPlaying = this.status === "playing";
        if (isPlaying) {
            this.pause();
        }
        this.speed = nextSpeed;
        if (isPlaying) {
            this.play();
        } else {
            this.emitUpdate();
        }
    }

    getState() {
        return this.buildState();
    }

    getMode() {
        return this.mode;
    }

    destroy() {
        this.pause();
        this.onUpdate = undefined;
    }
}
