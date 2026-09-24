/**
 * Chia hóa đơn on a real session (M5): one route, five steps.
 *
 *   bắt đầu (a photo, previewed before it is sent, or typed lines) → xem lại
 *   (lines from the photo or typed) → gán món (the group picks who had what;
 *   the server holds the bill and the assignment) → kết quả (the server's
 *   split) → đã ghi (the expense is in the ledger, settlement reads it).
 *
 * Nothing here computes a share. The server splits; this screen draws.
 *
 * Going back: the top bar's chevron steps back inside the flow on the middle
 * steps, so a typed bill is never discarded by the one control everybody
 * reaches for first. It leaves the route only from the first and last step.
 *
 * ## The bill as a sheet you can check (UI v2, đợt 6)
 *
 * A line is a row -- name, quantity, sum -- and opens to its fields only when
 * tapped, so thirty lines are thirty rows, not ninety inputs. Lines the
 * reader flagged («cần kiểm») open on arrival; a bill of three lines opens
 * whole. Assigning is the same row with the roster beneath it, plus «Cả
 * nhóm», «Bỏ hết» and «Như món trên» so eight people are not tapped eight
 * times per line. The result answers «phần của bạn?» first, then shows
 * everyone. When the ledger has taken the expense, one stamp lands on the
 * page -- once per expense, never on a number the server has not confirmed.
 */
