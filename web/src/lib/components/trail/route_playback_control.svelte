<script lang="ts">
    import { _ } from "svelte-i18n";

    interface Props {
        visible?: boolean;
        playing?: boolean;
        progress?: number;
        mode?: "realtime" | "constant";
        photoDurationMs?: number;
        followCamera?: boolean;
        enable3d?: boolean;
        allow3d?: boolean;
        recording?: boolean;
        canRecord?: boolean;
        distanceLabel?: string;
        durationLabel?: string;
        onplaypause?: () => void;
        onprogresschange?: (progress: number) => void;
        onphotodurationchange?: (durationMs: number) => void;
        onfollowcamerachange?: (enabled: boolean) => void;
        onenable3dchange?: (enabled: boolean) => void;
        onrecordtoggle?: () => void;
    }

    let {
        visible = false,
        playing = false,
        progress = 0,
        mode = "constant",
        photoDurationMs = 1500,
        followCamera = true,
        enable3d = false,
        allow3d = true,
        recording = false,
        canRecord = false,
        distanceLabel = "",
        durationLabel = "",
        onplaypause,
        onprogresschange,
        onphotodurationchange,
        onfollowcamerachange,
        onenable3dchange,
        onrecordtoggle,
    }: Props = $props();

    const photoDurationOptions = [1000, 1500, 2000, 3000];

    function formatPhotoDuration(ms: number) {
        const seconds = ms / 1000;
        return Number.isInteger(seconds) ? `${seconds}s` : `${seconds.toFixed(1)}s`;
    }
</script>

{#if visible}
    <div class="route-playback-control bg-menu-background border-t border-black/10 dark:border-white/10 p-3 md:p-4">
        <div class="flex items-center gap-2">
            <button class="btn-secondary shrink-0" onclick={() => onplaypause?.()}>
                <i class="fa-solid fa-{playing ? 'pause' : 'play'} mr-2"></i>
                {playing ? $_("pause-route") : $_("play-route")}
            </button>
            {#if canRecord}
                <button
                    class="btn-secondary shrink-0"
                    class:text-red-600={recording}
                    onclick={() => onrecordtoggle?.()}
                >
                    <i class="fa-solid fa-{recording ? 'stop' : 'circle'} mr-2 {recording ? '' : 'text-red-600'}"></i>
                    {recording ? $_("stop-recording") : $_("record-animation")}
                </button>
            {/if}
            <input
                class="w-full"
                type="range"
                min="0"
                max="1"
                step="0.001"
                value={progress}
                oninput={(event) =>
                    onprogresschange?.(
                        Number((event.currentTarget as HTMLInputElement).value),
                    )}
            />
        </div>

        <div class="mt-3 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div class="text-sm text-gray-600 dark:text-gray-300">
                <span>{distanceLabel}</span>
                {#if durationLabel}
                    <span class="mx-2">•</span>
                    <span>{durationLabel}</span>
                {/if}
                <span class="mx-2">•</span>
                <span>{mode === "realtime" ? $_("realtime") : $_("constant-speed")}</span>
            </div>

            <div class="flex flex-wrap items-center gap-2">
                <label class="text-sm">{$_("photo-duration")}</label>
                <select
                    class="input py-1 px-2 min-w-20"
                    value={String(photoDurationMs)}
                    onchange={(event) =>
                        onphotodurationchange?.(
                            Number((event.currentTarget as HTMLSelectElement).value),
                        )}
                >
                    {#each photoDurationOptions as option}
                        <option value={String(option)}>{formatPhotoDuration(option)}</option>
                    {/each}
                </select>

                <label class="flex items-center gap-2 text-sm">
                    <input
                        type="checkbox"
                        checked={followCamera}
                        onchange={(event) =>
                            onfollowcamerachange?.(
                                (event.currentTarget as HTMLInputElement).checked,
                            )}
                    />
                    {$_("follow-camera")}
                </label>

                <label class="flex items-center gap-2 text-sm opacity-100" class:opacity-50={!allow3d}>
                    <input
                        type="checkbox"
                        checked={enable3d}
                        disabled={!allow3d}
                        onchange={(event) =>
                            onenable3dchange?.(
                                (event.currentTarget as HTMLInputElement).checked,
                            )}
                    />
                    {$_("3d-view")}
                </label>
            </div>
        </div>
    </div>
{/if}

<style>
    .route-playback-control {
        flex: 0 0 auto;
        width: 100%;
    }
</style>
