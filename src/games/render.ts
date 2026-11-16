import type { Card, PlayEvent, Snapshot } from "../net/types";
import { formatCard, suitClass, tenhalfPoints } from "./cards";
import { gameName } from "./meta";

function el(html: string): string {
  return html;
}

function cardsHtml(cards: Card[] | undefined, faceDown = false): string {
  if (!cards?.length) return `<span class="muted">暂无牌</span>`;
  if (faceDown) {
    return `<div class="cards">${cards.map(() => `<div class="card back">?</div>`).join("")}</div>`;
  }
  return `<div class="cards">${cards
    .map(
      (c, i) =>
        `<div class="card ${suitClass(c.Suit)}" style="animation-delay:${i * 40}ms">${formatCard(c)}</div>`,
    )
    .join("")}</div>`;
}

function diceHtml(dice: number[] | undefined): string {
  if (!dice?.length) return "";
  return `<div class="cards">${dice
    .map((d, i) => `<div class="dice" style="animation-delay:${i * 50}ms">${d}</div>`)
    .join("")}</div>`;
}

function memberName(seat: number, roomMembers?: { Seat: number; Nickname: string }[]): string {
  const found = roomMembers?.find((m) => m.Seat === seat);
  return found?.Nickname || `${seat}号`;
}

export function renderPublic(snapshot: Snapshot, roomMembers?: { Seat: number; Nickname: string }[]): string {
  const p = snapshot.Public || {};
  const turn = Number(p.Turn ?? 0);
  switch (snapshot.GameCode) {
    case "dice": {
      const rolls = (p.Rolls as { Seat: number; Dice: number[]; Sum: number }[]) || [];
      return el(`
        <div class="row"><span class="badge">每人 ${p.Count ?? "?"} 颗</span>${turn ? `<span class="seat-chip turn">轮到 ${memberName(turn, roomMembers)}</span>` : ""}</div>
        <div class="stack">${rolls
          .map(
            (r) => `
          <div class="member">
            <div class="row"><span class="seat">${r.Seat}</span><strong>${memberName(r.Seat, roomMembers)}</strong></div>
            <div class="row">${diceHtml(r.Dice)}<strong>${r.Sum}</strong></div>
          </div>`,
          )
          .join("") || `<div class="muted">还没有人摇骰子</div>`}</div>
      `);
    }
    case "highcard": {
      const drawn = (p.Drawn as { Seat: number; Card: Card }[]) || [];
      return el(`
        <div class="row">${turn ? `<span class="seat-chip turn">轮到 ${memberName(turn, roomMembers)}</span>` : ""}</div>
        <div class="cards">${drawn
          .map(
            (d, i) => `
          <div>
            <div class="muted" style="margin-bottom:4px">${memberName(d.Seat, roomMembers)}</div>
            <div class="card ${suitClass(d.Card.Suit)}" style="animation-delay:${i * 40}ms">${formatCard(d.Card)}</div>
          </div>`,
          )
          .join("") || `<div class="muted">牌桌空着，等待抽牌</div>`}</div>
      `);
    }
    case "tenhalf": {
      const hands = (p.Hands as { Seat: number; Cards: Card[]; Sum: number; Bust: boolean; Done: boolean }[]) || [];
      return el(`
        <div class="row">
          <span class="badge">庄家 ${p.Dealer ?? "?"}</span>
          ${turn ? `<span class="seat-chip turn">轮到 ${memberName(turn, roomMembers)}</span>` : `<span class="badge">${snapshot.Phase}</span>`}
        </div>
        <div class="stack">${hands
          .map(
            (h) => `
          <div class="member ${h.Seat === turn ? "me" : ""}">
            <div>
              <div class="row"><span class="seat">${h.Seat}</span><strong>${memberName(h.Seat, roomMembers)}</strong>
                ${h.Seat === p.Dealer ? `<span class="badge warn">庄</span>` : ""}
                ${h.Bust ? `<span class="badge">爆</span>` : ""}
                ${h.Done ? `<span class="badge ok">停</span>` : ""}
              </div>
              ${cardsHtml(h.Cards)}
            </div>
            <strong>${tenhalfPoints(h.Sum)}</strong>
          </div>`,
          )
          .join("")}</div>
      `);
    }
    case "zhajinhua": {
      const chips = (p.Chips as { Seat: number; Chips: number }[]) || [];
      const folded = new Set((p.Folded as number[]) || []);
      const looked = new Set((p.Looked as number[]) || []);
      return el(`
        <div class="row">
          <span class="badge warn">底池 ${p.Pot ?? 0}</span>
          <span class="badge">当前注 ${p.Stake ?? 0}</span>
          ${turn ? `<span class="seat-chip turn">轮到 ${memberName(turn, roomMembers)}</span>` : ""}
        </div>
        <div class="stack">${chips
          .map(
            (c) => `
          <div class="member ${c.Seat === turn ? "me" : ""}">
            <div class="row">
              <span class="seat">${c.Seat}</span>
              <strong>${memberName(c.Seat, roomMembers)}</strong>
              ${looked.has(c.Seat) ? `<span class="badge">已看</span>` : ""}
              ${folded.has(c.Seat) ? `<span class="badge">弃牌</span>` : ""}
            </div>
            <strong>${c.Chips}</strong>
          </div>`,
          )
          .join("")}</div>
      `);
    }
    case "douniu": {
      const grabs = (p.Grabs as { Seat: number; Multiple: number }[]) || [];
      return el(`
        <div class="row">
          ${p.Banker != null ? `<span class="badge warn">庄 ${p.Banker} ×${p.Multiple ?? 1}</span>` : `<span class="badge">抢庄中</span>`}
        </div>
        <div class="stack">${grabs
          .map(
            (g) => `
          <div class="member">
            <div class="row"><span class="seat">${g.Seat}</span><strong>${memberName(g.Seat, roomMembers)}</strong></div>
            <strong>×${g.Multiple}</strong>
          </div>`,
          )
          .join("") || `<div class="muted">还没有人抢庄</div>`}</div>
      `);
    }
    case "spy": {
      const speeches = (p.Speeches as { Seat: number; Text: string }[]) || [];
      const votes = (p.Votes as { Seat: number; TargetSeat: number }[]) || [];
      return el(`
        <div class="row">${turn ? `<span class="seat-chip turn">轮到 ${memberName(turn, roomMembers)}</span>` : `<span class="badge">${snapshot.Phase}</span>`}</div>
        <div class="stack">
          ${speeches.map((s) => `<div class="member"><strong>${memberName(s.Seat, roomMembers)}</strong><span>${escapeHtml(s.Text)}</span></div>`).join("")}
          ${votes.length ? `<h3>投票</h3>${votes.map((v) => `<div class="muted">${memberName(v.Seat, roomMembers)} → ${memberName(v.TargetSeat, roomMembers)}</div>`).join("")}` : ""}
        </div>
      `);
    }
    case "wolf": {
      const alive = (p.Alive as number[]) || [];
      const dead = (p.Dead as number[]) || [];
      const speeches = (p.Speeches as { Seat: number; Text: string }[]) || [];
      const votes = (p.Votes as { Seat: number; TargetSeat: number }[]) || [];
      return el(`
        <div class="row">
          <span class="badge">${snapshot.Phase}</span>
          ${turn ? `<span class="seat-chip turn">轮到 ${memberName(turn, roomMembers)}</span>` : `<span class="muted">夜里看 Legal，不看 Turn</span>`}
        </div>
        <div class="row">${alive.map((s) => `<span class="seat-chip">${s}</span>`).join("")}</div>
        ${dead.length ? `<div class="muted">出局：${dead.join("、")}</div>` : ""}
        <div class="stack">
          ${speeches.map((s) => `<div class="member"><strong>${memberName(s.Seat, roomMembers)}</strong><span>${escapeHtml(s.Text)}</span></div>`).join("")}
          ${votes.map((v) => `<div class="muted">${v.Seat} → ${v.TargetSeat}</div>`).join("")}
        </div>
      `);
    }
    default:
      return `<pre class="muted">${escapeHtml(JSON.stringify(p, null, 2))}</pre>`;
  }
}

