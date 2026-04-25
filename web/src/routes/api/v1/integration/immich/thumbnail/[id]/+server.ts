import { error, type RequestEvent } from "@sveltejs/kit";
import { z } from "zod";

/**
 * @swagger
 * /api/v1/integration/immich/thumbnail/{id}:
 *   get:
 *     summary: Proxy Immich thumbnail
 *     tags:
 *       - Integrations
 *     parameters:
 *       - in: path
 *         name: id
 *         required: true
 *         schema:
 *           type: string
 *         description: Immich asset id
 *     responses:
 *       200:
 *         description: Thumbnail image stream
 *         content:
 *           image/*:
 *             schema:
 *               type: string
 *               format: binary
 *       400:
 *         description: Invalid asset id
 *       401:
 *         description: Unauthorized
 *       404:
 *         description: Thumbnail not available
 */
export async function GET(event: RequestEvent) {
    const safeParams = z.object({ id: z.string().uuid() }).safeParse(event.params);
    if (!safeParams.success) {
        throw error(400, "Invalid asset id");
    }

    const headers: HeadersInit = {};
    const token = event.locals.pb.authStore.token;
    if (token) {
        headers.Authorization = `Bearer ${token}`;
    }

    const url = event.locals.pb.buildURL(`/integration/immich/thumbnail/${safeParams.data.id}`);
    const response = await event.fetch(url, { headers });

    if (!response.ok) {
        throw error(response.status, "Thumbnail not available");
    }

    return new Response(response.body, {
        headers: response.headers,
        status: response.status,
    });
}
