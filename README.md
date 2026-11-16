# 开局 · 桌面派对客户端

对接 [`table-server`](https://github.com/duskroot/table-server) 的成品客户端：

1. **可玩 H5**（本目录）— 真牌面 / 骰子 / 筹码贴图 + 音效，立刻联调  
2. **Cocos Creator 3.8 工程**（`client-cocos/`）— 同源素材与协议，用于导出**微信小游戏**

## 快速开始（H5 成品）

```bash
# 另开终端启动 table-server :8080
npm install
cp .env.example .env
npm run dev
```

浏览器打开提示地址。Vite 将 `/table` 代理到 `VITE_API_PROXY_TARGET`。

```bash
npm run build
npm run smoke      # API 双人对骰子
npm run ui:smoke   # 浏览器 UI 冒烟（需 Chrome）
```

## Cocos / 微信

见 [client-cocos/README.md](client-cocos/README.md)。用 Creator **3.8.6** 打开 `client-cocos/`，挂载 `GameApp`，构建微信小游戏。

## 素材

Kenney Boardgame Pack（**CC0**）：牌、牌背、骰子、筹码、SFX。说明见 [public/game/ATTRIBUTION.md](public/game/ATTRIBUTION.md)。

## 功能

- 登录 / 大厅（7 种玩法）/ 房间 / 对局 / 战绩  
- 服务端权威：只渲染 Snapshot + Event，`Legal` 驱动按钮  
- `PullEvent` 1s 轮询，`ClientSeq` 幂等出手  
- 发牌 / 掷骰 / 筹码动画与音效  

接口原文：[docs/客户端接口.md](docs/客户端接口.md)

## 配置中心

后端通过 Eosconfig 拉 MySQL/Redis。密钥只放后端环境变量 `CONFIG_TOKEN`，**不要写入本仓库**。
