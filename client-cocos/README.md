# 开局 · Cocos Creator 3.8 客户端

对接 `table-server` 的 Cocos 成品工程，内置 **Kenney Boardgame Pack（CC0）** 牌面 / 骰子 / 筹码 / 音效，以及绿绒牌桌贴图。

## 环境

- Cocos Creator **3.8.6**（Windows / macOS）
- 设计分辨率 750×1334（竖屏，适合微信小游戏）

本云环境无 Linux 版 Creator，因此这里交付的是**可直接打开的完整工程 + 同源可玩 H5 预览**（仓库根目录 `npm run dev`）。

## 打开工程

1. 安装 [Cocos Creator 3.8.6](https://www.cocos.com/creator-download)
2. 打开本目录 `client-cocos/`
3. 新建场景 `assets/scenes/Main.scene`（或打开后创建）
4. 添加 `Canvas`，挂载脚本 `assets/scripts/GameApp.ts`
5. 在 Inspector 里把 `Api Base` 改成你的后端地址（默认 `http://127.0.0.1:8080`）
6. 点击预览。导出微信小游戏：菜单 **项目 → 构建发布 → 微信小游戏**

首次打开会重建 `library/`、`temp/`，属正常现象。

## 资源说明

| 路径 | 内容 |
|---|---|
| `assets/resources/textures/cards` | 52 张扑克 + Joker（Kenney） |
| `assets/resources/textures/backs` | 牌背 |
| `assets/resources/textures/dice` | 红/白骰子 1–6 |
| `assets/resources/textures/chips` | 筹码 |
| `assets/resources/textures/bg` | 牌桌绒布 |
| `assets/resources/sounds` | 发牌/掷骰/筹码 OGG |
| `assets/scripts` | 会话/房间/对局协议与 UI |

授权见 `../public/game/ATTRIBUTION.md`。

## 协议层

与根目录 H5 共用同一套 `table/v1` 约定：

- `LoginByWechat` → `CreateRoom` / `JoinRoom` → `SetReady` → `StartPlay`
- 对局每秒 `PullEvent`，按钮只看 `Legal`
- `SubmitAction` 使用递增 `ClientSeq`

微信端 `platform/Auth.ts` 会走 `wx.login`；浏览器预览走本地 Code。

## 与 H5 的关系

| | H5（仓库根） | Cocos（本目录） |
|---|---|---|
| 用途 | 立刻可玩的成品预览 | 导出微信小游戏 / App |
| 贴图 | `public/game/**` | `assets/resources/**`（同源拷贝） |
| 协议 | `src/net` | `assets/scripts/net` |

先用 H5 联调后端与玩法，确认无误后再用 Creator 出微信包。
