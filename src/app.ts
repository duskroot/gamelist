import { createApi } from "./net/api";
import { createHttp } from "./net/http";
import { ApiError } from "./net/types";
import {
  clearToken,
  getLoginCode,
  loadNickname,
  loadToken,
  saveNickname,
  saveToken,
} from "./platform/auth";
import { PlayLoop } from "./play/loop";
import { GAMES, gameByCode, gameName } from "./games/meta";
import { formatEvent, renderPrivate, renderPublic } from "./games/render";
import {
  applyLogin,
  applySession,
  beginPlay,
  getState,
  logoutLocal,
  patch,
  resetPlay,
  setBusy,
  setRoom,
  setRooms,
  setScreen,
  showToast,
  subscribe,
} from "./state/store";
import { ROOM_POLL_MS } from "./config";

type CreateForm = {
  gameCode: string;
  variant: string;
  maxSeats: number;
  configText: string;
};

export function bootApp(root: HTMLElement): void {
  let createForm: CreateForm = {
    gameCode: "dice",
    variant: "classic",
    maxSeats: 4,
    configText: JSON.stringify({ Count: 2 }, null, 0),
  };
  let joinCode = "";
  let roomPoll = 0;
  let deadlineTimer = 0;

  const http = createHttp({
    getToken: () => getState().token,
    onUnauthorized: () => {
      clearToken();
      logoutLocal();
      showToast("登录已失效，请重新进入", "error");
    },
  });
  const api = createApi(http);
  const playLoop = new PlayLoop(api);

  async function withBusy<T>(fn: () => Promise<T>): Promise<T | undefined> {
    setBusy(true);
    try {
      return await fn();
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : "操作失败";
      showToast(msg, "error");
      patch({ lastError: msg });
      return undefined;
    } finally {
      setBusy(false);
    }
  }

  async function bootstrap(): Promise<void> {
    patch({ bootMessage: "检查牌桌服务…" });
    try {
      await api.describeHealth();
    } catch {
      patch({
        screen: "login",
        bootMessage: "后端未就绪，仍可尝试登录（请确认 Vite 代理与 table-server）",
      });
      render();
      return;
    }

    const token = loadToken();
    if (!token) {
      patch({ screen: "login", bootMessage: "" });
      return;
    }
    patch({ token });
    const ok = await withBusy(async () => {
      const session = await api.describeSession();
      applySession(session);
      await refreshRooms();
    });
    if (!ok) {
      clearToken();
      logoutLocal();
    }
    render();
  }

  async function doLogin(nickname: string): Promise<void> {
    saveNickname(nickname);
    await withBusy(async () => {
      const code = await getLoginCode();
      const result = await api.loginByWechat({ Code: code, Nickname: nickname || undefined });
      saveToken(result.Token);
      applyLogin(result, result.Token);
      showToast(`欢迎，${result.Nickname || "玩家"}`, "ok");
      await refreshRooms();
    });
  }

  async function refreshRooms(): Promise<void> {
    const rooms = await api.listRoom();
    setRooms(rooms || []);
  }

  async function enterRoom(roomId: string): Promise<void> {
    await withBusy(async () => {
      const room = await api.describeRoom({ RoomId: roomId });
      setRoom(room);
      setScreen("room");
      startRoomPoll(room.RoomId);
      if (room.Status === "playing" && room.PlayId) {
        const snap = await api.describeSnapshot({ RoomId: room.RoomId, PlayId: room.PlayId });
        beginPlay(snap);
        playLoop.start(snap.PlayId, room.RoomId);
      }
    });
  }

  function startRoomPoll(roomId: string): void {
    stopRoomPoll();
    roomPoll = window.setInterval(() => {
      void (async () => {
        try {
          const room = await api.describeRoom({ RoomId: roomId });
          setRoom(room);
          const st = getState();
          if (room.Status === "playing" && room.PlayId && st.screen === "room") {
            const snap = await api.describeSnapshot({ RoomId: room.RoomId, PlayId: room.PlayId });
            beginPlay(snap);
            playLoop.start(snap.PlayId, room.RoomId);
          }
          if (room.Status === "open" && st.screen === "play" && st.snapshot?.Settled) {
            // after settle, room returns to open
            playLoop.stop();
          }
        } catch {
          // ignore
        }
      })();
    }, ROOM_POLL_MS);
  }

  function stopRoomPoll(): void {
    window.clearInterval(roomPoll);
    roomPoll = 0;
  }

  function stopPlay(): void {
    playLoop.stop();
    resetPlay();
  }

  function countdownText(deadline?: string): string {
    if (!deadline) return "—";
    const ms = Date.parse(deadline) - Date.now();
    if (Number.isNaN(ms)) return deadline;
    if (ms <= 0) return "0s";
    return `${Math.ceil(ms / 1000)}s`;
  }

  function render(): void {
    const st = getState();
    root.innerHTML = "";
    const shell = document.createElement("div");
    shell.className = "shell";

    if (st.screen === "boot") {
      shell.innerHTML = `
        <section class="brand-hero">
          <div class="brand-mark">开局</div>
          <p class="brand-sub">${st.bootMessage}</p>
        </section>`;
    } else if (st.screen === "login") {
      shell.append(renderLogin(st.bootMessage));
    } else {
      shell.append(renderTopbar());
      if (st.screen === "lobby") shell.append(renderLobby());
      if (st.screen === "room") shell.append(renderRoom());
      if (st.screen === "play") shell.append(renderPlay());
      if (st.screen === "records") shell.append(renderRecords());
    }

    root.append(shell, busyNode(st.busy), toastNode(st.toast));
  }

  function renderTopbar(): HTMLElement {
    const st = getState();
    const bar = document.createElement("div");
    bar.className = "topbar";
    bar.innerHTML = `
      <div>
        <div class="brand-mini">开局</div>
        <div class="muted">${st.session?.Nickname || "玩家"} · #${st.session?.UserId ?? "-"}</div>
      </div>
      <div class="row">
        <button class="btn ghost" data-nav="lobby">大厅</button>
        <button class="btn ghost" data-nav="records">战绩</button>
        <button class="btn ghost" data-act="logout">退出</button>
      </div>`;
    return bar;
  }

  function renderLogin(hint: string): HTMLElement {
    const panel = document.createElement("div");
    panel.innerHTML = `
      <section class="brand-hero">
        <div class="brand-mark">开局</div>
        <p class="brand-sub">先玩 H5，再迁 Cocos / 微信小游戏。服务端权威发牌，客户端只渲染快照与事件。</p>
      </section>
      <section class="panel stack">
        <h2>进入牌桌</h2>
        ${hint ? `<p class="muted">${hint}</p>` : ""}
        <label class="field">昵称
          <input id="nick" maxlength="32" placeholder="怎么称呼你" value="${escapeAttr(loadNickname())}" />
        </label>
        <button class="btn" data-act="login">开始玩</button>
        <p class="muted">H5 用本地固定 Code 模拟微信登录；同一设备会回到同一用户。</p>
      </section>`;
    return panel;
  }

  function renderLobby(): HTMLElement {
    const st = getState();
    const meta = gameByCode(createForm.gameCode)!;
    const wrap = document.createElement("div");
    wrap.className = "stack";
    wrap.innerHTML = `
      <section class="panel">
        <h2>创建房间</h2>
        <div class="game-grid">
          ${GAMES.map(
            (g) => `
            <button class="game-tile ${g.code === createForm.gameCode ? "active" : ""}" data-game="${g.code}">
              <strong>${g.name}</strong>
              <span>${g.blurb}<br>${g.min}–${g.max} 人</span>
            </button>`,
          ).join("")}
        </div>
        <div class="row" style="margin-top:14px">
          <label class="field grow">变体
            <select id="variant">
              ${meta.variants.map((v) => `<option value="${v}" ${v === createForm.variant ? "selected" : ""}>${v}</option>`).join("")}
            </select>
          </label>
          <label class="field" style="width:120px">人数上限
            <input id="maxSeats" type="number" min="${meta.min}" max="${meta.max}" value="${createForm.maxSeats}" />
          </label>
        </div>
        <label class="field" style="margin-top:10px">Config（JSON）
          <textarea id="configText">${escapeHtml(createForm.configText)}</textarea>
        </label>
        <div class="row" style="margin-top:12px">
          <button class="btn" data-act="create">创建并进入</button>
        </div>
      </section>
      <section class="panel">
        <h2>加入房间</h2>
        <div class="row">
          <label class="field grow">6 位房间号
            <input id="joinCode" maxlength="6" inputmode="numeric" placeholder="123456" value="${escapeAttr(joinCode)}" />
          </label>
          <button class="btn mint" data-act="join">加入</button>
        </div>
      </section>
      <section class="panel">
        <div class="row" style="justify-content:space-between">
          <h2 style="margin:0">我的房间</h2>
          <button class="btn ghost" data-act="refreshRooms">刷新</button>
        </div>
        <div class="stack" style="margin-top:12px">
          ${(st.rooms || [])
            .map(
              (r) => `
            <div class="member">
              <div>
                <strong>${gameName(r.GameCode)}</strong>
                <div class="muted">#${r.Code} · ${r.Status} · ${r.Members.length}/${r.MaxSeats}</div>
              </div>
              <button class="btn ghost" data-enter="${r.RoomId}">进入</button>
            </div>`,
            )
            .join("") || `<div class="muted">还没有房间，创建一个吧</div>`}
        </div>
      </section>`;
    return wrap;
  }

  function renderRoom(): HTMLElement {
    const st = getState();
    const room = st.room;
    if (!room) {
      const empty = document.createElement("div");
      empty.className = "panel";
      empty.textContent = "房间不存在";
      return empty;
    }
    const me = room.Members.find((m) => m.UserId === st.session?.UserId);
    const isHost = room.HostUserId === st.session?.UserId;
    const wrap = document.createElement("div");
    wrap.className = "stack";
    wrap.innerHTML = `
      <section class="panel">
        <div class="row" style="justify-content:space-between">
          <div>
            <h2 style="margin:0">${gameName(room.GameCode)}</h2>
            <div class="muted">房间号 <strong style="color:var(--amber);font-size:1.2rem">${room.Code}</strong> · ${room.Status} · ${room.Variant}</div>
          </div>
          <button class="btn ghost" data-act="backLobby">返回大厅</button>
        </div>
        <div class="member-list" style="margin-top:14px">
          ${room.Members.map((m) => {
            const host = m.UserId === room.HostUserId;
            return `
              <div class="member ${m.UserId === st.session?.UserId ? "me" : ""}">
                <div class="row">
                  <span class="seat">${m.Seat}</span>
                  <div>
                    <strong>${escapeHtml(m.Nickname || `用户${m.UserId}`)}</strong>
                    <div class="row" style="margin-top:4px">
                      ${host ? `<span class="badge warn">房主</span>` : ""}
                      ${m.Ready ? `<span class="badge ok">已准备</span>` : `<span class="badge">未准备</span>`}
                      ${m.Away ? `<span class="badge">离座</span>` : ""}
                    </div>
                  </div>
                </div>
                <div class="row">
                  ${
                    isHost && m.UserId !== st.session?.UserId && room.Status === "open"
                      ? `<button class="btn ghost" data-kick="${m.UserId}">踢出</button>
                         <button class="btn ghost" data-host="${m.UserId}">移交</button>`
                      : ""
                  }
                </div>
              </div>`;
          }).join("")}
        </div>
        <div class="row" style="margin-top:14px">
          ${
            room.Status === "open"
              ? `<button class="btn mint" data-act="ready">${me?.Ready ? "取消准备" : "准备"}</button>`
              : ""
          }
          ${isHost && room.Status === "open" ? `<button class="btn" data-act="start">开始游戏</button>` : ""}
          ${room.Status === "playing" && room.PlayId ? `<button class="btn" data-act="resume">进入对局</button>` : ""}
          <button class="btn ghost" data-act="leave">离开</button>
          ${isHost ? `<button class="btn danger" data-act="deleteRoom">解散</button>` : ""}
        </div>
      </section>`;
    return wrap;
  }

  function renderPlay(): HTMLElement {
    const st = getState();
    const snap = st.snapshot;
    const room = st.room;
    const wrap = document.createElement("div");
    wrap.className = "stack";
    if (!snap) {
      wrap.innerHTML = `<section class="panel">没有对局快照<button class="btn ghost" data-act="backRoom">返回房间</button></section>`;
      return wrap;
    }

    const legal = snap.Legal || [];
    wrap.innerHTML = `
      <section class="panel">
        <div class="phase-bar">
          <div>
            <h2 style="margin:0">${gameName(snap.GameCode)}</h2>
            <div class="muted">阶段 ${snap.Phase}${snap.Settled ? " · 已结算" : ""} · 我是 ${snap.Seat} 号</div>
          </div>
          <div class="row">
            <span class="badge deadline">倒计时 ${countdownText(snap.Deadline)}</span>
            <button class="btn ghost" data-act="backRoom">房间</button>
          </div>
        </div>
        <div class="felt-table">
          <div class="public-board">${renderPublic(snap, room?.Members)}</div>
          <div class="private-box">
            <div class="muted" style="margin-bottom:6px">仅自己可见</div>
            ${renderPrivate(snap)}
          </div>
        </div>
        <div class="action-bar">
          ${
            snap.Settled
              ? `<button class="btn mint" data-act="afterSettle">回到房间再开一局</button>`
              : legal.length
                ? legal.map((a) => actionButton(a.Type, a.Hint || a.Type)).join("")
                : `<span class="muted">当前不能出手，等待其他玩家或服务端超时代操作</span>`
          }
        </div>
      </section>
      <section class="panel">
        <h3>事件</h3>
        <div class="event-log">
          ${[...st.events].reverse().map((e) => `<div>${escapeHtml(formatEvent(e))}</div>`).join("") || `<div class="muted">暂无事件</div>`}
        </div>
      </section>
      <div id="actionModalHost"></div>`;
    return wrap;
  }

  function actionButton(type: string, label: string): string {
    const needsPayload = ["Bet", "Compare", "Grab", "Speak", "Vote", "Knife", "Check", "Potion"].includes(type);
    if (!needsPayload) {
      return `<button class="btn" data-submit="${type}">${escapeHtml(label)}</button>`;
    }
    return `<button class="btn" data-compose="${type}">${escapeHtml(label)}</button>`;
  }

  function renderRecords(): HTMLElement {
    const wrap = document.createElement("div");
    wrap.className = "panel stack";
    wrap.innerHTML = `
      <div class="row" style="justify-content:space-between">
        <h2 style="margin:0">我的战绩</h2>
        <button class="btn ghost" data-act="loadRecords">刷新</button>
      </div>
      <div id="recordList" class="stack"><div class="muted">点击刷新加载</div></div>`;
    return wrap;
  }

  function busyNode(busy: boolean): HTMLElement {
    const n = document.createElement("div");
    n.className = `busy-overlay${busy ? " on" : ""}`;
    return n;
  }

  function toastNode(toast: ReturnType<typeof getState>["toast"]): HTMLElement {
    const n = document.createElement("div");
    if (!toast) return n;
    n.className = `toast ${toast.tone}`;
    n.textContent = toast.text;
    return n;
  }

  function bindEvents(): void {
    root.addEventListener("click", (event) => {
      const target = (event.target as HTMLElement | null)?.closest<HTMLElement>(
        "[data-act],[data-nav],[data-game],[data-enter],[data-kick],[data-host],[data-submit],[data-compose],[data-record]",
      );
      if (!target) return;

      if (target.dataset.nav) {
        const nav = target.dataset.nav;
        if (nav === "lobby") {
          stopPlay();
          stopRoomPoll();
          setScreen("lobby");
          void refreshRooms();
        } else if (nav === "records") {
          setScreen("records");
          void loadRecords();
        }
        return;
      }

      if (target.dataset.game) {
        const code = target.dataset.game;
        const meta = gameByCode(code)!;
        createForm = {
          gameCode: code,
          variant: meta.variants[0],
          maxSeats: Math.min(Math.max(createForm.maxSeats, meta.min), meta.max),
          configText: JSON.stringify(meta.defaultConfig),
        };
        render();
        return;
      }

      if (target.dataset.enter) {
        void enterRoom(target.dataset.enter);
        return;
      }
      if (target.dataset.kick) {
        void withBusy(async () => {
          const room = await api.kickMember({
            RoomId: getState().room!.RoomId,
            UserId: Number(target.dataset.kick),
          });
          setRoom(room);
        });
        return;
      }
      if (target.dataset.host) {
        void withBusy(async () => {
          const room = await api.transferHost({
            RoomId: getState().room!.RoomId,
            UserId: Number(target.dataset.host),
          });
          setRoom(room);
          showToast("已移交房主", "ok");
        });
        return;
      }
      if (target.dataset.submit) {
        void playLoop.submit(target.dataset.submit);
        return;
      }
      if (target.dataset.compose) {
        openActionComposer(target.dataset.compose);
        return;
      }
      if (target.dataset.record) {
        void withBusy(async () => {
          const detail = await api.describeRecord({ PlayId: target.dataset.record! });
          showToast(
            detail.Items.map((i) => `${i.Seat}号 ${i.Delta > 0 ? "+" : ""}${i.Delta}`).join(" / "),
            "info",
          );
        });
        return;
      }

      switch (target.dataset.act) {
        case "login": {
          const nick = root.querySelector<HTMLInputElement>("#nick")?.value.trim() || "";
          void doLogin(nick);
          break;
        }
        case "logout":
          playLoop.stop();
          stopRoomPoll();
          clearToken();
          logoutLocal();
          showToast("已退出", "info");
          break;
        case "create":
          void withBusy(async () => {
            let config: Record<string, unknown> = {};
            try {
              config = createForm.configText.trim() ? JSON.parse(createForm.configText) : {};
            } catch {
              throw new ApiError("300013", "Config 不是合法 JSON");
            }
            const room = await api.createRoom({
              GameCode: createForm.gameCode,
              Variant: createForm.variant,
              MaxSeats: createForm.maxSeats,
              Config: config,
            });
            setRoom(room);
            setScreen("room");
            startRoomPoll(room.RoomId);
            showToast(`房间 ${room.Code} 已创建`, "ok");
            await refreshRooms();
          });
          break;
        case "join":
          void withBusy(async () => {
            const code = root.querySelector<HTMLInputElement>("#joinCode")?.value.trim() || joinCode;
            if (!/^\d{6}$/.test(code)) throw new ApiError("300013", "请输入 6 位房间号");
            const room = await api.joinRoom({ Code: code });
            setRoom(room);
            setScreen("room");
            startRoomPoll(room.RoomId);
            showToast("已加入房间", "ok");
          });
          break;
        case "refreshRooms":
          void withBusy(async () => {
            await refreshRooms();
            showToast("已刷新", "ok");
          });
          break;
        case "backLobby":
          stopPlay();
          stopRoomPoll();
          setRoom(null);
          setScreen("lobby");
          void refreshRooms();
          break;
        case "backRoom": {
          const room = getState().room;
          if (room?.Status === "open") {
            playLoop.stop();
            resetPlay();
          }
          setScreen("room");
          break;
        }
        case "ready":
          void withBusy(async () => {
            const room = getState().room!;
            const me = room.Members.find((m) => m.UserId === getState().session?.UserId);
            const next = await api.setReady({ RoomId: room.RoomId, Ready: !me?.Ready });
            setRoom(next);
            showToast(
              next.Members.find((m) => m.UserId === getState().session?.UserId)?.Ready
                ? "已准备"
                : "已取消准备",
              "ok",
            );
          });
          break;
        case "start":
          void withBusy(async () => {
            const room = getState().room!;
            const snap = await api.startPlay({ RoomId: room.RoomId });
            const freshRoom = await api.describeRoom({ RoomId: room.RoomId });
            setRoom(freshRoom);
            beginPlay(snap);
            playLoop.start(snap.PlayId, room.RoomId);
            showToast("对局开始", "ok");
          });
          break;
        case "resume":
          void withBusy(async () => {
            const room = getState().room!;
            const snap = await api.describeSnapshot({ RoomId: room.RoomId, PlayId: room.PlayId });
            beginPlay(snap);
            playLoop.start(snap.PlayId, room.RoomId);
          });
          break;
        case "leave":
          void withBusy(async () => {
            const room = getState().room!;
            await api.leaveRoom({ RoomId: room.RoomId });
            stopPlay();
            stopRoomPoll();
            setRoom(null);
            setScreen("lobby");
            await refreshRooms();
            showToast("已离开房间", "info");
          });
          break;
        case "deleteRoom":
          void withBusy(async () => {
            const room = getState().room!;
            await api.deleteRoom({ RoomId: room.RoomId });
            stopPlay();
            stopRoomPoll();
            setRoom(null);
            setScreen("lobby");
            await refreshRooms();
            showToast("房间已解散", "info");
          });
          break;
        case "afterSettle":
          playLoop.stop();
          resetPlay();
          setScreen("room");
          {
            const room = getState().room;
            if (room) void enterRoom(room.RoomId);
          }
          break;
        case "loadRecords":
          void loadRecords();
          break;
        case "confirmAction": {
          const host = root.querySelector("#actionModalHost");
          if (!host) break;
          const type = host.getAttribute("data-pending-type") || "";
          const payload = readComposePayload(type, host);
          host.innerHTML = "";
          host.removeAttribute("data-pending-type");
          void playLoop.submit(type, payload);
          break;
        }
        case "cancelAction": {
          const host = root.querySelector("#actionModalHost");
          if (host) {
            host.innerHTML = "";
            host.removeAttribute("data-pending-type");
          }
          break;
        }
        default:
          break;
      }
    });

    root.addEventListener("change", (event) => {
      const el = event.target as HTMLElement;
      if (el.id === "variant") createForm.variant = (el as HTMLSelectElement).value;
      if (el.id === "maxSeats") createForm.maxSeats = Number((el as HTMLInputElement).value) || createForm.maxSeats;
      if (el.id === "configText") createForm.configText = (el as HTMLTextAreaElement).value;
    });

    root.addEventListener("input", (event) => {
      const el = event.target as HTMLElement;
      if (el.id === "joinCode") joinCode = (el as HTMLInputElement).value.trim();
      if (el.id === "configText") createForm.configText = (el as HTMLTextAreaElement).value;
    });
  }

  function openActionComposer(type: string): void {
    const host = root.querySelector("#actionModalHost");
    if (!host) return;
    const snap = getState().snapshot!;
    const room = getState().room;
    const seats = room?.Members.map((m) => m.Seat) || [];
    const fields = composeFields(type, seats, snap);
    host.setAttribute("data-pending-type", type);
    host.innerHTML = `
      <div class="modal-backdrop">
        <div class="modal stack">
          <h3>${type}</h3>
          ${fields}
          <div class="row">
            <button class="btn" data-act="confirmAction">确认</button>
            <button class="btn ghost" data-act="cancelAction">取消</button>
          </div>
        </div>
      </div>`;
  }

  function composeFields(type: string, seats: number[], snap: NonNullable<ReturnType<typeof getState>["snapshot"]>): string {
    switch (type) {
      case "Bet": {
        const stake = Number(snap.Public?.Stake ?? 1);
        return `<label class="field">金额（≥ ${stake}）<input id="f_amount" type="number" min="${stake}" value="${stake}" /></label>`;
      }
      case "Compare":
      case "Vote":
      case "Knife":
      case "Check":
        return `<label class="field">目标座位
          <select id="f_target">${seats
            .filter((s) => s !== snap.Seat || type === "Knife" || type === "Check")
            .map((s) => `<option value="${s}">${s} 号</option>`)
            .join("")}</select>
        </label>`;
      case "Grab":
        return `<label class="field">倍数 0–3
          <input id="f_multiple" type="number" min="0" max="3" value="1" />
        </label>`;
      case "Speak":
        return `<label class="field">发言（1–40字）
          <textarea id="f_text" maxlength="40" placeholder="说点什么"></textarea>
        </label>`;
      case "Potion":
        return `
          <label class="field">是否使用解药
            <select id="f_save"><option value="false">否</option><option value="true">是</option></select>
          </label>
          <label class="field">毒药目标（-1 表示不用）
            <input id="f_poison" type="number" value="-1" />
          </label>`;
      default:
        return `<div class="muted">无额外参数</div>`;
    }
  }

  function readComposePayload(type: string, host: Element): Record<string, unknown> {
    switch (type) {
      case "Bet":
        return { Amount: Number(host.querySelector<HTMLInputElement>("#f_amount")?.value || 0) };
      case "Compare":
      case "Vote":
      case "Knife":
      case "Check":
        return { TargetSeat: Number(host.querySelector<HTMLSelectElement>("#f_target")?.value || 0) };
      case "Grab":
        return { Multiple: Number(host.querySelector<HTMLInputElement>("#f_multiple")?.value || 0) };
      case "Speak":
        return { Text: host.querySelector<HTMLTextAreaElement>("#f_text")?.value.trim() || "" };
      case "Potion":
        return {
          Save: host.querySelector<HTMLSelectElement>("#f_save")?.value === "true",
          Poison: Number(host.querySelector<HTMLInputElement>("#f_poison")?.value || -1),
        };
      default:
        return {};
    }
  }

  async function loadRecords(): Promise<void> {
    await withBusy(async () => {
      const list = await api.listRecord({ Page: 1, PageSize: 20 });
      const box = root.querySelector("#recordList");
      if (!box) return;
      box.innerHTML =
        list.Items?.map((item) => {
          const cls = item.Delta > 0 ? "delta-pos" : item.Delta < 0 ? "delta-neg" : "";
          return `<div class="record-item">
            <div class="row" style="justify-content:space-between">
              <strong>${gameName(item.GameCode)}</strong>
              <span class="${cls}">${item.Delta > 0 ? "+" : ""}${item.Delta}</span>
            </div>
            <div class="muted">${item.CreatedAt} · 座位 ${item.Seat} · ${item.PlayId}</div>
            <button class="btn ghost" data-record="${item.PlayId}">详情</button>
          </div>`;
        }).join("") || `<div class="muted">暂无战绩</div>`;

    });
  }

  bindEvents();
  subscribe(() => {
    render();
  });

  window.clearInterval(deadlineTimer);
  deadlineTimer = window.setInterval(() => {
    if (getState().screen === "play" && getState().snapshot?.Deadline) {
      const node = root.querySelector(".deadline");
      if (node) node.textContent = `倒计时 ${countdownText(getState().snapshot?.Deadline)}`;
    }
  }, 250);

  void bootstrap();
}

function escapeHtml(s: string): string {
  return s
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function escapeAttr(s: string): string {
  return escapeHtml(s).replaceAll("'", "&#39;");
}
