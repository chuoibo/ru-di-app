/**
 * One collection round (đợt thu) on a real session (M5, slice v-b).
 *
 * What the screen draws is the server's board: who transfers to whom, how
 * much, and whether it arrived -- status derived on the server from confirmed
 * receipts, never from a button here. The three things a person can do:
 *
 *   - publish the round (the organiser): obligations become real and every
 *     sender gets a private guest link, returned exactly once;
 *   - send a link (the organiser): one share sheet per person, never a bulk
 *     send, and a dismissed sheet is not counted as sent;
 *   - say the money arrived (the recipient of that obligation, nobody else).
 *
 * A phone that did not publish the round has no links to send: the server
 * keeps only a digest of each token, and this screen says so.
 *
 * UI v2 (đợt 6): the board is a list of rows -- who → whom, the state as a
 * word, the sum -- ordered by what is still waiting; the sentences that
 * explain a state sit beside the action they explain and nowhere else.
 *
 * UI v3 (ADR-0037, «sân khấu giấy»): the board is a page of the group's
 * ledger, each person in their own ink; a strip of teal tape fills as the
 * server counts transfers arrived, always beside the count in words; Nếp
 * bows at the head of the page when money has just arrived (M4); publishing
 * is the teal seal with its consequence in full; the private links travel in
 * one envelope, still sent one person at a time.
 */
import { useRouter } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { chiaSe, type KetQuaChiaSe } from "../../web/chia-se";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import type { Phien } from "../../../phien";
import { danhSachThanhVien } from "../../../screens/vao-cua/cong-api";
import { tenCua, type ThanhVien } from "../../chia-bill/hoa-don";
import {
  TU_NGHIA_VU,
  cauTrangThaiDot,
  daPhat,
  docBangThu,
  docDotThuCuaNhom,
  loiNhanChiaSe,
  nghiaVuToiNhan,
  phatDotThu,
  tomTatBang,
  xacNhanDaNhan,
  type Envelope,
  type NghiaVu,
  type TrangThaiDot,
} from "../../dot-thu/dot-thu";
import { docLinkDot, luuLinkDot } from "../../dot-thu/kho-link";
import { mucNguoi, typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { Avatar } from "../../ui/Avatar";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { ChuThichLe } from "../../ui/ChuThichLe";
import { DaiTienDo } from "../../ui/DaiTienDo";
import { ErrorState } from "../../ui/ErrorState";
import { Money } from "../../ui/Money";
import { NepDien } from "../../ui/NepDien";
import { PhongBi } from "../../ui/PhongBi";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../../ui/Skeleton";
import { Stamp } from "../../ui/Stamp";
import { StampButton } from "../../ui/StampButton";
import { TrangSo } from "../../ui/TrangSo";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; nghiaVu: NghiaVu[]; soTranhCai: number; trangThai: TrangThaiDot | null; links: Envelope[] | null }
  | { pha: "hong"; loi: string };

/** The board's sections: transfers by the person paid, in the order they first appear. */
function gomTheoNguoiNhan(bang: readonly NghiaVu[]): [string, NghiaVu[]][] {
  const nhom = new Map<string, NghiaVu[]>();
  for (const n of bang) nhom.set(n.recipientId, [...(nhom.get(n.recipientId) ?? []), n]);
  return [...nhom.entries()];
}

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

function tenHienThi(ten: string | null | undefined): string {
  if (typeof ten === "string" && ten.trim() !== "") return ten;
  return "Thành viên";
}

/** Money that arrived is a fact and gets the filled seal; the rest is outline. */
function daVeRoi(trangThai: NghiaVu["trangThai"]): boolean {
  return trangThai === "confirmed" || trangThai === "over_confirmed";
}

