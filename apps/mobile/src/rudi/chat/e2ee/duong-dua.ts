/** A deterministic illustration of a frozen draw, never a random source. */
export function duongDua(seed: string, runner: number, winner: number): number[] {
  let word = 0x811c9dc5;
  for (const char of `${seed}:${runner}`) word = Math.imul(word ^ char.charCodeAt(0), 16777619) >>> 0;
  const speed = Array.from({ length: 24 }, (_, step) => {
    word ^= word << 13; word ^= word >>> 17; word ^= word << 5;
    const jitter = (word >>> 0) / 2 ** 32;
    return 0.22 + jitter * 0.9 + (Math.sin(step * 0.86 + runner * 1.9) + 1) * 0.55;
  });
  // A late surge reconciles the authored pace with the already chosen winner.
  if (runner === winner) for (let i = 19; i < speed.length; i++) speed[i] *= 2.3;
  const total = speed.reduce((sum, value) => sum + value, 0);
  const end = runner === winner ? 1 : 0.78 + ((word >>> 0) % 120) / 1000;
  let distance = 0;
  return [0, ...speed.map((value) => { distance += value; return distance / total * end; })];
}

export const MOC_DUONG_DUA = Array.from({ length: 25 }, (_, index) => index / 24);

/** Size the arena by readable sprites, not a repeated row per option. */
export function khungDuongDua(width: number, count: number, compact: boolean) {
  const arenaWidth = Math.max(160, Math.min(width, 560));
  const duck = count > 6 ? 32 : count > 4 ? 36 : count === 4 ? 40 : 48;
  const height = compact ? Math.max(176, Math.min(288, 64 + count * 32)) : Math.max(224, 66 + count * 30);
  return { width: arenaWidth, height, duck, finish: arenaWidth - 20, travel: arenaWidth - 32 - duck * 0.95 };
}

/** Vertical lane positions centered in the channel without sprite overlap. */
export function viTriVatDua(arena: { height: number; duck: number }, count: number, compact = false): number[] {
  const lanes = Math.max(2, count);
  const bank = compact ? 12 : 14;
  const available = Math.max(arena.duck, arena.height - bank * 2 - arena.duck);
  const pitch = Math.min(arena.duck + (compact ? 10 : 14), available / (lanes - 1));
  const span = pitch * (lanes - 1);
  const start = (arena.height - arena.duck - span) / 2;
  return Array.from({ length: lanes }, (_, index) => Math.round((start + pitch * index) * 100) / 100);
}
