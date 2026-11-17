# den 第一期设计：大厅、房间、摇骰子

日期：2026-11-17

状态：已确认。2026-11-17 用户回复「开始」。实现拆成两份计划：先做 den-api，再做 Cocos 客户端。

## 目标

做出一款原创派对手游的第一期可玩闭环：游客进入大厅，用房间号或快速匹配坐到同一桌，由服务端摇骰并完成一局「比大小」。同一套房间能切换到「大话骰」；大话骰的服务端规则和接口在本期完成，客户端场景不进入本期手测。

玩法、角色和界面都是原创。猛兽派对只作为派对大厅、短局、可爱角色这个品类的参考，不复制其名称、美术、文案或关卡。

## 范围

本期做：

- Cocos Creator 3.8 手游客户端，场景只有 Boot、Hall、Room、DiceTable
- 游客身份：设备号加昵称
- 房间号加入，以及按模式快速匹配
- 比大小可从客户端打完一局
- 大话骰的服务端规则、HTTP 接口和单测
- Go Hertz 模块化单体 `den-api`
- PostgreSQL 作为事实来源，Redis 作为登录态、在线状态、房间热数据和匹配队列
- Docker Compose 启动 PostgreSQL 和 Redis

本期不做：

- 谁是卧底、狼人杀、语音、支付、手机号或账号密码
- 把 `den-api` 拆成多个进程，或把 WebSocket 拆成独立进程
- 大话骰的客户端场景和手测
- 客户端自动化测试和房间全链路自动化测试
- 商用 AI 出图流水线。占位图使用 Kenney 的 CC0 素材，署名不作为上线条件

## 已确认决策

| 项 | 决定 |
| --- | --- |
| 第一期闭环 | 大厅 + 房间 + 摇骰子 |
| 摇骰子顺序 | 同一房间先做比大小，大话骰是第二个模式 |
| 进房 | 6 位房间号，以及快速匹配 |
| 身份 | 游客。设备号加昵称，不绑定手机 |
| 架构 | Hertz + PostgreSQL + Redis + WebSocket |
| 产品模块 | `den` |
| 服务模块 | `account`、`hall`、`room`、`match`、`dice` |
| 骰子权威 | 服务端生成。客户端上报的点数无效 |
| 已处理错误 | HTTP 200，响应体带 `Error`，不带 `Result` |
| 未处理异常 | HTTP 500，`Code` 为 `100001`，线上 `Detail` 为空 |
| 匹配等待 | `StartMatch` 立即返回 `MatchId`，客户端调用 `DescribeMatch` |

## 架构

所有业务命令使用 POST。路径固定为：

```text
/den/v1/{service}/{Action}
```

WebSocket 只有一条：

```text
/den/v1/room/Subscribe
```

登录令牌放在连接成功后的第一帧，不放在握手头里。推送帧的 JSON 与 HTTP 封套相同，`Action` 使用推送事件名。

运行时是一个 `den-api` 进程加两个容器：

```text
Cocos 客户端
  HttpClient  -- POST -->  den-api
  RoomSocket  -- WS   -->  den-api
den-api
  PostgreSQL  玩家、房间、座位、匹配单、对局记录
  Redis       令牌、在线状态、房间热数据、匹配队列
```

`dice` 只读取 `room` 给出的座位，不修改房间成员。

## 组件

| 组件 | 职责 | 依赖 |
| --- | --- | --- |
| Boot | 读取或生成 `DeviceId`，调用 `GuestLogin`，保存令牌 | HttpClient |
| Hall | 调用 `EnterHall`，建房或发起匹配 | HttpClient |
| Room | 展示座位和准备状态，连接 Subscribe | HttpClient、RoomSocket |
| DiceTable | 本期只渲染比大小 | HttpClient、RoomSocket |
| account | 游客登录、修改昵称、校验令牌 | PostgreSQL、Redis |
| hall | 刷新在线状态 | Redis |
| room | 建房、加入、离开、准备、开局、描述房间 | PostgreSQL、Redis |
| match | 入队、取消、查询匹配结果；配齐后调用 room 建房 | Redis、room |
| dice | 比大小和大话骰的规则与记录 | PostgreSQL、Redis、room 的座位 |

