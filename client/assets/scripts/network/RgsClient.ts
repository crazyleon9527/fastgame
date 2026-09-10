import { GameConfig } from '../config/GameConfig';
import { GameLogger } from '../util/GameLogger';
import { RequestSigner } from './RequestSigner';
import { SecureEnvelope } from './SecureEnvelope';
import { GameSession, SessionInfo } from './GameSession';

const TRACE_HEADER = 'X-Trace-Id';

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
        const { data, traceId } = await this.signedPost<SessionInfo>(path, body);
        if (traceId) {
            GameLogger.setTraceId(traceId);
        }
        this.session.set(data);
        GameLogger.info('session_created', { merchantId, userId, gameCode, traceId });
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
        const timestamp = Math.floor(Date.now() / 1000).toString();
        const headers: Record<string, string> = { 'Content-Type': 'application/json', 'X-Timestamp': timestamp };

        if (this.merchantSecret) {
            const nonce = crypto.randomUUID();
            const payload = RequestSigner.buildPayload('POST', path, body, timestamp, nonce);
            headers['X-Nonce'] = nonce;
            headers['X-Signature'] = await RequestSigner.sign(this.merchantSecret, payload);
        }

        const sessionSign = await SecureEnvelope.sign(
            this.session.dynamicSessionKey,
            req.roundId,
            req.action,
            timestamp,
        );
        headers['X-Session-Sign'] = sessionSign;

        const resp = await fetch(`${this.baseUrl}${path}`, {
            method: 'POST',
            headers: this.withTraceHeader(headers),
            body,
        });
        this.captureTraceId(resp);
        if (!resp.ok) {
            const text = await resp.text();
            GameLogger.error('bet_failed', { roundId: req.roundId, status: resp.status, traceId: GameLogger.getTraceId() });
            throw new Error(`request failed: ${resp.status} ${text} trace=${GameLogger.getTraceId()}`);
        }
        const data: BetResponse = await resp.json();
        GameLogger.info('bet_ok', { roundId: req.roundId, traceId: GameLogger.getTraceId() });
        return data;
    }

    async fetchReplay(roundId: string): Promise<ReplayResponse> {
        const url = `${this.baseUrl}/api/v1/game/replay/${encodeURIComponent(roundId)}`;
        const resp = await fetch(url);
        if (!resp.ok) {
            throw new Error(`replay failed: ${resp.status}`);
        }
        return resp.json();
    }

    private withTraceHeader(headers: Record<string, string>): Record<string, string> {
        const traceId = GameLogger.getTraceId();
        if (traceId) {
            return { ...headers, [TRACE_HEADER]: traceId };
        }
        return headers;
    }

    private captureTraceId(resp: Response): void {
        const traceId = resp.headers.get(TRACE_HEADER);
        if (traceId) {
            GameLogger.setTraceId(traceId);
        }
    }

    private async signedPost<T>(path: string, body: string): Promise<{ data: T; traceId?: string }> {
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
            headers: this.withTraceHeader(headers),
            body,
        });
        this.captureTraceId(resp);
        const traceId = GameLogger.getTraceId() || undefined;
        if (!resp.ok) {
            const text = await resp.text();
            GameLogger.error('request_failed', { path, status: resp.status, traceId });
            throw new Error(`request failed: ${resp.status} ${text} trace=${traceId ?? ''}`);
        }
        const data: T = await resp.json();
        return { data, traceId };
    }
}
