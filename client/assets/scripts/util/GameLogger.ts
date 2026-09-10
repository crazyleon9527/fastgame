import { DEBUG } from 'cc/env';

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

export interface LogContext {
    traceId?: string;
    roundId?: string;
    merchantId?: string;
    userId?: number;
    [key: string]: string | number | boolean | undefined;
}

/** 轻量结构化日志 — DEBUG 输出 JSON，便于与 RGS trace_id 对齐 */
export class GameLogger {
    private static traceId = '';

    static setTraceId(traceId: string): void {
        this.traceId = traceId || '';
    }

    static getTraceId(): string {
        return this.traceId;
    }

    static log(level: LogLevel, event: string, ctx: LogContext = {}): void {
        const payload = {
            ts: new Date().toISOString(),
            level,
            event,
            trace_id: ctx.traceId ?? this.traceId ?? undefined,
            ...ctx,
        };
        const line = JSON.stringify(payload);
        switch (level) {
            case 'debug':
                if (DEBUG) console.debug(line);
                break;
            case 'info':
                console.info(line);
                break;
            case 'warn':
                console.warn(line);
                break;
            default:
                console.error(line);
        }
    }

    static info(event: string, ctx?: LogContext): void {
        this.log('info', event, ctx);
    }

    static warn(event: string, ctx?: LogContext): void {
        this.log('warn', event, ctx);
    }

    static error(event: string, ctx?: LogContext): void {
        this.log('error', event, ctx);
    }
}
