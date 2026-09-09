/**
 * The bill on the fixture build, and the settlement on both builds.
 *
 * Fixture: a printed sample receipt on the table, then «ai dùng món nào»
 * with the same collapsed rows and roster the live flow uses, then the
 * settlement draft. Live: the settlement as the ledger has it. Every sum is
 * `Money`; nothing here computes a share (see `QuyetToanLive`).
 *
 * UI v2 (đợt 6): the receipt is an input, not a stage -- the actions come
 * first and the paper is shown at reading size; a line is a row that opens
 * to its roster; the settlement answers what is owed and to whom in rows,
 * with the state as a word beside each.
 */
import { Ionicons } from "@expo/vector-icons";
import { DongTien } from "../ui/DongTien";
import * as ImagePicker from "expo-image-picker";
import { Image } from "expo-image";
import { LinearGradient } from "expo-linear-gradient";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, BASE_URL, attemptFor, scanReceipt, type Attempt } from "../../api";
import { docQuyetToanLive, dongHeroQuyetToan, tenCua, type QuyetToanLive } from "../doc-live";
import { cauTomTatDot, cauTrangThaiDot, docDotThuCuaNhom, moDotThu, type DotThuTomTat } from "../dot-thu/dot-thu";
import { DEMO_PEOPLE } from "../nhom-demo";
import { BILL_ITEMS, COLLECTOR_INDEX, DEMO_GROUP, PEOPLE, demoAssets, formatVnd } from "../fixtures";
import { noiLuuNgan } from "../luu-tru";
import { useRudiSession } from "../session";
import { bongDen, giayHoaDon, lopPhu, typography, useRudiTheme } from "../theme";
import {
  Chip,
  DemoBadge,
  Heading,
  Inline,
  ListRow,
  RudiButton,
  RudiScreen,
  SectionHeader,
  TopBar,
} from "../ui";
import { aiCoGi } from "../chia-bill/ai-co-gi";
import { AiCoGi } from "../ui/AiCoGi";
import { Avatar } from "../ui/Avatar";
import { HaiCot } from "../ui/HaiCot";
import { useAdaptiveLayout } from "../ui/useAdaptiveLayout";
import { ErrorState } from "../ui/ErrorState";
import { Money } from "../ui/Money";
import { RosterPicker } from "../ui/RosterPicker";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../ui/Skeleton";
import { Stamp } from "../ui/Stamp";

function ReceiptPaper({ compact = false }: { compact?: boolean }) {
  const { colors } = useRudiTheme();
  const detectedTotal = BILL_ITEMS.reduce((sum, item) => sum + item.amount, 0);

  return (
    <LinearGradient
      colors={[giayHoaDon.nen[0], giayHoaDon.nen[1], giayHoaDon.nen[2]]}
      end={{ x: 1, y: 1 }}
      start={{ x: 0, y: 0 }}
      style={[styles.receipt, compact && styles.receiptCompact]}
    >
      <View pointerEvents="none" style={styles.paperHighlight} />
      <Text style={styles.receiptStore}>TIỆM NƯỚNG XÓM LÈO</Text>
      <Text style={styles.receiptSmall}>Đà Lạt, Lâm Đồng</Text>
      <View style={styles.receiptDash} />
      <Text style={styles.receiptTitle}>HÓA ĐƠN THANH TOÁN</Text>
      <Text style={styles.receiptSmall}>Bàn: 07</Text>
      <Text style={styles.receiptSmall}>Ngày: 17/10/2026 19:45</Text>
      <View style={styles.receiptDash} />
      {!compact ? (
        <>
          <View style={styles.receiptHeader}>
            <Text style={[styles.receiptCell, styles.receiptIndex]}>STT</Text>
            <Text style={[styles.receiptCell, styles.receiptName]}>MÓN</Text>
            <Text style={[styles.receiptCell, styles.receiptAmount]}>THÀNH TIỀN</Text>
          </View>
          {BILL_ITEMS.map((item, index) => (
            <View key={item.name} style={styles.receiptRow}>
              <Text style={[styles.receiptCell, styles.receiptIndex]}>{index + 1}</Text>
              <Text style={[styles.receiptCell, styles.receiptName]}>{item.name}</Text>
              <Text style={[styles.receiptCell, styles.receiptAmount]}>{item.amount.toLocaleString("vi-VN")}</Text>
            </View>
          ))}
        </>
      ) : null}
      <View style={styles.receiptDash} />
      <View style={styles.receiptTotal}>
        <Text style={styles.receiptTotalLabel}>TỔNG CỘNG</Text>
        <Text style={styles.receiptTotalValue}>{formatVnd(detectedTotal)}</Text>
      </View>
      {!compact ? <Text style={[styles.receiptThanks, { color: colors.inkSoft }]}>Cảm ơn quý khách!</Text> : null}
    </LinearGradient>
  );
}

