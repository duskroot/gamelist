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
    code = `h5_${crypto.randomUUID().replace(/-/g, "").slice(0, 16)}`;
    localStorage.setItem(STORAGE_CODE, code);
  }
  return code;
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
