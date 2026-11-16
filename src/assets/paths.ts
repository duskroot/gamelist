import type { Card } from "../net/types";

const SUIT_NAME = ["Spades", "Hearts", "Clubs", "Diamonds"] as const;
const RANK_NAME: Record<number, string> = {
  1: "A",
  11: "J",
  12: "Q",
  13: "K",
};

export function cardFaceUrl(card: Card): string {
  const suit = SUIT_NAME[card.Suit] ?? "Spades";
  const rank = RANK_NAME[card.Rank] ?? String(card.Rank);
  return `/game/cards/card${suit}${rank}.png`;
}

export function cardBackUrl(variant: "blue" | "red" | "green" = "blue"): string {
  return `/game/backs/cardBack_${variant}5.png`;
}

export function diceUrl(value: number, color: "white" | "red" = "white"): string {
  const v = Math.min(6, Math.max(1, value | 0));
  return color === "red" ? `/game/dice/dieRed${v}.png` : `/game/dice/dieWhite${v}.png`;
}

export function chipUrl(color: "black" | "blue" | "red" | "green" | "white" = "red"): string {
  const map: Record<string, string> = {
    black: "/game/chips/chipBlackWhite.png",
    blue: "/game/chips/chipBlueWhite.png",
    red: "/game/chips/chipRedWhite.png",
    green: "/game/chips/chipGreenWhite.png",
    white: "/game/chips/chipWhiteBlue.png",
  };
  return map[color] || map.red;
}

export function gameIconUrl(code: string): string {
  return `/game/ui/${code}.svg`;
}

export function tableFeltUrl(): string {
  return "/game/bg/table_felt.png";
}

export function preloadCoreAssets(): Promise<void[]> {
  const urls = [
    tableFeltUrl(),
    cardBackUrl(),
    ...[1, 2, 3, 4, 5, 6].map((n) => diceUrl(n)),
    chipUrl("red"),
    chipUrl("blue"),
  ];
  return Promise.all(
    urls.map(
      (src) =>
        new Promise<void>((resolve) => {
          const img = new Image();
          img.onload = () => resolve();
          img.onerror = () => resolve();
          img.src = src;
        }),
    ),
  );
}
