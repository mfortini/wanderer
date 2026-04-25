import { handleError } from "$lib/util/api_util";
import { json, type RequestEvent } from "@sveltejs/kit";

/**
 * @swagger
 * /api/v1/integration/immich/check:
 *   post:
 *     summary: Check Immich integration connection
 *     tags:
 *       - Integrations
 *     requestBody:
 *       required: false
 *       content:
 *         application/json:
 *           schema:
 *             type: object
 *             properties:
 *               url:
 *                 type: string
 *               apiKey:
 *                 type: string
 *               photoMode:
 *                 type: string
 *                 enum: [copy, link_private, link_public]
 *     responses:
 *       200:
 *         description: Connection check succeeded
 *         content:
 *           application/json:
 *             schema:
 *               type: object
 *               properties:
 *                 ok:
 *                   type: boolean
 *       400:
 *         description: Connection check failed
 *       401:
 *         description: Unauthorized
 */
export async function POST(event: RequestEvent) {
    try {
        const data = await event.request.json();
        const r = await event.locals.pb.send("/integration/immich/check", {
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