export function ReceiptReviewScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const session = useRudiSession();
  const [busy, setBusy] = useState(false);
  const [scanNote, setScanNote] = useState<string | null>(null);

  const pickPhoto = async () => {
    setBusy(true);
    setScanNote(null);
    try {
      const picked = await ImagePicker.launchImageLibraryAsync({
        mediaTypes: ["images"],
        quality: 0.8,
      });
      if (picked.canceled || !picked.assets[0]) {
        setScanNote("Không chọn ảnh.");
        return;
      }
      session.setReceiptPicked(true);
      try {
        await scanReceipt(
          { uri: picked.assets[0].uri, bytes: picked.assets[0].fileSize ?? 0 },
          DEMO_PEOPLE[0].personId,
        );
        setScanNote("Đã gửi ảnh. Các dòng bên dưới vẫn là bill mẫu.");
      } catch (error) {
        const message = error instanceof ApiError ? error.message : "Không đọc được bill.";
        setScanNote(message);
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <RudiScreen tone="split" testID="receipt-review-screen">
      <TopBar title="Xem lại hóa đơn" right={<DemoBadge />} />
      {/* The decision first, the paper after: what this bill is, and what to do with it. */}
      <Heading
        title="Giấy mẫu Tiệm Nướng Xóm Lèo"
        subtitle={`Bill mẫu · ${BILL_ITEMS.length} dòng · tổng ${formatVnd(DEMO_GROUP.billTotalVnd)}. Bạn đang thử bằng dữ liệu mẫu.`}
      />
      <Inline gap={10}>
        <RudiButton
          full={false}
          icon="images-outline"
          label="Chọn ảnh bill"
          loading={busy}
          onPress={() => void pickPhoto()}
          style={styles.flex}
          tone="split"
          variant="outline"
        />
        <RudiButton
          full={false}
          icon="arrow-forward"
          label="Dùng giấy mẫu"
          onPress={() => router.push(("/smart-split/" + DEMO_GROUP.id + "/assignment") as never)}
          style={styles.flex}
          tone="split"
        />
      </Inline>
      {scanNote ? (
        <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]}>{scanNote}</Text>
      ) : (
        <Text style={[typography.caption, { color: colors.inkFaint }]}>
          {session.receiptPicked ? "Đã chọn ảnh trên máy." : "Chọn ảnh từ thư viện để thử đọc bill."}
        </Text>
      )}
      <View style={styles.wood}>
        <Image contentFit="cover" source={demoAssets.wood} style={StyleSheet.absoluteFill} />
        <LinearGradient
          colors={[lopPhu.toi(0.34), lopPhu.toi(0.02), lopPhu.toi(0.38)]}
          locations={[0, 0.52, 1]}
          style={StyleSheet.absoluteFill}
        />
        <ReceiptPaper />
      </View>
    </RudiScreen>
  );
}

