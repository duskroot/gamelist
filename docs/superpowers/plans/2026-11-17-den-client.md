# den-client Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a Cocos Creator 3.8 mobile client that logs in as a guest, enters a hall, joins a high-roll table by room code or by a 2-seat quick match, and shows the server's dice result.

**Architecture:** Plain TypeScript in `client/src` owns the session. It talks to the existing `den-api` with POST `/den/v1/{service}/{Action}` and one WebSocket at `/den/v1/room/Subscribe`. Cocos scripts in `client/cocos/assets/scripts` only render that session. Node tests fake the transport, so the session is verified without the Creator editor.

**Tech Stack:** Node.js 24 with `node --experimental-strip-types --test`. TypeScript kept as source, no compile step for the tests. Cocos Creator 3.8 for the scene scripts. The live server is the `den-api` already on `main`, default `http://127.0.0.1:8080`.

## Global Constraints

- Product module is `den`. Business path is `/den/v1/{service}/{Action}`. This client calls only `account`, `hall`, `room`, `match`, and `dice`.
- Business commands are POST. The WebSocket URL is `/den/v1/room/Subscribe`. Its first frame is `{"Token":"<token>","RoomId":"<room-id>"}`.
- Handled failures are HTTP 200 with `ResponseMetadata` and `Error`, and no `Result`. The client treats any body that contains `Error` as a failure, including HTTP 500 `Code` `100001`.
- `GuestLogin` sends `DeviceId` and optional `Nickname`. The same device id is stored and reused. The token from `Result.Token` is sent as `Authorization: Bearer <token>` on every later call.
- This client sends `Mode` `HighRoll` only. It does not send `LiarsDice`, and it does not render call or open controls.
- Create-room omits `MaxSeats`, so the server default of 4 applies. Quick match sends `MaxSeats` 2, because two players must be able to match. The server only matches equal `MaxSeats`.
- After `StartMatch`, the client calls `DescribeMatch` once per second, at most 30 times. A `Waiting` result after 30 calls stops the poll and leaves the match open. `Matched` carries `RoomId` and `RoomCode`.
- Dice values shown to the player come from `Roll` or `DescribeRound`. The client never invents pips. Before `Revealed` is true, it shows only `YourDice`. `WinnerSeat` `-1` with `Outcome` `Draw` is a draw.
- JSON fields use the server's PascalCase names: `UserId`, `Token`, `Nickname`, `RoomId`, `RoomCode`, `SeatNo`, `RoundId`, `MatchId`, `Status`, `Seats`, `YourDice`, `Hands`, `Outcome`, `WinnerSeat`.
- Do not read, modify, or commit `/opt/game/.env`. Do not change git config. Commit with `git -c user.name='den-api' -c user.email='den-api@localhost'`.
- Do not copy 猛兽派对 names, art, or text. This plan uses text labels, not a downloaded art pack. Kenney assets are not required for the automated tests.
- Scenes in the Creator project are only Boot, Hall, Room, and DiceTable. The automated gate is `node --experimental-strip-types --test`. The four manual checks run in Creator when it is installed. If the `CocosCreator` binary is absent, record that in the task report and do not claim the manual checks passed.

---

## File structure

- `client/package.json` — `type: module`, no dependencies.
- `client/src/api.ts` — envelope reader and path constants.
- `client/src/api.test.ts` — envelope tests.
- `client/src/session.ts` — guest, hall, room, match, roll, socket.
- `client/src/session.test.ts` — fake transport tests.
- `client/cocos/assets/scripts/DenRoot.ts` — Creator component that calls `Session`.
- `client/cocos/assets/scripts/den-scenes.md` is not created. Scene node names live in `DenRoot.ts`.

The server under `server/` is not modified.

## Shared types

```ts
export type Store = { get(key: string): string | null; set(key: string, value: string): void }
export type Transport = {
  post(path: string, body: unknown, token?: string): Promise<unknown>
  connect(url: string): Promise<GameSocket>
}
export type GameSocket = {
  send(text: string): void
  onMessage(handler: (text: string) => void): void
  close(): void
}
```

`Session` is constructed as `new Session(transport, store, { httpBase, wsBase, sleep, now })`. Tests pass `sleep: async () => {}`. The default `sleep` waits the requested milliseconds.

---

### Task 1: API envelope

