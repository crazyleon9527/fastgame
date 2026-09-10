/**
 * HMAC-SHA256 请求签名 — 与 RGS pkg/security/sign.go 对齐
 */
export class RequestSigner {
    static buildPayload(
        method: string,
        path: string,
        body: string,
        timestamp: string,
        nonce: string,
    ): string {
        return [method.toUpperCase(), path, body, timestamp, nonce].join('|');
    }

    static async sign(secret: string, payload: string): Promise<string> {
        const enc = new TextEncoder();
        const key = await crypto.subtle.importKey(
            'raw',
            enc.encode(secret),
            { name: 'HMAC', hash: 'SHA-256' },
            false,
            ['sign'],
        );
        const sig = await crypto.subtle.sign('HMAC', key, enc.encode(payload));
        return Array.from(new Uint8Array(sig))
            .map((b) => b.toString(16).padStart(2, '0'))
            .join('');
    }
}
