---
title: Integrations
description: How to set up third-party integrations with wanderer.
---

The integrations settings are split into two groups:

- **Trail integrations** import routes and activities into <span class="-tracking-[0.075em]">wanderer</span>. Currently, this includes **Strava**, **komoot**, and **Hammerhead**.
- **Photo integrations** attach external photos to existing trails and waypoints. Currently, this includes **Immich**.

It is important to note that synchronization only works from the provider to <span class="-tracking-[0.075em]">wanderer</span> and not the other way around. Additionally, if a trail has already been synced to <span class="-tracking-[0.075em]">wanderer</span>, subsequent changes made in the provider will not be transferred unless the trail is deleted in <span class="-tracking-[0.075em]">wanderer</span>. Hammerhead also supports manual uploads from a trail's action menu, which is separate from the nightly sync.

## Trail Integrations

Trail integrations create or update trails in <span class="-tracking-[0.075em]">wanderer</span> from external route providers.

### Strava

#### Creating an App in Strava

Before integrating Strava with <span class="-tracking-[0.075em]">wanderer</span>, you need to create an API application in Strava. Visit [Strava's API settings](https://www.strava.com/settings/api) and follow the steps to create a new API application. Your setup should resemble the following:

![Strava API Application](../../../assets/guides/strava_api_app.png)

#### Setting Up the Integration

1. Copy the **Client ID** and **Client Secret**.
2. Go to the integrations page in <span class="-tracking-[0.075em]">wanderer</span>'s settings.
3. Click the settings button for the Strava integration.
4. Enter your **Client ID** and **Client Secret**.
5. Choose whether you want to sync routes, activities, or both.

![wanderer Strava Integration](../../../assets/guides/wanderer_integration_strava.png)

6. Save the settings and toggle the integration on.
7. You will be redirected to Strava's authorization page. Keep all checkboxes selected and click **Authorize**.
8. You will then be redirected back to <span class="-tracking-[0.075em]">wanderer</span>. The Strava integration is now active.

### komoot

The komoot integration requires only your komoot username and password:

1. Open the komoot settings from the integrations menu.
2. Enter your komoot credentials.
3. Save the settings.
4. Toggle the integration on. It will become active immediately.

Your planned and completed trails will now sync with <span class="-tracking-[0.075em]">wanderer</span>.

### Hammerhead

The Hammerhead integration requires your Hammerhead account details:

1. Open the Hammerhead settings from the integrations menu.
2. Enter your Hammerhead email and password.
3. Choose whether you want to sync planned tours, completed tours, or both.
4. (Optional) Set an "ignore trails before" date to avoid syncing duplicates if your Hammerhead account is already connected to other services.
5. Save the settings and toggle the integration on. It will become active immediately after a successful login.

## Photo Integrations

Photo integrations do not create trails by themselves. They use existing trail, waypoint, and GPX data to find matching photos.

### Immich

The Immich integration lets <span class="-tracking-[0.075em]">wanderer</span> search your Immich library for geotagged photos near a trail and attach them as trail or waypoint photos. It works with trails imported from Strava, komoot, Hammerhead, and GPX uploads, depending on the providers you enable in the Immich settings.

Before setting up the integration, create an API key in Immich with the following permissions:

| Permission | Required for |
|---|---|
| `asset.read` | Reading asset metadata and EXIF data |
| `asset.view` | Accessing thumbnails |
| `asset.download` | Downloading originals (copy and link modes) |

1. Go to the integrations page in <span class="-tracking-[0.075em]">wanderer</span>'s settings.
2. Open the Immich settings.
3. Enter your Immich URL, for example `https://immich.example.com`.
4. Enter your Immich API key.
5. Adjust the matching settings if needed.
6. Choose how photos should be stored.
7. Select the providers Immich should be used with.
8. Save the settings and toggle the integration on.

:::note
For security reasons, the Immich URL must use `http` or `https`, must not include credentials, and must resolve to a public address. Local and private network addresses are blocked.
:::

#### Matching Settings

Immich matching uses photo timestamps and GPS metadata where possible:

- **Time window**: How far before and after the trail's timestamps <span class="-tracking-[0.075em]">wanderer</span> should search for photos. The default is `120` minutes.
- **Distance threshold**: The maximum distance between a photo location and the trail or waypoint. The default is `150` meters.
- **Max waypoints**: The maximum number of waypoints that can be created automatically from matching photos. The default is `25`.
- **Providers**: Controls whether Immich matching runs for Strava, komoot, Hammerhead, GPX uploads, or a subset of them.

If a GPX track has timestamps, <span class="-tracking-[0.075em]">wanderer</span> searches around the recorded activity time. If the track has no timestamps, manual Immich import can search older photos based on the trail date.

#### Photo Storage Modes

Immich photos can be stored in three ways:

- **Copy**: Downloads the photo from Immich and stores a copy in <span class="-tracking-[0.075em]">wanderer</span>. This is the most self-contained option.
- **Private link**: Keeps the photo in Immich and loads it through <span class="-tracking-[0.075em]">wanderer</span> when needed. If a trail is made public, private-linked photos are copied into <span class="-tracking-[0.075em]">wanderer</span> so public visitors do not depend on private Immich access.
- **Public link**: Keeps the photo in Immich and serves it through the Immich link. Use this only when your Immich setup and sharing policy allow it.

Copied photos remain available even if Immich is offline. Linked photos depend on the Immich server, the API key, and the referenced asset still being available.

#### Adding Immich Photos

Once Immich is active, photos can be attached in several places:

- Automatically after supported trail imports or GPX uploads, if the provider is enabled in the Immich settings.
- From a trail, by importing matching Immich photos as new waypoints.
- From a waypoint, by importing nearby Immich photos into the existing waypoint.
- From the photo picker, by selecting matching Immich photos alongside local uploads.

<span class="-tracking-[0.075em]">wanderer</span> avoids importing the same Immich asset more than once for the same user.

## Sync Interval

By default, trail integrations are synced every night at **02:00 AM**. You can modify this schedule using the `POCKETBASE_CRON_SYNC_SCHEDULE` [environment variable](/run/environment-configuration#pocketbase).

Photo integrations are not started as independent scheduled jobs. They are only affected indirectly when a scheduled trail import creates or updates a trail from a provider that is enabled in the photo integration settings.

:::note
Please set a reasonable sync interval. Strava, komoot, Hammerhead, and Immich can impose usage limits or operational constraints. Exceeding these limits may result in rejected requests or account restrictions.
:::
