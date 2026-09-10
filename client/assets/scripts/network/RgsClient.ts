import { GameConfig } from '../config/GameConfig';
import { RequestSigner } from './RequestSigner';
import { GameSession, SessionInfo } from './GameSession';

/** 客户端仅上报：action(cast) + betAmount(minor) + roundId + sequenceId */
export interface BetRequest {
    merchantId: string;
    userId: number;
    sessionToken: string;
    gameCode: string;
    action: 'cast';
    betAmount: number;
    roundId: string;
    sequenceId: number;
    clientSeed?: string;
}

export interface ProvablyFairProof {
    serverSeedHash: string;
    serverSeed: string;
    clientSeed: string;
    nonce: string;
    roll: number;
}

export interface ReplayPoint {
    x: number;
    y: number;
}

export interface ReplayScenePayload {
    weather: string;
    fishSpecies: string;
    fishPath: ReplayPoint[];
    biteProp: string;
    castDurationMs: number;
    fishSpeed: number;
}

export interface ReplayPayload {
    inputs: {
        serverSeed: string;
        clientSeed: string;
        nonce: string;
        betAmount: number;
    };
    scene: ReplayScenePayload;
}

/** RGS 结算响应 — winAmount/balance/multiplier 均为 minor units (Scale 10000) */
export interface BetResponse {
    roundId: string;
    winAmount: number;
    multiplier: number;
    balance: number;
    rtpTier: string;
    fishState: 'miss' | 'bite' | 'big_win' | string;
    animationKey: string;
    sequenceId: number;
    provablyFair: ProvablyFairProof;
    replay?: ReplayPayload;
}

export interface BalanceResponse {
    balance: number;
}

export interface ReplayResponse {
    roundId: string;
    sequenceId: number;
    replay: ReplayPayload;
    provablyFair: ProvablyFairProof;
    winAmount: number;
    multiplier: number;
    fishState: string;
    animationKey: string;
}

export class RgsClient {
    private baseUrl: string;
    private merchantSecret?: string;
    readonly session = new GameSession();

    constructor(baseUrl = GameConfig.gatewayUrl, merchantSecret = GameConfig.merchantSecret) {
        this.baseUrl = baseUrl.replace(/\/$/, '');
        this.merchantSecret = merchantSecret || undefined;
    }

    async createSession(merchantId: string, userId: number, gameCode: string, clientSeed?: string): Promise<SessionInfo> {
        const path = '/api/v1/game/session';
        const body = JSON.stringify({ merchantId, userId, gameCode, clientSeed: clientSeed || undefined });
        const data: SessionInfo = await this.signedPost(path, body);
        this.session.set(data);
        return data;
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
        return this.signedPost(path, body);
    }

    async fetchReplay(roundId: string): Promise<ReplayResponse> {
        const url = `${this.baseUrl}/api/v1/game/replay/${encodeURIComponent(roundId)}`;
        const resp = await fetch(url);
        if (!resp.ok) {
            throw new Error(`replay failed: ${resp.status}`);
        }
        return resp.json();
    }

    private async signedPost(path: string, body: string): Promise<any> {
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
            throw new Error(`request failed: ${resp.status} ${text}`);
        }
        return resp.json();
    }
}
