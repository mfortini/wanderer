import { handleError } from "$lib/util/api_util";
import { json, type RequestEvent } from "@sveltejs/kit";

/**
 * @swagger
 * /api/v1/integration/immich/import-to-waypoint:
 *   post:
 *     summary: Import Immich photos to an existing waypoint
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
 *               - waypointId
 *               - assetIds
 *             properties:
 *               trailId:
 *                 type: string
 *               waypointId:
 *                 type: string
 *               assetIds:
 *                 type: array
 *                 items:
 *                   type: string
 *     responses:
 *       200:
 *         description: Import accepted
 *         content:
 *           application/json:
 *             schema:
 *               type: object
 *       400:
 *         description: Invalid request or missing integration
 *       401:
 *         description: Unauthorized
 *       403:
 *         description: Insufficient permissions for waypoint
 *       500:
 *         description: Internal Server Error
 */
export async function POST(event: RequestEvent) {
    try {
        const data = await event.request.json();
        const r = await event.locals.pb.send("/integration/immich/import-to-waypoint", {
            method: "POST",
            body: JSON.stringify(data),
            headers: { "Content-Type": "application/json" },
        });
        return json(r);
    } catch (e: any) {
        return handleError(e);
    }
}
