# 开局 · H5 客户端

对接 `table-server` 的成品 H5 网页端。先在浏览器联调房间 / 对局 / 战绩，后续把同一套 TypeScript 协议层迁入 Cocos Creator，再导出微信小游戏。

## 功能

- 登录：`LoginByWechat`（H5 用本地固定 Code 模拟 `wx.login`）
- 大厅：创建 / 加入 / 列出房间，覆盖全部玩法 Config
- 房间：准备、开局、踢人、移交房主、离开、解散
- 对局：快照 + 1s `PullEvent`、Legal 按钮、`ClientSeq` 幂等出手
- 玩法渲染：骰子、大小牌、十点半、炸金花、斗牛、卧底、狼人杀
- 战绩：`ListRecord` / `DescribeRecord`

## 快速开始

1. 启动后端 `table-server`（监听 `8080`，`DEV=1`，用 Eosconfig 拉 MySQL/Redis）。
2. 安装并启动本客户端：

```bash
npm install
cp .env.example .env
npm run dev
```

浏览器打开终端提示的本地地址。Vite 把 `/table` 代理到 `VITE_API_PROXY_TARGET`（默认 `http://127.0.0.1:8080`）。

```bash
npm run build      # 类型检查 + 生产构建
npm run smoke      # 双人对骰子整局冒烟（需后端已启动）
```

## 配置中心

后端通过 Eosconfig 拉库表连接，与本前端无关：

- 地址：`https://config.nginx.systems/`
- AppId：`table-server`
- 密钥放在后端环境变量 `CONFIG_TOKEN`，**不要写进本仓库**

## 目录

```text
src/net        HTTP 信封与全部 API
src/play       PullEvent 循环与 SubmitAction
src/games      玩法元数据与桌面渲染
src/platform   H5 / 微信登录适配
src/ui 逻辑    在 app.ts（大厅/房间/对局/战绩）
docs/          客户端接口原文
cocos/         迁 Cocos / 小游戏说明
```

## 联调注意

- 业务成功失败都是 HTTP 200，先看 `Error`。
- Token 用 `Authorization: Bearer` 或 `X-Token`。
- 回合倒计时看快照 `Deadline`；超时由服务端代操作。
- 详见 [docs/客户端接口.md](docs/客户端接口.md)。