import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import * as ImagePicker from "expo-image-picker";
import { useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt, type ChiaBill } from "../../../api";
import {
  blockingProblem as loiGanMon,
  everyoneShares,
  isOn,
  signature,
  toggle,
  whoOn,
  type Assignment,
} from "../../../assignment";
import type { BillWire } from "../../../bill";
import type { Phien } from "../../../phien";
import {
  blockingProblem as loiHoaDon,
  removeLine,
  renameLine,
  setLineTotal,
  setQuantity,
  type BillReading,
} from "../../../receipt";
import { danhSachThanhVien } from "../../../screens/vao-cua/cong-api";
import { celebrateOnce } from "../../motion";
import {
  cauNguonBill,
  cauSauKhiScanHong,
  cauTongMon,
  chiaTrenMayChu,
  docBillTuAnh,
  ghiVaoSo,
  hangKetQua,
  hoaDonTrong,
  luuGanMonTrenMayChu,
  nhanDongMon,
  taoBillTrenMayChu,
  tenCua,
  themMon,
  type ThanhVien,
} from "../../chia-bill/hoa-don";
import { aiCoGi } from "../../chia-bill/ai-co-gi";
import { typography, useRudiTheme } from "../../theme";
import { AiNote, Chip, Field, Heading, Inline, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { AiCoGi } from "../../ui/AiCoGi";
import { DongTien } from "../../ui/DongTien";
import { HaiCot } from "../../ui/HaiCot";
import { Money } from "../../ui/Money";
import { RosterPicker } from "../../ui/RosterPicker";
import { Stamp } from "../../ui/Stamp";
import { Stepper } from "../../ui/Stepper";
import { useMotion } from "../../ui/useMotion";

type Buoc =
  | { ten: "bat-dau" }
  | { ten: "xem-anh"; uri: string; bytes: number }
  | { ten: "xem-lai" }
  | { ten: "gan-mon"; bill: BillWire }
  | { ten: "ket-qua"; bill: BillWire; chia: ChiaBill }
  | { ten: "da-ghi"; expenseVersionId: string; tenKhoan: string; tongVnd: number; nguoiTraId: string };

const CAC_BUOC = ["Bill", "Xem lại", "Gán món", "Kết quả", "Ghi sổ"];
/** A bill this short opens every line at once; longer bills open one at a time. */
const MO_HET_DEN = 3;

/** Which of the five steps a state belongs to; the photo preview is still step one. */
function soBuoc(buoc: Buoc): number {
  switch (buoc.ten) {
    case "bat-dau":
    case "xem-anh":
      return 1;
    case "xem-lai":
      return 2;
    case "gan-mon":
      return 3;
    case "ket-qua":
      return 4;
    case "da-ghi":
      return 5;
  }
}

function tieuDeBuoc(buoc: Buoc): string {
  switch (buoc.ten) {
    case "bat-dau":
    case "xem-anh":
    case "da-ghi":
      return "Chia hóa đơn";
    case "xem-lai":
      return "Xem lại hóa đơn";
    case "gan-mon":
      return "Ai dùng món nào?";
    case "ket-qua":
      return "Rủ Đi chia";
  }
}

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

/** A member's display name, or the plain word when the server has none. */
function tenHienThi(ten: string | null | undefined): string {
  if (typeof ten === "string" && ten.trim() !== "") return ten;
  return "Thành viên";
}

/** Which lines start open: all of a short bill, else only the flagged ones. */
function moBanDau(reading: BillReading): Set<string> {
  if (reading.lines.length <= MO_HET_DEN) return new Set(reading.lines.map((l) => l.id));
  return new Set(reading.lines.filter((l) => nhanDongMon(reading, l)?.canKiem).map((l) => l.id));
}

/** The assignment with one line set to exactly `ids`, through the module's own toggle. */
function datNguoi(a: Assignment, lineId: string, ids: readonly string[], roster: readonly string[]): Assignment {
  let ra = a;
  for (const id of roster) {
    const nen = ids.includes(id);
    if (isOn(ra, lineId, id) !== nen) ra = toggle(ra, lineId, id);
  }
  return ra;
}

/** The stamp that lands once when the ledger has taken the expense. */
function DauDaGhi({ khoa }: { khoa: string }) {
  const motion = useMotion();
  const daThay = useRef(new Set<string>());
  const tien = useSharedValue(0);
  useEffect(() => {
    tien.value = 0;
    tien.value = withTiming(1, { duration: celebrateOnce(daThay.current, khoa, motion.reduced) });
  }, [khoa, motion.reduced, tien]);
  const style = useAnimatedStyle(() => ({
    opacity: tien.value,
    transform: [{ scale: 1.18 - tien.value * 0.18 }, { rotate: "-3deg" }],
  }));
  return (
    <Animated.View style={style}>
      <Stamp label="Đã ghi sổ" tone="split" variant="ink" />
    </Animated.View>
  );
}

export function ChiaBillLiveScreen({ phien, dip }: { phien: Phien; dip?: string }) {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  // The step CTA is the last thing in the scroll; at font 1.3 it met the gesture pill.
  const insets = useSafeAreaInsets();
  const contextId = phien.context_id;
  const [buoc, setBuoc] = useState<Buoc>({ ten: "bat-dau" });
  const [reading, setReading] = useState<BillReading>(hoaDonTrong());
  const [assignment, setAssignment] = useState<Assignment>({});
  const [moRong, setMoRong] = useState<Set<string>>(new Set());
  const [roster, setRoster] = useState<ThanhVien[]>([]);
  const [payerId, setPayerId] = useState(phien.person_id);
  const [occasion, setOccasion] = useState(dip ?? "");
  const [thongBao, setThongBao] = useState<string | null>(null);
  const [ban, setBan] = useState(false);
  const attempts = useRef<Record<string, Attempt>>({});

  useEffect(() => {
    if (contextId === null) return;
    let song = true;
    void danhSachThanhVien(contextId, phien.person_id)
      .then((ds) => {
        if (!song) return;
        setRoster(ds.map((tv) => ({ id: tv.person_id, name: tenHienThi(tv.display_name) })));
      })
      .catch((error: unknown) => {
        if (song) setThongBao(loiRaChu(error));
      });
    return () => {
      song = false;
    };
  }, [contextId, phien.person_id]);

  if (contextId === null) {
    return (
      <RudiScreen tone="split" testID="receipt-review-screen">
        <TopBar title="Chia hóa đơn" />
        <Heading title="Vào một nhóm trước" subtitle="Hóa đơn là của nhóm; chưa có nhóm thì chưa có ai để chia." />
        <RudiButton label="Tới Tin nhắn" onPress={() => router.push("/(tabs)/messages" as never)} tone="split" variant="outline" />
      </RudiScreen>
    );
  }
  const ctx = contextId;
  const rosterIds = roster.map((tv) => tv.id);

  const chay = async (viec: () => Promise<void>) => {
    setBan(true);
    setThongBao(null);
    try {
      await viec();
    } catch (error) {
      setThongBao(loiRaChu(error));
    } finally {
      setBan(false);
    }
  };

  const doiMo = (id: string) =>
    setMoRong((cu) => {
      const moi = new Set(cu);
      if (moi.has(id)) moi.delete(id);
      else moi.add(id);
      return moi;
    });

  // Pick only. The photo is shown before anything is sent: the user sees what
  // the server will read, and can pick again, before a round trip is spent.
  const chonAnh = () =>
    chay(async () => {
      const picked = await ImagePicker.launchImageLibraryAsync({ mediaTypes: ["images"], quality: 0.8 });
      if (picked.canceled || !picked.assets[0]) {
        setThongBao("Không chọn ảnh.");
        return;
      }
      const anh = picked.assets[0];
      setBuoc({ ten: "xem-anh", uri: anh.uri, bytes: anh.fileSize === undefined ? 0 : anh.fileSize });
    });

  const docAnh = (uri: string, bytes: number) =>
    chay(async () => {
      try {
        const doc = await docBillTuAnh({ uri, bytes }, phien.person_id);
        setReading(doc);
        setMoRong(moBanDau(doc));
        setBuoc({ ten: "xem-lai" });
      } catch (error) {
        setThongBao(cauSauKhiScanHong(loiRaChu(error)));
      }
    });

  const nhapTay = () => {
    const doc = themMon(hoaDonTrong());
    setReading(doc);
    setMoRong(moBanDau(doc));
    setBuoc({ ten: "xem-lai" });
  };

  const themMonMoi = () => {
    const doc = themMon(reading);
    const moi = doc.lines[doc.lines.length - 1];
    setReading(doc);
    setMoRong((cu) => {
      // A short bill stays fully open; on a long one only the new line opens.
      const ra = doc.lines.length <= MO_HET_DEN ? new Set(doc.lines.map((l) => l.id)) : new Set<string>();
      if (moi) ra.add(moi.id);
      return ra;
    });
  };

  const sangGanMon = () =>
    chay(async () => {
      const loi = loiHoaDon(reading);
      if (loi !== null) {
        setThongBao(loi);
        return;
      }
      const a = everyoneShares(reading.lines, rosterIds);
      const bill = await taoBillTrenMayChu(reading, ctx, a, phien.person_id, attemptFor(attempts.current, `tao-bill:${signature(reading, rosterIds, a)}`));
      setAssignment(a);
      setMoRong(moBanDau(reading));
      setBuoc({ ten: "gan-mon", bill });
    });

  const xemKetQua = (bill: BillWire) =>
    chay(async () => {
      const loi = loiGanMon(reading, rosterIds, assignment);
      if (loi !== null) {
        setThongBao(loi);
        return;
      }
      const sig = signature(reading, rosterIds, assignment);
      const daLuu = await luuGanMonTrenMayChu(bill.id, reading, assignment, phien.person_id, ctx, attemptFor(attempts.current, `gan-mon:${bill.id}:${sig}`));
      const chia = await chiaTrenMayChu(daLuu.id, phien.person_id, ctx, attemptFor(attempts.current, `chia:${daLuu.id}:${sig}`));
      setBuoc({ ten: "ket-qua", bill: daLuu, chia });
    });

  const ghi = (chia: ChiaBill) =>
    chay(async () => {
      const ten = occasion.trim() === "" ? "Hóa đơn của nhóm" : occasion.trim();
      const kq = await ghiVaoSo({ reading, assignment, roster, contextId: ctx, payerId, occasion: ten, attempts: attempts.current });
      setBuoc({ ten: "da-ghi", expenseVersionId: kq.expenseVersionId, tenKhoan: ten, tongVnd: chia.totalAmountVnd, nguoiTraId: payerId });
    });

  // One step back inside the flow. On the middle steps this is what the top
  // bar's chevron does; the route is left only from the first and last step.
  const quayLai = () => {
    if (buoc.ten === "xem-anh" || buoc.ten === "xem-lai") setBuoc({ ten: "bat-dau" });
    else if (buoc.ten === "gan-mon") setBuoc({ ten: "xem-lai" });
    else if (buoc.ten === "ket-qua") setBuoc({ ten: "gan-mon", bill: buoc.bill });
  };
  const luiTrongLuong = buoc.ten !== "bat-dau" && buoc.ten !== "da-ghi";

  /** Why the step's primary action is waiting, in one sentence; nothing when it is not. */
  const lyDoKhoa =
    buoc.ten === "xem-lai" ? loiHoaDon(reading) : buoc.ten === "gan-mon" ? loiGanMon(reading, rosterIds, assignment) : null;

  const phanCuaToi = buoc.ten === "ket-qua" ? hangKetQua(buoc.chia, roster).find((h) => h.id === phien.person_id) : undefined;

  // The step's one decision stays above the gesture bar however long the
  // line editor grows: on the review step the open editor pushed «Tiếp» below
  // the fold and the live board could not reach it (2026-09-06).
  const nutChinh =
    buoc.ten === "xem-lai" ? (
      <RudiButton disabled={ban} label="Tiếp: ai dùng món nào?" loading={ban} onPress={() => void sangGanMon()} tone="split" />
    ) : buoc.ten === "gan-mon" ? (
      <RudiButton disabled={ban} label="Xem kết quả" loading={ban} onPress={() => void xemKetQua(buoc.bill)} tone="split" />
    ) : buoc.ten === "ket-qua" ? (
      <RudiButton disabled={ban} icon="book-outline" label="Ghi vào sổ" loading={ban} onPress={() => void ghi(buoc.chia)} tone="split" />
    ) : null;

  return (
    <RudiScreen bottomInset={Math.max(insets.bottom, 16) + 40} footer={nutChinh} footerInset={Math.max(insets.bottom, 12) + 4} tone="split" testID="receipt-review-screen">
      <TopBar onBack={luiTrongLuong ? quayLai : undefined} title={tieuDeBuoc(buoc)} />
      <Stepper current={soBuoc(buoc) - 1} lockedReason={lyDoKhoa} steps={CAC_BUOC} tone="split" />
      {thongBao !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{thongBao}</Text> : null}

      {buoc.ten === "bat-dau" ? (
        <>
          <Heading title="Bill hôm nay" subtitle="Chụp hoặc chọn ảnh hoá đơn để Rủ Đi đọc từng món, hoặc gõ tay. Ai dùng món nào thì hỏi ở bước sau." />
          <RudiButton disabled={ban} icon="images-outline" label="Chọn ảnh bill" loading={ban} onPress={() => void chonAnh()} tone="split" />
          <RudiButton disabled={ban} icon="create-outline" label="Nhập tay" onPress={nhapTay} tone="split" variant="outline" />
        </>
      ) : null}

      {buoc.ten === "xem-anh" ? (
        <>
          <Heading title="Ảnh này đúng bill chứ?" subtitle="Rủ Đi sẽ đọc từng món từ ảnh này. Chưa gửi gì cho tới khi bạn bấm dùng." />
          <Image accessibilityLabel="Ảnh bill đã chọn" contentFit="cover" source={{ uri: buoc.uri }} style={[styles.anh, { borderRadius: radius.small, backgroundColor: colors.line }]} />
          <RudiButton disabled={ban} icon="scan-outline" label="Dùng ảnh này" loading={ban} onPress={() => void docAnh(buoc.uri, buoc.bytes)} tone="split" />
          <RudiButton disabled={ban} icon="images-outline" label="Chọn ảnh khác" onPress={() => void chonAnh()} tone="split" variant="outline" />
        </>
      ) : null}

      {buoc.ten === "xem-lai" ? (
        <>
          <Heading title={cauTongMon(reading)} subtitle={cauNguonBill(reading)} />
          {reading.warnings.map((w) => (
            <AiNote key={w}>{w}</AiNote>
          ))}
          <View>
            {reading.lines.map((line, i) => {
              const nhan = nhanDongMon(reading, line);
              const mo = moRong.has(line.id);
              const ten = line.name.trim() === "" ? `Món ${i + 1}` : line.name;
              return (
                <View key={line.id} style={[styles.dong, { borderBottomColor: colors.line }]}>
                  <Pressable
                    accessibilityLabel={`${mo ? "Gấp" : "Sửa"} ${ten}`}
                    accessibilityRole="button"
                    accessibilityState={{ expanded: mo }}
                    onPress={() => doiMo(line.id)}
                    style={({ pressed }) => [styles.dongDau, pressed && styles.bam]}
                  >
                    <View style={styles.flex}>
                      <Text style={[typography.label, { color: colors.ink }]}>{ten}</Text>
                      {nhan !== null ? (
                        <Text style={[typography.caption, { color: nhan.canKiem ? colors.warn : colors.inkSoft }]}>{nhan.chu}</Text>
                      ) : null}
                    </View>
                    <Text style={[typography.caption, { color: colors.inkSoft }]}>{line.quantity} ×</Text>
                    <Money size="label" vnd={line.lineTotalVnd} />
                    <Ionicons color={colors.inkFaint} name={mo ? "chevron-up" : "chevron-down"} size={18} />
                  </Pressable>
                  {mo ? (
                    <View style={styles.sua}>
                      <Field
                        accessibilityLabel={`Ô tên món ${i + 1}`}
                        label="Món"
                        onChangeText={(t) => setReading((r) => renameLine(r, line.id, t))}
                        placeholder="Ví dụ: Bún bò"
                        value={line.name}
                      />
                      <View style={styles.hang}>
                        <View style={styles.oNho}>
                          <Field
                            accessibilityLabel={`Ô số lượng món ${i + 1}`}
                            keyboardType="number-pad"
                            label="Số lượng"
                            onChangeText={(t) => {
                              const kq = setQuantity(reading, line.id, t);
                              if (kq.ok) setReading(kq.reading);
                            }}
                            value={String(line.quantity)}
                          />
                        </View>
                        <View style={styles.flex}>
                          <Field
                            accessibilityLabel={`Ô tiền món ${i + 1}`}
                            keyboardType="number-pad"
                            label="Thành tiền (đồng)"
                            onChangeText={(t) => {
                              const kq = setLineTotal(reading, line.id, t);
                              if (kq.ok) setReading(kq.reading);
                            }}
                            value={line.lineTotalVnd === 0 ? "" : String(line.lineTotalVnd)}
                          />
                        </View>
                      </View>
                      <RudiButton compact full={false} icon="trash-outline" label="Bỏ món này" onPress={() => setReading((r) => removeLine(r, line.id))} tone="split" variant="ghost" />
                    </View>
                  ) : null}
                </View>
              );
            })}
          </View>
          <RudiButton icon="add" label="Thêm món" onPress={themMonMoi} tone="split" variant="soft" />
        </>
      ) : null}

      {buoc.ten === "gan-mon" ? (
        // Wide window: the dish list on the left, «Ai có gì» beside it -- the
        // same assignment read per person, plus the dishes still waiting. On
        // a phone the per-line names already say it, so the pane is dropped.
        <HaiCot
          phaiChiKhiRong
          phai={<AiCoGi bang={aiCoGi(reading.lines, roster, assignment)} />}
          trai={
            <>
          <Heading title={cauTongMon(reading)} subtitle="Chạm một món để sửa ai dùng. Bản gán được lưu lại; tổng bill không đổi khi bạn sửa người." />
          <View>
            {reading.lines.map((line, i) => {
              const mo = moRong.has(line.id);
              const dangDung = roster.filter((person) => isOn(assignment, line.id, person.id));
              const truoc = i > 0 ? whoOn(assignment, reading.lines[i - 1].id) : null;
              return (
                <View key={line.id} style={[styles.dong, { borderBottomColor: colors.line }]}>
                  <Pressable
                    accessibilityLabel={`Sửa người dùng ${line.name}`}
                    accessibilityRole="button"
                    accessibilityState={{ expanded: mo }}
                    onPress={() => doiMo(line.id)}
                    style={({ pressed }) => [styles.dongDau, pressed && styles.bam]}
                  >
                    <View style={styles.flex}>
                      <Text style={[typography.label, { color: colors.ink }]}>{line.name}</Text>
                      <Text style={[typography.caption, { color: dangDung.length === 0 ? colors.warn : colors.inkSoft }]}>
                        {dangDung.length === 0 ? "Chưa chọn người" : `${dangDung.length} người`}
                        {dangDung.length > 0 ? ` · ${dangDung.map((p) => p.name).join(", ")}` : ""}
                      </Text>
                    </View>
                    <Money size="label" vnd={line.lineTotalVnd} />
                    <Ionicons color={colors.inkFaint} name={mo ? "chevron-up" : "chevron-down"} size={18} />
                  </Pressable>
                  {mo ? (
                    <View style={styles.sua}>
                      <RosterPicker
                        disabled={ban}
                        nhanCho={(ten) => `${ten} · ${line.name}`}
                        onToggle={(id) => setAssignment((current) => toggle(current, line.id, id))}
                        people={roster}
                        selected={dangDung.map((person) => person.id)}
                      />
                      <Inline gap={6} wrap>
                        <Chip label="Cả nhóm" onPress={() => setAssignment((a) => datNguoi(a, line.id, rosterIds, rosterIds))} tone="split" />
                        <Chip label="Bỏ hết" onPress={() => setAssignment((a) => datNguoi(a, line.id, [], rosterIds))} tone="split" />
                        {truoc !== null ? (
                          <Chip label="Như món trên" onPress={() => setAssignment((a) => datNguoi(a, line.id, truoc, rosterIds))} tone="split" />
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
      ) : null}

      {buoc.ten === "ket-qua" ? (
        // Wide window: the ledger on the left, the two things still to decide
        // (who paid, what to call it) on the right, in view beside the numbers.
        <HaiCot
          phai={
            <>
          <SectionHeader title="Ai đã trả bill?" />
          <Inline gap={6} wrap>
            {roster.map((tv) => (
              <Chip accessibilityLabel={`Người trả ${tv.name}`} key={tv.id} label={tv.name} onPress={() => setPayerId(tv.id)} selected={payerId === tv.id} tone="split" />
            ))}
          </Inline>
          <Field accessibilityLabel="Ô tên khoản chi" label="Gọi khoản này là" onChangeText={setOccasion} placeholder="Ví dụ: Tối nay Xóm Lào" value={occasion} />
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            Ghi vào sổ là tạo khoản chi với đúng các số ở trên; tổng được kiểm cho khớp trước khi ghi.
          </Text>
            </>
          }
          trai={
            <>
          {phanCuaToi !== undefined ? (
            // The person's own share is the first line of the ledger, not a
            // tinted block with a label over a big number (the hero-metric
            // template the report and the finish review both refused).
            <View>
              <DongTien
                dam
                nhan="Phần của bạn"
                phu={`Rủ Đi chia ${cauTongMon(reading)} theo bản gán ${buoc.chia.assignmentState === "confirmed" ? "đã chốt" : "đang gợi ý"}${phanCuaToi.lamTron ? "; lẻ đồng dồn về bạn" : ""}.`}
                tone="split"
                vnd={Number(phanCuaToi.tien.replace(/\D/g, ""))}
              />
            </View>
          ) : (
            <Heading title={cauTongMon(reading)} subtitle={buoc.chia.assignmentState === "confirmed" ? "Chia theo bản gán đã chốt." : "Chia theo bản gán đang gợi ý."} />
          )}
          <SectionHeader title="Phần của mỗi người" />
          <View>
            {hangKetQua(buoc.chia, roster).map((h) => (
              <View key={h.id} style={[styles.hangKetQua, { borderBottomColor: colors.line }]}>
                <Text style={[typography.body, styles.flex, { color: colors.ink }]}>{h.ten}</Text>
                {h.id === payerId ? <Chip label="Đã trả bill" tone="split" selected /> : null}
                {h.lamTron ? <Chip label="+lẻ đồng" tone="split" /> : null}
                <Text style={[typography.money, { color: colors.ink }]}>{h.tien}</Text>
              </View>
            ))}
          </View>
          {buoc.chia.roundingGainers.length > 0 ? (
            <Text style={[typography.caption, { color: colors.inkSoft }]}>
              Lẻ đồng dồn về: {buoc.chia.roundingGainers.map((id) => tenCua(roster, id)).join(", ")} (chia lẻ tự động, tổng vẫn khớp).
            </Text>
          ) : null}
          {buoc.chia.warnings.map((w) => (
            <Text key={w} style={[typography.caption, { color: colors.warn }]}>
              {w}
            </Text>
          ))}
            </>
          }
        />
      ) : null}

      {buoc.ten === "da-ghi" ? (
        <>
          <View style={styles.dauGhi}>
            <DauDaGhi khoa={buoc.expenseVersionId} />
          </View>
          <Heading
            title={`Đã ghi: ${buoc.tenKhoan}`}
            subtitle={`${dinhDangTien(buoc.tongVnd)}, ${tenCua(roster, buoc.nguoiTraId)} đã trả. Mỗi người phần của mình như đã chia; quyết toán tính lại từ sổ.`}
          />
          <RudiButton icon="wallet-outline" label="Xem quyết toán" onPress={() => router.replace(`/settlements/${ctx}` as never)} tone="split" />
          <RudiButton label="Về Tin nhắn" onPress={() => router.replace("/(tabs)/messages" as never)} tone="split" variant="outline" />
        </>
      ) : null}
    </RudiScreen>
  );
}

/** The one formatter, for a sentence that also carries words. */
function dinhDangTien(vnd: number): string {
  return `${vnd.toLocaleString("vi-VN")}đ`;
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  hang: { flexDirection: "row", gap: 10 },
  oNho: { width: 120 },
  anh: { width: "100%", aspectRatio: 3 / 4 },
  dong: { borderBottomWidth: StyleSheet.hairlineWidth, paddingVertical: 4 },
  dongDau: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 56, paddingVertical: 6 },
  sua: { gap: 10, paddingBottom: 12 },
  bam: { opacity: 0.75 },
  phanToi: { gap: 4, padding: 16 },
  hangKetQua: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 52, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth },
  dauGhi: { alignItems: "flex-start", marginTop: 4 },
});
