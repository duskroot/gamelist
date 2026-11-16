export type Envelope<T> = {
  ResponseMetadata: {
    RequestId: string;
    Action: string;
    Version: string;
    Service: string;
  };
  Result?: T;
  Error?: {
    Code: string;
    Message: string;
    Detail?: string;
  };
};

export type LoginResult = {
  Token: string;
  UserId: number;
  Nickname: string;
  OpenId: string;
  Avatar?: string;
};

export type SessionView = {
  UserId: number;
  Nickname: string;
  Avatar: string;
  OpenId: string;
};

export type MemberView = {
  Seat: number;
  UserId: number;
  Nickname: string;
  Ready: boolean;
  Away: boolean;
};

export type RoomView = {
  RoomId: string;
  Code: string;
  HostUserId: number;
  GameCode: string;
  Variant: string;
  Status: "open" | "playing" | "closed" | string;
  Config: Record<string, unknown>;
  MaxSeats: number;
  PlayId: string;
  Members: MemberView[];
};

export type Card = { Rank: number; Suit: number };

export type LegalAction = {
  Type: string;
  Hint?: string;
};

export type Snapshot = {
  PlayId: string;
  RoomId: string;
  GameCode: string;
  Variant: string;
  Seat: number;
  Cursor: number;
  Phase: string;
  Settled: boolean;
  Deadline?: string;
  Public: Record<string, unknown>;
  Private: Record<string, unknown>;
  Legal: LegalAction[];
};

export type PlayEvent = {
  Seq: number;
  Type: string;
  Payload: Record<string, unknown>;
};

export type PullEventResult = {
  PlayId: string;
  Cursor: number;
  Events: PlayEvent[];
};

export type SubmitResult = {
  PlayId: string;
  Cursor: number;
  Idempotent: boolean;
};

export type RecordItem = {
  PlayId: string;
  RoomId: string;
  GameCode: string;
  Variant: string;
  Seat: number;
  Delta: number;
  Detail: Record<string, unknown>;
  CreatedAt: string;
};

export type ListRecordResult = {
  Page: number;
  PageSize: number;
  Items: RecordItem[];
};

export type DescribeRecordResult = {
  PlayId: string;
  RoomId: string;
  GameCode: string;
  Variant: string;
  Items: RecordItem[];
};

export class ApiError extends Error {
  code: string;
  detail: string;
  requestId: string;

  constructor(code: string, message: string, detail = "", requestId = "") {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.detail = detail;
    this.requestId = requestId;
  }
}
