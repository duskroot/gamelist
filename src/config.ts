/** Runtime config. Empty VITE_API_BASE means same-origin `/table` via Vite proxy. */
export const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined)?.replace(/\/$/, "") ?? "";

export const PULL_INTERVAL_MS = 1000;
export const ROOM_POLL_MS = 2500;
export const STORAGE_TOKEN = "kaiju.token";
export const STORAGE_NICK = "kaiju.nickname";
export const STORAGE_CODE = "kaiju.devCode";
