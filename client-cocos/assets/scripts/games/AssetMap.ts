import type { Card } from '../net/Types';

const SUIT = ['Spades', 'Hearts', 'Clubs', 'Diamonds'];
const RANK: Record<number, string> = { 1: 'A', 11: 'J', 12: 'Q', 13: 'K' };

/** resources 相对路径（不含扩展名），对应 assets/textures 下的文件需放到 resources 或动态加载。 */
export function cardFacePath(card: Card): string {
  const suit = SUIT[card.Suit] ?? 'Spades';
  const rank = RANK[card.Rank] ?? String(card.Rank);
  return `textures/cards/card${suit}${rank}`;
}

export function cardBackPath(): string {
  return 'textures/backs/cardBack_blue5';
}

export function dicePath(value: number, color: 'white' | 'red' = 'white'): string {
  const v = Math.min(6, Math.max(1, value | 0));
  return color === 'red' ? `textures/dice/dieRed${v}` : `textures/dice/dieWhite${v}`;
}

export function chipPath(): string {
  return 'textures/chips/chipRedWhite';
}

export function feltPath(): string {
  return 'textures/bg/table_felt';
}