/** The same «who had what» rows the live flow draws, on the sample bill. */
export function OcrAssignmentScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { twoPane } = useAdaptiveLayout();
  const session = useRudiSession();
  const detectedTotal = session.money.billTotal;
  const [moRong, setMoRong] = useState<Set<number>>(() => new Set([0]));
  const doiMo = (i: number) =>
    setMoRong((cu) => {
      const moi = new Set(cu);
      if (moi.has(i)) moi.delete(i);
      else moi.add(i);
      return moi;
    });
  /** Set one line to exactly `ids` through the session's own toggle. */
  const datNguoi = (itemIndex: number, ids: readonly string[]) => {
    const dang = session.assignments[itemIndex].map((i) => PEOPLE[i].id);
    PEOPLE.forEach((p, i) => {
      const nen = ids.includes(p.id);
      if (dang.includes(p.id) !== nen) session.toggleAssignment(itemIndex, i);
    });
  };

  // The fixture keeps its assignment by index; «Ai có gì» reads it by id.
  const bangAiCoGi = aiCoGi(
    BILL_ITEMS.map((item, i) => ({ id: String(i), name: item.name })),
    PEOPLE.map((p) => ({ id: p.id, name: p.name })),
    Object.fromEntries(session.assignments.map((people, i) => [String(i), people.map((index) => PEOPLE[index].id)])),
  );
  const tongBill = (
    <View style={styles.tongRow}>
      <View style={styles.flex}>
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Tổng hóa đơn Xóm Lèo</Text>
        <Text style={[typography.caption, { color: colors.inkSoft }]}>Không gồm homestay / xăng</Text>
      </View>
      <Money tone="split" vnd={detectedTotal} />
    </View>
  );

  return (
    <RudiScreen tone="split" testID="ocr-assignment-screen">
      <TopBar title="Ai dùng món nào?" right={<DemoBadge compactLabel="Nháp" label="Nháp trên máy" />} />
      <HaiCot
        phaiChiKhiRong
        phai={
          <>
            <AiCoGi bang={bangAiCoGi} />
            {tongBill}
          </>
        }
        trai={
          <>
      <Heading title={`${BILL_ITEMS.length} món · ${formatVnd(detectedTotal)}`} subtitle="Chạm một món để sửa ai dùng. Tổng bill giữ nguyên khi bạn sửa người." />
      <View>
        {BILL_ITEMS.map((item, itemIndex) => {
          const mo = moRong.has(itemIndex);
          const dangDung = session.assignments[itemIndex].map((index) => PEOPLE[index]);
          return (
            <View key={item.name} style={[styles.dongMon, { borderBottomColor: colors.line }]}>
              <Pressable
                accessibilityLabel={`Sửa người dùng ${item.name}`}
                accessibilityRole="button"
                accessibilityState={{ expanded: mo }}
                onPress={() => doiMo(itemIndex)}
                style={({ pressed }) => [styles.dongMonDau, pressed && styles.pressed]}
              >
                <View style={styles.flex}>
                  <Text style={[typography.label, { color: colors.ink }]}>{item.name}</Text>
                  {/* The count is its own node, the names another: the count is what a glance reads. */}
                  <Text style={[typography.caption, { color: dangDung.length === 0 ? colors.warn : colors.inkSoft }]}>
                    {dangDung.length === 0 ? "Chưa chọn người" : `${dangDung.length} người`}
                  </Text>
                  {dangDung.length > 0 ? (
                    <Text numberOfLines={1} style={[typography.caption, { color: colors.inkFaint }]}>{dangDung.map((p) => p.name).join(", ")}</Text>
                  ) : null}
                </View>
                <Money size="label" vnd={item.amount} />
                <Ionicons color={colors.inkFaint} name={mo ? "chevron-up" : "chevron-down"} size={18} />
              </Pressable>
              {mo ? (
                <View style={styles.sua}>
                  <RosterPicker
                    nhanCho={(ten) => `${ten} · ${item.name}`}
                    onToggle={(id) => session.toggleAssignment(itemIndex, PEOPLE.findIndex((person) => person.id === id))}
                    people={PEOPLE}
                    selected={dangDung.map((p) => p.id)}
                  />
                  <Inline gap={6} wrap>
                    <Chip label="Cả nhóm" onPress={() => datNguoi(itemIndex, PEOPLE.map((p) => p.id))} tone="split" />
                    <Chip label="Bỏ hết" onPress={() => datNguoi(itemIndex, [])} tone="split" />
                    {itemIndex > 0 ? (
                      <Chip label="Như món trên" onPress={() => datNguoi(itemIndex, session.assignments[itemIndex - 1].map((i) => PEOPLE[i].id))} tone="split" />
                    ) : null}
                  </Inline>
                </View>
              ) : null}
            </View>
          );
        })}
      </View>
          </>
        }
      />
      {twoPane ? null : tongBill}
      <RudiButton
        disabled={session.assignments.some((people) => people.length === 0)}
        icon="checkmark-circle-outline"
        label="Xác nhận cách chia"
        onPress={() => router.replace(("/settlements/" + DEMO_GROUP.id) as never)}
        tone="split"
      />
    </RudiScreen>
  );
}

