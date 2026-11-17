import type { LoginResult, PlayEvent, RoomView, SessionView, Snapshot } from "../net/types";

export type Screen = "boot" | "login" | "lobby" | "room" | "play" | "records";

export type Toast = { id: number; text: string; tone: "info" | "error" | "ok" };

export type AppState = {
  screen: Screen;
  bootMessage: string;
  session: SessionView | null;
  token: string | null;
  rooms: RoomView[];
  room: RoomView | null;
  snapshot: Snapshot | null;
  events: PlayEvent[];
  clientSeq: number;
  cursor: number;
  busy: boolean;
  toast: Toast | null;
  lastError: string;
};

type Listener = (changed: (keyof AppState)[]) => void;

const state: AppState = {
  screen: "boot",
  bootMessage: "连接牌桌…",
  session: null,
  token: null,
  rooms: [],
  room: null,
  snapshot: null,
  events: [],
  clientSeq: 0,
  cursor: 0,
  busy: false,
  toast: null,
  lastError: "",
};

const listeners = new Set<Listener>();
let toastTimer = 0;
let toastSeq = 0;

export function getState(): AppState {
  return state;
}

export function subscribe(listener: Listener): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function emit(changed: (keyof AppState)[]): void {
  if (!changed.length) return;
  for (const listener of listeners) listener(changed);
}

function sameValue(a: unknown, b: unknown): boolean {
  if (Object.is(a, b)) return true;
  if (typeof a !== "object" || typeof b !== "object" || !a || !b) return false;
  try {
    return JSON.stringify(a) === JSON.stringify(b);
  } catch {
    return false;
  }
}

export function patch(partial: Partial<AppState>): void {
  const changed: (keyof AppState)[] = [];
  for (const key of Object.keys(partial) as (keyof AppState)[]) {
    const next = partial[key];
    if (sameValue(state[key], next)) continue;
    (state as Record<string, unknown>)[key] = next as unknown;
    changed.push(key);
  }
  emit(changed);
}

export function setScreen(screen: Screen): void {
  patch({ screen });
}

export function setBusy(busy: boolean): void {
  patch({ busy });
}

export function showToast(text: string, tone: Toast["tone"] = "info"): void {
  window.clearTimeout(toastTimer);
  const id = ++toastSeq;
  patch({ toast: { id, text, tone } });
  toastTimer = window.setTimeout(() => {
    if (state.toast?.id === id) patch({ toast: null });
  }, 2800);
}

export function applyLogin(result: LoginResult, token: string): void {
  patch({
    token,
    session: {
      UserId: result.UserId,
      Nickname: result.Nickname,
      Avatar: result.Avatar ?? "",
      OpenId: result.OpenId,
    },
    screen: "lobby",
  });
}

export function applySession(session: SessionView): void {
  patch({ session, screen: "lobby" });
}

export function setRooms(rooms: RoomView[]): void {
  patch({ rooms });
}

export function setRoom(room: RoomView | null): void {
  patch({ room });
}

export function beginPlay(snapshot: Snapshot): void {
  patch({
    screen: "play",
    snapshot,
    cursor: snapshot.Cursor ?? 0,
    events: [],
    clientSeq: 0,
  });
}

export function setSnapshot(snapshot: Snapshot): void {
  patch({ snapshot });
}

export function appendEvents(events: PlayEvent[], cursor: number): void {
  if (!events.length) {
    if (cursor !== state.cursor) patch({ cursor });
    return;
  }
  patch({
    cursor,
    events: [...state.events, ...events].slice(-80),
  });
}

export function nextClientSeq(): number {
  const next = state.clientSeq + 1;
  patch({ clientSeq: next });
  return next;
}

export function resetPlay(): void {
  patch({
    snapshot: null,
    events: [],
    clientSeq: 0,
    cursor: 0,
  });
}

export function logoutLocal(): void {
  patch({
    token: null,
    session: null,
    rooms: [],
    room: null,
    snapshot: null,
    events: [],
    clientSeq: 0,
    cursor: 0,
    screen: "login",
  });
}
