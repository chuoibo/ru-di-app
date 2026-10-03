import { docSo as docSoDoi } from "../to-giay/to-giay-song";
import { caHaiDongY } from "../to-giay/so-doi-map";
/**
 * The settlement as the ledger has it. Every sum is `Money`; nothing here
 * computes a share (see `QuyetToanLive`). Splitting a bill is
 * `chia-bill/ChiaBillLive.tsx`.
 *
 * UI v2 (đợt 6): the settlement answers what is owed and to whom in rows,
 * with the state as a word beside each.
 */
import { Ionicons } from "@expo/vector-icons";
import { useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, BASE_URL, attemptFor, type Attempt } from "../../api";
import { docQuyetToanLive, dongChiTieuChung, dongHeroQuyetToan, tenCua, type QuyetToanLive } from "../doc-live";
import { laPair } from "../nhan-rieng/nhan-rieng";
import { banTinhCua } from "../so/ban-tinh";
import { cauTomTatDot, cauTrangThaiDot, demKhoanChuaVaoDot, docDotThuCuaNhom, moDotThu, type DotThuTomTat } from "../dot-thu/dot-thu";
import { nguCanhMo } from "../ngu-canh-mo";
import { useRudiSession } from "../session";
import { CuaDangNhap } from "../ui/CuaDangNhap";
import { bongDen, giayHoaDon, lopPhu, mucNguoi, typography, useRudiTheme } from "../theme";
import { ListRow, RudiButton, RudiScreen, SectionHeader, TopBar } from "../ui";
import { Avatar } from "../ui/Avatar";
import { EmptyState } from "../ui/EmptyState";
import { ErrorState } from "../ui/ErrorState";
import { Money } from "../ui/Money";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../ui/Skeleton";
import { SoDoChuyen } from "../ui/SoDoChuyen";
import { TrangSo } from "../ui/TrangSo";
import {  } from "../../ui/a11y";

/** Transfers by the person paid, in the order they first appear. */
function gomTheoNguoiNhan<T extends { toId: string }>(ds: readonly T[]): [string, T[]][] {
  const nhom = new Map<string, T[]>();
  for (const d of ds) nhom.set(d.toId, [...(nhom.get(d.toId) ?? []), d]);
  return [...nhom.entries()];
}

export function SettlementScreen() {
  const session = useRudiSession();
  const router = useRouter();
  const params = useLocalSearchParams<{ id?: string }>();
  // `/settlements/{id}` names the ledger. It used to be ignored, so a bill
  // split in a pair (never the current group) showed the current group's
  // ledger under «Xem quyết toán». Only a context the person is active in.
  const mo = session.phien === null ? null : nguCanhMo(session.phien, params.id);
  if (session.phien !== null && mo?.kieu === "khac") {
    return <QuyetToanLive actorId={session.phien.person_id} contextId={mo.contextId} key={mo.contextId} />;
  }
  if (session.nguon.kieu === "live") {
    return <QuyetToanLive actorId={session.nguon.actorId} contextId={session.nguon.contextId} />;
  }
  if (session.phien === null) return <CuaDangNhap />;
  // Signed in with no group: there is no ledger to settle.
  return (
    <RudiScreen tone="split" testID="settlement-screen">
      <TopBar title="Quyết toán" />
      <EmptyState
        action={{ label: "Tới Tin nhắn", onPress: () => router.replace("/messages" as never) }}
        body="Quyết toán tính từ sổ của một nhóm. Vào hoặc tạo một nhóm trước."
        kind="first-use"
        title="Chưa có sổ nào để quyết toán"
      />
    </RudiScreen>
  );
}

/**
 * The settlement as the ledger has it.
 *
 * Nothing here computes money. `/contexts/{id}/balances` recomputes the net and
 * the minimal transfer set per request, and re-deriving either on the phone
 * would be the second allocator this repo has already thrown out once.
 */