**Files:**
- Create: `client/package.json`
- Create: `client/src/api.ts`
- Create: `client/src/api.test.ts`

**Interfaces:**
- Consumes: the live server's JSON envelope
- Produces: `readResult<T>(body: unknown): T`, `ApiError`, and `paths`

- [ ] **Step 1: Write the failing test**

Create `client/package.json`:

```json
{
  "name": "den-client",
  "private": true,
  "type": "module"
}
```

Create `client/src/api.test.ts`:

```ts
import assert from 'node:assert/strict'
import test from 'node:test'
import { ApiError, paths, readResult } from './api.ts'

test('readResult returns Result', () => {
  const userId = readResult<{ UserId: string }>({
    ResponseMetadata: { RequestId: 'r', Action: 'GuestLogin', Version: 'v1', Service: 'account' },
    Result: { UserId: 'u1' },
  })
  assert.equal(userId.UserId, 'u1')
})

test('readResult throws the server Error and ignores any Result', () => {
  assert.throws(
    () => readResult({
      ResponseMetadata: { RequestId: 'r', Action: 'CreateRoom', Version: 'v1', Service: 'room' },
      Error: { Code: '100002', Message: 'Invalid Parameter', Detail: 'Mode' },
      Result: { RoomId: 'nope' },
    }),
    (err: unknown) => err instanceof ApiError && err.code === '100002' && err.message === 'Invalid Parameter',
  )
})

test('paths stay on the den v1 prefix', () => {
  assert.equal(paths.guestLogin, '/den/v1/account/GuestLogin')
  assert.equal(paths.roll, '/den/v1/dice/Roll')
  assert.equal(paths.subscribe, '/den/v1/room/Subscribe')
  assert.equal(JSON.stringify(paths).includes('LiarsDice'), false)
})
```

- [ ] **Step 2: Run test to verify it fails**

From `/opt/game/client`:

```bash
node --experimental-strip-types --test src/api.test.ts
```

Expected: FAIL because `./api.ts` does not exist.

- [ ] **Step 3: Write minimal implementation**

Create `client/src/api.ts`:

```ts
export class ApiError extends Error {
  constructor(readonly code: string, message: string, readonly detail = '') {
    super(message)
  }
}

type ErrorBody = { Code?: string; Message?: string; Detail?: string }

export function readResult<T>(body: unknown): T {
  if (!body || typeof body !== 'object') {
    throw new ApiError('100001', 'System Error', 'empty body')
  }
  const record = body as { Error?: ErrorBody; Result?: T }
  if (record.Error) {
    throw new ApiError(record.Error.Code ?? '100001', record.Error.Message ?? 'System Error', record.Error.Detail ?? '')
  }
  if (record.Result === undefined) {
    throw new ApiError('100001', 'System Error', 'missing Result')
  }
  return record.Result
}

export const paths = {
  guestLogin: '/den/v1/account/GuestLogin',
  updateNickname: '/den/v1/account/UpdateNickname',
  enterHall: '/den/v1/hall/EnterHall',
  createRoom: '/den/v1/room/CreateRoom',
  joinRoom: '/den/v1/room/JoinRoom',
  leaveRoom: '/den/v1/room/LeaveRoom',
  ready: '/den/v1/room/Ready',
  startGame: '/den/v1/room/StartGame',
  describeRoom: '/den/v1/room/DescribeRoom',
  startMatch: '/den/v1/match/StartMatch',
  describeMatch: '/den/v1/match/DescribeMatch',
  cancelMatch: '/den/v1/match/CancelMatch',
  roll: '/den/v1/dice/Roll',
  describeRound: '/den/v1/dice/DescribeRound',
  subscribe: '/den/v1/room/Subscribe',
} as const
```

- [ ] **Step 4: Run test to verify it passes**

```bash
node --experimental-strip-types --test src/api.test.ts
```

Expected: 3 tests pass.

- [ ] **Step 5: Commit**

From `/opt/game`:

```bash
git add client/package.json client/src/api.ts client/src/api.test.ts
git -c user.name='den-api' -c user.email='den-api@localhost' commit -m "$(cat <<'EOF'
feat: add den client envelope reader

EOF
)"
```

---

### Task 2: Guest boot

**Files:**
- Create: `client/src/session.ts`
- Create: `client/src/session.test.ts`

