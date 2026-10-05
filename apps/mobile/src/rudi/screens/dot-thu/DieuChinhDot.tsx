/**
 * «Sửa sau khi phát» on a round's page (ADR-0056).
 *
 * A round already frozen or published is not edited in place: its expense is
 * corrected by an amendment that every person whose transfer changes agrees
 * to, and only then does the board change. Until then the old figures stand,
 * and the page says so beside every new figure it shows.
 *
 * Three states, one at a time, so the page never asks two things at once:
 *
 *   - an amendment is waiting: what changes, old → new per transfer, who has
 *     agreed, and -- for a person it touches -- the two answers; for its
 *     proposer, the review links to send to guests, one person at a time;
 *   - nothing waiting: «Đề xuất sửa một khoản», for whoever may propose;
 *   - composing: the expense's shares, one row per member, the new total
 *     against the old, and the reason the others will read.
 *
 * Settled amendments stay as a short history under it, with the seal
 * «Đã điều chỉnh» on the ones that changed the board (spec L557).
 */
import { useCallback, useEffect, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { attemptFor, ApiError, thongDiepNguoiDoc, type Attempt, type Envelope } from "../../../api";
import { tenCua, type ThanhVien } from "../../chia-bill/hoa-don";
import {
  cauTrangThaiDieuChinh,
  coTheDeXuat,
  deXuatDieuChinh,
  dieuChinhDangMo,
  docHoSoDieuChinh,
  linkSauDieuChinh,
  loiNhanLinkDuyet,
  loiPhan,
  phanBoBanDau,
  toiCanTraLoi,
  tongPhanBo,
  traLoiDieuChinh,
  type DieuChinh,
  type HoSoDieuChinh,
  type KhoanTrongDot,
  type LinkDuyet,
} from "../../dot-thu/dieu-chinh";
import type { NghiaVu } from "../../dot-thu/dot-thu";
import { docLinkDuyet, luuLinkDot, luuLinkDuyet } from "../../dot-thu/kho-link";
import { typography, useRudiTheme } from "../../theme";
import { RudiButton, SectionHeader } from "../../ui";
import { Avatar } from "../../ui/Avatar";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { ChuThichLe } from "../../ui/ChuThichLe";
import { Money } from "../../ui/Money";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { PhongBi } from "../../ui/PhongBi";
import { Stamp } from "../../ui/Stamp";
import { StampButton } from "../../ui/StampButton";
import { chiaSe } from "../../web/chia-se";

type Soan = { khoan: KhoanTrongDot; o: Record<string, string>; lyDo: string };

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

/** The expense by its description, else by its amount: never a bare «khoản chi» (UI review 05/10). */
function moTaKhoan(k: KhoanTrongDot | undefined): string {
  if (k === undefined) return "khoản chi";
  return k.moTa?.trim() ? k.moTa.trim() : `khoản ${k.tongVnd.toLocaleString("vi-VN")}đ`;
}

/** «An, Chi» from ids, or null for none. */
function danhTen(ids: readonly string[], ten: (id: string) => string): string | null {
  return ids.length === 0 ? null : ids.map(ten).join(", ");
}

function ngay(iso: string): string {
  const d = new Date(iso);
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")} ${d.getDate()}/${d.getMonth() + 1}`;
}

/** A typed share: whole đồng, digits only; "" is 0 while typing. */
function docO(text: string): number | null {
  if (text.trim() === "") return 0;
  if (!/^\d{1,13}$/.test(text.trim())) return null;
  const n = Number(text.trim());
  return loiPhan(n) === null ? n : null;
}

export function DieuChinhDot({
  actorId,
  contextId,
  batchId,
  roster,
  bang,
  links,
  docLaiBang,
  onTomTat,
}: {
  actorId: string;
  contextId: string;
  batchId: string;
  roster: ThanhVien[];
  bang: NghiaVu[];
  links: Envelope[] | null;
  docLaiBang: () => Promise<void>;
  /** Told whenever the round's amendments are read, so the page head can say one is waiting. */
  onTomTat?: (tom: { dangMo: boolean; canToiTraLoi: boolean }) => void;
}) {
  const { colors, radius } = useRudiTheme();
  const [hoSo, setHoSo] = useState<HoSoDieuChinh | null>(null);
  const [linkDuyet, setLinkDuyet] = useState<LinkDuyet[] | null>(null);
  const [soan, setSoan] = useState<Soan | null>(null);
  const [ban, setBan] = useState(false);
  const [loi, setLoi] = useState<{ noi: string; cau: string } | null>(null);
  const [daGui, setDaGui] = useState<Record<string, true>>({});
  const attempts = useRef<Record<string, Attempt>>({});

  const doc = useCallback(async () => {
    const hs = await docHoSoDieuChinh(contextId, batchId, actorId);
    const mo = dieuChinhDangMo(hs);
    setHoSo(hs);
    setLinkDuyet(mo === null ? null : await docLinkDuyet(mo.id));
    return hs;
  }, [actorId, batchId, contextId]);

  useEffect(() => {
    if (hoSo === null || onTomTat === undefined) return;
    const mo = dieuChinhDangMo(hoSo);
    onTomTat({ dangMo: mo !== null, canToiTraLoi: mo !== null && toiCanTraLoi(mo, actorId) });
  }, [actorId, hoSo, onTomTat]);

  useEffect(() => {
    let song = true;
    doc().catch((error: unknown) => {
      if (song) setLoi({ noi: "doc", cau: loiRaChu(error) });
    });
    return () => {
      song = false;
    };
  }, [doc]);

  const chay = async (noi: string, viec: () => Promise<void>) => {
    setBan(true);
    setLoi(null);
    try {
      await viec();
    } catch (error) {
      setLoi({ noi, cau: loiRaChu(error) });
    } finally {
      setBan(false);
    }
  };
  const loiO = (noi: string) => (loi?.noi === noi ? loi.cau : null);

  // Once an amendment this phone proposed has applied, the guests it touched
  // hold their review URL, which now opens their new share: the round's
  // stored links are brought up to date from it and from the new board.
  const sauKhiXong = async (id: string) => {
    await docLaiBang();
    const hs = await doc();
    const xong = hs.dieuChinh.find((d) => d.id === id);
    if (xong?.trangThai === "applied" && links !== null) {
      const cua = await docLinkDuyet(xong.id);
      if (cua !== null && cua.length > 0) await luuLinkDot(batchId, linkSauDieuChinh(links, cua, bang));
      await docLaiBang();
    }
  };

  if (hoSo === null) {
    return loiO("doc") === null ? null : (
      <>
        <SectionHeader title="Sửa sau khi phát" />
        <CauTaiCho cau={loiO("doc")} />
      </>
    );
  }

  const mo = dieuChinhDangMo(hoSo);
  const daXong = hoSo.dieuChinh.filter((d) => d.trangThai !== "proposed");
  const khoanDuocSua = hoSo.khoan.filter((k) => coTheDeXuat(hoSo, k, actorId));
  const ten = (id: string) => tenCua(roster, id);

  const traLoi = (d: DieuChinh, dongY: boolean) =>
    chay("tra-loi", async () => {
      await traLoiDieuChinh({ contextId, batchId, amendmentId: d.id, actorId, dongY, attempt: attemptFor(attempts.current, `tl:${d.id}:${dongY}`) });
      await sauKhiXong(d.id);
    });

  const guiDeXuat = (s: Soan) =>
    chay("gui", async () => {
      const phanBo: Record<string, number> = {};
      for (const [id, text] of Object.entries(s.o)) {
        const n = docO(text);
        if (n === null) throw new ApiError(422, "allocation_amount_invalid", "Mỗi phần là số đồng nguyên, không âm.");
        if (n > 0 || s.khoan.phanBo.some((p) => p.personId === id)) phanBo[id] = n;
      }
      const kq = await deXuatDieuChinh({
        contextId,
        batchId,
        actorId,
        expenseId: s.khoan.expenseId,
        lyDo: s.lyDo.trim(),
        phanBo,
        attempt: attemptFor(attempts.current, `dx:${s.khoan.expenseId}:${JSON.stringify(phanBo)}:${s.lyDo.trim()}`),
      });
      await luuLinkDuyet(kq.id, kq.links);
      setSoan(null);
      await sauKhiXong(kq.id);
    });

  const guiLink = async (l: LinkDuyet) => {
    const ketQua = await chiaSe({ text: loiNhanLinkDuyet(ten(l.senderId), l.url), title: `Đề xuất sửa cho ${ten(l.senderId)}` });
    if (ketQua !== "huy") setDaGui((d) => ({ ...d, [l.senderId]: true }));
  };

  return (
    <>
      <SectionHeader title="Sửa sau khi phát" />

      {mo !== null ? (
        <View style={[styles.the, { borderColor: colors.split, borderRadius: radius.base, backgroundColor: colors.card }]} testID="dieu-chinh-dang-mo">
          <Text style={[typography.caption, { color: colors.split }]}>{cauTrangThaiDieuChinh(mo).toUpperCase()}</Text>
          <Text style={[typography.h2, { color: colors.ink }]}>
            {`${ten(mo.nguoiDeXuat)} đề xuất sửa «${moTaKhoan(hoSo.khoan.find((k) => k.expenseId === mo.expenseId))}»`}
          </Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>{`Lý do: ${mo.lyDo}`}</Text>
          {mo.dong.map((d) => (
            <View key={`${d.senderId}>${d.recipientId}`} style={[styles.dong, { borderBottomColor: colors.line }]}>
              <Avatar name={ten(d.senderId)} personId={d.senderId} size={28} />
              <Text style={[typography.label, styles.flex, { color: colors.ink }]}>{`${ten(d.senderId)} → ${ten(d.recipientId)}`}</Text>
              <View style={styles.cotPhai}>
                <Text style={[typography.caption, styles.gach, { color: colors.inkSoft }]}>{d.cuVnd === 0 ? "chưa có" : `${d.cuVnd.toLocaleString("vi-VN")}đ`}</Text>
                {d.moiVnd === 0 ? <Text style={[typography.label, { color: colors.ink }]}>bỏ lượt này</Text> : <Money size="label" tone="split" vnd={d.moiVnd} />}
              </View>
            </View>
          ))}
          {/* Who has answered, in three plain lines rather than a wall of
              upper-case seals (UI review 05/10). */}
          <View style={styles.ben}>
            {(
              [
                ["Đã đồng ý", mo.ben.filter((b) => b.traLoi === "dong_y").map((b) => b.personId), colors.split],
                ["Chưa trả lời", mo.choAi, colors.ink],
                ["Không đồng ý", mo.ben.filter((b) => b.traLoi === "khong_dong_y").map((b) => b.personId), colors.warn],
              ] as const
            ).map(([nhan, ids, mau]) =>
              danhTen(ids, ten) === null ? null : (
                <Text key={nhan} style={[typography.body, { color: colors.ink }]}>
                  <Text style={[typography.label, { color: mau }]}>{`${nhan}: `}</Text>
                  {danhTen(ids, ten)}
                </Text>
              ),
            )}
          </View>
          <ChuThichLe>{`Số mới chỉ áp dụng khi mọi người có tên ở trên đồng ý, chậm nhất ${ngay(mo.hetHan)}. Tới lúc đó, bảng thu ở trên vẫn là số đang tính.`}</ChuThichLe>
          {toiCanTraLoi(mo, actorId) ? (
            <View style={styles.cot}>
              <StampButton disabled={ban} label="Đồng ý với số mới" loading={ban} onPress={() => void traLoi(mo, true)} size="vua" tone="split" />
              <RudiButton disabled={ban} label="Không đồng ý" lyDo="Đang gửi câu trả lời" onPress={() => void traLoi(mo, false)} tone="warn" variant="ghost" />
              <CauTaiCho cau={loiO("tra-loi")} />
            </View>
          ) : null}
          {mo.nguoiDeXuat === actorId && linkDuyet !== null && linkDuyet.length > 0 ? (
            <PhongBi testID="phong-bi-link-duyet">
              <Text style={[typography.caption, { color: colors.inkSoft }]}>
                Khách không có tài khoản trả lời qua link riêng. Link cũ của họ đã ngừng; link này mở ra đúng phần đề xuất, và sau đó là phần của họ.
              </Text>
              {linkDuyet.map((l) => (
                <View key={l.senderId} style={styles.dong}>
                  <Avatar name={ten(l.senderId)} personId={l.senderId} size={28} />
                  <Text style={[typography.label, styles.flex, { color: colors.ink }]}>{ten(l.senderId)}</Text>
                  <RudiButton
                    accessibilityLabel={daGui[l.senderId] ? `Gửi lại cho ${ten(l.senderId)}` : `Gửi cho ${ten(l.senderId)}`}
                    compact
                    full={false}
                    icon="share-social-outline"
                    label={daGui[l.senderId] ? "Gửi lại" : "Gửi"}
                    onPress={() => void guiLink(l)}
                    tone="split"
                    variant="outline"
                  />
                </View>
              ))}
            </PhongBi>
          ) : null}
          {mo.nguoiDeXuat === actorId && linkDuyet === null && mo.choAi.length > 0 ? (
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Link duyệt của đề xuất này được tạo ở máy khác; máy này không lấy lại được.</Text>
          ) : null}
        </View>
      ) : null}

      {mo === null && soan === null && khoanDuocSua.length > 0 ? (
        <>
          <Text style={[typography.body, { color: colors.inkSoft }]}>
            Ghi sai một khoản sau khi đã phát? Đề xuất số đúng: chỉ người có lượt chuyển thay đổi mới cần đồng ý, và bảng thu chỉ đổi khi họ đồng ý hết.
          </Text>
          {khoanDuocSua.map((k) => (
            <RudiButton
              key={k.expenseId}
              icon="create-outline"
              label={`Đề xuất sửa «${moTaKhoan(k)}» · ${k.tongVnd.toLocaleString("vi-VN")}đ`}
              onPress={() => setSoan({ khoan: k, lyDo: "", o: Object.fromEntries(Object.entries(phanBoBanDau(k, roster.map((r) => r.id))).map(([id, v]) => [id, String(v)])) })}
              tone="split"
              variant="outline"
            />
          ))}
        </>
      ) : null}

      {soan !== null ? <SoanDeXuat ban={ban} loi={loiO("gui")} onGui={guiDeXuat} onThoi={() => setSoan(null)} roster={roster} soan={soan} setSoan={setSoan} /> : null}

      {daXong.length > 0 ? (
        <View style={styles.cot}>
          {daXong.slice(0, 5).map((d) => (
            <View key={d.id} style={[styles.dong, { borderBottomColor: colors.line }]}>
              <Stamp label={cauTrangThaiDieuChinh(d)} tone={d.trangThai === "applied" ? "split" : "ink"} variant={d.trangThai === "applied" ? "ink" : "outline"} />
              <Text numberOfLines={2} style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>
                {`«${moTaKhoan(hoSo.khoan.find((k) => k.expenseId === d.expenseId))}» · ${d.lyDo}${d.xongLuc ? ` · ${ngay(d.xongLuc)}` : ""}`}
              </Text>
            </View>
          ))}
        </View>
      ) : null}
    </>
  );
}

function SoanDeXuat({
  soan,
  setSoan,
  roster,
  ban,
  loi,
  onGui,
  onThoi,
}: {
  soan: Soan;
  setSoan: (s: Soan) => void;
  roster: ThanhVien[];
  ban: boolean;
  loi: string | null;
  onGui: (s: Soan) => void;
  onThoi: () => void;
}) {
  const { colors, radius } = useRudiTheme();
  const so: Record<string, number> = {};
  let hong = false;
  for (const [id, text] of Object.entries(soan.o)) {
    const n = docO(text);
    if (n === null) hong = true;
    else so[id] = n;
  }
  const tongMoi = hong ? null : tongPhanBo(so);
  const lyDoTrong = soan.lyDo.trim() === "";
  const lyDo = hong ? "Có ô chưa phải số đồng nguyên" : tongMoi === null || tongMoi <= 0 ? "Tổng mới phải lớn hơn 0" : lyDoTrong ? "Ghi lý do sửa" : undefined;
  const nguoi = [...new Set([...soan.khoan.phanBo.map((p) => p.personId), ...roster.map((r) => r.id)])];
  return (
    <View style={[styles.the, { borderColor: colors.lineStrong, borderRadius: radius.base, backgroundColor: colors.card, borderStyle: "dashed" }]} testID="soan-dieu-chinh">
      <Text style={[typography.h2, { color: colors.ink }]}>{`Sửa «${moTaKhoan(soan.khoan)}»`}</Text>
      <Text style={[typography.body, { color: colors.inkSoft }]}>{`${tenCua(roster, soan.khoan.nguoiTra)} đã trả. Gõ phần đúng của từng người, đồng nguyên; để 0 nếu người đó không có phần.`}</Text>
      {nguoi.map((id) => {
        const cu = soan.khoan.phanBo.find((p) => p.personId === id)?.vnd ?? 0;
        const text = soan.o[id] ?? "0";
        const n = docO(text);
        return (
          <View key={id} style={styles.dongNhap}>
            <Avatar name={tenCua(roster, id)} personId={id} size={28} />
            <View style={styles.flex}>
              <ONhapMuc
                accessibilityLabel={`Phần của ${tenCua(roster, id)}, đồng`}
                error={n === null ? "Số đồng nguyên, không âm." : null}
                helper={n !== null && n !== cu ? `trước: ${cu.toLocaleString("vi-VN")}đ` : undefined}
                keyboardType="number-pad"
                label={tenCua(roster, id)}
                maxLength={13}
                onChangeText={(t) => setSoan({ ...soan, o: { ...soan.o, [id]: t.replace(/[^\d]/g, "") } })}
                value={text}
              />
            </View>
          </View>
        );
      })}
      <View style={styles.dong}>
        <Text style={[typography.label, styles.flex, { color: colors.ink }]}>Tổng mới</Text>
        {tongMoi === null ? <Text style={[typography.label, { color: colors.warn }]}>chưa tính được</Text> : <Money size="label" tone="split" vnd={tongMoi} />}
      </View>
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{`Tổng cũ ${soan.khoan.tongVnd.toLocaleString("vi-VN")}đ.`}</Text>
      <ONhapMuc
        accessibilityLabel="Lý do sửa"
        label="Lý do (mọi người liên quan sẽ đọc)"
        maxLength={500}
        multiline
        numberOfLines={2}
        onChangeText={(t) => setSoan({ ...soan, lyDo: t })}
        placeholder="Ví dụ: Chi không ăn món lẩu, Bình gọi thêm"
        value={soan.lyDo}
      />
      <StampButton disabled={ban || lyDo !== undefined} label="Gửi đề xuất" loading={ban} lyDo={lyDo} onPress={() => onGui(soan)} size="vua" tone="split" />
      <CauTaiCho cau={loi} />
      <RudiButton disabled={ban} label="Thôi, không sửa" lyDo="Đang gửi đề xuất" onPress={onThoi} tone="split" variant="ghost" />
    </View>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  the: { gap: 10, padding: 16, borderWidth: 1.5 },
  dong: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 44, paddingVertical: 6, borderBottomWidth: StyleSheet.hairlineWidth },
  dongNhap: { flexDirection: "row", alignItems: "center", gap: 10 },
  cotPhai: { alignItems: "flex-end", gap: 2 },
  gach: { textDecorationLine: "line-through" },
  ben: { gap: 4 },
  cot: { gap: 8 },
});
