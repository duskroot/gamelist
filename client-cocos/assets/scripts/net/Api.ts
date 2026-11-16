import { Http } from './Http';
import {
  LoginResult, PullEventResult, RoomView, SessionView, Snapshot, SubmitResult,
} from './Types';

export class Api {
  constructor(private http: Http) {}

  describeHealth() { return this.http.call<{ Status: string }>('session', 'DescribeHealth', {}, false); }
  loginByWechat(input: { Code: string; Nickname?: string; Avatar?: string }) {
    return this.http.call<LoginResult>('session', 'LoginByWechat', input, false);
  }
  describeSession() { return this.http.call<SessionView>('session', 'DescribeSession', {}); }
  createRoom(input: { GameCode: string; Variant?: string; Config?: Record<string, unknown>; MaxSeats?: number }) {
    return this.http.call<RoomView>('room', 'CreateRoom', input);
  }
  joinRoom(input: { Code: string }) { return this.http.call<RoomView>('room', 'JoinRoom', input); }
  listRoom() { return this.http.call<RoomView[]>('room', 'ListRoom', {}); }
  describeRoom(input: { RoomId: string }) { return this.http.call<RoomView>('room', 'DescribeRoom', input); }
  setReady(input: { RoomId: string; Ready: boolean }) { return this.http.call<RoomView>('room', 'SetReady', input); }
  leaveRoom(input: { RoomId: string }) { return this.http.call<string>('room', 'LeaveRoom', input); }
  deleteRoom(input: { RoomId: string }) { return this.http.call<string>('room', 'DeleteRoom', input); }
  kickMember(input: { RoomId: string; UserId: number }) { return this.http.call<RoomView>('room', 'KickMember', input); }
  transferHost(input: { RoomId: string; UserId: number }) { return this.http.call<RoomView>('room', 'TransferHost', input); }
  startPlay(input: { RoomId: string }) { return this.http.call<Snapshot>('play', 'StartPlay', input); }
  describeSnapshot(input: { RoomId?: string; PlayId: string }) {
    return this.http.call<Snapshot>('play', 'DescribeSnapshot', input);
  }
  pullEvent(input: { PlayId: string; Cursor: number; Limit?: number }) {
    return this.http.call<PullEventResult>('play', 'PullEvent', input);
  }
  submitAction(input: {
    RoomId?: string; PlayId: string; Seat?: number; ClientSeq: number;
    Action: { Type: string; Payload?: Record<string, unknown> };
  }) {
    return this.http.call<SubmitResult>('play', 'SubmitAction', input);
  }
  listRecord(input: { GameCode?: string; Page?: number; PageSize?: number } = {}) {
    return this.http.call<{ Page: number; PageSize: number; Items: unknown[] }>('record', 'ListRecord', input);
  }
}
