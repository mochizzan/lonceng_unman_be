// ============================================================================
// shared/api-client.js
// Thin fetch wrapper for eval API endpoints. Centralizes JSON encoding,
// response parsing, and error surfacing so page scripts only describe payload.
// ============================================================================

/**
 * Posts a JSON payload to an eval endpoint and returns the parsed envelope.
 *
 * @param {string} url - Full URL (e.g. /api/v1/eval/{npm}/krs/{file}).
 * @param {object} body - Object to JSON-encode as the request body.
 * @returns {Promise<{ok: boolean, status: number, data: any, message: string}>}
 */
export async function postJSON(url, body) {
    const resp = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    });

    let payload = null;
    try {
        payload = await resp.json();
    } catch {
        payload = null;
    }

    return {
        ok: resp.ok,
        status: resp.status,
        data: payload?.data ?? null,
        message: payload?.message ?? '',
    };
}