**Interfaces:**
- Consumes: `readResult`, `paths.guestLogin`, `paths.enterHall`
- Produces: `Session.boot(nickname: string)`, `Session.enterHall()`, `session.userId`, `session.nickname`, `session.token`

- [ ] **Step 1: Write the failing test**

Create `client/src/session.test.ts`:

```ts
import assert from 'node:assert/strict'
import test from 'node:test'
import { Session, MemoryStore, type Transport } from './session.ts'

function scripted(responses: unknown[]): Transport & { posts: { path: string; body: unknown; token?: string }[] } {
  const posts: { path: string; body: unknown; token?: string }[] = []
  return {
    posts,
    async post(path, body, token) {
      posts.push({ path, body, token })
      const next = responses.shift()
      if (next instanceof Error) throw next
      return next
    },
    async connect() {
      throw new Error('no socket in this test')
    },
  }
}

test('boot stores the device id and reuses it', async () => {
  const store = new MemoryStore()
  const transport = scripted([
    { Result: { UserId: 'u1', Token: 'tok-1', Nickname: '玩家甲乙' } },
    { Result: { UserId: 'u1', Token: 'tok-2', Nickname: '玩家甲乙' } },
  ])
  const first = new Session(transport, store, { httpBase: 'http://127.0.0.1:8080', wsBase: 'ws://127.0.0.1:8080' })
  await first.boot('玩家甲乙')
  const deviceId = store.get('den.deviceId')
  assert.equal(transport.posts[0].path, '/den/v1/account/GuestLogin')
  assert.equal(transport.posts[0].token, undefined)
  assert.equal((transport.posts[0].body as { Nickname: string }).Nickname, '玩家甲乙')
  const second = new Session(transport, store, { httpBase: 'http://127.0.0.1:8080', wsBase: 'ws://127.0.0.1:8080' })
  await second.boot('')
  assert.equal((transport.posts[1].body as { DeviceId: string }).DeviceId, deviceId)
  assert.equal(second.token, 'tok-2')
})

test('enterHall sends the bearer token', async () => {
  const store = new MemoryStore()
  const transport = scripted([
    { Result: { UserId: 'u1', Token: 'tok-1', Nickname: '玩家甲乙' } },
    { Result: { UserId: 'u1', Nickname: '玩家甲乙' } },
  ])
  const session = new Session(transport, store, { httpBase: 'http://127.0.0.1:8080', wsBase: 'ws://127.0.0.1:8080' })
  await session.boot('玩家甲乙')
  await session.enterHall()
  assert.equal(transport.posts[1].path, '/den/v1/hall/EnterHall')
  assert.equal(transport.posts[1].token, 'tok-1')
  assert.deepEqual(transport.posts[1].body, {})
})
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /opt/game/client && node --experimental-strip-types --test src/session.test.ts
```

Expected: FAIL because `./session.ts` does not exist.

- [ ] **Step 3: Write minimal implementation**

Create `client/src/session.ts` with `MemoryStore`, `Session.boot`, and `Session.enterHall`.

`boot` reads `den.deviceId`. If missing, set it to `crypto.randomUUID()` before the request. POST `paths.guestLogin` with `{ DeviceId, Nickname }` and no token. An empty nickname is sent as `""` so the server applies its default. Save `Result.Token` in the session and in `den.token`. Save `UserId` and `Nickname` on the session.

`enterHall` POSTs `paths.enterHall` with `{}` and the current token. It refreshes `nickname` from `Result.Nickname`.

Both methods pass the response through `readResult`. A server `Error` therefore rejects the promise.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /opt/game/client && node --experimental-strip-types --test src/session.test.ts src/api.test.ts
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add client/src/session.ts client/src/session.test.ts
git -c user.name='den-api' -c user.email='den-api@localhost' commit -m "$(cat <<'EOF'
feat: boot the den client as a guest

