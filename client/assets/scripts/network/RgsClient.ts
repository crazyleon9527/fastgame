import { GameConfig } from '../config/GameConfig';
import { RequestSigner } from './RequestSigner';

export interface BetRequest {
    merchantId: string;
    userId: number;
    roundId: string;
    gameCode: string;
    betAmount: number;
    idempotencyToken: string;
}

/** RGS 结算响应 — 客户端严禁自行计算，只消费此结构 */
export interface BetResponse {
    roundId: string;
    winAmount: number;
    multiplier: number;
    balance: number;
    rtpTier: string;
    fishState: 'miss' | 'bite' | 'big_win' | string;
    animationKey: string;
}

export interface BalanceResponse {
    balance: number;
}

export class RgsClient {
    private baseUrl: string;
    private merchantSecret?: string;

    constructor(baseUrl = GameConfig.gatewayUrl, merchantSecret = GameConfig.merchantSecret) {
        this.baseUrl = baseUrl.replace(/\/$/, '');
        this.merchantSecret = merchantSecret || undefined;
    }

    async getBalance(merchantId: string, userId: number): Promise<number> {
        const url = `${this.baseUrl}/api/v1/game/balance?merchantId=${encodeURIComponent(merchantId)}&userId=${userId}`;
        const resp = await fetch(url);
        if (!resp.ok) {
            throw new Error(`balance failed: ${resp.status}`);
        }
        const data: BalanceResponse = await resp.json();
        return data.balance;
    }

    async bet(req: BetRequest): Promise<BetResponse> {
        const path = '/api/v1/game/bet';
        const body = JSON.stringify(req);
        const headers: Record<string, string> = { 'Content-Type': 'application/json' };

        if (this.merchantSecret) {
            const timestamp = Math.floor(Date.now() / 1000).toString();
            const nonce = crypto.randomUUID();
            const payload = RequestSigner.buildPayload('POST', path, body, timestamp, nonce);
            const signature = await RequestSigner.sign(this.merchantSecret, payload);
            headers['X-Timestamp'] = timestamp;
            headers['X-Nonce'] = nonce;
            headers['X-Signature'] = signature;
        }

        const resp = await fetch(`${this.baseUrl}${path}`, {
            method: 'POST',
            headers,
            body,
        });
        if (!resp.ok) {
            const text = await resp.text();
            throw new Error(`bet failed: ${resp.status} ${text}`);
        }
        return resp.json();
    }
}
