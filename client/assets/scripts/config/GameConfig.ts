/**
 * 游戏配置 — 通过 Gateway 统一接入
 */
export const GameConfig = {
    /** HTTP Gateway 地址 */
    gatewayUrl: 'http://localhost:18000',

    /** WebSocket 大奖广播地址 */
    wsUrl: 'ws://localhost:18000/ws/bigwin',

    merchantId: 'm001',
    /** 商户 API 私钥；生产环境由构建注入，启用后 RGS 需关闭 SkipSignVerify */
    merchantSecret: '',

    /** 可选 Client Seed，留空则由服务端生成 */
    clientSeed: '',

    gameCode: 'fishing',

    /** 演示用玩家 ID，生产环境由登录态注入 */
    userId: 10001,

    defaultBet: 10,
};