export function DotThuLiveScreen({ phien, batchId }: { phien: Phien; batchId: string }) {
  const router = useRouter();
  const { colors, dark, radius } = useRudiTheme();
  const contextId = phien.context_id;
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [roster, setRoster] = useState<ThanhVien[]>([]);
  // A failure is said where it happened: under the row whose action failed,
  // under the publish seal, beside «Làm mới» -- not at the top of the page.
  const [loiTai, setLoiTai] = useState<{ noi: string; cau: string } | null>(null);
  const [ban, setBan] = useState(false);
  // Per sender: the sheet was opened and not dismissed. Not "delivered" -- the
  // phone cannot know that -- and the caption says as much.
  const [daMoKhay, setDaMoKhay] = useState<Record<string, Exclude<KetQuaChiaSe, "huy">>>({});
  // Publishing cannot be undone and hands out links exactly once, so the first
  // tap only opens the sentence that says so; the second tap publishes.
  const [sapPhat, setSapPhat] = useState(false);
  const attempts = useRef<Record<string, Attempt>>({});
  // The obligation just confirmed under this finger: its seal lands after the
  // server has said so (the state is re-read, never assumed). Declared before
  // the early return below, so the hook order never changes.
  const [vuaNhan, setVuaNhan] = useState<string | null>(null);

  const doc = useCallback(async () => {
    if (contextId === null) return;
    const [bang, danhSach, links] = await Promise.all([
      docBangThu(contextId, batchId, phien.person_id),
      docDotThuCuaNhom(contextId, phien.person_id),
      docLinkDot(batchId),
    ]);
    const dot = danhSach.find((d) => d.id === batchId);
    setTrang({ pha: "xong", nghiaVu: bang.nghiaVu, soTranhCai: bang.soTranhCai, trangThai: dot === undefined ? null : dot.trangThai, links });
  }, [batchId, contextId, phien.person_id]);

  useEffect(() => {
    if (contextId === null) return;
    let song = true;
    void danhSachThanhVien(contextId, phien.person_id)
      .then((ds) => {
        if (song) setRoster(ds.map((tv) => ({ id: tv.person_id, name: tenHienThi(tv.display_name) })));
      })
      .catch(() => undefined);
    doc().catch((error: unknown) => {
      if (song) setTrang({ pha: "hong", loi: loiRaChu(error) });
    });
    return () => {
      song = false;
    };
  }, [contextId, doc, phien.person_id]);

  if (contextId === null) {
    return (
      <RudiScreen tone="split" testID="collection-batch-screen">
        <TopBar title="Đợt thu" />
        <Heading title="Vào một nhóm trước" subtitle="Đợt thu là của nhóm; chưa có nhóm thì chưa có ai để thu." />
        <RudiButton label="Tới Tin nhắn" onPress={() => router.push("/(tabs)/messages" as never)} tone="split" variant="outline" />
      </RudiScreen>
    );
  }

  const chay = async (noi: string, viec: () => Promise<void>) => {
    setBan(true);
    setLoiTai(null);
    try {
      await viec();
    } catch (error) {
      setLoiTai({ noi, cau: loiRaChu(error) });
    } finally {
      setBan(false);
    }
  };
  const loiO = (noi: string) => (loiTai?.noi === noi ? loiTai.cau : null);

  const phat = () =>
    chay("phat", async () => {
      const links = await phatDotThu(batchId, phien.person_id, attemptFor(attempts.current, `phat:${batchId}`), roster);
      await luuLinkDot(batchId, links);
      setSapPhat(false);
      await doc();
    });

  // Each link's outcome is said on its own row, under the finger that sent it
  // (QA UI-049): «đã mở khay», «đã chép», or the link itself to copy by hand.
  // Never the page-top network sentence: sharing does not touch the network.
  const guiLink = async (envelope: Envelope) => {
    const ketQua = await chiaSe({ text: loiNhanChiaSe(envelope), title: `Phần của ${envelope.senderName}` });
    if (ketQua === "huy") return;
    setDaMoKhay((hienTai) => ({ ...hienTai, [envelope.senderId]: ketQua }));
  };

  const daNhan = (n: NghiaVu) =>
    chay(n.id, async () => {
      await xacNhanDaNhan(n.id, n.amountVnd, phien.person_id, attemptFor(attempts.current, `nhan:${n.id}:${n.amountVnd}`));
      await doc();
      setVuaNhan(n.id);
    });

  const docLai = () => chay("lam-moi", doc);

  if (trang.pha === "dang-doc") {
    return (
      <RudiScreen tone="split" testID="collection-batch-screen">
        <TopBar title="Đợt thu" />
        <SkeletonGroup style={styles.khung}>
          <SkeletonLines lastWidth="50%" lineHeight={24} lines={2} />
          <SkeletonRow leading={0} />
          <SkeletonRow leading={0} />
        </SkeletonGroup>
      </RudiScreen>
    );
  }
  if (trang.pha === "hong") {
    return (
      <RudiScreen tone="split" testID="collection-batch-screen">
        <TopBar title="Đợt thu" />
        <ErrorState body={trang.loi} onRetry={docLai} title="Chưa đọc được bảng thu" />
      </RudiScreen>
    );
  }

  const tom = tomTatBang(trang.nghiaVu);
  const daPhatRoi = trang.trangThai !== null && daPhat(trang.trangThai);
  const toiNhan = new Set(nghiaVuToiNhan(trang.nghiaVu, phien.person_id).map((n) => n.id));
  // What is still waiting comes first; what has arrived settles to the foot.
  const bang = [...trang.nghiaVu].sort((a, b) => Number(daVeRoi(a.trangThai)) - Number(daVeRoi(b.trangThai)));

  return (
    <RudiScreen tone="split" testID="collection-batch-screen">
      <TopBar subtitle={trang.trangThai === null ? undefined : cauTrangThaiDot(trang.trangThai)} title="Đợt thu" />
      {/* The head of the collection's page: the count the server derived from
          receipts, a strip of teal tape that fills as transfers arrive, and
          Nếp bowing its thanks when money has just arrived (M4) -- at the head
          of the page, never on a row. */}
      <View style={styles.dau}>
        <Text style={[typography.h1, { color: colors.ink }]}>
          {tom.daVe}/{tom.tong} lượt chuyển đã về
        </Text>
        {/* Nếp stands beside the tape and the sentence, never beside the count
            it would push onto two lines. */}
        <View style={styles.dauTrang}>
          <View style={[styles.flex, styles.dau]}>
            <DaiTienDo da={tom.daVe} tong={tom.tong} testID="dai-tien-do" />
            {/* The people count only when it says something the heading does
                not: someone with two transfers makes the two differ. */}
            <Text style={[typography.body, { color: colors.inkSoft }]}>
              {tom.nguoiGui !== tom.tong ? `${tom.nguoiXong} trên ${tom.nguoiGui} người đã xong phần mình. ` : ""}
              {daPhatRoi ? "Đã phát: mỗi người xem phần của mình qua link riêng." : "Chưa phát: chưa ai bị nhắn gì."}
            </Text>
          </View>
          <NepDien khoanhKhac="M4" suKien={vuaNhan} />
        </View>
      </View>

      {/* The board is the group's ledger page, one section per person paid:
          «Chuyển cho An», then one entry per sender. The receiver is said once,
          over their section, not on every line with a second figure; a seal
          marks what has happened (đã về, thắc mắc), and a transfer still to
          come carries none (B4 finish review: nineteen «CHƯA CHUYỂN» seals). */}
      <TrangSo ke={false} testID="trang-so-thu">
        {daPhatRoi ? (
          <ChuThichLe>Người được nhận tiền là người bấm «Tiền đã về»; số ở trên đếm từ biên nhận, không phải ai tự khai.</ChuThichLe>
        ) : null}
        {trang.nghiaVu.length === 0 ? (
          <Text style={[typography.body, { color: colors.inkSoft }]}>Đợt này không ai phải chuyển tiền.</Text>
        ) : null}
        {gomTheoNguoiNhan(bang).map(([nguoiNhan, ds]) => (
          <View key={nguoiNhan} style={styles.nhom}>
            <View accessibilityRole="header" style={[styles.dauNhom, { borderBottomColor: colors.lineStrong }]}>
              <Avatar name={tenCua(roster, nguoiNhan)} personId={nguoiNhan} size={28} />
              <Text style={[typography.title, styles.flex, { color: colors.ink }]}>{`Chuyển cho ${tenCua(roster, nguoiNhan)}`}</Text>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>{`${ds.length} lượt`}</Text>
            </View>
            {ds.map((n) => {
              const ve = daVeRoi(n.trangThai);
              const dau = n.trangThai === "outstanding" ? null : TU_NGHIA_VU[n.trangThai];
              return (
                <View key={n.id} style={[styles.hang, { borderBottomColor: colors.line }]}>
                  <View style={styles.dong}>
                    <Avatar name={tenCua(roster, n.senderId)} personId={n.senderId} size={32} />
                    <View style={[styles.flex, styles.cot]}>
                      <Text style={[typography.label, { color: colors.ink }]}>{tenCua(roster, n.senderId)}</Text>
                      {dau !== null || n.tranhCai ? (
                        <View style={styles.dauHang}>
                          {dau !== null ? <Stamp dong={vuaNhan === n.id && ve} label={dau} tone="split" variant={ve ? "ink" : "outline"} /> : null}
                          {n.tranhCai && n.trangThai !== "disputed" ? (
                            <Text style={[typography.caption, { color: colors.warn }]}>đang thắc mắc</Text>
                          ) : null}
                        </View>
                      ) : (
                        <Text style={[typography.caption, { color: colors.inkSoft }]}>chưa về</Text>
                      )}
                    </View>
                    {/* The amount, and under it the row's own action: nineteen
                        full-width «Tiền đã về từ …» buttons, each on a line of
                        its own, turned the ledger into a wall of buttons (B4). */}
                    <View style={styles.cotPhai}>
                      <Money size="label" tone={ve ? "split" : "ink"} vnd={n.amountVnd} />
                      {daPhatRoi && toiNhan.has(n.id) && !ve ? (
                        <RudiButton
                          accessibilityLabel={`Tiền đã về từ ${tenCua(roster, n.senderId)}`}
                          compact
                          disabled={ban}
                          full={false}
                          icon="checkmark-done-outline"
                          label="Đã về"
                          onPress={() => void daNhan(n)}
                          tone="split"
                          variant="outline"
                        />
                      ) : null}
                    </View>
                  </View>
                  <CauTaiCho cau={loiO(n.id)} />
                </View>
              );
            })}
          </View>
        ))}
      </TrangSo>

      {/* Publishing is a decision about money: the teal seal (ADR-0037 D14).
          What it cannot undo is written in full beside it, never under a flap. */}
      {!daPhatRoi && !sapPhat ? (
        <>
          <StampButton disabled={ban} label="Phát đợt thu" onPress={() => setSapPhat(true)} size="vua" tilt={-1} tone="split" />
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            Phát là không hoàn lại: mỗi người nợ nhận một link riêng để xem phần của mình, nghĩa vụ chỉ tồn tại từ lúc đó. Link chỉ hiện một lần và được giữ trên máy này. Chỉ phát được khi người ứng tiền đã xác nhận.
          </Text>
        </>
      ) : null}
      {!daPhatRoi && sapPhat ? (
        <View style={[styles.xacNhan, { borderColor: colors.split, borderRadius: radius.base, backgroundColor: colors.card }]}>
          <Text style={[typography.h2, { color: colors.ink }]}>Phát đợt thu này?</Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>
            Không hoàn lại được. {tom.nguoiGui} người sẽ có link riêng; link chỉ hiện một lần và chỉ máy này giữ.
          </Text>
          <StampButton disabled={ban} label="Phát, không hoàn lại" loading={ban} onPress={() => void phat()} size="vua" tone="split" />
          <CauTaiCho cau={loiO("phat")} />
          <RudiButton disabled={ban} label="Thôi, chưa phát" onPress={() => setSapPhat(false)} tone="split" variant="ghost" />
        </View>
      ) : null}

      {daPhatRoi ? (
        <>
          <SectionHeader title="Gửi link riêng" />
          {trang.links === null ? (
            <Text style={[typography.body, { color: colors.inkSoft }]}>
              Link của đợt này được phát ở máy khác. Rủ Đi chỉ giữ dấu vân của link, nên máy này không lấy lại được; bảng thu ở trên vẫn là thật.
            </Text>
          ) : null}
          {trang.links !== null && trang.links.length === 0 ? (
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Không có link nào: đợt này không ai phải chuyển.</Text>
          ) : null}
          {trang.links !== null && trang.links.length > 0 ? (
            // Every private link is a letter to its one person, all of them in
            // the one envelope this phone keeps; each still goes out on its own.
            <PhongBi testID="phong-bi-link">
              {trang.links.map((env) => (
                <View key={env.senderId} style={[styles.hang, { borderBottomColor: colors.line }]}>
                  <View style={styles.dong}>
                    <Avatar name={env.senderName} personId={env.senderId} size={28} />
                    <View style={styles.flex}>
                      <Text style={[typography.label, { color: mucNguoi(env.senderId, dark) }]}>{env.senderName}</Text>
                      <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: daMoKhay[env.senderId] === "khong-duoc" ? colors.warn : colors.inkSoft }]}>
                        {cauGuiLink(daMoKhay[env.senderId], env.senderName)}
                      </Text>
                      {daMoKhay[env.senderId] === "khong-duoc" ? (
                        <Text selectable style={[typography.caption, { color: colors.ink }]}>
                          {env.url}
                        </Text>
                      ) : null}
                    </View>
                    <View style={styles.cotPhai}>
                      <Money size="label" vnd={env.amountVnd} />
                      <RudiButton
                        accessibilityLabel={daMoKhay[env.senderId] !== undefined ? `Gửi lại cho ${env.senderName}` : `Gửi cho ${env.senderName}`}
                        compact
                        disabled={ban}
                        full={false}
                        icon="share-social-outline"
                        label={daMoKhay[env.senderId] !== undefined ? "Gửi lại" : "Gửi"}
                        onPress={() => void guiLink(env)}
                        tone="split"
                        variant="outline"
                      />
                    </View>
                  </View>
                </View>
              ))}
            </PhongBi>
          ) : null}
        </>
      ) : null}

      <CauTaiCho cau={loiO("lam-moi")} />
      <RudiButton disabled={ban} icon="refresh-outline" label="Làm mới" onPress={docLai} tone="split" variant="ghost" />
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  khung: { gap: 14 },
  dauTrang: { flexDirection: "row", alignItems: "center", gap: 12 },
  dau: { gap: 8 },
  hang: { gap: 10, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  dong: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 48 },
  cot: { gap: 4 },
  dauHang: { flexDirection: "row", alignItems: "center", gap: 8, flexWrap: "wrap" },
  // The amount, and the row's own action under it.
  cotPhai: { alignItems: "flex-end", gap: 6 },
  nhom: { gap: 0 },
  dauNhom: { flexDirection: "row", alignItems: "center", gap: 10, paddingTop: 14, paddingBottom: 8, borderBottomWidth: 1 },
  xacNhan: { gap: 10, padding: 16, borderWidth: 1.5, borderStyle: "dashed" },
});

/** The line under a person's name once their link has been sent somewhere. */
function cauGuiLink(ketQua: Exclude<KetQuaChiaSe, "huy"> | undefined, ten: string): string {
  // No platform reports whether the link was then sent; the row says what is
  // known, and the button beside it already reads «Gửi lại cho …».
  if (ketQua === "da-mo-khay") return `Đã mở khay chia sẻ cho ${ten}.`;
  if (ketQua === "da-chep") return `Đã chép link. Dán vào tin nhắn gửi ${ten}.`;
  if (ketQua === "khong-duoc") return "Máy này không mở được khay chia sẻ. Chép link dưới đây gửi tay:";
  return "Chưa gửi link. Mỗi người một link riêng.";
}