EOF
)"
```

---

### Task 3: Room code flow

**Files:**
- Modify: `client/src/session.ts`
- Modify: `client/src/session.test.ts`

**Interfaces:**
- Consumes: `Session.boot`
- Produces: `createRoom()`, `joinRoom(code: string)`, `setReady(ready: boolean)`, `startGame()`, `describeRoom()`, and fields `roomId`, `roomCode`, `seatNo`, `roundId`, `seats`

- [ ] **Step 1: Write the failing test**

Add this test to `client/src/session.test.ts`. Keep the Task 2 tests.

```ts
test('host creates a high-roll room and the guest joins by code', async () => {
  const store = new MemoryStore()
  const transport = scripted([
    { Result: { UserId: 'host', Token: 'host-token', Nickname: '房主' } },
    { Result: { RoomId: 'room-1', RoomCode: '123456', SeatNo: 0 } },
    { Result: { RoomId: 'room-1', RoomCode: '123456', Mode: 'HighRoll', MaxSeats: 4, HostUserId: 'host', Status: 'Open', RoundId: '', Seats: [
      { SeatNo: 0, UserId: 'host', Nickname: '房主', Ready: false },
      { SeatNo: 1, UserId: 'guest', Nickname: '客人', Ready: true },
    ] } },
    { Result: { RoomId: 'room-1', Ready: true } },
    { Result: { RoundId: 'round-1' } },
  ])
  const session = new Session(transport, store, { httpBase: 'http://127.0.0.1:8080', wsBase: 'ws://127.0.0.1:8080' })
  await session.boot('房主')
  await session.createRoom()
  assert.equal(transport.posts[1].path, '/den/v1/room/CreateRoom')
  assert.deepEqual(transport.posts[1].body, { Mode: 'HighRoll' })
  assert.equal(JSON.stringify(transport.posts[1].body).includes('LiarsDice'), false)
  assert.equal(session.roomCode, '123456')
  await session.describeRoom()
  assert.equal(session.seats.length, 2)
  await session.setReady(true)
  assert.deepEqual(transport.posts[3].body, { RoomId: 'room-1', Ready: true })
  const roundId = await session.startGame()
  assert.equal(roundId, 'round-1')
  assert.equal(transport.posts[4].path, '/den/v1/room/StartGame')
})
```

Add a second test that boots a guest, calls `joinRoom('123456')`, and expects POST `/den/v1/room/JoinRoom` with `{ RoomCode: '123456' }` and `seatNo` 1.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /opt/game/client && node --experimental-strip-types --test src/session.test.ts
```

Expected: FAIL because `createRoom` is not a function.

- [ ] **Step 3: Write minimal implementation**

Add the five methods. `createRoom` posts `{ Mode: 'HighRoll' }` only. `joinRoom` posts `{ RoomCode: code }`. `setReady` and `startGame` and `describeRoom` post `{ RoomId: this.roomId }`, and `setReady` also sends `Ready`. `describeRoom` replaces `seats` with `Result.Seats` and stores `roomCode`, `roundId`, and `hostUserId`. `startGame` stores `roundId`.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /opt/game/client && node --experimental-strip-types --test src/session.test.ts
```

Expected: PASS, including the Task 2 tests.

- [ ] **Step 5: Commit**

```bash
git add client/src/session.ts client/src/session.test.ts
git -c user.name='den-api' -c user.email='den-api@localhost' commit -m "$(cat <<'EOF'
feat: join a den room by code

EOF
)"
```

---

### Task 4: Two-seat quick match

**Files:**
- Modify: `client/src/session.ts`
- Modify: `client/src/session.test.ts`

**Interfaces:**
- Consumes: `Session.boot`
- Produces: `startQuickMatch(): Promise<'Matched' | 'Waiting'>`. On `Matched`, `roomId` and `roomCode` are set. `matchId` is set in both cases.

- [ ] **Step 1: Write the failing test**

```ts
test('quick match asks for two seats and polls until matched', async () => {
  const store = new MemoryStore()
  let describes = 0
  const transport = scripted([])
  transport.post = async (path, body, token) => {
    transport.posts.push({ path, body, token })
    if (path.endsWith('GuestLogin')) return { Result: { UserId: 'u', Token: 't', Nickname: '甲' } }
    if (path.endsWith('StartMatch')) return { Result: { MatchId: 'm1', Status: 'Waiting' } }
    describes += 1
    if (describes < 3) return { Result: { MatchId: 'm1', Status: 'Waiting', RoomId: '', RoomCode: '' } }
    return { Result: { MatchId: 'm1', Status: 'Matched', RoomId: 'room-9', RoomCode: '654321' } }
  }
  let sleeps = 0
  const session = new Session(transport, store, {
    httpBase: 'http://127.0.0.1:8080',
    wsBase: 'ws://127.0.0.1:8080',
    sleep: async (ms) => { sleeps += 1; assert.equal(ms, 1000) },
  })
  await session.boot('甲')
  const status = await session.startQuickMatch()
  assert.equal(status, 'Matched')
  assert.deepEqual(transport.posts[1].body, { Mode: 'HighRoll', MaxSeats: 2 })
  assert.equal(describes, 3)
  assert.equal(sleeps, 2)
  assert.equal(session.roomCode, '654321')
})

