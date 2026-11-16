import type { Api } from "../net/api";
import { ApiError } from "../net/types";
import { PULL_INTERVAL_MS } from "../config";
import { play as playSfx } from "../audio/sfx";
import {
  appendEvents,
  getState,
  nextClientSeq,
  setRoom,
  setSnapshot,
  showToast,
} from "../state/store";

export class PlayLoop {
  private timer = 0;
  private pulling = false;
  private roomTimer = 0;

  constructor(private api: Api) {}

  start(playId: string, roomId: string): void {
    this.stop();
    const tick = async () => {
      if (this.pulling) return;
      this.pulling = true;
      try {
        const { cursor, snapshot } = getState();
        if (!snapshot || snapshot.PlayId !== playId) return;
        const pulled = await this.api.pullEvent({ PlayId: playId, Cursor: cursor, Limit: 100 });
        if (pulled.Events.length) {
          for (const ev of pulled.Events) {
            if (ev.Type === "Rolled") playSfx("dice");
            else if (ev.Type === "Bet") playSfx("chip");
            else if (ev.Type === "Settle") playSfx("win");
            else if (ev.Type === "Deal" || ev.Type === "Dealt" || ev.Type === "Drawn") playSfx("deal");
            else playSfx("slide");
          }
          appendEvents(pulled.Events, pulled.Cursor);
          const fresh = await this.api.describeSnapshot({ PlayId: playId, RoomId: roomId });
          setSnapshot(fresh);
        } else {
          appendEvents([], pulled.Cursor);
          // refresh deadline / legal even when quiet
          const fresh = await this.api.describeSnapshot({ PlayId: playId, RoomId: roomId });
          setSnapshot(fresh);
        }
      } catch (err) {
        if (err instanceof ApiError && err.code === "200001") return;
        // keep looping; surface soft error
        console.warn("pull failed", err);
      } finally {
        this.pulling = false;
      }
    };
    void tick();
    this.timer = window.setInterval(() => void tick(), PULL_INTERVAL_MS);

    this.roomTimer = window.setInterval(() => {
      void this.refreshRoom(roomId);
    }, 2000);
  }

  stop(): void {
    window.clearInterval(this.timer);
    window.clearInterval(this.roomTimer);
    this.timer = 0;
    this.roomTimer = 0;
  }

  async submit(type: string, payload?: Record<string, unknown>): Promise<boolean> {
    const { snapshot, room } = getState();
    if (!snapshot || snapshot.Settled) return false;
    const clientSeq = nextClientSeq();
    try {
      const result = await this.api.submitAction({
        RoomId: room?.RoomId || snapshot.RoomId,
        PlayId: snapshot.PlayId,
        Seat: snapshot.Seat,
        ClientSeq: clientSeq,
        Action: payload ? { Type: type, Payload: payload } : { Type: type },
      });
      if (result.Idempotent) {
        showToast("动作已确认（重放）", "info");
      }
      const fresh = await this.api.describeSnapshot({
        PlayId: snapshot.PlayId,
        RoomId: room?.RoomId || snapshot.RoomId,
      });
      setSnapshot(fresh);
      return true;
    } catch (err) {
      if (err instanceof ApiError) {
        if (err.code === "400202") {
          showToast("还没轮到你，正在刷新快照", "info");
          const fresh = await this.api.describeSnapshot({
            PlayId: snapshot.PlayId,
            RoomId: room?.RoomId || snapshot.RoomId,
          });
          setSnapshot(fresh);
        } else {
          showToast(err.message, "error");
        }
      } else {
        showToast("出手失败", "error");
      }
      return false;
    }
  }

  private async refreshRoom(roomId: string): Promise<void> {
    try {
      const room = await this.api.describeRoom({ RoomId: roomId });
      setRoom(room);
    } catch {
      // ignore soft poll errors
    }
  }
}
