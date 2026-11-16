import { Api } from '../net/Api';
import { ApiError, Snapshot } from '../net/Types';
import { PULL_INTERVAL_MS } from '../Config';

export type PlayLoopHooks = {
  onSnapshot: (snap: Snapshot) => void;
  onToast?: (msg: string, tone?: 'info' | 'error' | 'ok') => void;
};

export class PlayLoop {
  private timer = 0;
  private pulling = false;
  private clientSeq = 0;
  private cursor = 0;
  private playId = '';
  private roomId = '';

  constructor(private api: Api, private hooks: PlayLoopHooks) {}

  start(playId: string, roomId: string, cursor = 0) {
    this.stop();
    this.playId = playId;
    this.roomId = roomId;
    this.cursor = cursor;
    this.clientSeq = 0;
    const tick = async () => {
      if (this.pulling || !this.playId) return;
      this.pulling = true;
      try {
        const pulled = await this.api.pullEvent({ PlayId: this.playId, Cursor: this.cursor, Limit: 100 });
        this.cursor = pulled.Cursor;
        const snap = await this.api.describeSnapshot({ PlayId: this.playId, RoomId: this.roomId });
        this.hooks.onSnapshot(snap);
      } catch (err) {
        if (!(err instanceof ApiError && err.code === '200001')) {
          console.warn('pull failed', err);
        }
      } finally {
        this.pulling = false;
      }
    };
    void tick();
    this.timer = setInterval(() => void tick(), PULL_INTERVAL_MS) as unknown as number;
  }

  stop() {
    clearInterval(this.timer);
    this.timer = 0;
    this.playId = '';
  }

  async submit(type: string, payload?: Record<string, unknown>, seat?: number) {
    if (!this.playId) return false;
    this.clientSeq += 1;
    const seq = this.clientSeq;
    try {
      await this.api.submitAction({
        RoomId: this.roomId,
        PlayId: this.playId,
        Seat: seat,
        ClientSeq: seq,
        Action: payload ? { Type: type, Payload: payload } : { Type: type },
      });
      const snap = await this.api.describeSnapshot({ PlayId: this.playId, RoomId: this.roomId });
      this.hooks.onSnapshot(snap);
      return true;
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : '出手失败';
      this.hooks.onToast?.(msg, 'error');
      if (err instanceof ApiError && err.code === '400202') {
        const snap = await this.api.describeSnapshot({ PlayId: this.playId, RoomId: this.roomId });
        this.hooks.onSnapshot(snap);
      }
      return false;
    }
  }
}