test('quick match stops after 30 waits', async () => {
  const store = new MemoryStore()
  const transport = scripted([])
  let describes = 0
  transport.post = async (path, body, token) => {
    transport.posts.push({ path, body, token })
    if (path.endsWith('GuestLogin')) return { Result: { UserId: 'u', Token: 't', Nickname: '甲' } }
    if (path.endsWith('StartMatch')) return { Result: { MatchId: 'm2', Status: 'Waiting' } }
    describes += 1
    return { Result: { MatchId: 'm2', Status: 'Waiting', RoomId: '', RoomCode: '' } }
  }
  const session = new Session(transport, store, {
    httpBase: 'http://127.0.0.1:8080',
    wsBase: 'ws://127.0.0.1:8080',
    sleep: async () => {},
  })
  await session.boot('甲')
  const status = await session.startQuickMatch()
  assert.equal(status, 'Waiting')
  assert.equal(describes, 30)
  assert.equal(session.roomId, '')
})
```

The first test replaces `transport.post` after `scripted([])`. That replacement must still push onto `transport.posts`. Implement `scripted` so the returned object keeps a stable `posts` array.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /opt/game/client && node --experimental-strip-types --test src/session.test.ts
```

Expected: FAIL because `startQuickMatch` is missing.

- [ ] **Step 3: Write minimal implementation**

`startQuickMatch` posts `{ Mode: 'HighRoll', MaxSeats: 2 }` to `paths.startMatch`. Then it calls `paths.describeMatch` with `{ MatchId }` up to 30 times. Between attempts, after a `Waiting` result and before the next call, it `await sleep(1000)`. Do not sleep after the 30th wait and do not sleep after `Matched`. On `Matched`, set `roomId` and `roomCode`. On timeout, return `'Waiting'` and leave `roomId` as `''`.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /opt/game/client && node --experimental-strip-types --test src/session.test.ts
```

Expected: PASS. The 30-wait test finishes immediately because `sleep` is a no-op.

- [ ] **Step 5: Commit**

```bash
git add client/src/session.ts client/src/session.test.ts
git -c user.name='den-api' -c user.email='den-api@localhost' commit -m "$(cat <<'EOF'
feat: poll a two-seat quick match

EOF
)"
```

---

### Task 5: High-roll result

**Files:**
- Modify: `client/src/session.ts`
- Modify: `client/src/session.test.ts`

**Interfaces:**
- Consumes: `roundId` from `startGame`
- Produces: `roll(): Promise<number[]>`, `refreshRound(): Promise<void>`, fields `yourDice`, `outcome`, `winnerSeat`, `revealed`, `hands`

- [ ] **Step 1: Write the failing test**

```ts
test('roll shows only server dice and a draw', async () => {
  const store = new MemoryStore()
  const transport = scripted([
    { Result: { UserId: 'u', Token: 't', Nickname: '甲' } },
    { Result: { RoomId: 'room-1', RoomCode: '123456', SeatNo: 0 } },
    { Result: { RoundId: 'round-1' } },
    { Result: { RoundId: 'round-1', SeatNo: 0, Dice: [1, 1, 1, 1, 1] } },
    { Result: { RoundId: 'round-1', Mode: 'HighRoll', Status: 'Resolved', Outcome: 'Draw', WinnerSeat: -1, YourSeat: 0, YourDice: [1, 1, 1, 1, 1], Revealed: true, Hands: [
      { SeatNo: 0, Dice: [1, 1, 1, 1, 1] },
      { SeatNo: 1, Dice: [1, 1, 1, 1, 1] },
    ] } },
  ])
  const session = new Session(transport, store, { httpBase: 'http://127.0.0.1:8080', wsBase: 'ws://127.0.0.1:8080' })
  await session.boot('甲')
  await session.createRoom()
  await session.startGame()
  const dice = await session.roll()
  assert.deepEqual(dice, [1, 1, 1, 1, 1])
  assert.deepEqual(transport.posts[3].body, { RoundId: 'round-1' })
  await session.refreshRound()
  assert.equal(session.outcome, 'Draw')
  assert.equal(session.winnerSeat, -1)
  assert.equal(session.revealed, true)
  assert.equal(session.hands.length, 2)
})
```

Add a test where `DescribeRound` returns `Revealed: false` and `Hands: []`. After `refreshRound`, `hands` is empty and `yourDice` is the server array. The test fails if the client copies the other seat's dice from anywhere else.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /opt/game/client && node --experimental-strip-types --test src/session.test.ts
```

