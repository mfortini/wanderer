<script lang="ts">
    import { page } from "$app/state";
    import HammerheadSettingsModal from "$lib/components/settings/integrations/hammerhead_settings_modal.svelte";
    import ImmichSettingsModal from "$lib/components/settings/integrations/immich_settings_modal.svelte";
    import IntegrationCard from "$lib/components/settings/integrations/integration_card.svelte";
    import KomootSettingsModal from "$lib/components/settings/integrations/komoot_settings_modal.svelte";
    import StravaSettingsModal from "$lib/components/settings/integrations/strava_settings_modal.svelte";
    import {
        Integration,
        type HammerheadIntegration,
        type ImmichIntegration,
        type KomootIntegration,
        type StravaIntegration,
    } from "$lib/models/integration.js";
    import {
        integrations_create,
        integrations_update,
    } from "$lib/stores/integration_store.js";
    import { show_toast } from "$lib/stores/toast_store.svelte.js";
    import { untrack } from "svelte";
    import { _ } from "svelte-i18n";
    import hammerheadLogoWhite from "$lib/assets/svgs/logos/hammerhead_white.svg";
    import hammerheadLogoDark from "$lib/assets/svgs/logos/hammerhead_dark.svg";
    import { theme } from "$lib/stores/theme_store";

    let { data } = $props();

    let integration = $state(untrack(() => data.integration));

    const scope = "read_all,activity:read_all";
    const redirectUri = page.url.href + "/callback/strava";

    let stravaSettingsModal: StravaSettingsModal;
    let stravaToggleValue: boolean = $derived(
        integration?.strava?.active || false,
    );

    let komootSettingsModal: KomootSettingsModal;
    let komootToggleValue: boolean = $state(
        untrack(() => data.integration?.komoot?.active ?? false),
    );

    let hammerheadSettingsModal: HammerheadSettingsModal;
    let hammerheadToggleValue: boolean = $state(
        untrack(() => data.integration?.hammerhead?.active ?? false),
    );

    let immichSettingsModal: ImmichSettingsModal;
    let immichToggleValue: boolean = $state(
        untrack(() => data.integration?.immich?.active ?? false),
    );

    async function onSettingsSave(
        form: StravaIntegration | KomootIntegration | HammerheadIntegration | ImmichIntegration,
        key: "strava" | "komoot" | "hammerhead" | "immich",
        materialize: boolean = false,
    ): Promise<boolean> {
        try {
            if (key == "immich") {
                let verified = await verifyImmich(form as ImmichIntegration);
                if (!verified) {
                    return false;
                }
            }

            if (integration) {
                integration[key] = form as any;
                integration = await integrations_update(integration);
            } else {
                const newIntegration: Integration = {
                    user: "",
                    [key]: form,
                };
                integration = await integrations_create(newIntegration);
            }

            if (key == "komoot" || key == "hammerhead") {
                let verified = await verifyLogin(key);
                if (!verified) {
                    return false;
                }
            }

            if (key == "immich" && materialize) {
                fetch("/api/v1/integration/immich/materialize-all", { method: "POST" }).catch(
                    () => {},
                );
            }

            show_toast({
                text: $_("settings-saved"),
                icon: "check",
                type: "success",
            });
            return true;
        } catch (e) {
            show_toast({
                text: $_("error-setting-up-integration", {
                    values: { provider: key },
                }),
                icon: "close",
                type: "error",
            });
            return false;
        }
    }

    async function onStravaToggle(value: boolean) {
        if (!integration?.strava) {
            return;
        }
        if (value) {
            const authUrl = `https://www.strava.com/oauth/authorize?client_id=${integration.strava.clientId}&response_type=code&redirect_uri=${redirectUri}&scope=${scope}&approval_prompt=auto`;
            window.location.href = authUrl;
        } else {
            const deauthUrl = `https://www.strava.com/oauth/deauthorize`;

            await fetch(deauthUrl, {
                method: "POST",
                headers: {
                    Authorization: `Bearer ${integration.strava.accessToken}`,
                },
            });

            integration.strava = {
                clientId: integration.strava.clientId,
                clientSecret: integration.strava.clientSecret,
                routes: integration.strava.routes,
                activities: integration.strava.activities,
                privacy: integration.strava.privacy,
                accessToken: undefined,
                refreshToken: undefined,
                expiresAt: undefined,
                active: false,
            };
            try {
                integration = await integrations_update(integration);
            } catch (e) {
                show_toast({
                    text: $_("error-disabling-strava-integration"),
                    icon: "close",
                    type: "error",
                });
            }

            show_toast({
                text: "Strava " + $_("integration-disabled"),
                icon: "check",
                type: "success",
            });
        }
    }

    async function onKomootToggle(value: boolean) {
        if (!integration?.komoot) {
            return;
        }
        if (value) {
            let verified = await verifyLogin("komoot");
            if (!verified) {
                return;
            }

            integration.komoot.active = true;
        } else {
            integration.komoot.active = false;
        }
        try {
            integration = await integrations_update(integration);
        } catch (e) {
            show_toast({
                text: $_("error-updating-komoot-integration"),
                icon: "close",
                type: "error",
            });
            return;
        }

        show_toast({
            text:
                "komoot " + $_(`integration-${value ? "enabled" : "disabled"}`),
            icon: "check",
            type: "success",
        });
    }

    async function verifyLogin(integrationName: string): Promise<boolean> {
        try {
            const r = await fetch(
                `/api/v1/integration/${integrationName}/login`,
                {
                    method: "GET",
                },
            );

            if (!r.ok) {
                throw Error();
            }
        } catch (e) {
            hammerheadToggleValue = false;
            show_toast({
                text: $_(`error-logging-in-to-${integrationName}`),
                icon: "close",
                type: "error",
            });
            return false;
        }

        return true;
    }

    async function verifyImmich(form?: ImmichIntegration): Promise<boolean> {
        try {
            const r = await fetch("/api/v1/integration/immich/check", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify(form ?? {}),
            });

            if (!r.ok) {
                throw Error();
            }
        } catch (e) {
            immichToggleValue = false;
            show_toast({
                text: $_("error-logging-in-to-immich"),
                icon: "close",
                type: "error",
            });
            return false;
        }

        return true;
    }

    async function onHammerheadToggle(value: boolean) {
        if (!integration?.hammerhead) {
            return;
        }
        if (value) {
            let verified = await verifyLogin("hammerhead");
            if (!verified) {
                return;
            }
            integration.hammerhead.active = true;
        } else {
            integration.hammerhead.active = false;
        }

        try {
            integration = await integrations_update(integration);
        } catch (e) {
            show_toast({
                text: $_("error-updating-hammerhead-integration"),
                icon: "close",
                type: "error",
            });
            return;
        }

        show_toast({
            text:
                "Hammerhead " +
                $_(`integration-${value ? "enabled" : "disabled"}`),
            icon: "check",
            type: "success",
        });
    }

    async function onImmichToggle(value: boolean) {
        if (!integration?.immich) {
            return;
        }
        if (value) {
            let verified = await verifyImmich();
            if (!verified) {
                immichToggleValue = false;
                return;
            }
        }
        integration.immich.active = value;

        try {
            integration = await integrations_update(integration);
        } catch (e) {
            immichToggleValue = !value;
            show_toast({
                text: $_("error-updating-immich-integration"),
                icon: "close",
                type: "error",
            });
            return;
        }

        show_toast({
            text: "Immich " + $_(`integration-${value ? "enabled" : "disabled"}`),
            icon: "check",
            type: "success",
        });
    }
