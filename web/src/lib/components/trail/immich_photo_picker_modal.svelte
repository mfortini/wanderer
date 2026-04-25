<script lang="ts">
    import Modal from "$lib/components/base/modal.svelte";
    import Button from "$lib/components/base/button.svelte";
    import { _ } from "svelte-i18n";

    interface Candidate {
        assetId: string;
        originalFileName: string;
        takenAt: string;
        lat: number;
        lon: number;
        distance: number;
        distanceFromStart: number;
        city: string;
        country: string;
    }

    interface CandidatesResult {
        hasTimestamps: boolean;
        candidates: Candidate[];
        hasMore: boolean;
        takenAfter: string;
    }

    interface Props {
        lat: number;
        lon: number;
        onselect?: (candidates: Candidate[]) => void;
    }

    let { lat, lon, onselect }: Props = $props();

    let modal: Modal;
    let result = $state<CandidatesResult | null>(null);
    let selected: Set<string> = $state(new Set());
    let loading = $state(false);
    let loadingMore = $state(false);
    let yearsBack = $state(1);
    let fetchError: string | null = $state(null);
    let exhausted = $state(false);
    let radiusDoubled = $state(false);

    export function openModal() {
        result = null;
        selected = new Set();
        yearsBack = 1;
        fetchError = null;
        exhausted = false;
        radiusDoubled = false;
        modal.openModal();
        fetchInitial();
    }

    async function doFetch(doubleRadius?: boolean): Promise<void> {
        const body: Record<string, unknown> = { lat, lon, yearsBack };
        if (doubleRadius) body.doubleRadius = true;
        const r = await fetch("/api/v1/integration/immich/candidates", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body),
        });
        if (!r.ok) {
            const data = await r.json();
            throw new Error(data.message ?? $_("error-generic"));
        }
        result = await r.json();
    }

    async function fetchInitial() {
        loading = true;
        fetchError = null;
        try {
            await doFetch();
        } catch (e: any) {
            fetchError = e.message;
        } finally {
            loading = false;
        }
    }

    async function expandRadius() {
        loading = true;
        fetchError = null;
        try {
            await doFetch(true);
            radiusDoubled = true;
        } catch (e: any) {
            fetchError = e.message;
        } finally {
            loading = false;
        }
    }

    async function loadOlderPhotos() {
        const prevCount = result?.candidates?.length ?? 0;
        loadingMore = true;
        exhausted = false;
        fetchError = null;
        try {
            while (true) {
                yearsBack++;
                await doFetch(radiusDoubled);
                const newCount = result?.candidates?.length ?? 0;
                if (newCount > prevCount) break;
                if (!result?.hasMore) {
                    exhausted = true;
                    break;
                }
            }
        } catch (e: any) {
            fetchError = e.message;
        } finally {
            loadingMore = false;
        }
    }

    function toggleCandidate(assetId: string) {
        const next = new Set(selected);
        if (next.has(assetId)) {
            next.delete(assetId);
        } else {
            next.add(assetId);
        }
        selected = next;
    }

    function toggleAll() {
        if (!result) return;
        if (selected.size === result.candidates.length) {
            selected = new Set();
        } else {
            selected = new Set(result.candidates.map((c) => c.assetId));
        }
    }

    function confirmSelection() {
        if (selected.size === 0) return;
        const chosen = (result?.candidates ?? []).filter((c) => selected.has(c.assetId));
        onselect?.(chosen);
        modal.closeModal();
    }

    function formatDate(dateStr: string): string {
        if (!dateStr) return "";
        try {
            return new Date(dateStr).toLocaleDateString();
        } catch {
            return dateStr;
        }
    }

    function locationLabel(c: Candidate): string {
        return [c.city, c.country].filter(Boolean).join(", ");
    }

    const allSelected = $derived(
        (result?.candidates?.length ?? 0) > 0 &&
            selected.size === (result?.candidates?.length ?? 0),
    );
</script>

<Modal
    bind:this={modal}
    id="immich-photo-picker-modal"
    title={$_("import-from-immich")}
    size="max-w-2xl"
>
    {#snippet content()}
        {#if loading}
            <div class="flex items-center justify-center py-12 text-gray-400">
                <i class="fa fa-spinner fa-spin text-2xl mr-3"></i>
                {$_("loading")}...
            </div>
        {:else if fetchError}
            <p class="text-red-500 py-4">{fetchError}</p>
        {:else if result?.candidates.length === 0}
            <p class="text-gray-500 py-4 text-center">{$_("no-photos-found")}</p>
            {#if !radiusDoubled}
                <div class="flex justify-center pb-4">
                    <Button onclick={expandRadius} secondary={true}>
                        {$_("expand-search-radius")}
                    </Button>
                </div>
            {/if}
        {:else if result}
            <div class="flex items-center justify-between mb-2">
                <label
                    class="flex items-center gap-2 text-sm text-gray-600 cursor-pointer select-none"
                >
                    <input
                        type="checkbox"
                        checked={allSelected}
                        onchange={toggleAll}
                        class="w-4 h-4"
                    />
                    {$_("select-all")}
                </label>
                <span class="text-sm text-gray-400">
                    {result.candidates.length}
                    {$_("photos-found")}
                </span>
            </div>
            <div class="space-y-1 max-h-[28rem] overflow-y-auto pr-1">
                {#each result.candidates as candidate (candidate.assetId)}
                    <label
                        class="flex items-center gap-3 p-2 rounded-lg hover:bg-input-background cursor-pointer"
                    >
                        <input
                            type="checkbox"
                            checked={selected.has(candidate.assetId)}
                            onchange={() => toggleCandidate(candidate.assetId)}
                            class="w-4 h-4 shrink-0"
                        />
                        <img
                            src="/api/v1/integration/immich/thumbnail/{candidate.assetId}"
                            alt={candidate.originalFileName}
                            class="w-16 h-16 object-cover rounded shrink-0 bg-gray-100"
                        />
                        <div class="min-w-0 flex-1">
                            <p class="font-medium truncate text-sm">
                                {candidate.originalFileName}
                            </p>
                            <p class="text-xs text-gray-500 mt-0.5">
                                {formatDate(candidate.takenAt)}
                                {#if locationLabel(candidate)}
                                    &middot; {locationLabel(candidate)}
                                {/if}
                            </p>
                            <p class="text-xs text-gray-400 mt-0.5">
                                {Math.round(candidate.distance)}m {$_("from-waypoint")}
                            </p>
                        </div>
                    </label>
                {/each}
            </div>
        {/if}
        <div class="mt-4 flex justify-center">
            {#if loadingMore}
                <div class="flex items-center gap-2 text-sm text-gray-400">
                    <i class="fa fa-spinner fa-spin"></i>
                    {$_("loading")}...
                </div>
            {:else if result?.hasMore && (result?.candidates?.length ?? 0) > 0}
                <Button onclick={loadOlderPhotos} secondary={true}>
                    {$_("load-older-photos")}
                </Button>
            {:else if exhausted}
                <p class="text-sm text-gray-500">{$_("no-more-photos-in-immich")}</p>
            {/if}
        </div>
    {/snippet}
    {#snippet footer()}
        <div class="flex items-center justify-between w-full">
            <span class="text-sm text-gray-500">
                {selected.size} {$_("selected")}
            </span>
            <div class="flex gap-2">
                <Button onclick={() => modal.closeModal()} secondary={true}
                    >{$_("cancel")}</Button
                >
                <Button
                    primary={true}
                    onclick={confirmSelection}
                    disabled={selected.size === 0}
                >
                    {$_("import")}
                </Button>
            </div>
        </div>
    {/snippet}
</Modal>