客户端把令牌放在请求头：

```text
Authorization: Bearer <token>
```

`GuestLogin` 不需要令牌。昵称允许重复。

WebSocket 不依赖握手头。连接成功后的第一帧是：

```json
{ "Token": "<token>", "RoomId": "<room-id>" }
```

令牌无效时推送 `100003` 并关闭连接。调用者不在该房间时推送 `200003` 并关闭连接。

## 接口

成功时 HTTP 200：

```json
{
  "ResponseMetadata": {
    "RequestId": "123e4567-e89b-12d3-a456-426614174001",
    "Action": "CreateRoom",
    "Version": "v1",
    "Service": "room"
  },
  "Result": {}
}
```

失败且已被处理时 HTTP 200，只有 `Error`，没有 `Result`：

```json
{
  "ResponseMetadata": {
    "RequestId": "123e4567-e89b-12d3-a456-426614174001",
    "Action": "CreateRoom",
    "Version": "v1",
    "Service": "room"
  },
  "Error": {
    "Code": "100002",
    "Message": "Invalid Parameter",
    "Detail": "MaxSeats must be between 2 and 6"
  }
}
```

`RequestId` 是服务端生成的 UUID。日志至少包含 `RequestId`、`Action`、`Version`、`Service`。

### account

`POST /den/v1/account/GuestLogin`

请求：`DeviceId`（UUID）、`Nickname`（可空）。

结果：`UserId`、`Token`、`Nickname`。同一 `DeviceId` 返回同一个 `UserId`。新用户昵称缺省为 `玩家` 加四位数字。令牌写入 Redis，有效期 7 天，再次登录会换发令牌。

`POST /den/v1/account/UpdateNickname`

请求：`Nickname`。长度按 Unicode 码点计，2 到 12。结果返回新昵称。

### hall

`POST /den/v1/hall/EnterHall`

无业务字段。刷新该用户的在线键，结果返回 `UserId` 和 `Nickname`。

### room

`POST /den/v1/room/CreateRoom`

请求：`Mode` 为 `HighRoll` 或 `LiarsDice`，`MaxSeats` 为 2 到 6，缺省 4。

结果：`RoomId`、`RoomCode`（6 位数字）、`SeatNo` 为 0。房主写入座位。房间状态为 `Open`。房间号冲突时换号重试。本期客户端只发送 `HighRoll`。服务端仍接受 `LiarsDice`，供接口测试和下一期场景使用。

`POST /den/v1/room/JoinRoom`

请求：`RoomCode`。事务占下一个空座。结果：`RoomId`、`SeatNo`。

`POST /den/v1/room/LeaveRoom`

请求：`RoomId`。房主离开且房内还有人时，最小座位号的玩家成为房主。无人时房间状态变为 `Closed`。

`POST /den/v1/room/Ready`

请求：`RoomId`、`Ready`（布尔）。

`POST /den/v1/room/StartGame`

请求：`RoomId`。只有房主可以调用。至少 2 人且全员已准备。房间状态从 `Open` 变为 `Playing`，并调用 `dice` 创建一局。结果：`RoundId`。结算推送 `RoundResolved` 后房间回到 `Open`，准备状态清空。

`POST /den/v1/room/DescribeRoom`

请求：`RoomId`。结果包含模式、座位、准备状态、房主和当前 `RoundId`（没有则为空字符串）。

### match

`POST /den/v1/match/StartMatch`

请求：`Mode`、`MaxSeats`（2 到 6，缺省 4）。同一用户同一时刻只有一个进行中的匹配。结果：`MatchId`、`Status` 为 `Waiting`。

配齐规则：队列中相同 `Mode` 且 `MaxSeats` 相同的玩家达到 `MaxSeats` 人数时，服务端创建房间，这些人的匹配状态变为 `Matched`，`RoomId` 和 `RoomCode` 写入匹配单。本期不做跨不同 `MaxSeats` 的撮合。客户端每 1 秒调用一次 `DescribeMatch`，最多 30 次。30 秒后仍是 `Waiting` 时停止自动轮询，匹配单保持 `Waiting`，玩家可以取消或再次查询。

