export type GameMeta = {
  code: string;
  name: string;
  blurb: string;
  min: number;
  max: number;
  variants: string[];
  defaultConfig: Record<string, unknown>;
};

export const GAMES: GameMeta[] = [
  {
    code: "dice",
    name: "摇骰子",
    blurb: "轮流掷骰，点数最高者胜",
    min: 2,
    max: 8,
    variants: ["classic"],
    defaultConfig: { Count: 2 },
  },
  {
    code: "highcard",
    name: "大小牌",
    blurb: "各抽一张明牌，比大小",
    min: 2,
    max: 8,
    variants: ["classic"],
    defaultConfig: {},
  },
  {
    code: "tenhalf",
    name: "十点半",
    blurb: "闲家要牌，庄家自动补牌",
    min: 2,
    max: 6,
    variants: ["classic"],
    defaultConfig: {},
  },
  {
    code: "zhajinhua",
    name: "炸金花",
    blurb: "看牌、下注、比牌、弃牌",
    min: 2,
    max: 6,
    variants: ["classic"],
    defaultConfig: { Ante: 1, Chips: 100 },
  },
  {
    code: "douniu",
    name: "斗牛",
    blurb: "五张暗牌抢庄，比牛结算",
    min: 2,
    max: 6,
    variants: ["classic"],
    defaultConfig: {},
  },
  {
    code: "spy",
    name: "谁是卧底",
    blurb: "描述词语，投票揪出卧底",
    min: 4,
    max: 10,
    variants: ["classic"],
    defaultConfig: { SpyCount: 1, BlankCount: 0 },
  },
  {
    code: "wolf",
    name: "狼人杀",
    blurb: "夜间行动，白天发言投票",
    min: 6,
    max: 12,
    variants: ["classic", "witch"],
    defaultConfig: {},
  },
];

export function gameByCode(code: string): GameMeta | undefined {
  return GAMES.find((g) => g.code === code);
}

export function gameName(code: string): string {
  return gameByCode(code)?.name ?? code;
}
