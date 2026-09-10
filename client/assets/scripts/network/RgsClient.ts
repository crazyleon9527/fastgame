import { GameConfig } from '../config/GameConfig';

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

    constructor(baseUrl = GameConfig.gatewayUrl) {
        this.baseUrl = baseUrl.replace(/\/$/, '');
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
        const resp = await fetch(`${this.baseUrl}/api/v1/game/bet`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(req),
        });
        if (!resp.ok) {
            const text = await resp.text();
            throw new Error(`bet failed: ${resp.status} ${text}`);
        }
        return resp.json();
    }
}
