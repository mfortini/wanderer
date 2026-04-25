import { Collection, handleError } from "$lib/util/api_util";
import { json, type RequestEvent } from "@sveltejs/kit";
import { z } from "zod";

/**
 * @swagger
 * /api/v1/assets/{id}:
 *   delete:
 *     summary: Delete asset
 *     tags:
 *       - Assets
 *     parameters:
 *       - in: path
 *         name: id
 *         required: true
 *         schema:
 *           type: string
 *     responses:
 *       200:
 *         description: Asset deleted
 *         content:
 *           application/json:
 *             schema:
 *               type: object
 *               properties:
 *                 acknowledged:
 *                   type: boolean
 *       400:
 *         description: Invalid asset id
 *       404:
 *         description: Asset not found
 *       500:
 *         description: Internal Server Error
 */
export async function DELETE(event: RequestEvent) {
    try {
        const safeParams = z.object({
            id: z.string().length(15),
        }).parse(event.params);

        const success = await event.locals.pb.collection(Collection.assets).delete(safeParams.id);
        return json({ acknowledged: success });
    } catch (e: any) {
        return handleError(e);
    }
}
