# Cocos / 微信小游戏接入说明

本仓库 H5 客户端已经按 `docs/客户端接口.md` 接好 `table-server`。迁到 Cocos Creator（再导出微信小游戏）时，**复用 `src/net`、`src/play`、`src/games`、`src/platform`**，不要在场景脚本里重写协议。

## 建议结构

1. 新建 Cocos Creator 3.x 工程，目标平台先选 **Web Mobile**，再加 **微信小游戏**。
2. 把 `src/net`、`src/play`、`src/games`、`src/platform`、`src/config.ts`、`src/state` 拷进 `assets/scripts/table/`。
3. UI 用 Cocos 节点重做；按钮点击仍只调用 `PlayLoop.submit` / `api.*`。
4. `platform/auth.ts` 里已预留 `wx.login`；小游戏构建后会走微信 code。
5. 请求根地址改成正式 API（HTTPS）。开发期可用代理；小游戏需在微信后台配置 request 合法域名。

## 客户端主循环（必须遵守）

1. `LoginByWechat` → 存 Token  
2. `CreateRoom` / `JoinRoom` → `SetReady` → 房主 `StartPlay`  
3. 每秒 `PullEvent`，保存 `Cursor`  
4. 只根据 `Legal` 渲染按钮  
5. `SubmitAction` 使用递增且可重试复用的 `ClientSeq`  
6. `Settled` 后停止出手，房间回到 `open` 后重新准备

服务端权威，客户端不要本地算胜负。
