/**
 * The credit a licensed photograph is allowed in on: who took it, under which
 * licence, and the qualifier that says what the picture is *of*. Pure, so a
 * node test can read the sentence the frames print (`MediaSlot`, the rows of
 * Khám phá, a stop of a route) without rendering any of them.
 */
import type { ImageSource } from "expo-image";

export interface Attribution {
  /** Photographer or uploader, as the licence requires it to be named. Null
   *  when the source cannot say: frames from the place feed are posts people
   *  published, with no recorded author, and inventing one is a false credit. */
  author: string | null;
  /** Licence short name, e.g. «CC BY-SA 4.0», or «Ảnh của nhóm». Null for the
   *  same reason: there is none, and saying there is would be a lie. */
  license: string | null;
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
export function cauGhiCong(a: Attribution): string | null {
  const parts = [a.author, a.license, a.source].filter(
    (part): part is string => typeof part === "string" && part.trim() !== "",
  );
  // Nothing to credit is not the same as an empty credit. A frame that prints
  // «Ảnh quanh đây: · » tells the reader there is an author it forgot to name.
  if (parts.length === 0) return null;
  return `${a.prefix ?? ""}${parts.join(" · ")}`;
}

/** What a frame prints when the address it was given did not load. */
export const CAU_ANH_HONG = "Chưa tải được ảnh";

/**
 * A catalogue photograph and the credit it may be shown under, as ONE value
 * whose address cannot be taken out on its own.
 *
 * The previous shape was `{ source, nguon }`, and the review of 08/09 (F31)
 * showed with a working probe why that is not enough: `const p = noi.anh;
 * p.source` and `const { source } = noi.anh` both hand a screen the picture
 * with the words left behind, and no source-scanning gate can be taught every
 * spelling of that. Here the address lives in a closure, so those two lines do
 * not compile at all -- the rule is carried by the type checker, which reads
 * every file on every build, instead of by a regular expression that has to
 * guess how the next author will write it.
 *
 * `ve()` returns the same frozen object on every call, so a frame may use it
 * in a dependency array without re-running an effect on each render.
 */
export type AnhCoGhiCong = { readonly ve: () => AnhDaMo };

/** The address and the sentence, handed over together or not at all. */
export type AnhDaMo = { readonly source: ImageSource; readonly ghiCong: string | null };

/** Mint a catalogue photograph. The only door into `AnhCoGhiCong`. */
export function anhDanhMuc(source: ImageSource, nguon: Attribution): AnhCoGhiCong {
  const daMo: AnhDaMo = Object.freeze({ source, ghiCong: cauGhiCong(nguon) });
  return { ve: () => daMo };
}

/**
 * Where a frame's picture comes from. The two branches owe different things:
 * a catalogue photograph owes its author and licence, and the group's own
 * photograph owes neither -- inventing a stock licence for a private picture
 * would be its own kind of lie (review 08/09 vòng 2, F31).
 */
export type NguonKhung =
  | { loai: "danh-muc"; anh: AnhCoGhiCong }
  | { loai: "nhom"; source: ImageSource }
  | null;

/**
 * A stable identity for the picture behind a `NguonKhung`.
 *
 * A frame resets its «did not load» state when the picture changes, and the
 * obvious dependency -- the address object -- is rebuilt on every render by
 * every live screen, so the reset fired on every frame and the failure state
 * could never be seen. Keying on the address itself fixes that for the group
 * branch too, which had the same fault before this change.
 */
export function khoaNguon(nguon: NguonKhung): string {
  if (nguon === null) return "";
  const s: unknown = nguon.loai === "nhom" ? nguon.source : nguon.anh.ve().source;
  if (typeof s === "number") return `n:${s}`;
  if (typeof s === "string") return `s:${s}`;
  if (typeof s === "object" && s !== null && "uri" in s) return `u:${String((s as { uri?: unknown }).uri ?? "")}`;
  return "?";
}

/** Everything a frame draws, decided in one place. */
export interface KhungDaVe {
  /** The picture, or null when there is none to draw or it failed. */
  source: ImageSource | null;
  /** The credit sentence; never null while a catalogue picture is drawn. */
  ghiCong: string | null;
  /** The word a person reads when an address was given and did not load. */
  canhBao: string | null;
}

/**
 * The whole render decision, as one pure function.
 *
 * It exists so the claim «a frame cannot draw a catalogue photograph without
 * its credit» is a property a test can prove over every input, rather than a
 * shape a reviewer has to re-check in three components. `MediaSlot` and the
 * rows call this and render exactly the three fields it returns.
 */
export function veKhung(nguon: NguonKhung, o: { hong: boolean }): KhungDaVe {
  if (nguon === null) return { source: null, ghiCong: null, canhBao: null };
  if (nguon.loai === "nhom") {
    return { source: o.hong ? null : nguon.source, ghiCong: null, canhBao: o.hong ? CAU_ANH_HONG : null };
  }
  const daMo = nguon.anh.ve();
  // The credit stays printed even when the picture failed: the words are what
  // the licence is owed, and a reader who sees the failure line still sees who
  // the picture would have been by.
  return { source: o.hong ? null : daMo.source, ghiCong: daMo.ghiCong, canhBao: o.hong ? CAU_ANH_HONG : null };
}
