/**
 * 游戏配置 — 通过 Gateway 统一接入
 */
export const GameConfig = {
    /** HTTP Gateway 地址 */
    gatewayUrl: 'http://localhost:18000',

    /** WebSocket 大奖广播地址 */
    wsUrl: 'ws://localhost:18000/ws/bigwin',

    merchantId: 'm001',
    gameCode: 'fishing',

    /** 演示用玩家 ID，生产环境由登录态注入 */
    userId: 10001,

    defaultBet: 10,
};
