import type { Asset } from "$lib/models/asset";
import { Collection, handleError } from "$lib/util/api_util";
import { json, type RequestEvent } from "@sveltejs/kit";
import { ClientResponseError } from "pocketbase";

/**
 * @swagger
 * /api/v1/assets:
 *   put:
 *     summary: Upload photo assets
 *     description: Uploads one or more photo files and attaches them to a trail, waypoint, or summit log owned by the authenticated user.
 *     tags:
 *       - Assets
 *     requestBody:
 *       required: true
 *       content:
 *         multipart/form-data:
 *           schema:
 *             type: object
 *             required:
 *               - files
 *             properties:
 *               files:
 *                 type: array
 *                 items:
 *                   type: string
 *                   format: binary
 *               trail:
 *                 type: string
 *                 description: Trail id to attach assets to
 *               waypoint:
 *                 type: string
 *                 description: Waypoint id to attach assets to
 *               summit_log:
 *                 type: string
 *                 description: Summit log id to attach assets to
 *               lat:
 *                 type: number
 *               lon:
 *                 type: number
 *     responses:
 *       200:
 *         description: Created assets
 *         content:
 *           application/json:
 *             schema:
 *               type: array
 *               items:
 *                 $ref: '#/components/schemas/Asset'
 *       400:
 *         description: Missing or invalid target relation
 *       401:
 *         description: Unauthorized
 *       403:
 *         description: Insufficient permissions for target trail
 *       500:
 *         description: Internal Server Error
 */
export async function PUT(event: RequestEvent) {
    try {
        if (!event.locals.user) {
            return json({ message: "Unauthorized" }, { status: 401 });
        }

        const data = await event.request.formData();
        const files = data.getAll("files").filter((file): file is File => file instanceof File);
        const target = await validateTarget(event, data);
        const created: Asset[] = [];

        for (const file of files) {
            const asset = new FormData();
            asset.append("type", "photo");
            asset.append("storage_mode", "copy");
            asset.append("remote_status", "available");
            asset.append("author", event.locals.user.id);
            asset.append("file", file);

            for (const key of ["trail", "waypoint", "summit_log"] as const) {
                const value = target[key];
                if (value) {
                    asset.append(key, value);
                }
            }

            for (const key of ["lat", "lon"]) {
                const value = data.get(key);
                if (typeof value === "string" && value.length) {
                    asset.append(key, value);
                }
            }

            created.push(await event.locals.pb.collection(Collection.assets).create<Asset>(asset));
        }

        return json(created);
    } catch (e: any) {
        return handleError(e);
    }
}

async function validateTarget(event: RequestEvent, data: FormData) {
    const user = event.locals.user!;
    const target = {
        trail: stringValue(data.get("trail")),
        waypoint: stringValue(data.get("waypoint")),
        summit_log: stringValue(data.get("summit_log")),
    };

    if (!target.trail && !target.waypoint && !target.summit_log) {
        throw new ClientResponseError({
            status: 400,
            response: { message: "Asset target is required" },
        });
    }

    if (target.waypoint) {
        const waypoint = await event.locals.pb.collection(Collection.waypoints).getOne(target.waypoint);
        if (!target.trail) {
            target.trail = waypoint.trail;
        } else if (waypoint.trail !== target.trail) {
            throw new ClientResponseError({
                status: 400,
                response: { message: "Waypoint does not belong to trail" },
            });
        }
    }

    if (target.summit_log) {
        const summitLog = await event.locals.pb.collection(Collection.summit_logs).getOne(target.summit_log);
        if (!target.trail) {
            target.trail = summitLog.trail;
        } else if (summitLog.trail !== target.trail) {
            throw new ClientResponseError({
                status: 400,
                response: { message: "Summit log does not belong to trail" },
            });
        }
    }

    if (target.trail) {
        const trail = await event.locals.pb.collection(Collection.trails).getOne(target.trail);
        if (trail.author !== user.actor) {
            throw new ClientResponseError({
                status: 403,
                response: { message: "Insufficient permissions for trail" },
            });
        }
    }

    return target;
}

function stringValue(value: FormDataEntryValue | null): string | undefined {
    return typeof value === "string" && value.length ? value : undefined;
}
