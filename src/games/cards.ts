import type { Card } from "../net/types";

const SUITS = ["♠", "♥", "♣", "♦"] as const;
const SUIT_CLASS = ["spade", "heart", "club", "diamond"] as const;

export function rankLabel(rank: number): string {
  if (rank === 1) return "A";
  if (rank === 11) return "J";
  if (rank === 12) return "Q";
  if (rank === 13) return "K";
  return String(rank);
}

export function suitGlyph(suit: number): string {
  return SUITS[suit] ?? "?";
}

export function suitClass(suit: number): string {
  return SUIT_CLASS[suit] ?? "spade";
}

export function formatCard(card: Card | undefined | null): string {
  if (!card) return "—";
  return `${suitGlyph(card.Suit)}${rankLabel(card.Rank)}`;
}

export function tenhalfPoints(sum: number | undefined): string {
  if (sum == null) return "—";
  const pts = sum / 2;
  return Number.isInteger(pts) ? String(pts) : pts.toFixed(1);
}

export function niuLabel(niu: number | undefined): string {
  if (niu == null) return "";
  if (niu === 0) return "没牛";
  if (niu === 10) return "牛牛";
  return `牛${niu}`;
}
