<script lang="ts">
    import { type Snippet } from "svelte";

    import { WaypointCreateSchema } from "$lib/models/api/waypoint_schema";
    import { convertDMSToDD } from "$lib/models/gpx/utils";
    import { show_toast } from "$lib/stores/toast_store.svelte";
    import { waypoint } from "$lib/stores/waypoint_store";
    import { cloneDeep } from "$lib/util/deep_util";
    import { icons } from "$lib/util/icon_util";
    import EXIF from "$lib/vendor/exif-js/exif";
    import { validator } from "@felte/validator-zod";
    import { createForm } from "felte";
    import { _ } from "svelte-i18n";
    import { z } from "zod";
    import Combobox from "../base/combobox.svelte";
    import Modal from "../base/modal.svelte";
    import TextField from "../base/text_field.svelte";
    import Textarea from "../base/textarea.svelte";
    import PhotoPicker from "../trail/photo_picker.svelte";
    import ImmichPhotoPickerModal from "../trail/immich_photo_picker_modal.svelte";
    import type { Waypoint } from "$lib/models/waypoint";

    interface Candidate {
        assetId: string;
        lat: number;
        lon: number;
        originalFileName: string;
        takenAt: string;
        city: string;
        country: string;
        distance: number;
        distanceFromStart: number;
    }

    interface Props {
        children?: Snippet<[any]>;
        onsave?: (waypoint: Waypoint) => void;
        immichActive?: boolean;
    }

    let { children, onsave, immichActive }: Props = $props();

    let modal: Modal;
    let immichPickerModal: ImmichPhotoPickerModal = $state()!;
    let pendingCandidates: Candidate[] = $state([]);

    const ClientWaypointCreateSchema = WaypointCreateSchema.extend({
        photos: z.array(z.string()).default([]),
        _photos: z.array(z.instanceof(File)).optional(),
    });

    const { form, errors, data, setFields } = createForm<
        z.infer<typeof ClientWaypointCreateSchema>
    >({
        initialValues: $waypoint,
        extend: validator({ schema: ClientWaypointCreateSchema }),
        onSubmit: async (form) => {
            const wp = {
                ...(form as Waypoint),
                photos: $data.photos ?? [],
                _photos: $data._photos ?? [],
            } as Waypoint;
            if (pendingCandidates.length > 0) {
                wp._immichCandidates = pendingCandidates.map((c) => ({
                    assetId: c.assetId,
                    lat: c.lat,
                    lon: c.lon,
                    originalFileName: c.originalFileName,
                }));
            }
            onsave?.(wp);
            modal.closeModal!();
        },
        transform: (values: unknown) => {
            const v = values as any;
            return {
                ...v,
                lat: parseFloat(v.lat),
                lon: parseFloat(v.lon),
            };
        },
    });

    $effect(() => {
        setFields(cloneDeep($waypoint));
        pendingCandidates = [];
    });

    let filteredIcons = $derived(
        ($data.icon?.length ?? 0) > 2
            ? icons
                  .filter((i) =>
                      i
                          .replaceAll("-", " ")
                          .includes($data.icon?.toLowerCase() ?? ""),
                  )
                  .map((i) => ({
                      text: i.replaceAll("-", " "),
                      value: i,
                      icon: i,
                  }))
            : [],
    );

    function getCoordinatesFromPhoto(src: string) {
        EXIF.getData({ src: src }, function (p) {
            const lat = EXIF.getTag(p, "GPSLatitude");
            const latDir = EXIF.getTag(p, "GPSLatitudeRef");
            const lon = EXIF.getTag(p, "GPSLongitude");
            const lonDir = EXIF.getTag(p, "GPSLongitudeRef");

            if (lat && lon) {
                setFields("lat", convertDMSToDD(lat, latDir));
                setFields("lon", convertDMSToDD(lon, lonDir));
            } else {
                show_toast({
                    text: $_('no-gps-data-in-image'),
                    icon: "close",
                    type: "error",
                });
            }
        });
    }

    function onImmichSelect(candidates: Candidate[]) {
        pendingCandidates = [...pendingCandidates, ...candidates.filter(
            (c) => !pendingCandidates.some((p) => p.assetId === c.assetId)
        )];
    }

    function removePendingCandidate(assetId: string) {
        pendingCandidates = pendingCandidates.filter((c) => c.assetId !== assetId);
    }

    const hasCoordinates = $derived(
        !isNaN(parseFloat(String($data.lat))) && !isNaN(parseFloat(String($data.lon)))
    );

    export function openModal() {
        modal.openModal();
    }

    const children_render = $derived(children);
</script>

<Modal
    id="waypoint-modal"
    size="md:min-w-2xl"
    title={$data.id ? $_("edit-waypoint") : $_("add-waypoint")}
    bind:this={modal}
>
    {#snippet children({ openModal })}
        {@render children_render?.({ openModal })}
    {/snippet}
    {#snippet content()}
        <form id="waypoint-form" class="modal-content space-y-4" use:form>
            <div class="flex gap-4">
                <div class="basis-2/3">
                    <TextField
                        name="name"
                        label={$_("name")}
                        error={$errors.name}
                    ></TextField>
                </div>

                <Combobox
                    name="icon"
                    icon={$data.icon}
                    bind:value={$data.icon}
                    items={filteredIcons}
                    label={$_("icon")}
                ></Combobox>
            </div>

            <Textarea
                name="description"
                label={$_("description")}
                error={$errors.description}
            ></Textarea>
            <div class="flex gap-4">
                <TextField name="lat" label={$_("latitude")} error={$errors.lat}
                ></TextField>
                <TextField
                    name="lon"
                    label={$_("longitude")}
                    error={$errors.lat}
                ></TextField>
            </div>
            <div>
                <label for="waypoint-photo-input" class="text-sm font-medium pb-1 block">
                    {$_("photos")}
                </label>
                <PhotoPicker
                    id="waypoint-photo-input"
                    parent={$data}
                    onexif={(src) => getCoordinatesFromPhoto(src)}
                    bind:photos={$data.photos}
                    bind:photoFiles={$data._photos}
                    showThumbnailControls={false}
                    showExifControls={true}
                    onimmich={immichActive && hasCoordinates ? () => immichPickerModal.openModal() : undefined}
                    immichPreviews={pendingCandidates.map(c => ({ assetId: c.assetId, filename: c.originalFileName }))}
                    onimmichdelete={removePendingCandidate}
                ></PhotoPicker>
            </div>
        </form>
    {/snippet}
    {#snippet footer()}
        <div class="flex items-center gap-4">
            <button class="btn-secondary" onclick={() => modal.closeModal()}
                >{$_("cancel")}</button
            >
            <button class="btn-primary" type="submit" form="waypoint-form"
                >{$_("save")}</button
            >
        </div>
    {/snippet}
</Modal>

{#if immichActive}
    <ImmichPhotoPickerModal
        bind:this={immichPickerModal}
        lat={parseFloat(String($data.lat)) || 0}
        lon={parseFloat(String($data.lon)) || 0}
        onselect={onImmichSelect}
    />
{/if}
