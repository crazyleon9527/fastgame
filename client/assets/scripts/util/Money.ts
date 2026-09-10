/** 与后端 pkg/money Scale=10000 对齐 */
export const MONEY_SCALE = 10000;

export function toMajor(minor: number): number {
    return minor / MONEY_SCALE;
}

export function toMinor(major: number): number {
    return Math.round(major * MONEY_SCALE);
}

export function formatMinor(minor: number, digits = 2): string {
    return toMajor(minor).toFixed(digits);
}

export function formatMultiplier(minor: number, digits = 2): string {
    return toMajor(minor).toFixed(digits);
}