`POST /den/v1/match/DescribeMatch`

请求：`MatchId`。结果：`Status`（`Waiting`、`Matched`、`Canceled`）、`RoomId`、`RoomCode`。未配齐时房间字段为空字符串。

`POST /den/v1/match/CancelMatch`

请求：`MatchId`。只有 `Waiting` 可以取消。

### dice

`POST /den/v1/dice/Roll`

请求：`RoundId`。服务端生成 5 颗 1 到 6 的骰子。同一局同一座位第二次调用返回 `300002`，不重新摇。

`POST /den/v1/dice/Call`

请求：`RoundId`、`Count`、`Face`。只用于 `LiarsDice`。`Face` 为 1 到 6，`Count` 至少为 1。

`POST /den/v1/dice/Open`

请求：`RoundId`。只用于 `LiarsDice`，表示开盅。

`POST /den/v1/dice/DescribeRound`

请求：`RoundId`。比大小在结算前，调用者只能看到自己的骰子。大话骰在开盅前，调用者只能看到自己的骰子；开盅后所有人看到全部骰子和判定。

### WebSocket 事件

客户端连接 `/den/v1/room/Subscribe` 后发送第一帧：

```json
{ "Token": "<token>", "RoomId": "<room-id>" }
```

服务端推送的 `Action`：

| Action | 何时 | Result 要点 |
| --- | --- | --- |
| `RoomChanged` | 座位、准备、房主变化 | 与 `DescribeRoom` 的 Result 相同 |
| `DiceHidden` | 某座位已摇完，且这些骰子对其他人保密 | `RoundId`、`SeatNo` |
| `DiceRolled` | 只推给摇骰的本人 | `RoundId`、`SeatNo`、`Dice` |
| `DiceCalled` | 有人叫点 | `RoundId`、`SeatNo`、`Count`、`Face` |
| `DiceOpened` | 开盅 | 全部骰子 |
| `RoundResolved` | 一局结束 | `RoundId`、`Outcome`、`WinnerSeat` |

`WinnerSeat` 在平局时为 `-1`。`Outcome` 只有 `Win` 和 `Draw`。`RoomChanged` 的 `Service` 是 `room`。骰子事件的 `Service` 是 `dice`。

## 数据流

1. Boot 在本地保存 `DeviceId`。没有则生成 UUID。调用 `GuestLogin`。
2. Hall 调用 `EnterHall`。
3. 建房：`CreateRoom` 写 PostgreSQL 的房间和 0 号座位，并写 Redis 热数据，返回 6 位房间号。
4. 加入：`JoinRoom` 在一个事务里占座。
5. 匹配：`StartMatch` 入 Redis 队列并返回 `MatchId`。客户端每 1 秒调用 `DescribeMatch`，最多 30 次。状态变为 `Matched` 后进入该房间。超时后匹配单仍是 `Waiting`。
6. 全员 `Ready` 后，房主 `StartGame`。
7. 进房后连接 Subscribe。
8. 比大小：每个在座玩家 `Roll`。到齐后服务端结算并写对局记录，推送 `RoundResolved`。
9. 大话骰：每人 `Roll`。到齐后按座位号从小到大叫点，0 号座位先叫。`Open` 后结算。

## 规则

### 比大小

每人 5 颗骰子。先比点数之和，大者胜。和相同，则把 5 颗骰子从大到小排序后按字典序比较，大者胜。仍然相同则平局，`WinnerSeat` 为 `-1`。本期不重摇。

### 大话骰

每人 5 颗骰子。1 到 6 都是普通点数，1 不是万能点，没有「斋」。

全部玩家摇完后才能叫点。在座座位号升序循环，0 号座位先行动。轮到的座位只能 `Call` 或 `Open`，其他座位调用返回 `300001`。

叫点必须严格大于上一手：`Count` 更大；或 `Count` 相同且 `Face` 更大。第一手的 `Count` 至少为 1。第一手之前不能 `Open`。

