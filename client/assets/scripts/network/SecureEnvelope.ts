/**
 * Secure Envelope — HMAC-SHA256(RoundID + Action + Timestamp, DynamicSessionKey)
 * 生产构建可编译为 WASM 模块；开发态使用 Web Crypto 等价实现
 */
export class SecureEnvelope {
    static async sign(
        dynamicSessionKey: string,
        roundId: string,
        action: string,
        timestamp: string,
    ): Promise<string> {
        const payload = roundId + action + timestamp;
        const enc = new TextEncoder();
        const key = await crypto.subtle.importKey(
            'raw',
            enc.encode(dynamicSessionKey),
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