/**
 * Two screens behind one name, and the switch is `nguon`, never a probe.
 *
 * The draft below is a picture of a fixture; the live one is a picture of a
 * ledger. Deciding between them by asking whether a server happens to answer
 * would let the two swap places with no action by the person holding the phone,
 * which is the failure this whole seam exists to prevent. See `src/rudi/nguon.ts`.
 */
export function SettlementScreen() {
  const session = useRudiSession();
  if (session.nguon.kieu === "live") {
    return <QuyetToanLive actorId={session.nguon.actorId} contextId={session.nguon.contextId} />;
  }
  return <QuyetToanNhap />;
}

/**
 * The settlement as the ledger has it.
 *
 * Nothing here computes money. `/contexts/{id}/balances` recomputes the net and
 * the minimal transfer set per request, and re-deriving either on the phone
 * would be the second allocator this repo has already thrown out once.
 */
function QuyetToanLive({ actorId, contextId }: { actorId: string; contextId: string }) {
  const { colors, radius } = useRudiTheme();
  const router = useRouter();
  const [du, setDu] = useState<QuyetToanLive | null>(null);
  const [loi, setLoi] = useState<string | null>(null);
  // The group's collection rounds, read beside the balances. `"hong"` keeps
  // the transfer list on screen when only this read fails: they answer
  // different questions and fail independently.
  const [dotThu, setDotThu] = useState<DotThuTomTat[] | "hong" | null>(null);
  const [dangMo, setDangMo] = useState(false);
  const [loiDot, setLoiDot] = useState<string | null>(null);
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
      router.push(`/batches/${dot.batchId}` as never);
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
        <TopBar title="Quyết toán chuyến đi" />
        <ErrorState body={loi} onRetry={() => void docSo()} title="Chưa đọc được sổ" />
      </RudiScreen>
    );
  }
  if (du === null) {
    return (
      <RudiScreen tone="split" testID="settlement-screen">
        <TopBar title="Quyết toán chuyến đi" />
        <SkeletonGroup style={styles.khung}>
          <SkeletonLines lastWidth="45%" lineHeight={24} lines={2} />
          <SkeletonRow leading={0} />
          <SkeletonRow leading={0} />
        </SkeletonGroup>
      </RudiScreen>
    );
  }
  const hero = dongHeroQuyetToan(du.tongChuyen, du.nguoi.length);
  return (
    <RudiScreen tone="split" testID="settlement-screen">
      <TopBar title="Quyết toán chuyến đi" />
      {/* The ledger's first line, not a hero: the sum the server holds, its name beside it. */}
      <View style={[styles.hangChuyen, { borderBottomColor: colors.line }]}>
        <View style={styles.flex}>
          <Text style={[typography.label, { color: colors.ink }]}>{hero.nhan}</Text>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>{hero.cau}</Text>
        </View>
        <Text style={[typography.money, { color: colors.split }]}>{hero.so}</Text>
      </View>
      <SectionHeader title="Các khoản chuyển" />
      {du.chuyenTien.length === 0 ? (
        <Text style={[typography.body, { color: colors.inkSoft }]}>Sổ không còn ai nợ ai: mọi khoản đã về hoặc chưa có khoản nào được ghi.</Text>
      ) : null}
      <View>
        {du.chuyenTien.map((row) => (
          <View key={`${row.fromId}-${row.toId}`} style={[styles.hangChuyen, { borderBottomColor: colors.line }]}>
            <View style={styles.flex}>
              <Text style={[typography.label, { color: colors.ink }]}>
                {tenCua(du.nguoi, row.fromId)} → {tenCua(du.nguoi, row.toId)}
              </Text>
              <Text style={[typography.caption, { color: colors.inkFaint }]}>Đề xuất, chưa phải nghĩa vụ</Text>
            </View>
            <Money tone="split" vnd={row.amountVnd} />
          </View>
        ))}
      </View>
      <View style={styles.ghiChu}>
        <Ionicons color={colors.split} name="shield-checkmark-outline" size={20} />
        <Text style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>
          {du.toiThieu
            ? "Máy chủ chứng minh đây là danh sách chuyển ngắn nhất."
            : "Danh sách này chưa được chứng minh là ngắn nhất."}{" "}
          Nghĩa vụ chỉ tồn tại sau khi một đợt thu được phát.
        </Text>
      </View>
      <SectionHeader title="Đợt thu" />
      {dotThu === null ? <Text style={[typography.caption, { color: colors.inkFaint }]}>Đang đọc các đợt thu…</Text> : null}
      {dotThu === "hong" ? <Text style={[typography.caption, { color: colors.warn }]}>Chưa đọc được các đợt thu của nhóm.</Text> : null}
      {Array.isArray(dotThu) && dotThu.length === 0 ? (
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Chưa có đợt thu nào. Các khoản chuyển ở trên mới là đề xuất.</Text>
      ) : null}
      {Array.isArray(dotThu)
        ? dotThu.map((dot) => (
            <ListRow
              icon="receipt-outline"
              key={dot.id}
              onPress={() => router.push(`/batches/${dot.id}` as never)}
              subtitle={cauTomTatDot(dot)}
              title={cauTrangThaiDot(dot.trangThai)}
              tone="split"
            />
          ))
        : null}
      {loiDot !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loiDot}</Text> : null}
      {du.chuyenTien.length > 0 ? (
        <>
          <RudiButton disabled={dangMo} icon="add" label="Tạo đợt thu từ sổ" loading={dangMo} onPress={() => void moDot()} tone="split" variant="soft" />
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            Gom mọi khoản đã ghi mà chưa vào đợt nào thành một đợt thu. Chưa phát thì chưa ai bị nhắn gì.
          </Text>
        </>
      ) : (
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Sổ không còn khoản nào ngoài đợt: không có gì để gom thêm.</Text>
      )}
    </RudiScreen>
  );
}