function QuyetToanLive({ actorId, contextId }: { actorId: string; contextId: string }) {
  const { colors, dark, radius } = useRudiTheme();
  const router = useRouter();
  const { phien } = useRudiSession();
  // A two-person notebook shows its money as shared spending (`ban-tinh.ts`
  // `tienHien`), with the transfer list one tap away instead of on top. Only a
  // `pair` context: a group that happens to have two members is not a couple,
  // and the screen does not guess.
  const [laDoi, setLaDoi] = useState(false);
  useFocusEffect(useCallback(() => {
    let current = true;
    setLaDoi(false);
    if (laPair(phien?.contexts?.find((c) => c.id === contextId))) {
      void docSoDoi(contextId, { actorId }).then((so) => { if (current) setLaDoi(caHaiDongY(so, "bat_doi") && banTinhCua("doi").tienHien === "chi-tieu-chung"); }).catch(() => undefined);
    }
    return () => { current = false; };
  }, [actorId, contextId, phien?.contexts]));
  const [moChuyen, setMoChuyen] = useState(false);
  const [du, setDu] = useState<QuyetToanLive | null>(null);
  const [loi, setLoi] = useState<string | null>(null);
  // The group's collection rounds, read beside the balances. `"hong"` keeps
  // the transfer list on screen when only this read fails: they answer
  // different questions and fail independently.
  const [dotThu, setDotThu] = useState<DotThuTomTat[] | "hong" | null>(null);
  const [dangMo, setDangMo] = useState(false);
  const [loiDot, setLoiDot] = useState<string | null>(null);
  // How many recorded expenses a new round would gather; null until read, or
  // when the read failed (then the button stays, and the server decides).
  const [chuaVaoDot, setChuaVaoDot] = useState<number | null>(null);
  const attempts = useRef<Record<string, Attempt>>({});

  // On focus, not on mount: coming back from a round (created or just
  // published) must show it, and a stack keeps this screen mounted underneath.
  useFocusEffect(
    useCallback(() => {
      let song = true;
      void docDotThuCuaNhom(contextId, actorId)
        .then((ds) => {
          if (song) setDotThu(ds);
        })
        .catch(() => {
          if (song) setDotThu("hong");
        });
      void demKhoanChuaVaoDot(contextId, actorId)
        .then((n) => {
          if (song) setChuaVaoDot(n);
        })
        .catch(() => {
          if (song) setChuaVaoDot(null);
        });
      return () => {
        song = false;
      };
    }, [actorId, contextId]),
  );

  const moDot = async () => {
    setDangMo(true);
    setLoiDot(null);
    try {
      const soDot = dotThu === null || dotThu === "hong" ? 0 : dotThu.length;
      const dot = await moDotThu({
        contextId,
        actorId,
        expenseVersionIds: null,
        attempt: attemptFor(attempts.current, `mo-dot:${contextId}:${soDot}`),
      });
      // The round opens in the ledger's own context: a pair's is never the current group.
      router.push(`/batches/${dot.batchId}?ctx=${contextId}` as never);
    } catch (error) {
      setLoiDot(error instanceof ApiError ? error.message : "Không mở được đợt thu.");
    } finally {
      setDangMo(false);
    }
  };

  const docSo = useCallback(() => {
    let song = true;
    setLoi(null);
    void docQuyetToanLive(actorId, contextId, BASE_URL)
      .then((ketQua) => {
        if (song) setDu(ketQua);
      })
      .catch((error: unknown) => {
        // The real sentence from the real failure. Falling back to the fixture
        // here would answer a broken request with somebody else's money.
        if (!song) return;
        setLoi(
          error instanceof ApiError
            ? error.message
            : "Không đọc được quyết toán.",
        );
      });
    return () => {
      song = false;
    };
  }, [actorId, contextId]);

  // On focus, like the rounds below: a receipt confirmed on the round screen
  // empties the ledger's transfer list, and coming back must show that.
  useFocusEffect(docSo);

  if (loi !== null) {
    return (
      <RudiScreen tone="split" testID="settlement-screen">
        <TopBar title="Quyết toán" />
        <ErrorState body={loi} onRetry={() => void docSo()} title="Chưa đọc được sổ" />
      </RudiScreen>
    );
  }
  if (du === null) {
    return (
      <RudiScreen tone="split" testID="settlement-screen">
        <TopBar title="Quyết toán" />
        <SkeletonGroup style={styles.khung}>
          <SkeletonLines lastWidth="45%" lineHeight={24} lines={2} />
          <SkeletonRow leading={0} />
          <SkeletonRow leading={0} />
        </SkeletonGroup>
      </RudiScreen>
    );
  }
  const hero = dongHeroQuyetToan(du.tongChuyen, du.nguoi.length);
  if (laDoi && !moChuyen) {
    const chung = dongChiTieuChung(du.nguoi, du.soDu);
    return (
      <RudiScreen tone="split" testID="settlement-screen">
        <TopBar title="Chi tiêu chung" />
        <Text style={[typography.body, { color: colors.inkSoft }]}>
          {chung.ngangNhau
            ? "Hai bạn đang ngang nhau: mỗi người đã trả đúng phần của mình."
            : "Tính từ sổ của hai bạn, mỗi lần mở lại tính lại. Không ai phải làm gì cả nếu hai bạn thấy ổn."}
        </Text>
        <View testID="chi-tieu-chung">
          {chung.dong.map((d) => (
            <View key={d.personId} style={[styles.hangChuyen, { borderBottomColor: colors.line }]}>
              <View style={styles.flex}>
                <Text style={[typography.label, { color: colors.ink }]}>{d.ten}</Text>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>{d.cau}</Text>
              </View>
              {d.vnd !== null ? <Money tone="split" vnd={d.vnd} /> : null}
            </View>
          ))}
        </View>
        {!chung.ngangNhau ? (
          <RudiButton icon="swap-horizontal-outline" label="Muốn cân lại? Xem cách chuyển" onPress={() => setMoChuyen(true)} tone="split" variant="ghost" />
        ) : null}
      </RudiScreen>
    );
  }
  return (
    <RudiScreen tone="split" testID="settlement-screen">
      <TopBar onBack={laDoi ? () => setMoChuyen(false) : undefined} title={laDoi ? "Cân lại chi tiêu" : "Quyết toán"} />
      {/* The ledger's first line, not a hero: the sum the server holds, its
          name beside it, written at the head of the group's ledger page. */}
      <TrangSo ke={false} testID="trang-so-quyet-toan">
        {/* A sum sits beside its name; a state («Chưa có chuyến») is not a sum
            and goes under the sentence: holding the right column, it pressed
            the sentence into nine lines at 320 (QA UI-061). In the money face
            it read as a value sitting where a number goes (QA 23/09). */}
        <View style={hero.laSo ? styles.hangDauSo : styles.cotDauSo}>
          <View style={hero.laSo ? styles.flex : undefined}>
            <Text style={[typography.label, { color: colors.ink }]}>{hero.nhan}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{hero.cau}</Text>
          </View>
          <Text style={hero.laSo ? [typography.money, { color: colors.split }] : [typography.caption, { color: colors.inkSoft }]}>{hero.so}</Text>
        </View>
      </TrangSo>
      {/* Who pays whom, drawn: ink arrows between the people, no number on
          them -- the amounts are the rows under it. */}
      {du.chuyenTien.length > 0 ? (
        <SoDoChuyen
          chuyen={du.chuyenTien.map((row) => ({ tu: row.fromId, toi: row.toId }))}
          nguoi={du.nguoi.map((n) => ({ id: n.personId, ten: n.ten }))}
          testID="so-do-chuyen"
        />
      ) : null}
      <SectionHeader title="Các khoản chuyển" />
      {/* What these rows are is said once, over them: printed under every row
          it read nineteen times on a twenty-person trip (Luật Nói Một Lần). */}
      {du.chuyenTien.length > 0 ? (
        <Text style={[typography.caption, { color: colors.inkSoft }]}>Đề xuất tính từ sổ, chưa phải nghĩa vụ: nghĩa vụ chỉ có khi một đợt thu được phát.</Text>
      ) : null}
      {du.chuyenTien.length === 0 ? (
        <Text style={[typography.body, { color: colors.inkSoft }]}>Sổ không còn ai nợ ai: mọi khoản đã về hoặc chưa có khoản nào được ghi.</Text>
      ) : null}
      {/* One section per person paid, said once over it: nineteen rows each
          ending «→ Chat Test 01» in display-size teal all shouted and none led
          (B4 finish review). The amounts are figures on the page, teal. */}
      {gomTheoNguoiNhan(du.chuyenTien).map(([nguoiNhan, ds]) => (
        <View key={nguoiNhan}>
          <View accessibilityRole="header" style={[styles.dauNhomChuyen, { borderBottomColor: colors.lineStrong }]}>
            <Avatar name={tenCua(du.nguoi, nguoiNhan)} personId={nguoiNhan} size={28} />
            <Text style={[typography.title, styles.flex, { color: mucNguoi(nguoiNhan, dark) }]}>{`Chuyển cho ${tenCua(du.nguoi, nguoiNhan)}`}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{`${ds.length} khoản`}</Text>
          </View>
          {ds.map((row) => (
            <View accessibilityLabel={`${tenCua(du.nguoi, row.fromId)} chuyển cho ${tenCua(du.nguoi, nguoiNhan)}`} key={`${row.fromId}-${row.toId}`} style={[styles.hangChuyen, { borderBottomColor: colors.line }]}>
              <Avatar name={tenCua(du.nguoi, row.fromId)} personId={row.fromId} size={32} />
              <Text style={[typography.label, styles.flex, { color: mucNguoi(row.fromId, dark) }]}>{tenCua(du.nguoi, row.fromId)}</Text>
              <Money size="label" tone="split" vnd={row.amountVnd} />
            </View>
          ))}
        </View>
      ))}
      <View style={styles.ghiChu}>
        <Ionicons color={colors.split} name="shield-checkmark-outline" size={20} />
        <Text style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>
          {du.toiThieu
            ? "Đây là danh sách chuyển ngắn nhất, tính từ sổ."
            : "Danh sách này chưa được chứng minh là ngắn nhất."}
        </Text>
      </View>
      <SectionHeader title="Đợt thu" />
      {dotThu === null ? <Text style={[typography.caption, { color: colors.inkFaint }]}>Đang đọc các đợt thu…</Text> : null}
      {dotThu === "hong" ? <Text style={[typography.caption, { color: colors.warn }]}>Chưa đọc được các đợt thu của nhóm.</Text> : null}
      {Array.isArray(dotThu) && dotThu.length === 0 ? (
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Chưa có đợt thu nào.</Text>
      ) : null}
      {Array.isArray(dotThu)
        ? dotThu.map((dot) => (
            <ListRow
              icon="receipt-outline"
              key={dot.id}
              onPress={() => router.push(`/batches/${dot.id}?ctx=${contextId}` as never)}
              subtitle={cauTomTatDot(dot)}
              title={cauTrangThaiDot(dot.trangThai)}
              tone="split"
            />
          ))
        : null}
      {loiDot !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loiDot}</Text> : null}
      {/* Offered only when the server has expenses to gather: offered
          whenever anyone owed anyone, it was refused every time all of them
          were already in a round (QA UI-058). */}
      {du.chuyenTien.length > 0 && chuaVaoDot === 0 ? (
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Mọi khoản đã ghi đều đã vào một đợt thu ở trên: chưa có gì mới để gom.</Text>
      ) : du.chuyenTien.length > 0 ? (
        <>
          <RudiButton disabled={dangMo} icon="add" label="Tạo đợt thu từ sổ" loading={dangMo} onPress={() => void moDot()} tone="split" variant="soft" />
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            {chuaVaoDot !== null ? `Gom ${chuaVaoDot} khoản đã ghi mà chưa vào đợt nào thành một đợt thu.` : "Gom mọi khoản đã ghi mà chưa vào đợt nào thành một đợt thu."} Chưa phát thì chưa ai bị nhắn gì.
          </Text>
        </>
      ) : (
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Sổ không còn khoản nào ngoài đợt: không có gì để gom thêm.</Text>
      )}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  khung: { gap: 14 },
  wood: { minHeight: 420, overflow: "hidden", borderRadius: 20, backgroundColor: giayHoaDon.khung, padding: 24, alignItems: "center", justifyContent: "center", borderWidth: 1, borderColor: lopPhu.trang(0.22) },
  receipt: { width: "88%", maxWidth: 390, minHeight: 380, borderRadius: 3, paddingHorizontal: 22, paddingVertical: 22, shadowColor: bongDen, shadowOpacity: 0.48, shadowRadius: 20, shadowOffset: { width: 0, height: 13 }, elevation: 14, transform: [{ rotate: "-1.2deg" }] },
  receiptCompact: { minHeight: 0, paddingVertical: 18 },
  paperHighlight: { position: "absolute", left: 8, top: 0, bottom: 0, width: 1, backgroundColor: lopPhu.trang(0.72) },
  receiptStore: { color: giayHoaDon.chuDam, textAlign: "center", fontSize: 19, lineHeight: 24, fontWeight: "900" },
  receiptSmall: { color: giayHoaDon.chuNhat, fontSize: 11, lineHeight: 18, fontWeight: "600" },
  receiptDash: { borderTopWidth: 1, borderStyle: "dashed", borderColor: giayHoaDon.chuMo, marginVertical: 12 },
  receiptTitle: { color: giayHoaDon.chuDam, fontSize: 13, lineHeight: 18, fontWeight: "900", textAlign: "center", marginBottom: 9 },
  receiptHeader: { flexDirection: "row", marginBottom: 8 },
  receiptRow: { flexDirection: "row", minHeight: 23 },
  receiptCell: { color: giayHoaDon.chuVua, fontSize: 11, lineHeight: 17 },
  receiptIndex: { width: 28 },
  receiptName: { flex: 1 },
  receiptAmount: { width: 80, textAlign: "right", fontVariant: ["tabular-nums"] },
  receiptTotal: { flexDirection: "row", justifyContent: "space-between" },
  receiptTotalLabel: { color: giayHoaDon.chuDam, fontSize: 15, fontWeight: "900" },
  receiptTotalValue: { color: giayHoaDon.chuDam, fontSize: 17, fontWeight: "900", fontVariant: ["tabular-nums"] },
  receiptThanks: { textAlign: "center", fontSize: 11, marginTop: 20 },
  dongMon: { borderBottomWidth: StyleSheet.hairlineWidth, paddingVertical: 4 },
  dongMonDau: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 56, paddingVertical: 6 },
  sua: { gap: 10, paddingBottom: 12 },
  pressed: { opacity: 0.75 },
  tongRow: { flexDirection: "row", alignItems: "center", gap: 12, paddingTop: 4 },
  tomTat: { paddingVertical: 2 },
  nguoiThu: { gap: 10, paddingVertical: 12, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
  nguoiThuDau: { flexDirection: "row", alignItems: "center", gap: 12 },
  hangChuyen: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 52, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth },
  dauNhomChuyen: { flexDirection: "row", alignItems: "center", gap: 10, paddingTop: 10, paddingBottom: 8, borderBottomWidth: 1 },
  hangDauSo: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 48 },
  cotNguoiThu: { gap: 4, alignItems: "flex-start" },
  cotDauSo: { gap: 6 },
  transferRight: { alignItems: "flex-end", gap: 6 },
  ghiChu: { flexDirection: "row", alignItems: "flex-start", gap: 9 },
});
