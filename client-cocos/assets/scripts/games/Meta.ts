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
  { code: 'dice', name: '摇骰子', blurb: '轮流掷骰', min: 2, max: 8, variants: ['classic'], defaultConfig: { Count: 2 } },
  { code: 'highcard', name: '大小牌', blurb: '抽明牌比大小', min: 2, max: 8, variants: ['classic'], defaultConfig: {} },
  { code: 'tenhalf', name: '十点半', blurb: '闲家要牌', min: 2, max: 6, variants: ['classic'], defaultConfig: {} },
  { code: 'zhajinhua', name: '炸金花', blurb: '看牌下注比牌', min: 2, max: 6, variants: ['classic'], defaultConfig: { Ante: 1, Chips: 100 } },
  { code: 'douniu', name: '斗牛', blurb: '五张暗牌抢庄', min: 2, max: 6, variants: ['classic'], defaultConfig: {} },
  { code: 'spy', name: '谁是卧底', blurb: '描述词语投票', min: 4, max: 10, variants: ['classic'], defaultConfig: { SpyCount: 1, BlankCount: 0 } },
  { code: 'wolf', name: '狼人杀', blurb: '夜间行动白天票', min: 6, max: 12, variants: ['classic', 'witch'], defaultConfig: {} },
];

export function gameName(code: string) {
  return GAMES.find((g) => g.code === code)?.name ?? code;
}