Expected: FAIL because `roll` is missing.

- [ ] **Step 3: Write minimal implementation**

`roll` posts `{ RoundId: this.roundId }` to `paths.roll` and sets `yourDice` from `Result.Dice`. `refreshRound` posts the same id to `paths.describeRound` and copies `YourDice`, `Outcome`, `WinnerSeat`, `Revealed`, and `Hands`. If `Hands` is missing or null, store `[]`. Do not sort or reroll the numbers.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /opt/game/client && node --experimental-strip-types --test src/session.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add client/src/session.ts client/src/session.test.ts
git -c user.name='den-api' -c user.email='den-api@localhost' commit -m "$(cat <<'EOF'
feat: show the server high-roll result

EOF
)"
```

---

### Task 6: Room socket and Creator binding

**Files:**
- Modify: `client/src/session.ts`
- Modify: `client/src/session.test.ts`
- Create: `client/cocos/assets/scripts/DenRoot.ts`
- Create: `client/src/cocos-entry.test.ts`

**Interfaces:**
- Consumes: `token`, `roomId`, `GameSocket`
- Produces: `connectRoom()`, which sends the first frame and applies `RoomChanged`, `DiceRolled`, and `RoundResolved`. `DenRoot` calls only `HighRoll` session methods.

- [ ] **Step 1: Write the failing test**

Add to `session.test.ts`:

```ts
test('socket sends the token frame and applies room and dice events', async () => {
  const store = new MemoryStore()
  let sent = ''
  let onMessage: (text: string) => void = () => {}
  const transport = scripted([
    { Result: { UserId: 'u', Token: 'tok', Nickname: '甲' } },
    { Result: { RoomId: 'room-1', RoomCode: '123456', SeatNo: 0 } },
  ])
  transport.connect = async (url) => {
    assert.equal(url, 'ws://127.0.0.1:8080/den/v1/room/Subscribe')
    return {
      send(text) { sent = text },
      onMessage(handler) { onMessage = handler },
      close() {},
    }
  }
  const session = new Session(transport, store, { httpBase: 'http://127.0.0.1:8080', wsBase: 'ws://127.0.0.1:8080' })
  await session.boot('甲')
  await session.createRoom()
  await session.connectRoom()
  assert.deepEqual(JSON.parse(sent), { Token: 'tok', RoomId: 'room-1' })
  onMessage(JSON.stringify({
    ResponseMetadata: { RequestId: 'r', Action: 'RoomChanged', Version: 'v1', Service: 'room' },
    Result: { RoomId: 'room-1', RoomCode: '123456', Mode: 'HighRoll', MaxSeats: 4, HostUserId: 'u', Status: 'Open', RoundId: 'round-1', Seats: [
      { SeatNo: 0, UserId: 'u', Nickname: '甲', Ready: true },
    ] },
  }))
  assert.equal(session.seats[0].Ready, true)
  assert.equal(session.roundId, 'round-1')
  onMessage(JSON.stringify({
    ResponseMetadata: { RequestId: 'r2', Action: 'RoundResolved', Version: 'v1', Service: 'dice' },
    Result: { RoundId: 'round-1', Outcome: 'Win', WinnerSeat: 0 },
  }))
  assert.equal(session.outcome, 'Win')
  assert.equal(session.winnerSeat, 0)
})
```

Create `client/src/cocos-entry.test.ts`:

```ts
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

test('creator entry only offers high roll', () => {
  const src = readFileSync(new URL('../cocos/assets/scripts/DenRoot.ts', import.meta.url), 'utf8')
  assert.equal(src.includes('LiarsDice'), false)
  assert.equal(src.includes('Call'), false)
  assert.equal(src.includes('Open'), false)
  for (const name of ['boot', 'enterHall', 'createRoom', 'joinRoom', 'startQuickMatch', 'setReady', 'startGame', 'roll', 'connectRoom']) {
    assert.equal(src.includes(name), true, name)
  }
})
```

The `Call` and `Open` assertions are exact identifiers. Do not put those words in comments in `DenRoot.ts`. Dice display text may say `点数`.

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /opt/game/client && node --experimental-strip-types --test src/session.test.ts src/cocos-entry.test.ts
```

