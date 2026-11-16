import { ApiError, Envelope } from './Types';

export type HttpOptions = {
  baseUrl: string;
  getToken: () => string;
  onUnauthorized?: () => void;
};

export class Http {
  constructor(private opts: HttpOptions) {}

  async call<T>(service: string, action: string, body: unknown = {}, needAuth = true): Promise<T> {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (needAuth) {
      const token = this.opts.getToken();
      if (!token) throw new ApiError('200001', '未登录或 Token 失效');
      headers.Authorization = `Bearer ${token}`;
    }
    const url = `${this.opts.baseUrl.replace(/\/$/, '')}/table/v1/${service}/${action}`;
    let response: Response;
    try {
      response = await fetch(url, { method: 'POST', headers, body: JSON.stringify(body ?? {}) });
    } catch (err) {
      throw new ApiError('100001', '网络不可用', err instanceof Error ? err.message : String(err));
    }
    const envelope = (await response.json()) as Envelope<T>;
    const requestId = envelope.ResponseMetadata?.RequestId ?? '';
    if (envelope.Error) {
      if (envelope.Error.Code === '200001') this.opts.onUnauthorized?.();
      throw new ApiError(envelope.Error.Code, envelope.Error.Message, envelope.Error.Detail ?? '', requestId);
    }
    return envelope.Result as T;
  }
}
