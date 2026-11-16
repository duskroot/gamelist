import { sys } from 'cc';

const TOKEN = 'kaiju.token';
const NICK = 'kaiju.nickname';
const CODE = 'kaiju.devCode';

declare const wx: { login: (o: { success: (r: { code: string }) => void; fail: (e: unknown) => void }) => void } | undefined;

export function getLoginCode(): Promise<string> {
  if (typeof wx !== 'undefined' && wx?.login) {
    return new Promise((resolve, reject) => wx.login({ success: (r) => resolve(r.code), fail: reject }));
  }
  let code = sys.localStorage.getItem(CODE);
  if (!code) {
    code = `cocos_${Date.now().toString(36)}`;
    sys.localStorage.setItem(CODE, code);
  }
  return Promise.resolve(code);
}

export function loadToken() { return sys.localStorage.getItem(TOKEN) || ''; }
export function saveToken(token: string) { sys.localStorage.setItem(TOKEN, token); }
export function clearToken() { sys.localStorage.removeItem(TOKEN); }
export function loadNickname() { return sys.localStorage.getItem(NICK) || ''; }
export function saveNickname(n: string) { sys.localStorage.setItem(NICK, n.slice(0, 32)); }