</script>

<svelte:head>
    <title>{$_("settings")} | wanderer</title>
</svelte:head>

<h3 class="text-2xl font-semibold">{$_("integrations")}</h3>
<hr class="mt-4 mb-6 border-input-border" />

<section class="space-y-4">
    <h4 class="text-xl font-medium mb-2">{$_("trail-integrations")}</h4>
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        <IntegrationCard
            img="https://upload.wikimedia.org/wikipedia/commons/c/cb/Strava_Logo.svg"
            title="Strava"
            description={$_("integration-description-strava")}
            disabled={!integration?.strava}
            active={stravaToggleValue}
            onclick={() => stravaSettingsModal.openModal()}
            ontoggle={onStravaToggle}
        ></IntegrationCard>
        <IntegrationCard
            img="https://upload.wikimedia.org/wikipedia/commons/8/82/Komoot-logo-type.svg"
            title="komoot"
            description={$_("integration-description-komoot")}
            disabled={!integration?.komoot}
            bind:active={komootToggleValue}
            onclick={() => komootSettingsModal.openModal()}
            ontoggle={onKomootToggle}
        ></IntegrationCard>
        <IntegrationCard
            img={$theme == "light" ? hammerheadLogoDark : hammerheadLogoWhite}
            title="Hammerhead"
            description={$_("integration-description-hammerhead")}
            disabled={!integration?.hammerhead}
            bind:active={hammerheadToggleValue}
            onclick={() => hammerheadSettingsModal.openModal()}
            ontoggle={onHammerheadToggle}
        ></IntegrationCard>
    </div>
</section>

<section class="mt-10 space-y-4">
    <h4 class="text-xl font-medium mb-2">{$_("photo-integrations")}</h4>
    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
        <IntegrationCard
            img="/immich.svg"
            title="Immich"
            description={$_("integration-description-immich")}
            disabled={!integration?.immich}
            bind:active={immichToggleValue}
            onclick={() => immichSettingsModal.openModal()}
            ontoggle={onImmichToggle}
        ></IntegrationCard>
    </div>
</section>

<StravaSettingsModal
    bind:this={stravaSettingsModal}
    {integration}
    onsave={(form) => onSettingsSave(form, "strava")}
></StravaSettingsModal>

<KomootSettingsModal
    bind:this={komootSettingsModal}
    {integration}
    onsave={(form) => onSettingsSave(form, "komoot")}
></KomootSettingsModal>

<HammerheadSettingsModal
    bind:this={hammerheadSettingsModal}
    {integration}
    onsave={(form) => onSettingsSave(form, "hammerhead")}
></HammerheadSettingsModal>

<ImmichSettingsModal
    bind:this={immichSettingsModal}
    {integration}
    onsave={(form, materialize) => onSettingsSave(form, "immich", materialize)}
></ImmichSettingsModal>
