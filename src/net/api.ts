import type { HttpClient } from "./http";
import type {
  DescribeRecordResult,
  ListRecordResult,
  LoginResult,
  PullEventResult,
  RoomView,
  SessionView,
  Snapshot,
  SubmitResult,
} from "./types";

export function createApi(http: HttpClient) {
  return {
    describeHealth: () => http.call<{ Status: string }>("session", "DescribeHealth", {}, false),

    loginByWechat: (input: { Code: string; Nickname?: string; Avatar?: string }) =>
      http.call<LoginResult>("session", "LoginByWechat", input, false),

    describeSession: () => http.call<SessionView>("session", "DescribeSession", {}),

    createRoom: (input: {
      GameCode: string;
      Variant?: string;
      Config?: Record<string, unknown>;
      MaxSeats?: number;
    }) => http.call<RoomView>("room", "CreateRoom", input),

    updateRoom: (input: {
      RoomId: string;
      GameCode?: string;
      Variant?: string;
      Config?: Record<string, unknown>;
      MaxSeats?: number;
    }) => http.call<RoomView>("room", "UpdateRoom", input),

    joinRoom: (input: { Code: string }) => http.call<RoomView>("room", "JoinRoom", input),

    listRoom: () => http.call<RoomView[]>("room", "ListRoom", {}),

    describeRoom: (input: { RoomId: string }) => http.call<RoomView>("room", "DescribeRoom", input),

    setReady: (input: { RoomId: string; Ready: boolean }) =>
      http.call<RoomView>("room", "SetReady", input),

    leaveRoom: (input: { RoomId: string }) => http.call<string>("room", "LeaveRoom", input),

    kickMember: (input: { RoomId: string; UserId: number }) =>
      http.call<RoomView>("room", "KickMember", input),

    transferHost: (input: { RoomId: string; UserId: number }) =>
      http.call<RoomView>("room", "TransferHost", input),

    deleteRoom: (input: { RoomId: string }) => http.call<string>("room", "DeleteRoom", input),

    startPlay: (input: { RoomId: string }) => http.call<Snapshot>("play", "StartPlay", input),

    describeSnapshot: (input: { RoomId?: string; PlayId: string }) =>
      http.call<Snapshot>("play", "DescribeSnapshot", input),

    listLegalAction: (input: { RoomId?: string; PlayId: string }) =>
      http.call<{ PlayId: string; Seat: number; Legal: Snapshot["Legal"] }>(
        "play",
        "ListLegalAction",
        input,
      ),

    pullEvent: (input: { PlayId: string; Cursor: number; Limit?: number }) =>
      http.call<PullEventResult>("play", "PullEvent", input),

    submitAction: (input: {
      RoomId?: string;
      PlayId: string;
      Seat?: number;
      ClientSeq: number;
      Action: { Type: string; Payload?: Record<string, unknown> };
    }) => http.call<SubmitResult>("play", "SubmitAction", input),

    listRecord: (input: { GameCode?: string; Page?: number; PageSize?: number } = {}) =>
      http.call<ListRecordResult>("record", "ListRecord", input),

    describeRecord: (input: { PlayId: string }) =>
      http.call<DescribeRecordResult>("record", "DescribeRecord", input),
  };
}

export type Api = ReturnType<typeof createApi>;
