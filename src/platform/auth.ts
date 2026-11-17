import { STORAGE_CODE, STORAGE_NICK, STORAGE_TOKEN } from "../config";

/** H5 用本地 code 模拟 wx.login；微信小游戏里替换为真正的 wx.login。 */
export async function getLoginCode(): Promise<string> {
  const wx = (globalThis as { wx?: { login: (o: { success: (r: { code: string }) => void; fail: (e: unknown) => void }) => void } }).wx;
  if (wx?.login) {
    return new Promise((resolve, reject) => {
      wx.login({
        success: (res) => resolve(res.code),
        fail: reject,
      });
    });
  }

  let code = localStorage.getItem(STORAGE_CODE);
  if (!code) {
    // http://IP 非安全上下文没有 randomUUID，必须兜底，否则登录会直接抛错显示「操作失败」
    code = `h5_${makeDevCode()}`;
    localStorage.setItem(STORAGE_CODE, code);
  }
  return code;
}

function makeDevCode(): string {
  try {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
      return crypto.randomUUID().replace(/-/g, "").slice(0, 16);
    }
  } catch {
    /* insecure context */
  }
  try {
    if (typeof crypto !== "undefined" && crypto.getRandomValues) {
      const bytes = new Uint8Array(8);
      crypto.getRandomValues(bytes);
      return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
    }
  } catch {
    /* ignore */
  }
  return `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 10)}`;
}

export function loadToken(): string | null {
  return localStorage.getItem(STORAGE_TOKEN);
}

export function saveToken(token: string): void {
  localStorage.setItem(STORAGE_TOKEN, token);
}

export function clearToken(): void {
  localStorage.removeItem(STORAGE_TOKEN);
}

export function loadNickname(): string {
  return localStorage.getItem(STORAGE_NICK) || "";
}

export function saveNickname(nickname: string): void {
  localStorage.setItem(STORAGE_NICK, nickname.slice(0, 32));
}
