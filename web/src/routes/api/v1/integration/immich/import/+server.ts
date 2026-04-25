import { handleError } from "$lib/util/api_util";
import { json, type RequestEvent } from "@sveltejs/kit";

/**
 * @swagger
 * /api/v1/integration/immich/import:
 *   post:
 *     summary: Import Immich photos as trail waypoints
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
 *               - assetIds
 *             properties:
 *               trailId:
 *                 type: string
 *               assetIds:
 *                 type: array
 *                 items:
 *                   type: string
 *     responses:
 *       200:
 *         description: Created or matched waypoints with their Immich asset id
 *         content:
 *           application/json:
 *             schema:
 *               type: array
 *               items:
 *                 type: object
 *                 properties:
 *                   assetId:
 *                     type: string
 *                   waypoint:
 *                     $ref: '#/components/schemas/Waypoint'
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
        const r = await event.locals.pb.send("/integration/immich/import", {
            method: "POST",
            body: JSON.stringify(data),
            headers: { "Content-Type": "application/json" },
        });
        return json(r);
    } catch (e: any) {
        return handleError(e);
    }
}
