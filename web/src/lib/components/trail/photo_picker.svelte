<script lang="ts">
    import { show_toast } from "$lib/stores/toast_store.svelte";
    import { getFileURL, readAsDataURLAsync } from "$lib/util/file_util";
    import { _ } from "svelte-i18n";
    import PhotoCard from "../photo_card.svelte";

    interface ImmichPreview {
        assetId: string;
        filename: string;
    }

    interface Props {
        id: string;
        photos: string[];
        photoFiles: File[] | undefined;
        parent: { [key: string]: any };
        thumbnail?: number;
        showThumbnailControls?: boolean;
        showExifControls?: boolean;
        maxSizeBytes?: number;
        onexif?: (src: string) => void;
        onimmich?: () => void;
        immichPreviews?: ImmichPreview[];
        onimmichdelete?: (assetId: string) => void;
    }

    let {
        id,
        photos = $bindable(),
        photoFiles = $bindable(),
        parent,
        thumbnail = $bindable(0),
        showThumbnailControls = true,
        showExifControls = false,
        maxSizeBytes = 20971520,
        onexif,
        onimmich,
        immichPreviews = [],
        onimmichdelete,
    }: Props = $props();

    let photoPreviews: string[] = $state([]);
    let showSourceMenu = $state(false);

    $effect(() => fetchPhotos(photoFiles ?? []));

    function fetchPhotos(photos: File[]) {
        Promise.all(
            photos.map(async (f) => {
                return await readAsDataURLAsync(f);
            }),
        ).then((v) => {
            photoPreviews = v;
        });
    }

    let offerUpload: boolean = $state(false);

    function handlePhotoDragOver(e: DragEvent) {
        e.preventDefault();
        offerUpload = true;
    }

    function handlePhotoDragLeave() {
        offerUpload = false;
    }

    function handlePhotoDrop(e: DragEvent) {
        e.preventDefault();
        offerUpload = false;
        handlePhotoSelection(e.dataTransfer?.files);
    }

    function openPhotoBrowser() {
        document.getElementById(`${id}-photo-input`)!.click();
    }

    function handlePlusClick() {
        if (onimmich) {
            showSourceMenu = !showSourceMenu;
        } else {
            openPhotoBrowser();
        }
    }

    function selectLocal() {
        showSourceMenu = false;
        openPhotoBrowser();
    }

    function selectImmich() {
        showSourceMenu = false;
        onimmich?.();
    }

    async function handlePhotoSelection(files?: FileList | null) {
        if (!files) {
            files = (
                document.getElementById(`${id}-photo-input`) as HTMLInputElement
            ).files;
        }

        if (!files) {
            return;
        }

        for (const file of files) {
            if (file.size > maxSizeBytes) {
                show_toast({
                    type: "error",
                    text: $_("file-too-big", {
                        values: { file: file.name, size: "20 MB" },
                    }),
                    icon: "close",
                });
                continue;
            }
            let photoFile = file;
            if (
                !file.type.startsWith("image") &&
                !["video/mp4", "video/ogg", "video/webm"].includes(file.type)
            ) {
                continue;
            } else if (file.type === "image/heic") {
                const heic2any = (await import("heic2any")).default;
                photoFile = new File(
                    [
                        (await heic2any({
                            blob: file,
                            toType: "image/jpeg",
                        })) as Blob,
                    ],
                    file.name,
                );
            }
            if (!photoFiles) {
                photoFiles = [];
            }
            photoFiles = [...photoFiles, photoFile];
        }
    }

    function makePhotoThumbnail(index: number) {
        thumbnail = index;
    }

    function handlePhotoDelete(index: number) {
        if (thumbnail == index) {
            thumbnail = 0;
        }

        if (index >= photos.length) {
            if (!photoFiles) {
                photoFiles = [];
            }
            const adjustedIndex = index - photos.length;
            photoFiles.splice(adjustedIndex, 1);
            photoPreviews.splice(adjustedIndex, 1);

            photoPreviews = [...photoPreviews];
        } else {
            photos.splice(index, 1);
            photos = [...photos];
        }
    }
</script>

<div
    class="flex gap-x-4 max-w-full shrink-0 rounded-xl {offerUpload
        ? 'outline-dashed outline-input-border'
        : ''}"
    role="dialog"
    tabindex="0"
    ondragover={handlePhotoDragOver}
    ondragleave={handlePhotoDragLeave}
    ondrop={handlePhotoDrop}
>
    <div class="relative shrink-0 grow-0 basis-auto">
        <button
            aria-label="Open photo browser"
            class="btn-secondary h-32 w-32"
            type="button"
            onclick={handlePlusClick}
        >
            <i class="fa fa-plus"></i>
            {#if onimmich}
                <i class="fa fa-chevron-down text-[10px] absolute bottom-2 right-2 opacity-40"></i>
            {/if}
        </button>
        {#if showSourceMenu}
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <div
                class="fixed inset-0 z-[9]"
                onclick={() => (showSourceMenu = false)}
            ></div>
            <div
                class="absolute top-full left-0 mt-1 bg-menu-background border border-input-border rounded-lg shadow-lg z-[10] min-w-[130px] overflow-hidden"
            >
                <button
                    type="button"
                    class="w-full text-left px-3 py-2 text-sm hover:bg-menu-item-background-hover flex items-center gap-2"
                    onclick={selectLocal}
                >
                    <i class="fa fa-folder-open w-4 text-center"></i>
                    {$_("local")}
                </button>
                <button
                    type="button"
                    class="w-full text-left px-3 py-2 text-sm hover:bg-menu-item-background-hover flex items-center gap-2 border-t border-input-border"
                    onclick={selectImmich}
                >
                    <img src="/immich.svg" alt="Immich" class="w-4 h-4" />
                    Immich
                </button>
            </div>
        {/if}
    </div>
    <input
        type="file"
        id="{id}-photo-input"
        accept="image/*,video/mp4"
        multiple={true}
        style="display: none;"
        onchange={() => handlePhotoSelection()}
    />
    <div class="flex overflow-x-auto gap-x-3 w-full">
        {#each (photos ?? []).concat(photoPreviews) as photo, i}
            <div class="shrink-0 grow-0 basis-auto">
                <PhotoCard
                    src={i >= photos.length ? photo : getFileURL(parent, photo)}
                    ondelete={() => handlePhotoDelete(i)}
                    isThumbnail={thumbnail === i}
                    onthumbnail={() => makePhotoThumbnail(i)}
                    {onexif}
                    {showThumbnailControls}
                    {showExifControls}
                ></PhotoCard>
            </div>
        {/each}
        {#each immichPreviews as preview (preview.assetId)}
            <div class="relative shrink-0 grow-0 basis-auto">
                <PhotoCard
                    src="/api/v1/integration/immich/thumbnail/{preview.assetId}"
                    ondelete={() => onimmichdelete?.(preview.assetId)}
                    showThumbnailControls={false}
                    showExifControls={false}
                ></PhotoCard>
                <div
                    class="absolute bottom-1 left-1 bg-black/60 rounded px-1 py-0.5 flex items-center pointer-events-none"
                >
                    <img src="/immich.svg" alt="Immich" class="w-3 h-3" />
                </div>
            </div>
        {/each}
    </div>
</div>
