/**
 * The wire as the notebook screens already read it.
 *
 * Pure, and apart from `SoDoiSong.tsx`, so it can be driven without a device:
 * this is the only new logic in the live provider, and a mistake in it is the
 * quiet kind -- «both agreed» reading one person's answer, or a closed row
 * losing the line somebody kept.
 */
import type { ToGiay } from "./to-giay";
import type { MucDich, SoHaiNguoi, ToTomTat } from "./to-giay-song";

/**
 * Write the two constraint boxes: in order, one at a time, and only the box
 * that changed; stop at the first write that does not land.
 *
 * The live notebook runs one command at a time and answers `false` to a second
 * one started while the first is in flight. The provider used to start both
 * together, so «Đừng» never reached the server while the sheet closed as if it
 * had -- for every person, on every save (QA 23/09).
 */
export async function ghiRangBuocTuanTu(
  hienTai: RangBuocDoc,
  moi: Partial<RangBuocDoc>,
  buoc: {
    dat: (kind: keyof RangBuocDoc, noiDung: string) => Promise<boolean>;
    xoa: (kind: keyof RangBuocDoc) => Promise<boolean>;
  },
): Promise<boolean> {
  for (const kind of ["khong_an_duoc", "dung"] as const) {
    const vao = moi[kind];
    if (vao === undefined) continue;
    const noiDung = vao.trim();
    if (noiDung === hienTai[kind].trim()) continue;
    const daXong = noiDung ? await buoc.dat(kind, noiDung) : await buoc.xoa(kind);
    if (!daXong) return false;
  }
  return true;
}

/** Neither line written. Not «unknown»: the notebook simply holds nothing here. */
export const KHONG_RANG_BUOC = { khong_an_duoc: "", dung: "" } as const;

export type RangBuocDoc = { khong_an_duoc: string; dung: string };

export function rangBuocCua(so: SoHaiNguoi | null, ownerId: string | null): RangBuocDoc {
  if (so === null || ownerId === null) return { ...KHONG_RANG_BUOC };
  const cua = so.constraints.filter((row) => row.owner_id === ownerId);
  return {
    khong_an_duoc: cua.find((row) => row.kind === "khong_an_duoc")?.content ?? "",
    dung: cua.find((row) => row.kind === "dung")?.content ?? "",
  };
}

/**
 * A rung is on only when BOTH have granted it.
 *
 * `my_consents` is mine and `their_consents_granted` is theirs, and reading
 * either alone would light a switch one person turned: nothing in a two-person
 * notebook is unlocked by somebody agreeing with themselves.
 */
export function caHaiDongY(so: SoHaiNguoi | null, purpose: MucDich): boolean {
  if (so === null) return false;
  // The server's own answer when it gives one: agreement is per PROPOSAL, and
  // two per-person yeses on two different proposals lit «Một đôi» on both
  // phones while nothing had been completed (QA 23/09).
  if (so.granted_purposes !== undefined) return so.granted_purposes.includes(purpose);
  const cuaToi = so.my_consents.find((row) => row.purpose === purpose)?.granted === true;
  return cuaToi && so.their_consents_granted[purpose] === true;
}

/**
 * A closed row, built from its summary.
 *
 * Real: id, state, version, week, deadline, the date, the first stop, the first
 * kept line. Invented: nothing -- the fields a `ToGiay` has and a summary does
 * not are left empty rather than guessed, because this object has exactly one
 * reader (the row under «Tờ đã khép») and it shows those facts. Anything that
 * wants more asks `GET /papers/{id}`, which is what opening the row does.
 */
export function toTomTatThanhTo(row: ToTomTat): ToGiay {
  return {
    id: row.id,
    state: row.state,
    version: row.version,
    author_type: "human",
    sent_by: null,
    versions: [
      {
        version: row.version,
        content: {
          ngay: row.ngay ?? "",
          chang: row.chang_dau ? [{ ...row.chang_dau, place_id: null, can_kiem: false }] : [],
        },
        ly_do: null,
        sent_at: null,
        sent_by: null,
        author_type: "human",
        my_response: null,
        their_agreed: false,
        viewed_by_recipient_at: null,
      },
    ],
    outing_id: null,
    keeps: row.dong_giu_dau ? [{ id: `${row.id}-giu`, line: row.dong_giu_dau, created_at: "" }] : [],
    tuan: row.tuan,
    expires_at: row.expires_at,
    // Never true on a row: «đã đi rồi» is a decision about the sheet in play,
    // and the sheet in play is fetched whole.
    co_the_ghi_da_di: false,
  };
}
