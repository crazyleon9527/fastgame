/**
 * 服务端下发的游戏会话 — 客户端不得自行生成 Server Seed
 */
export interface SessionInfo {
    sessionToken: string;
    serverSeedHash: string;
    clientSeed: string;
    nextSequenceId: number;
    expiresAt: number;
}

export class GameSession {
    private info: SessionInfo | null = null;

    set(info: SessionInfo): void {
        this.info = info;
    }

    get token(): string {
        if (!this.info) throw new Error('session not initialized');
        return this.info.sessionToken;
    }

    get serverSeedHash(): string {
        return this.info?.serverSeedHash ?? '';
    }

    /** 消费并返回当前序号，随后递增 */
    consumeSequence(): number {
        if (!this.info) throw new Error('session not initialized');
        const seq = this.info.nextSequenceId;
        this.info.nextSequenceId += 1;
        return seq;
    }

    get clientSeed(): string {
        return this.info?.clientSeed ?? '';
    }

    isExpired(): boolean {
        if (!this.info) return true;
        return Date.now() / 1000 > this.info.expiresAt;
    }
}
