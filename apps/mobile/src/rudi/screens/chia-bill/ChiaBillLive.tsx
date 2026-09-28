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
  itemsTotalVnd,
  removeLine,
  renameLine,
  setLineTotal,
  setQuantity,
  type BillReading,
} from "../../../receipt";
import { danhSachThanhVien } from "../../../screens/vao-cua/cong-api";
import { TIET_MUC } from "../../art/nep-dien";
import {
  cauCanhBaoChia,
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
import { cauSoPhan, goiYTien, hienO, loiO, type KieuO } from "../../chia-bill/o-so";
import { mucNguoi, typography, useRudiTheme } from "../../theme";
import { AiNote, Chip, Heading, Inline, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { AiCoGi } from "../../ui/AiCoGi";
import { Avatar } from "../../ui/Avatar";
import { BanAn } from "../../ui/BanAn";
import { BanGanMon } from "../../ui/BanGanMon";
import { ChuThichLe } from "../../ui/ChuThichLe";
import { CuongPhieu } from "../../ui/CuongPhieu";
import { DauLon } from "../../ui/DauLon";
import { HaiCot } from "../../ui/HaiCot";
import { DongHoaDon, HoaDonGiay, TieuDeHoaDon, VachCat } from "../../ui/HoaDonGiay";
import { KhungAnh } from "../../ui/KhungAnh";
import { LatTrang } from "../../ui/LatTrang";
import { Money } from "../../ui/Money";
import { NapGiay } from "../../ui/NapGiay";
import { NepDien } from "../../ui/NepDien";
import { NepTinh } from "../../ui/NepRoi";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { RosterPicker } from "../../ui/RosterPicker";
import { StampButton } from "../../ui/StampButton";
import { Stepper } from "../../ui/Stepper";
import { DongSo, TrangSo } from "../../ui/TrangSo";

type Buoc =
  | { ten: "bat-dau" }
  | { ten: "xem-anh"; uri: string; bytes: number }
  | { ten: "xem-lai" }
  | { ten: "gan-mon"; bill: BillWire }
  | { ten: "ket-qua"; bill: BillWire; chia: ChiaBill }
  | {
      ten: "da-ghi";
      expenseVersionId: string;
      tenKhoan: string;
      tongVnd: number;
      nguoiTraId: string;
      /** The server's shares as they were recorded: copied onto the ledger page. */
      hang: ReturnType<typeof hangKetQua>;
    };

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

export function ChiaBillLiveScreen({ phien, dip }: { phien: Phien; dip?: string }) {
  const router = useRouter();
  const { colors, dark } = useRudiTheme();
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
  // What the person is typing in a number box, per `${line}:${kind}`, while
  // that box has focus. The bill only receives what parses (`o-so.ts`).
  const [nhapSo, setNhapSo] = useState<Record<string, string>>({});
  const [thongBao, setThongBao] = useState<string | null>(null);
  const [ban, setBan] = useState(false);
  // The dish on the bill table at the assign step; the first line until another is tapped.
  const [monTrenBan, setMonTrenBan] = useState<string | null>(null);
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

  const khoaO = (id: string, kieu: KieuO) => `${id}:${kieu}`;
  const oNhap = (id: string, kieu: KieuO): string | undefined => nhapSo[khoaO(id, kieu)];
  const datNhap = (id: string, kieu: KieuO, t: string) => setNhapSo((n) => ({ ...n, [khoaO(id, kieu)]: t }));
  // Leaving the box drops the draft: the box shows the committed value again,
  // formatted («350.000»), and a quantity left empty goes back to what it was.
  const boNhap = (id: string, kieu: KieuO) =>
    setNhapSo((n) => {
      const { [khoaO(id, kieu)]: _bo, ...con } = n;
      return con;
    });
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
      setBuoc({ ten: "da-ghi", expenseVersionId: kq.expenseVersionId, tenKhoan: ten, tongVnd: chia.totalAmountVnd, nguoiTraId: payerId, hang: hangKetQua(chia, roster) });
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

  // The step's one decision stays above the gesture bar however long the
  // line editor grows: on the review step the open editor pushed «Tiếp» below
  // the fold and the live board could not reach it (2026-09-06). Recording the
  // expense is a decision about money: the teal seal (ADR-0037 D14).
  const nutChinh =
    buoc.ten === "xem-lai" ? (
      <RudiButton disabled={ban} label="Tiếp: ai dùng món nào?" loading={ban} onPress={() => void sangGanMon()} tone="split" />
    ) : buoc.ten === "gan-mon" ? (
      <RudiButton disabled={ban} label="Xem kết quả" loading={ban} onPress={() => void xemKetQua(buoc.bill)} tone="split" />
    ) : buoc.ten === "ket-qua" ? (
      <StampButton disabled={ban} label="Ghi vào sổ" loading={ban} onPress={() => void ghi(buoc.chia)} size="vua" tilt={-1} tone="split" />
    ) : null;

  const monBan = buoc.ten === "gan-mon" ? (reading.lines.find((l) => l.id === monTrenBan) ?? reading.lines[0] ?? null) : null;

  return (
    <RudiScreen
      bottomInset={Math.max(insets.bottom, 16) + 40}
      cuonVeDau={buoc.ten}
      footer={nutChinh}
      footerInset={Math.max(insets.bottom, 12) + 4}
      tone="split"
      testID="receipt-review-screen"
    >
      <TopBar onBack={luiTrongLuong ? quayLai : undefined} title={tieuDeBuoc(buoc)} />
      <Stepper current={soBuoc(buoc) - 1} lockedReason={lyDoKhoa} steps={CAC_BUOC} tone="split" />
      {thongBao !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{thongBao}</Text> : null}

      {/* One page per step, turned over the spine (ADR-0037 D1): the next page
          is already lying there, only the finished one turns away. */}
      <LatTrang khoa={buoc.ten} thuTu={soBuoc(buoc) * 2 + (buoc.ten === "xem-anh" ? 1 : 0)}>
      <View style={styles.trang}>

      {buoc.ten === "bat-dau" ? (
        <>
          <Text style={[typography.body, { color: colors.inkSoft }]}>Chụp hoặc chọn ảnh hoá đơn để Rủ Đi đọc từng món, hoặc gõ tay.</Text>
          {/* The table seen from above: an empty receipt waiting on it is the
              button, and Nếp stands by with the camera (a still, not a moment). */}
          <BanAn>
          <View style={styles.banTrong}>
            <Pressable
              accessibilityLabel="Chọn ảnh bill"
              accessibilityRole="button"
              accessibilityState={{ disabled: ban, busy: ban }}
              disabled={ban}
              onPress={() => void chonAnh()}
              style={({ pressed }) => [styles.billTrong, { opacity: ban ? 0.6 : pressed ? 0.85 : 1 }]}
              testID="bill-trong"
            >
              <HoaDonGiay rangTren>
                <TieuDeHoaDon phu="Chưa có món nào" ten="Bill hôm nay" />
                <VachCat />
                {[0.8, 0.55, 0.7].map((r, i) => (
                  <View importantForAccessibility="no-hide-descendants" key={i} style={styles.dongMo}>
                    <View style={[styles.vachMo, { width: `${r * 60}%`, backgroundColor: colors.line }]} />
                    <View style={[styles.vachMo, { width: "18%", backgroundColor: colors.line }]} />
                  </View>
                ))}
                <VachCat />
                <View style={styles.hangNut}>
                  <Ionicons color={colors.split} name="camera-outline" size={22} />
                  <Text style={[typography.title, { color: colors.split }]}>Chọn ảnh bill</Text>
                </View>
              </HoaDonGiay>
            </Pressable>
            <NepTinh style={styles.nepCam} tm={TIET_MUC["cam-may"]} width={96} />
          </View>
          </BanAn>
          {/* Typing is a pencil line on the same page, not a second button. */}
          <Pressable accessibilityRole="button" disabled={ban} onPress={nhapTay} style={({ pressed }) => [styles.butChi, { borderBottomColor: colors.lineStrong, opacity: pressed ? 0.7 : 1 }]}>
            <Ionicons color={colors.inkSoft} name="pencil" size={18} />
            <Text style={[typography.title, { color: colors.ink }]}>Nhập tay</Text>
          </Pressable>
          <NapGiay tieuDe="Cách chia">
            <Text style={[typography.body, { color: colors.ink }]}>Rủ Đi đọc từng món trên hoá đơn. Bước sau, cả nhóm chọn ai dùng món nào; máy chủ chia mỗi món cho đúng những người đó, lẻ đồng dồn về một người, và tổng luôn khớp hoá đơn.</Text>
          </NapGiay>
        </>
      ) : null}

      {buoc.ten === "xem-anh" ? (
        <>
          <View style={styles.hangDau}>
            <Heading title="Ảnh này đúng bill chứ?" subtitle="Rủ Đi sẽ đọc từng món từ ảnh này." />
            <NepDien khoanhKhac="M2" suKien={buoc.uri} />
          </View>
          <KhungAnh tilt={-1}>
            <Image accessibilityLabel="Ảnh bill đã chọn" contentFit="cover" source={{ uri: buoc.uri }} style={[styles.anh, { backgroundColor: colors.line }]} />
          </KhungAnh>
          <ChuThichLe icon="lock-closed-outline">Chưa gửi gì cho tới khi bạn bấm dùng.</ChuThichLe>
          <RudiButton disabled={ban} icon="scan-outline" label="Dùng ảnh này" loading={ban} onPress={() => void docAnh(buoc.uri, buoc.bytes)} tone="split" />
          <RudiButton disabled={ban} icon="images-outline" label="Chọn ảnh khác" onPress={() => void chonAnh()} tone="split" variant="outline" />
        </>
      ) : null}

      {buoc.ten === "xem-lai" ? (
        <>
          {reading.warnings.map((w) => (
            <AiNote key={w}>{w}</AiNote>
          ))}
          {/* The bill IS a thermal receipt: the occasion and the count head the
              paper, each dish is a printed line that opens to its fields in
              place, and the total closes it. On `card`, never on dark `paper`
              (the «cần kiểm» warning read 4.00:1 there). */}
          <HoaDonGiay rangTren testID="to-hoa-don">
            <View style={styles.dauHoaDon}>
              <Text style={[typography.stamp, { color: colors.inkSoft }]}>{occasion.trim() || "Buổi hôm nay"}</Text>
              <Text style={[typography.h2, { color: colors.ink }]}>{cauTongMon(reading)}</Text>
              <Text style={[typography.note, { color: colors.inkSoft }]}>{cauNguonBill(reading)}</Text>
            </View>
            <VachCat />
            {reading.lines.map((line, i) => {
              const nhan = nhanDongMon(reading, line);
              const mo = moRong.has(line.id);
              const ten = line.name.trim() === "" ? `Món ${i + 1}` : line.name;
              return (
                <View key={line.id} style={styles.dong}>
                  <Pressable
                    accessibilityLabel={`${mo ? "Gấp" : "Sửa"} ${ten}`}
                    accessibilityRole="button"
                    accessibilityState={{ expanded: mo }}
                    onPress={() => doiMo(line.id)}
                    style={({ pressed }) => [styles.dongDau, pressed && styles.bam]}
                  >
                    <View style={styles.tenMon}>
                      <Text style={[typography.label, { color: colors.ink }]}>{ten}</Text>
                      {cauSoPhan(line.quantity) !== null ? (
                        <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauSoPhan(line.quantity)}</Text>
                      ) : null}
                      {nhan !== null ? (
                        <Text style={[typography.caption, { color: nhan.canKiem ? colors.warn : colors.inkSoft }]}>{nhan.chu}</Text>
                      ) : null}
                    </View>
                    {/* The leader between a dish and its sum, as on a printed bill. */}
                    <View style={[styles.chamDan, { borderBottomColor: colors.lineStrong }]} />
                    <Money size="label" vnd={line.lineTotalVnd} />
                    <Ionicons color={colors.inkFaint} name={mo ? "chevron-up" : "chevron-down"} size={18} />
                  </Pressable>
                  {mo ? (
                    <View style={styles.sua}>
                      <ONhapMuc
                        accessibilityLabel={`Ô tên món ${i + 1}`}
                        label="Món"
                        onChangeText={(t) => setReading((r) => renameLine(r, line.id, t))}
                        placeholder="Ví dụ: Bún bò"
                        value={line.name}
                      />
                      <View style={styles.hang}>
                        <View style={styles.oNho}>
                          <ONhapMuc
                            accessibilityLabel={`Ô số lượng món ${i + 1}`}
                            error={oNhap(line.id, "so-luong") === undefined ? null : loiO("so-luong", oNhap(line.id, "so-luong") ?? "")}
                            keyboardType="number-pad"
                            label="Số phần"
                            maxLength={2}
                            onBlur={() => boNhap(line.id, "so-luong")}
                            onChangeText={(t) => {
                              datNhap(line.id, "so-luong", t);
                              const kq = setQuantity(reading, line.id, t);
                              if (kq.ok && loiO("so-luong", t) === null) setReading(kq.reading);
                            }}
                            value={oNhap(line.id, "so-luong") ?? hienO("so-luong", line.quantity)}
                          />
                        </View>
                        <View style={styles.flex}>
                          <ONhapMuc
                            accessibilityLabel={`Ô tiền món ${i + 1}`}
                            error={oNhap(line.id, "tien") === undefined ? null : loiO("tien", oNhap(line.id, "tien") ?? "")}
                            helper={goiYTien(line.quantity, line.lineTotalVnd)}
                            keyboardType="number-pad"
                            label="Thành tiền (đồng)"
                            onBlur={() => boNhap(line.id, "tien")}
                            onChangeText={(t) => {
                              datNhap(line.id, "tien", t);
                              const kq = setLineTotal(reading, line.id, t === "" ? "0" : t);
                              if (kq.ok) setReading(kq.reading);
                            }}
                            style={styles.soTien}
                            value={oNhap(line.id, "tien") ?? hienO("tien", line.lineTotalVnd)}
                          />
                        </View>
                      </View>
                      <RudiButton compact full={false} icon="trash-outline" label="Bỏ món này" onPress={() => setReading((r) => removeLine(r, line.id))} tone="split" variant="ghost" />
                    </View>
                  ) : null}
                </View>
              );
            })}
            {/* A new line is torn onto the bottom of the receipt. */}
            <Pressable accessibilityRole="button" onPress={themMonMoi} style={({ pressed }) => [styles.themDong, { borderColor: colors.lineStrong, opacity: pressed ? 0.7 : 1 }]}>
              <Ionicons color={colors.split} name="add" size={20} />
              <Text style={[typography.label, { color: colors.split }]}>Thêm món</Text>
            </Pressable>
            <VachCat />
            <DongHoaDon dam phai={<Money vnd={itemsTotalVnd(reading)} />} trai="Tổng" />
          </HoaDonGiay>
        </>
      ) : null}

      {buoc.ten === "gan-mon" ? (
        // Wide window: the table and the dish list on the left page, «Ai có gì»
        // on the right -- the same assignment read per person. On a phone the
        // per-line names already say it, so the pane is dropped.
        <HaiCot
          phaiChiKhiRong
          phai={<AiCoGi bang={aiCoGi(reading.lines, roster, assignment)} />}
          trai={
            <>
          <Heading title={cauTongMon(reading)} />
          <BanGanMon
            dangDung={monBan ? roster.filter((p) => isOn(assignment, monBan.id, p.id)).map((p) => p.id) : []}
            disabled={ban}
            mon={monBan ? { id: monBan.id, ten: monBan.name, tienVnd: monBan.lineTotalVnd } : null}
            nguoi={roster}
            onToggle={(id) => {
              if (monBan) setAssignment((current) => toggle(current, monBan.id, id));
            }}
            testID="ban-gan-mon"
          />
          <ChuThichLe>Chạm ghế hoặc kéo món tới ghế. Tổng không đổi.</ChuThichLe>
          <View>
            {reading.lines.map((line, i) => {
              const mo = moRong.has(line.id);
              const dangDung = roster.filter((person) => isOn(assignment, line.id, person.id));
              const truoc = i > 0 ? whoOn(assignment, reading.lines[i - 1].id) : null;
              const trenBan = monBan?.id === line.id;
              return (
                <View key={line.id} style={[styles.dongGan, { borderBottomColor: colors.line, backgroundColor: trenBan ? colors.splitSoft : "transparent" }]}>
                  <Pressable
                    accessibilityLabel={`Sửa người dùng ${line.name}`}
                    accessibilityRole="button"
                    accessibilityState={{ expanded: mo }}
                    onPress={() => {
                      // A dish off the table goes onto it (and opens); the one
                      // already there folds or opens like any row.
                      if (trenBan) doiMo(line.id);
                      else {
                        setMonTrenBan(line.id);
                        if (!mo) doiMo(line.id);
                      }
                    }}
                    style={({ pressed }) => [styles.dongDau, pressed && styles.bam]}
                  >
                    <View style={styles.flex}>
                      <Text style={[typography.label, { color: colors.ink }]}>{line.name}</Text>
                      <Text style={[typography.caption, { color: dangDung.length === 0 ? colors.warn : colors.inkSoft }]}>
                        {dangDung.length === 0 ? "Chưa chọn người" : `${dangDung.length} người`}
                        {/* Open, the figures below say who; folded, the names do. */}
                        {dangDung.length > 0 && !mo ? ` · ${dangDung.map((p) => p.name).join(", ")}` : ""}
                      </Text>
                    </View>
                    <Money size="label" vnd={line.lineTotalVnd} />
                    <Ionicons color={colors.inkFaint} name={mo ? "chevron-up" : "chevron-down"} size={18} />
                  </Pressable>
                  {mo ? (
                    <View style={styles.sua}>
                      {/* Two people: the two figures ARE the choice; three shortcut
                          buttons outnumbered them, and «Cả nhóm» was the wrong word for
                          a couple (QA 23/09). One «Cả hai» stays, in the same row. */}
                      <RosterPicker
                        disabled={ban}
                        kieu="nhan"
                        nhanCho={(ten) => `${ten} · ${line.name}`}
                        onToggle={(id) => setAssignment((current) => toggle(current, line.id, id))}
                        people={roster}
                        selected={dangDung.map((person) => person.id)}
                        them={
                          roster.length <= 2 ? (
                            <Chip label="Cả hai" onPress={() => setAssignment((a) => datNguoi(a, line.id, rosterIds, rosterIds))} tone="split" />
                          ) : (
                            <>
                              <Chip label="Cả nhóm" onPress={() => setAssignment((a) => datNguoi(a, line.id, rosterIds, rosterIds))} tone="split" />
                              <Chip label="Bỏ hết" onPress={() => setAssignment((a) => datNguoi(a, line.id, [], rosterIds))} tone="split" />
                              {truoc !== null ? (
                                <Chip label="Như món trên" onPress={() => setAssignment((a) => datNguoi(a, line.id, truoc, rosterIds))} tone="split" />
                              ) : null}
                            </>
                          )
                        }
                      />
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
        // Wide window: the stubs on the left, the two things still to decide
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
          <ONhapMuc accessibilityLabel="Ô tên khoản chi" label="Gọi khoản này là" onChangeText={setOccasion} placeholder="Ví dụ: Tối nay Xóm Lào" value={occasion} />
          <ChuThichLe icon="book-outline">Ghi vào sổ là tạo khoản chi với đúng các số ở trên; tổng được kiểm cho khớp trước khi ghi.</ChuThichLe>
            </>
          }
          trai={
            <>
          <Text style={[typography.body, { color: colors.inkSoft }]}>
            {`Rủ Đi chia ${cauTongMon(reading)} theo bản gán ${buoc.chia.assignmentState === "confirmed" ? "đã chốt" : "đang gợi ý"}.`}
          </Text>
          <SectionHeader title="Phần của mỗi người" />
          {/* Each share is a stub torn off the bill, in its person's ink; yours
              comes first and stands a paper height higher. */}
          <View style={styles.cuong}>
            {[...hangKetQua(buoc.chia, roster)]
              .sort((a, b) => (a.id === phien.person_id ? -1 : b.id === phien.person_id ? 1 : 0))
              .map((h) => {
                const laToi = h.id === phien.person_id;
                return (
                  <CuongPhieu key={h.id} mau={mucNguoi(h.id, dark)} noi={laToi}>
                    <View style={styles.hangCuong}>
                      <Avatar name={h.ten} personId={h.id} size={32} />
                      <View style={styles.flex}>
                        <Text style={[laToi ? typography.title : typography.body, { color: colors.ink }]}>{laToi ? "Phần của bạn" : h.ten}</Text>
                        {laToi ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{h.ten}</Text> : null}
                      </View>
                      <Text style={[typography.money, { color: colors.split }]}>{h.tien}</Text>
                    </View>
                    {h.id === payerId || h.lamTron ? (
                      <Inline gap={6} wrap>
                        {h.id === payerId ? <Chip label="Đã trả bill" tone="split" selected /> : null}
                        {h.lamTron ? <Chip label="+lẻ đồng" tone="split" /> : null}
                      </Inline>
                    ) : null}
                  </CuongPhieu>
                );
              })}
          </View>
          {buoc.chia.roundingGainers.length > 0 ? (
            <ChuThichLe>{`Lẻ đồng dồn về: ${buoc.chia.roundingGainers.map((id) => tenCua(roster, id)).join(", ")} (chia lẻ tự động, tổng vẫn khớp).`}</ChuThichLe>
          ) : null}
          {buoc.chia.warnings.map((w) => (
            <Text key={w} style={[typography.caption, { color: colors.warn }]}>
              {cauCanhBaoChia(w)}
            </Text>
          ))}
            </>
          }
        />
      ) : null}

      {buoc.ten === "da-ghi" ? (
        <>
          {/* The bill goes into the book: every share is copied onto a ledger
              page, and Nếp brings the stamp down on it; the seal lands on the
              same beat (300 + 130 ms, `NHIP_DAU`). */}
          <View style={styles.hangSo}>
            <NepDien khoanhKhac="M3" suKien={buoc.expenseVersionId} />
            <TrangSo style={styles.flex} testID="trang-so-da-ghi">
              <DongSo dau trai={`Sổ chi · ${buoc.tenKhoan}`} />
              {buoc.hang.map((h) => (
                <DongSo key={h.id} mau={mucNguoi(h.id, dark)} phai={h.tien} trai={h.id === buoc.nguoiTraId ? `${h.ten} (trả)` : h.ten} />
              ))}
              <DauLon co="vua" dong key={buoc.expenseVersionId} nhan="Đã ghi sổ" style={styles.dauGhi} tre={300} />
            </TrangSo>
          </View>
          <Heading
            title={`Đã ghi: ${buoc.tenKhoan}`}
            subtitle={`${dinhDangTien(buoc.tongVnd)}, ${tenCua(roster, buoc.nguoiTraId)} đã trả. Mỗi người phần của mình như đã chia; quyết toán tính lại từ sổ.`}
          />
          <RudiButton icon="wallet-outline" label="Xem quyết toán" onPress={() => router.replace(`/settlements/${ctx}` as never)} tone="split" />
          <RudiButton label="Về Tin nhắn" onPress={() => router.replace("/(tabs)/messages" as never)} tone="split" variant="outline" />
        </>
      ) : null}
      </View>
      </LatTrang>
    </RudiScreen>
  );
}


/** The one formatter, for a sentence that also carries words. */
function dinhDangTien(vnd: number): string {
  return `${vnd.toLocaleString("vi-VN")}đ`;
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  trang: { gap: 18 },
  hang: { flexDirection: "row", gap: 12 },
  oNho: { width: 110 },
  anh: { width: "100%", aspectRatio: 3 / 4 },
  banTrong: { flexDirection: "row", alignItems: "flex-end", gap: 4 },
  billTrong: { flex: 1 },
  nepCam: { marginBottom: -4 },
  dongMo: { flexDirection: "row", justifyContent: "space-between", paddingVertical: 5 },
  vachMo: { height: 8, borderRadius: 4 },
  hangNut: { flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 8, minHeight: 44 },
  butChi: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 48, alignSelf: "flex-start", borderBottomWidth: 1, borderStyle: "dashed", paddingRight: 12 },
  hangDau: { flexDirection: "row", alignItems: "center", gap: 12 },
  dauHoaDon: { gap: 2, alignItems: "center" },
  dong: { paddingVertical: 2 },
  dongGan: { borderBottomWidth: StyleSheet.hairlineWidth, paddingVertical: 4, paddingHorizontal: 6, borderRadius: 8 },
  dongDau: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 52, paddingVertical: 4 },
  // A dotted leader that takes the room between the dish and its sum, and
  // gives it up first when the name is long.
  tenMon: { flexShrink: 1 },
  chamDan: { flexGrow: 1, flexShrink: 1, minWidth: 12, maxWidth: 120, alignSelf: "flex-end", marginBottom: 16, borderBottomWidth: 1.5, borderStyle: "dotted" },
  sua: { gap: 12, paddingBottom: 12 },
  soTien: { fontVariant: ["tabular-nums"] },
  themDong: { flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 6, minHeight: 48, borderWidth: 1, borderStyle: "dashed", borderRadius: 6 },
  bam: { opacity: 0.75 },
  cuong: { gap: 10 },
  hangCuong: { flexDirection: "row", alignItems: "center", gap: 10 },
  hangSo: { flexDirection: "row", alignItems: "flex-start", gap: 10 },
  dauGhi: { marginTop: 10 },
});