Expected: FAIL because `connectRoom` and `DenRoot.ts` are missing.

- [ ] **Step 3: Write minimal implementation**

`connectRoom` opens `wsBase + paths.subscribe`, sends `JSON.stringify({ Token: this.token, RoomId: this.roomId })`, and registers `onMessage`. A message with `Error` is stored on `session.lastError` and does not clear the room. `RoomChanged` replaces seats, room code, round id, and status from `Result`. `DiceRolled` replaces `yourDice` when `Result.SeatNo === this.seatNo`. `RoundResolved` sets `outcome` and `winnerSeat`. Ignore `DiceHidden`.

Create `client/cocos/assets/scripts/DenRoot.ts`. It imports `Session` from `../../../src/session.ts` and `{ _decorator, Component, Label } from 'cc'`. `onLoad` builds a session with `fetchTransport` and `browserStore` from `session.ts`, then its public methods call `boot`, `enterHall`, `createRoom`, `joinRoom`, `startQuickMatch`, `setReady(true)`, `startGame`, `roll`, and `connectRoom`. `render()` writes five labels: nickname, room code, seat list, your dice, and outcome. Outcome text is `平局` when `winnerSeat === -1`, otherwise `座位 ${winnerSeat} 胜`. Export `fetchTransport` and `browserStore` from `session.ts`. `fetchTransport.post` uses `fetch(httpBase + path)`, sets `Content-Type: application/json`, sets `Authorization` only when token is non-empty, and returns `response.json()`. `browserStore` uses `globalThis.localStorage` when it exists.

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /opt/game/client && node --experimental-strip-types --test src/api.test.ts src/session.test.ts src/cocos-entry.test.ts
```

Expected: PASS. `DenRoot.ts` is not executed, so the `cc` import is not loaded.

Manual checks, only if `CocosCreator` is on `PATH`. Start Postgres and Redis with `docker compose -f /opt/game/server/docker-compose.yml up -d`, then run `/opt/game/server` with `go run ./cmd/den-api`. In Creator 3.8, open `client/cocos` after creating a project around these scripts, and confirm:

1. Two previews boot as guests and reach the hall.
2. One creates a room and the other joins with the 6-digit code.
3. Both quick-match with two seats and land in one room.
4. Both ready, the host starts, both roll, and both show the same win or `平局`.

If `CocosCreator` is absent, write that sentence in the task report. Do not mark those four checks passed.

- [ ] **Step 5: Commit**

```bash
git add client/src client/cocos/assets/scripts/DenRoot.ts
git -c user.name='den-api' -c user.email='den-api@localhost' commit -m "$(cat <<'EOF'
feat: bind the den hall to the room socket

EOF
)"
```

Do not stage `/opt/game/.env`.

---

## Spec coverage

| Spec requirement | Task |
| --- | --- |
| Boot saves `DeviceId` and calls `GuestLogin` | 2 |
| Hall calls `EnterHall` | 2 |
| Create and join by 6-digit code, client sends only `HighRoll` | 3 |
| Ready and start | 3 |
| Quick match, 1 second poll, stop after 30 waits | 4 |
| Two players can match because `MaxSeats` is 2 | 4 |
| Server dice, hidden hands, draw is `WinnerSeat` `-1` | 5 |
| WebSocket first frame and `RoomChanged` / `RoundResolved` | 6 |
| Creator scenes Boot, Hall, Room, DiceTable | 6, as labels inside `DenRoot` until Creator is installed |
| Liar's dice scene | out of this plan |
| Kenney art | not required for these tests |

## Self-review

- `createRoom` does not send `MaxSeats`. `startQuickMatch` sends `2`. Those are different on purpose.
- `DescribeMatch` is called 30 times in the timeout test, with a sleep only between a wait and the next call. That is 29 sleeps, but the matched test sleeps twice for three describes. The timeout test does not assert the sleep count.
- `DenRoot.ts` must not contain the identifiers `LiarsDice`, `Call`, or `Open`.