function QuyetToanNhap() {
  const { colors, radius } = useRudiTheme();
  const session = useRudiSession();
  const picture = session.money;
  const collector = PEOPLE[COLLECTOR_INDEX];
  // The row the person just marked: its seal lands; rows already marked when
  // the screen opened simply carry theirs (no replay on remount).
  const [vuaTra, setVuaTra] = useState<number | null>(null);
  const paidCount = session.paidFromIndexes.length;
  const pendingCount = picture.transfers.filter((row) => !session.paidFromIndexes.includes(row.fromIndex)).length;
  const paidSum = picture.transfers
    .filter((row) => session.paidFromIndexes.includes(row.fromIndex))
    .reduce((sum, row) => sum + row.amount, 0);

  return (
    <RudiScreen tone="split" testID="settlement-screen">
      <TopBar title="Quyết toán chuyến đi" right={<DemoBadge />} />
      {/* A ledger, not a dashboard: every sum is a row, teal only on the number. */}
      <View>
        <DongTien dam nhan="Tổng chi tiêu cả chuyến (8 người)" phu="Nháp trên máy, chưa confirm vào sổ cái" tone="split" vnd={picture.tripTotal} />
        <DongTien nhan="Bill Xóm Lèo" vnd={picture.billTotal} />
        <DongTien nhan="Homestay + xăng" phu="Phần còn lại của chuyến" vnd={picture.otherTotal} />
      </View>
      <View style={styles.tomTat}>
        <Text style={[typography.body, { color: colors.ink }]}>
          {String(paidCount)} đã trả · {String(pendingCount)} đang chờ · {String(PEOPLE.length)} thành viên
        </Text>
      </View>
      <View style={[styles.nguoiThu, { borderTopColor: colors.line, borderBottomColor: colors.line }]}>
        <View style={styles.nguoiThuDau}>
          <Avatar name={collector.name} ring size={44} tone="split" />
          <View style={styles.flex}>
            <Text style={[typography.caption, { color: colors.inkFaint }]}>{collector.name} sẽ nhận (bill Xóm Lèo)</Text>
            <Money tone="split" vnd={picture.collectorReceives} />
          </View>
          <Stamp label="Người thu bill" tone="split" />
        </View>
        <DongTien nhan="Đã nhận" tone="split" vnd={paidSum} />
        <DongTien cuoi nhan="Còn chờ" vnd={picture.collectorReceives - paidSum} />
      </View>
      <SectionHeader title="Các khoản chuyển (chỉ bill Xóm Lèo)" />
      <View>
        {picture.transfers.map((item) => {
          const person = PEOPLE[item.fromIndex];
          const paid = session.paidFromIndexes.includes(item.fromIndex);
          return (
            <View key={person.id} style={[styles.hangChuyen, { borderBottomColor: colors.line }]}>
              <Avatar name={person.name} size={40} tone="split" />
              <View style={styles.flex}>
                <Text style={[typography.label, { color: colors.ink }]}>{person.name} → {collector.name}</Text>
                <Text style={[typography.caption, { color: colors.inkFaint }]}>
                  {paid ? "Đã xác nhận trong ứng dụng" : "Chờ người nhận xác nhận"}
                </Text>
              </View>
              <View style={styles.transferRight}>
                <Money size="label" vnd={item.amount} />
                {paid ? (
                  <Stamp dong={vuaTra === item.fromIndex} label="Đã trả" tone="split" />
                ) : (
                  <RudiButton
                    accessibilityLabel={`Đánh dấu ${person.name} đã trả`}
                    compact
                    full={false}
                    label="Đánh dấu đã trả"
                    onPress={() => {
                      setVuaTra(item.fromIndex);
                      session.markPaid(item.fromIndex);
                    }}
                    tone="split"
                    variant="outline"
                  />
                )}
              </View>
            </View>
          );
        })}
      </View>
      <View style={styles.ghiChu}>
        <Ionicons color={colors.split} name="shield-checkmark-outline" size={20} />
        <Text style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>
          “Đã trả” là xác nhận trong Rủ Đi, không phải bằng chứng chuyển tiền. Chuyển bằng cách nào là việc giữa hai người; app dừng ở phần của mỗi người.
        </Text>
      </View>
      <RudiButton
        icon="notifications-outline"
        label={
          session.remindedPending
            ? `Đã nhắc ${pendingCount} người ${noiLuuNgan(session.luuTruSong)}`
            : `Nhắc ${pendingCount} người đang chờ`
        }
        onPress={() => session.remindPending()}
        tone="split"
      />
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
  hangChuyen: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 64, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  transferRight: { alignItems: "flex-end", gap: 6 },
  ghiChu: { flexDirection: "row", alignItems: "flex-start", gap: 9 },
});
