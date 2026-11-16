import { API_BASE } from "../config";
import { ApiError, type Envelope } from "./types";

export type HttpClientOptions = {
  getToken: () => string | null;
  onUnauthorized?: () => void;
};

export function createHttp(opts: HttpClientOptions) {
  async function call<T>(service: string, action: string, body: unknown = {}, needAuth = true): Promise<T> {
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (needAuth) {
      const token = opts.getToken();
      if (!token) {
        throw new ApiError("200001", "未登录或 Token 失效");
      }
      headers.Authorization = `Bearer ${token}`;
    }

    const url = `${API_BASE}/table/v1/${service}/${action}`;
    let response: Response;
    try {
      response = await fetch(url, {
        method: "POST",
        headers,
        body: JSON.stringify(body ?? {}),
      });
    } catch (err) {
      throw new ApiError("100001", "网络不可用", err instanceof Error ? err.message : String(err));
    }

    let envelope: Envelope<T>;
    try {
      envelope = (await response.json()) as Envelope<T>;
    } catch {
      throw new ApiError("100001", "响应不是合法 JSON", `HTTP ${response.status}`);
    }

    const requestId = envelope.ResponseMetadata?.RequestId ?? "";
    if (envelope.Error) {
      if (envelope.Error.Code === "200001") {
        opts.onUnauthorized?.();
      }
      throw new ApiError(
        envelope.Error.Code,
        envelope.Error.Message,
        envelope.Error.Detail ?? "",
        requestId,
      );
    }
    return envelope.Result as T;
  }

  return { call };
}

export type HttpClient = ReturnType<typeof createHttp>;