export function renderPrivate(snapshot: Snapshot): string {
  const priv = snapshot.Private || {};
  if (!Object.keys(priv).length) return `<div class="muted">本阶段没有私密信息</div>`;

  const parts: string[] = [];
  if (priv.Cards) parts.push(`<div><div class="muted">手牌</div>${cardsHtml(priv.Cards as Card[])}</div>`);
  if (priv.Role) parts.push(`<div>身份：<strong>${escapeHtml(String(priv.Role))}</strong></div>`);
  if ("Word" in priv) parts.push(`<div>词语：<strong>${escapeHtml(String(priv.Word || "（白板无词）"))}</strong></div>`);
  if (priv.Wolves) parts.push(`<div>狼队友：${(priv.Wolves as number[]).join("、")}</div>`);
  if (priv.Checks) {
    const checks = priv.Checks as { TargetSeat: number; Wolf: boolean }[];
    parts.push(
      `<div>查验：${checks.map((c) => `${c.TargetSeat}${c.Wolf ? "狼" : "好人"}`).join("、") || "无"}</div>`,
    );
  }
  if (priv.Potions) {
    const pot = priv.Potions as { SaveLeft: number; PoisonLeft: number; Knifed: number };
    parts.push(`<div>解药${pot.SaveLeft} / 毒药${pot.PoisonLeft} / 今晚被刀 ${pot.Knifed || "无"}</div>`);
  }
  return parts.join("") || `<pre class="muted">${escapeHtml(JSON.stringify(priv, null, 2))}</pre>`;
}

export function formatEvent(ev: PlayEvent): string {
  const p = ev.Payload || {};
  switch (ev.Type) {
    case "Rolled":
      return `#${ev.Seq} ${p.Seat}号摇出 ${(p.Dice as number[])?.join("+")} = ${p.Sum}`;
    case "Drawn":
      return `#${ev.Seq} ${p.Seat}号抽到 ${formatCard(p.Card as Card)}`;
    case "Dealt":
      return `#${ev.Seq} 发牌完成`;
    case "Deal":
      return `#${ev.Seq} 发牌`;
    case "Bet":
      return `#${ev.Seq} ${p.Seat}号下注 ${p.Amount}（实扣 ${p.Paid}）`;
    case "Dawn":
      return `#${ev.Seq} 天亮，出局 ${(p.Dead as number[])?.join("、") || "无人"}`;
    case "VoteResult":
      return `#${ev.Seq} 投票结果，出局 ${p.Out ?? "平票"}`;
    case "Settle":
      return `#${ev.Seq} 结算 ${escapeHtml(JSON.stringify(p))}`;
    case "Start":
      return `#${ev.Seq} 开局 ${gameName(String(p.GameCode || ""))}`;
    default:
      return `#${ev.Seq} ${ev.Type} ${escapeHtml(JSON.stringify(p))}`;
  }
}

function escapeHtml(s: string): string {
  return s
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}
