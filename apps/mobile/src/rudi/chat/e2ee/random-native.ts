import { createRandomRound, randomWinner, type RandomMode, type RandomOption, type RandomRound } from "./random-v2";

export class RandomSourceUnavailable extends Error {
  constructor() { super("Bộ tạo ngẫu nhiên an toàn chưa sẵn sàng. Hãy cập nhật ứng dụng rồi thử lại."); }
}

/** Expo Core's native UUID v4 generator has no insecure JS fallback. */
export function newNativeRandomRound(request: string, mode: RandomMode, options: RandomOption[]): RandomRound {
  const native = (globalThis as unknown as { expo?: { uuidv4?: () => string } }).expo?.uuidv4;
  if (!native) throw new RandomSourceUnavailable();
  const next = () => {
    let value: string;
    try { value = native(); } catch { throw new RandomSourceUnavailable(); }
    if (!/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(value)) throw new RandomSourceUnavailable();
    return value;
  };
  const id = next();
  createRandomRound({ id, request, mode, options, seed: "a".repeat(32) });
  for (let attempt = 0; attempt < 3; attempt++) {
    const seed = Array.from({ length: 4 }, () => next().slice(0, 8)).join("");
    if (randomWinner(seed, options.length) !== null) return createRandomRound({ id, request, mode, options, seed });
  }
  throw new RandomSourceUnavailable();
}
