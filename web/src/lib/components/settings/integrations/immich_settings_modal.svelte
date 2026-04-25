<script lang="ts">
    import Modal from "$lib/components/base/modal.svelte";
    import Select, { type SelectItem } from "$lib/components/base/select.svelte";
    import TextField from "$lib/components/base/text_field.svelte";
    import { ImmichSchema } from "$lib/models/api/integration_schema";
    import type { ImmichIntegration, Integration } from "$lib/models/integration";
    import { validator } from "@felte/validator-zod";
    import { createForm } from "felte";
    import { _ } from "svelte-i18n";

    interface Props {
        integration?: Integration;
        onsave?: (immichIntegration: ImmichIntegration, materialize: "all" | "public" | false) => void | boolean | Promise<void | boolean>;
    }

    let { integration, onsave }: Props = $props();

    let modal: Modal;

    // Tracks whether we're waiting for the user to confirm materialization.
    let pendingForm: ImmichIntegration | null = $state(null);

    export function openModal() {
        errors.set({});
        pendingForm = null;
        modal.openModal();
    }

    const getInitialFormValues = () => ({
        url: integration?.immich?.url ?? "",
        apiKey: integration?.immich?.apiKey ?? "",
        timeWindowMinutes: integration?.immich?.timeWindowMinutes ?? 120,
        maxDistanceMeters: integration?.immich?.maxDistanceMeters ?? 150,
        maxWaypoints: integration?.immich?.maxWaypoints ?? 25,
        photoMode: integration?.immich?.photoMode ?? "copy",
        providers: integration?.immich?.providers ?? ["strava", "komoot", "hammerhead", "upload"],
        active: integration?.immich?.active ?? false,
    });

    const {
        form,
        errors,
    } = createForm({
        initialValues: getInitialFormValues(),
        extend: validator({
            schema: ImmichSchema,
        }),
        onSubmit: async (values) => {
            const currentActive = integration?.immich?.active;
            values.active = currentActive ?? values.active;
            const formValues = values as ImmichIntegration;

            const scope = materializeScope(integration?.immich?.photoMode, formValues.photoMode);
            if (scope !== null) {
                pendingForm = formValues;
                pendingScope = scope;
                return;
            }

            const saved = await onsave?.(formValues, false);
            if (saved === false) return;
            modal.closeModal();
        },
    });

    // pendingScope is set when the user needs to confirm whether to download existing assets.
    let pendingScope: "all" | "public" | null = $state(null);

    async function confirmMaterialize(materialize: "all" | "public" | false) {
        if (!pendingForm) return;
        const f = pendingForm;
        pendingForm = null;
        pendingScope = null;
        const saved = await onsave?.(f, materialize);
        if (saved === false) return;
        modal.closeModal();
    }

    function materializeScope(oldMode: string | undefined, newMode: string): "all" | "public" | null {
        if (!oldMode || !integration?.immich) return null;
        if (newMode === "copy" && (oldMode === "link_private" || oldMode === "link_public")) return "all";
        if (newMode === "link_private" && oldMode === "link_public") return "public";
        return null;
    }

    const providerOptions = [
        { value: "strava", label: "Strava" },
        { value: "komoot", label: "komoot" },
        { value: "hammerhead", label: "Hammerhead" },
        { value: "upload", label: $_("upload-gpx") },
    ];

    const photoModeOptions: SelectItem[] = [
        { value: "copy", text: $_("immich-photo-mode-copy") },
        { value: "link_private", text: $_("immich-photo-mode-link-private") },
        { value: "link_public", text: $_("immich-photo-mode-link-public") },
    ];
</script>

<Modal
    id="immich-settings-modal"
    size="md:min-w-lg"
    title={"Immich " + $_("settings")}
    bind:this={modal}
>
    {#snippet content()}
        {#if pendingForm}
            <div class="space-y-2">
                <p class="font-medium">{$_("immich-materialize-confirm-title")}</p>
                <p class="text-sm text-gray-500">{$_("immich-materialize-confirm-body")}</p>
            </div>
        {:else}
            <form class="space-y-4" id="immich-settings-form" use:form>
                <TextField
                    label={$_("immich-url-label")}
                    name="url"
                    placeholder="https://immich.example.com"
                    error={$errors.url}
                ></TextField>
                <TextField
                    label={$_("immich-api-key-label")}
                    name="apiKey"
                    type="password"
                    placeholder={integration?.immich
                        ? `(${$_("unchanged")})`
                        : "eyJhbGciOiJIUzI..."}
                    error={$errors.apiKey}
                ></TextField>
                <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <TextField
                        label={$_("immich-time-window-minutes")}
                        name="timeWindowMinutes"
                        error={$errors.timeWindowMinutes}
                    ></TextField>
                    <TextField
                        label={$_("immich-distance-threshold-meters")}
                        name="maxDistanceMeters"
                        error={$errors.maxDistanceMeters}
                    ></TextField>
                    <TextField
                        label={$_("immich-max-waypoints")}
                        name="maxWaypoints"
                        error={$errors.maxWaypoints}
                    ></TextField>
                </div>
                <Select
                    label={$_("immich-photo-mode-label")}
                    name="photoMode"
                    items={photoModeOptions}
                    value={integration?.immich?.photoMode ?? "copy"}
                ></Select>
                <fieldset class="space-y-2">
                    <p class="text-xs text-gray-500">
                        {$_("immich-provider-label")}
                    </p>
                    <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
                        {#each providerOptions as provider}
                            <label class="flex items-center gap-2 text-sm">
                                <input
                                    class="checkbox"
                                    type="checkbox"
                                    name="providers"
                                    value={provider.value}
                                />
                                <span>{provider.label}</span>
                            </label>
                        {/each}
                    </div>
                </fieldset>
                <p class="text-xs text-gray-500">
                    {$_("immich-settings-hint")}
                </p>
            </form>
        {/if}
    {/snippet}
    {#snippet footer()}
        <div class="flex items-center gap-4">
            {#if pendingForm}
                <button class="btn-secondary" onclick={() => confirmMaterialize(false)}>
                    {$_("immich-materialize-keep-links")}
                </button>
                <button class="btn-primary" onclick={() => confirmMaterialize(pendingScope!)}>

                    {$_("immich-materialize-download-now")}
                </button>
            {:else}
                <button class="btn-secondary" onclick={() => modal.closeModal()}>
                    {$_("cancel")}
                </button>
                <button
                    class="btn-primary"
                    form="immich-settings-form"
                    type="submit"
                >
                    {$_("save")}
                </button>
            {/if}
        </div>
    {/snippet}
</Modal>
