<script lang="ts">
    import Modal from "$lib/components/base/modal.svelte";
    import Button from "$lib/components/base/button.svelte";
    import type { Waypoint } from "$lib/models/waypoint";
    import { formatDistance } from "$lib/util/format_util";
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

    interface ImportResult {
        assetId: string;
        waypoint: Waypoint;
    }

    interface Props {
        trailId: string;
        onsave?: (waypoints: Waypoint[]) => void;
    }

    let { trailId, onsave }: Props = $props();

    let modal: Modal;
    let result = $state<CandidatesResult | null>(null);
    let selected: Set<string> = $state(new Set());
    let loading = $state(false);
    let loadingMore = $state(false);
    let importing = $state(false);
    let yearsBack = $state(1);
    let fetchError: string | null = $state(null);
    let importError: string | null = $state(null);

    export function openModal() {
        result = null;
        selected = new Set();
        yearsBack = 1;
        fetchError = null;
        importError = null;
        modal.openModal();
        fetchCandidates(false);
    }

    async function fetchCandidates(more: boolean) {
        if (more) {
            yearsBack++;
            loadingMore = true;
        } else {
            loading = true;
        }
        fetchError = null;
        try {
            const r = await fetch("/api/v1/integration/immich/candidates", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ trailId, yearsBack }),
            });
            if (!r.ok) {
                const data = await r.json();
                throw new Error(data.message ?? $_("error-generic"));
            }
            const fresh: CandidatesResult = await r.json();
            // merge with existing selection — new window includes all previous results
            result = fresh;
        } catch (e: any) {
            fetchError = e.message;
        } finally {
            loading = false;
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

    async function importSelected() {
        if (selected.size === 0) return;
        importing = true;
        importError = null;
        try {
            const r = await fetch("/api/v1/integration/immich/import", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ trailId, assetIds: [...selected] }),
            });
            if (!r.ok) {
                const data = await r.json();
                throw new Error(data.message ?? $_("error-generic"));
            }
            const selectedCandidates = (result?.candidates ?? []).filter((c) =>
                selected.has(c.assetId),
            );
            const candidatesByAssetId = new Map(
                selectedCandidates.map((candidate) => [candidate.assetId, candidate]),
            );
            const responsePayload: ImportResult[] = await r.json();
            const waypoints: Waypoint[] = responsePayload.map((importResult) => {
                const wp = importResult.waypoint;
                const candidate = candidatesByAssetId.get(importResult.assetId);
                if (!candidate) {
                    return wp;
                }
                return {
                    ...wp,
                    _immichCandidates: [{
                        assetId: candidate.assetId,
                        lat: candidate.lat,
                        lon: candidate.lon,
                        originalFileName: candidate.originalFileName,
                    }],
                };
            });
            onsave?.(waypoints);
            modal.closeModal();
        } catch (e: any) {
            importError = e.message;
        } finally {
            importing = false;
        }
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
        result !== null && result.candidates.length > 0 && selected.size === result.candidates.length,
    );
</script>

<Modal bind:this={modal} id="immich-waypoint-modal" title={$_("import-from-immich")} size="max-w-2xl">
    {#snippet content()}
        {#if loading}
            <div class="flex items-center justify-center py-12 text-gray-400">
                <i class="fa fa-spinner fa-spin text-2xl mr-3"></i>
                {$_("loading")}...
            </div>
        {:else if fetchError}
            <p class="text-red-500 py-4">{fetchError}</p>
        {:else if result?.candidates.length === 0}
            <p class="text-gray-500 py-8 text-center">{$_("no-photos-found")}</p>
        {:else if result}
            <div class="flex items-center justify-between mb-2">
                <label class="flex items-center gap-2 text-sm text-gray-600 cursor-pointer select-none">
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
                            <p class="font-medium truncate text-sm">{candidate.originalFileName}</p>
                            <p class="text-xs text-gray-500 mt-0.5">
                                {formatDate(candidate.takenAt)}
                                {#if locationLabel(candidate)}
                                    &middot; {locationLabel(candidate)}
                                {/if}
                            </p>
                            <p class="text-xs text-gray-400 mt-0.5">
                                {Math.round(candidate.distance)}m {$_("from-track")}
                                &middot; {formatDistance(candidate.distanceFromStart)} {$_("from-start")}
                            </p>
                        </div>
                    </label>
                {/each}
            </div>
            {#if result.hasMore}
                <div class="mt-4 flex justify-center">
                    <Button onclick={() => fetchCandidates(true)} secondary={true} loading={loadingMore}>
                        {$_("load-older-photos")} ({yearsBack + 1} {$_("years")})
                    </Button>
                </div>
            {/if}
            {#if importError}
                <p class="text-red-500 text-sm mt-3">{importError}</p>
            {/if}
        {/if}
    {/snippet}
    {#snippet footer()}
        <div class="flex items-center justify-between w-full">
            <span class="text-sm text-gray-500">
                {selected.size} {$_("selected")}
            </span>
            <div class="flex gap-2">
                <Button onclick={() => modal.closeModal()} secondary={true}>{$_("cancel")}</Button>
                <Button
                    onclick={importSelected}
                    primary={true}
                    loading={importing}
                    disabled={selected.size === 0 || importing}
                >
                    {$_("import")}
                </Button>
            </div>
        </div>
    {/snippet}
</Modal>
