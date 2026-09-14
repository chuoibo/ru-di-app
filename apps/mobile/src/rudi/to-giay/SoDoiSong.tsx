/**
 * The same door, answered by the server.
 *
 * Every screen of the two-person notebook asks `useSoDoi()`. Phase 2 answered
 * it from a fixture store; this answers it from `useToGiay`, and that is why
 * Phase 4 rewrote no screen: a screen neither knows nor needs to know which
 * one replied.
 *
 * ## What is not the same, and is written down rather than faked
 *
 * - `nguoiKia` is `null`. Those buttons play the other person's side on one
 *   phone; on the live build the other person has their own.
 * - `daDong` is always false. A closed notebook and one nobody has opened look
 *   identical from here, and that is the rule rather than a gap: closing is
 *   closing, and opening another is a new agreement (§7.6).
 * - `luotCuaToi` is always true. The turn decides whose name Nếp drafts a sheet
 *   FOR, not who may ask, and slice 1 has no turn on the wire (ADR-0027 §6.3).
 */
import { type ReactNode, useMemo } from "react";

import { type NoiDungTo } from "./to-giay";
import { SoDoiContext, type SoDoiApi } from "./SoDoi";
import { caHaiDongY, rangBuocCua, toTomTatThanhTo } from "./so-doi-map";
import { useToGiay } from "./useToGiay";

export function SoDoiSongProvider({
  contextId,
  toiId,
  tenNguoiKia,
  children,
}: {
  contextId: string;
  toiId: string;
  tenNguoiKia: string;
  children: ReactNode;
}) {
  const song = useToGiay(contextId, toiId);

  const api = useMemo<SoDoiApi>(() => {
    const so = song.so;
    const nguoiKiaId = so?.participants.find((id) => id !== toiId) ?? null;
    const toMo = song.to ?? undefined;
    const toKhac = song.ds.filter((row) => row.id !== toMo?.id).map(toTomTatThanhTo);
    return {
      lapSo: so?.cycle_state === "active",
      batDoi: caHaiDongY(so, "bat_doi"),
      docChat: caHaiDongY(so, "doc_chat"),
      luotCuaToi: true,
      rangBuoc: { toi: rangBuocCua(so, toiId), nguoiKia: rangBuocCua(so, nguoiKiaId) },
      toGiay: toMo ? [toMo, ...toKhac] : toKhac,
      deNghiCho: (so?.pending_proposals ?? []).map((d) => ({
        id: d.id,
        purpose: d.purpose,
        // Ai đề nghị quyết định ai bấm được nút đồng ý. Máy chủ trả
        // `proposed_by_id`; đọc nó ở đây là chỗ duy nhất biết điều đó.
        cuaToi: d.proposed_by_id === toiId,
      })),
      daDong: false,

      capId: contextId,
      toiId,
      nguoiKiaId: nguoiKiaId ?? "",
      tenNguoiKia,
      toMo,
      toKhac,
      xemTruocDongSo: () => song.xemTruocDong(),

      deNghiLapSo: () => void song.xinLapSo(),
      deNghiBatDoi: () => void song.xinBac("bat_doi"),
      thuHoiBatDoi: () => void song.thuHoi("bat_doi"),
      datRangBuoc: (rb) => {
        // Two fields, two writes, and an empty one is a delete: the route takes
        // one kind at a time and refuses a blank line, because emptying a
        // constraint is what DELETE is for.
        if (rb.khong_an_duoc !== undefined) {
          const noi_dung = rb.khong_an_duoc.trim();
          void (noi_dung ? song.datRangBuocNay("khong_an_duoc", noi_dung) : song.xoaRangBuocNay("khong_an_duoc"));
        }
        if (rb.dung !== undefined) {
          const noi_dung = rb.dung.trim();
          void (noi_dung ? song.datRangBuocNay("dung", noi_dung) : song.xoaRangBuocNay("dung"));
        }
      },
      dongSo: (revision: string) => void song.dongSoNay(revision),

      dongYDeNghi: (id: string) => void song.dongYDeNghiNay(id),

      ruDiChoi: () => void song.xinTo(),
      suaNhap: (_id: string, content: NoiDungTo, lyDo: string | null) => void song.suaNhap(content, lyDo),
      gui: () => void song.gui(),
      // «Bỏ» a draft and «nghỉ tuần» are one command on the wire: §3.3's table
      // says a draft nobody received is simply dropped for the week.
      boNhap: () => void song.nghiTuanNay(),
      dongY: () => void song.dongYTo(),
      deNghiSua: (_id: string, content: NoiDungTo, lyDo: string | null) => void song.deNghiSuaTo(content, lyDo),
      rut: () => void song.rut(),
      nghiTuan: () => void song.nghiTuanNay(),
      daDi: () => void song.ghiDaDiRoi(),
      giu: (_id: string, line: string) => void song.giuDong(line),
      huy: () => void song.nghiTuanNay(),

      nguoiKia: null,
    };
  }, [contextId, song, tenNguoiKia, toiId]);

  return <SoDoiContext.Provider value={api}>{children}</SoDoiContext.Provider>;
}
