import { GameConfig } from '../config/GameConfig';

export interface BigWinPayload {
    type: string;
    roundId: string;
    userId: number;
    merchantId: string;
    gameCode: string;
    winAmount: number;
    multiplier: number;
    occurredAt: string;
}

export type BigWinHandler = (payload: BigWinPayload) => void;

/**
 * 全服大奖 WebSocket — 订阅 Gateway /ws/bigwin
 */
export class BigWinSocket {
    private ws: WebSocket | null = null;
    private handler: BigWinHandler | null = null;
    private url: string;

    constructor(url = GameConfig.wsUrl) {
        this.url = url;
    }

    connect(onMessage: BigWinHandler): void {
        this.handler = onMessage;
        this.ws = new WebSocket(this.url);
        this.ws.onmessage = (ev) => {
            try {
                const data = JSON.parse(ev.data as string) as BigWinPayload;
                this.handler?.(data);
            } catch {
                // ignore malformed
            }
        };
        this.ws.onclose = () => {
            setTimeout(() => this.connect(onMessage), 3000);
        };
    }

    disconnect(): void {
        this.ws?.close();
        this.ws = null;
    }
}
