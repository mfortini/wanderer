import { handleError } from "$lib/util/api_util";
import { json, type RequestEvent } from "@sveltejs/kit";

/**
 * @swagger
 * /api/v1/integration/immich/candidates:
 *   post:
 *     summary: Find Immich photo candidates
 *     description: Finds geotagged Immich photo candidates near a trail or coordinate for the authenticated user's Immich integration.
 *     tags:
 *       - Integrations
 *     requestBody:
 *       required: true
 *       content:
 *         application/json:
 *           schema:
 *             type: object
 *             properties:
 *               trailId:
 *                 type: string
 *               lat:
 *                 type: number
 *               lon:
 *                 type: number
 *               yearsBack:
 *                 type: integer
 *               doubleRadius:
 *                 type: boolean
 *     responses:
 *       200:
 *         description: Candidate photos
 *         content:
 *           application/json:
 *             schema:
 *               type: object
 *               properties:
 *                 hasTimestamps:
 *                   type: boolean
 *                 hasMore:
 *                   type: boolean
 *                 takenAfter:
 *                   type: string
 *                 candidates:
 *                   type: array
 *                   items:
 *                     $ref: '#/components/schemas/ImmichCandidate'
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
        const r = await event.locals.pb.send("/integration/immich/candidates", {
            method: "POST",
            body: JSON.stringify(data),
            headers: { "Content-Type": "application/json" },
        });
        return json(r);
    } catch (e: any) {
        return handleError(e);
    }
}
