import { handleError } from "$lib/util/api_util";
import { json, type RequestEvent } from "@sveltejs/kit";

/**
 * @swagger
 * /api/v1/integration/immich/attach:
 *   post:
 *     summary: Attach Immich photos to a trail
 *     description: Searches the authenticated user's Immich library and attaches matching photos to an existing trail for a provider.
 *     tags:
 *       - Integrations
 *     requestBody:
 *       required: true
 *       content:
 *         application/json:
 *           schema:
 *             type: object
 *             required:
 *               - trailId
 *             properties:
 *               trailId:
 *                 type: string
 *               provider:
 *                 type: string
 *                 enum: [strava, komoot, hammerhead, upload]
 *     responses:
 *       200:
 *         description: Attach completed or skipped
 *       400:
 *         description: Invalid request
 *       401:
 *         description: Unauthorized
 *       403:
 *         description: Insufficient permissions for trail
 *       500:
 *         description: Internal Server Error
 */
export async function POST(event: RequestEvent) {
    try {
        const data = await event.request.json();
        const r = await event.locals.pb.send("/integration/immich/attach", {
            method: "POST",
            body: JSON.stringify(data),
            headers: {
                "Content-Type": "application/json",
            },
        });
        return json(r);
    } catch (e: any) {
        return handleError(e);
    }
}
