/**
 * The credit a licensed photograph is allowed in on: who took it, under which
 * licence, and the qualifier that says what the picture is *of*. Pure, so a
 * node test can read the sentence the frames print (`MediaSlot`, the rows of
 * Khám phá, a stop of a route) without rendering any of them.
 */
import type { ImageSource } from "expo-image";

export interface Attribution {
  /** Photographer or uploader, as the licence requires it to be named. */
  author: string;
  /** Licence short name, e.g. «CC BY-SA 4.0», or «Ảnh của nhóm». */
  license: string;
  /** Where the file came from; shown as text, opened by the screen if it wants. */
  source?: string;
  /**
   * A qualifier the credit must not be read without, e.g. «Ảnh quanh đây: ».
   *
   * It belongs here rather than in a line the screen draws next to the slot,
   * because a qualifier that can be laid out separately is a qualifier that
   * can end up on the other side of a scroll from the picture it qualifies.
   */
  prefix?: string;
}

/**
 * The qualifier of a stock photograph that illustrates a *kind* of place and
 * is not a picture of the place itself (assets/rudi/README.md). One constant,
 * so the fixture cannot spell it two ways.
 */
export const TIEN_TO_MINH_HOA = "Ảnh minh hoạ: ";

/**
 * The credit as one sentence: qualifier, author, licence, source. Every frame
 * that shows a credited picture prints this and nothing else, so the words
 * beside a 44dp thumbnail are the words under a full-width slot.
 */
export function cauGhiCong(a: Attribution): string {
  return `${a.prefix ?? ""}${a.author} · ${a.license}${a.source ? ` · ${a.source}` : ""}`;
}

/**
 * A photograph together with the credit it may be shown under. A frame that
 * takes this type takes the credit with it; there is no way to hand it the
 * picture alone.
 */
export type AnhCoGhiCong = { source: ImageSource; nguon: Attribution };
