import { handleError } from "$lib/util/api_util";
import { json, type RequestEvent } from "@sveltejs/kit";

/**
 * @swagger
 * /api/v1/integration/immich/materialize-all:
 *   post:
 *     summary: Download all linked Immich photos to local storage
 *     description: Triggers a background job that downloads all remote Immich assets across all of the authenticated user's trails.
 *     tags:
 *       - Integrations
 *     responses:
 *       200:
 *         description: Materialization started
 *       401:
 *         description: Unauthorized
 *       500:
 *         description: Internal Server Error
 */
export async function POST(event: RequestEvent) {
    try {
        const body = await event.request.json().catch(() => ({}));
        const r = await event.locals.pb.send("/integration/immich/materialize-all", {
            method: "POST",
            body: JSON.stringify(body),
            headers: { "Content-Type": "application/json" },
        });
        return json(r);
    } catch (e: any) {
        return handleError(e);
    }
}
