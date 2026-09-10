# Provably Fair 验算工具

公开页面，玩家与运营均可使用，**无需登录**。

## 访问

```
http://localhost:18000/verify/
```

## URL 预填参数

```
/verify/?serverSeed=...&serverSeedHash=...&clientSeed=...&nonce=round-1&roll=0.72&betAmount=10
```

## 功能

- SHA-256 校验 Server Seed Hash
- HMAC-SHA256 复现 Roll 值
- 可选对比服务端返回的 Roll
- 可选根据 Bet Amount 推导 fishState / multiplier / winAmount
- 粘贴 `/game/bet` JSON 一键导入