`Open` 针对上一手叫点。统计全桌该 `Face` 的颗数。实际颗数大于或等于叫点的 `Count` 时，叫点者胜。实际颗数小于 `Count` 时，开盅者胜。本期一局只结算一次，不扣血、不连庄。

## 错误码

| Code | Message | 含义 |
| --- | --- | --- |
| 100001 | System Error | 未处理异常。线上 Detail 为空 |
| 100002 | Invalid Parameter | 参数不合法。Detail 说明字段 |
| 100003 | Unauthorized | 缺少令牌、令牌无效或过期 |
| 200001 | Room Not Found | 房间不存在或已关闭 |
| 200002 | Room Full | 没有空座 |
| 200003 | Not In Room | 调用者不在该房间 |
| 200004 | Not Host | 只有房主可以开局 |
| 200005 | Not Ready | 不足 2 人，或有人未准备 |
| 300001 | Not Your Turn | 还没轮到该座位叫点或开盅 |
| 300002 | Illegal Action | 重复摇骰、模式不匹配，或叫点没有严格增大 |
| 400001 | Match Not Found | 匹配单不存在 |
| 400002 | Match Closed | 匹配已取消或已配齐，不能再取消 |
| 400003 | Already Matching | 该用户已有进行中的匹配 |

WebSocket 上的业务失败使用同一套 `Error` 对象。

## 存储

PostgreSQL 表：

- `users`：`user_id`、`device_id` 唯一、`nickname`、`created_at`
- `rooms`：`room_id`、`room_code`、`mode`、`max_seats`、`host_user_id`、`status`（`Open`、`Playing`、`Closed`）、`created_at`
- `room_seats`：`room_id`、`seat_no`、`user_id`、`ready`。同一房间的 `seat_no` 唯一，同一房间的 `user_id` 唯一
- `matches`：`match_id`、`user_id`、`mode`、`max_seats`、`status`、`room_id`、`created_at`
- `dice_rounds`：`round_id`、`room_id`、`mode`、`status`（`Rolling`、`Calling`、`Resolved`）、`winner_seat`、`outcome`、`created_at`。比大小到齐后直接从 `Rolling` 进入 `Resolved`。大话骰到齐后进入 `Calling`
- `dice_hands`：`round_id`、`seat_no`、`user_id`、`die_1` 到 `die_5`
- `dice_calls`：`round_id`、`seq`、`seat_no`、`count`、`face`。`seq` 从 1 递增。开盅不写入此表，开盅结果写在 `dice_rounds`

Redis 键：

- `den:token:{token}` → `user_id`，7 天
- `den:online:{user_id}` → 心跳时间，60 秒
- `den:room:{room_id}` → 热座位 JSON
- `den:match:queue:{mode}:{max_seats}` → 等待中的用户列表
- `den:match:user:{user_id}` → 当前 `match_id`

开发库和测试库分开。测试使用 testcontainers 启动自己的 PostgreSQL 和 Redis。

## 测试

服务端先写失败测试，再写实现。`go test ./...` 必须通过。覆盖：

- 成功和失败封套的字段
- 同一设备号登录得到同一个 `UserId`
- 房间占座事务：满员返回 `200002`，同一用户不能占两座
- 比大小的和、字典序和平局
- 大话骰的叫点必须严格增大，开盅判定与「1 不是万能点」一致
- 同一局同一座位第二次 `Roll` 返回 `300002`，骰子不变

客户端本期手测四条：

1. 游客进入大厅
2. 另一名玩家用房间号入座
3. 两名玩家通过快速匹配进入同一房间
4. 打完一局比大小，双方看到同一个胜负结果

大话骰以服务端单测为通过标准，不进入这四条手测。

## 后续子项目

这些子项目各自再走设计、计划和实现，不放进本期计划：

1. 大话骰客户端场景
2. 谁是卧底
3. 狼人杀
4. 正式账号绑定

实现计划确认后，按任务边界派专家代理。共享同一批接口的任务顺序执行。接口稳定之后，客户端场景和服务端规则测试可以并行。本期不在设计评审通过前派代理写代码。
