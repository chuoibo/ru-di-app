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
import { Pressable, StyleSheet, Text, TextInput, View } from "react-native";
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
  dongCong,
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
import { typography, useRudiTheme } from "../../theme";
import { AiNote, Chip, Heading, Inline, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { AiCoGi } from "../../ui/AiCoGi";
import { Avatar } from "../../ui/Avatar";
import { BanAn } from "../../ui/BanAn";
import { BanGanMon } from "../../ui/BanGanMon";
import { BAN_DAI_TU } from "../../ui/hinh-tien";
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
import { Stamp } from "../../ui/Stamp";
import { StampButton } from "../../ui/StampButton";
import { Stepper } from "../../ui/Stepper";
import { DongSo, TrangSo } from "../../ui/TrangSo";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { boNhapBill, buocMoLai, coMonDangGo, docNhapBill, luuNhapBill } from "../../chia-bill/nhap-bill";

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

/** The step a kept bill can reopen at: the review, or the table with the
 *  server's bill. A result is never kept: it reopens at the table, so a split
 *  is always computed again rather than shown from before. */
type BuocLuu = { ten: "xem-lai" } | { ten: "gan-mon"; bill: BillWire };

/** What is kept of a bill in progress (`chia-bill/nhap-bill.ts`). */
type NhapDaGo = { reading: BillReading; assignment: Assignment; payerId: string; occasion: string; luc: number; buoc?: BuocLuu };

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
  const { colors } = useRudiTheme();
  // The step CTA is the last thing in the scroll; at font 1.3 it met the gesture pill.
  const insets = useSafeAreaInsets();
  const contextId = phien.context_id;
  // A bill typed here before comes back as it was (QA UI-052): after a
  // reload, or Back then Forward, at the step the person was on; after a
  // while away, offered from step 1 instead of a blank receipt.
  const [daGoTruoc] = useState(() => (contextId === null ? null : docNhapBill<NhapDaGo>(contextId)));
  const [buoc, setBuoc] = useState<Buoc>(() => buocMoLai(daGoTruoc, Date.now()) ?? { ten: "bat-dau" });
  const [reading, setReading] = useState<BillReading>(daGoTruoc?.reading ?? hoaDonTrong());
  const [assignment, setAssignment] = useState<Assignment>(daGoTruoc?.assignment ?? {});
  const [moRong, setMoRong] = useState<Set<string>>(() => (buoc.ten === "xem-lai" && daGoTruoc ? moBanDau(daGoTruoc.reading) : new Set()));
  const [roster, setRoster] = useState<ThanhVien[]>([]);
  const [payerId, setPayerId] = useState(daGoTruoc?.payerId ?? phien.person_id);
  const [occasion, setOccasion] = useState(daGoTruoc?.occasion ?? dip ?? "");
  // «Bắt đầu bill mới» over a bill in progress asks first, in place.
  const [xacNhanBo, setXacNhanBo] = useState(false);
  // A failed read: «thu-lai» may work again, «khong-bat» will not until the
  // server is set up, so «Dùng ảnh này» steps aside (QA UI-056).
  const [docAnhHong, setDocAnhHong] = useState<"thu-lai" | "khong-bat" | null>(null);
  // What the person is typing in a number box, per `${line}:${kind}`, while
  // that box has focus. The bill only receives what parses (`o-so.ts`).
  const [nhapSo, setNhapSo] = useState<Record<string, string>>({});
  const [thongBao, setThongBao] = useState<string | null>(null);
  // The dish the review step refused, and which of its fields: opened, marked
  // in place and given the cursor, so the footer's reason points somewhere.
  const [dongLoi, setDongLoi] = useState<{ id: string; o: "ten" | "tien" } | null>(null);
  const oLoiRef = useRef<TextInput | null>(null);
  useEffect(() => {
    if (dongLoi !== null) oLoiRef.current?.focus();
  }, [dongLoi]);
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

  // Kept on every change; dropped once it is in the book, and when the last
  // dish is taken off (a reload must not bring back what was removed).
  useEffect(() => {
    if (contextId === null) return;
    if (buoc.ten === "da-ghi" || !coMonDangGo(reading.lines)) {
      boNhapBill(contextId);
      return;
    }
    const buocLuu: BuocLuu | undefined =
      buoc.ten === "gan-mon" || buoc.ten === "ket-qua" ? { ten: "gan-mon", bill: buoc.bill } : buoc.ten === "xem-lai" ? { ten: "xem-lai" } : undefined;
    luuNhapBill(contextId, { reading, assignment, payerId, occasion, luc: Date.now(), buoc: buocLuu } satisfies NhapDaGo);
  }, [contextId, buoc, reading, assignment, payerId, occasion]);

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
      setDocAnhHong(null);
      try {
        const doc = await docBillTuAnh({ uri, bytes }, phien.person_id);
        setReading(doc);
        setAssignment({});
        setMoRong(moBanDau(doc));
        setBuoc({ ten: "xem-lai" });
      } catch (error) {
        const khongBat = error instanceof ApiError && (error.code === "receipt_reader_not_configured" || error.code === "permission_denied");
        // Reading is off: typing becomes the step's main button, and the
        // sentence says why in two lines and points at it (QA UI-056: the way
        // on that the sentence names is on the screen); four lines of it sat
        // over the button before (B4 finish review).
        setThongBao(
          error instanceof ApiError && error.code === "receipt_reader_not_configured"
            ? "Phần đọc ảnh bill của Rủ Đi chưa bật; ảnh của bạn không có lỗi. Bạn có thể nhập món bằng tay."
            : cauSauKhiScanHong(loiRaChu(error)),
        );
        setDocAnhHong(khongBat ? "khong-bat" : "thu-lai");
      }
    });

  const coBillDo = coMonDangGo(reading.lines);
  /** A fresh hand-typed bill; over one in progress only after the person said so. */
  const nhapTay = () => {
    const doc = themMon(hoaDonTrong());
    setReading(doc);
    setAssignment({});
    setMoRong(moBanDau(doc));
    setXacNhanBo(false);
    setDocAnhHong(null);
    setThongBao(null);
    setBuoc({ ten: "xem-lai" });
  };
  const tiepTuc = () => {
    setMoRong(moBanDau(reading));
    setDocAnhHong(null);
    setThongBao(null);
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
        const khongTen = reading.lines.find((l) => l.name.trim() === "");
        const khongTien = reading.lines.find((l) => l.lineTotalVnd <= 0);
        const dong = khongTen ? { id: khongTen.id, o: "ten" as const } : khongTien ? { id: khongTien.id, o: "tien" as const } : null;
        if (dong !== null) {
          setMoRong((m) => new Set(m).add(dong.id));
          setDongLoi(dong);
        }
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
    ) : buoc.ten === "xem-anh" && docAnhHong === "khong-bat" ? (
      <RudiButton disabled={ban} icon="pencil" label={coBillDo ? "Tiếp tục bill đang gõ" : "Nhập tay"} onPress={coBillDo ? tiepTuc : nhapTay} tone="split" />
    ) : null;

  const monBan = buoc.ten === "gan-mon" ? (reading.lines.find((l) => l.id === monTrenBan) ?? reading.lines[0] ?? null) : null;

  return (
    <RudiScreen
      bottomInset={Math.max(insets.bottom, 16) + 40}
      cuonVeDau={buoc.ten}
      footer={
        // What the step refused or what failed is said right above its button,
        // in the footer that never scrolls away: at the top of the page it sat
        // at y −248, −256, −578 (QA UI-051).
        thongBao !== null || nutChinh ? (
          <View style={styles.chan}>
            <CauTaiCho cau={thongBao} hanhDong={docAnhHong === "thu-lai" && buoc.ten === "xem-anh" ? { label: "Nhập tay", onPress: coBillDo ? tiepTuc : nhapTay } : undefined} />
            {nutChinh}
          </View>
        ) : undefined
      }
      footerInset={Math.max(insets.bottom, 12) + 4}
      tone="split"
      testID="receipt-review-screen"
    >
      <TopBar onBack={luiTrongLuong ? quayLai : undefined} title={tieuDeBuoc(buoc)} />
      <Stepper current={soBuoc(buoc) - 1} lockedReason={lyDoKhoa} steps={CAC_BUOC} tone="split" />

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
            {coBillDo ? (
              // The bill in progress lies on the table, and it is the button:
              // its dishes, its total, «Tiếp tục» (QA UI-052).
              <Pressable
                accessibilityLabel={`Tiếp tục bill đang gõ, ${reading.lines.length} món`}
                accessibilityRole="button"
                disabled={ban}
                onPress={tiepTuc}
                style={({ pressed }) => [styles.billTrong, { opacity: pressed ? 0.85 : 1 }]}
                testID="bill-dang-go"
              >
                <HoaDonGiay rangTren>
                  <TieuDeHoaDon phu={`${reading.lines.length} món, chưa ghi sổ`} ten="Bill đang gõ" />
                  <VachCat />
                  {reading.lines.slice(0, 3).map((l) => (
                    <DongHoaDon key={l.id} phai={<Money size="label" vnd={l.lineTotalVnd} />} trai={<Text numberOfLines={1} style={[typography.body, { color: colors.ink }]}>{l.name.trim() || "Món chưa tên"}</Text>} />
                  ))}
                  {reading.lines.length > 3 ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{`và ${reading.lines.length - 3} món nữa`}</Text> : null}
                  <VachCat />
                  <DongHoaDon dam phai={<Money vnd={itemsTotalVnd(reading)} />} trai="Tổng" />
                  <View style={styles.hangNut}>
                    <Ionicons color={colors.split} name="arrow-forward" size={22} />
                    <Text style={[typography.title, { color: colors.split }]}>Tiếp tục bill này</Text>
                  </View>
                </HoaDonGiay>
              </Pressable>
            ) : (
            <Pressable
              accessibilityLabel="Chọn ảnh bill"
              accessibilityRole="button"
              aria-busy={ban}
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
            )}
            <NepTinh style={styles.nepCam} tm={TIET_MUC["cam-may"]} width={96} />
          </View>
          </BanAn>
          {/* Typing is a pencil line on the same page, not a second button. With
              a bill in progress, the other ways in are pencil lines too, and a
              fresh one asks before it drops the dishes. */}
          {coBillDo ? (
            <>
              <Pressable accessibilityRole="button" disabled={ban} onPress={() => void chonAnh()} style={({ pressed }) => [styles.butChi, { borderBottomColor: colors.lineStrong, opacity: pressed ? 0.7 : 1 }]}>
                <Ionicons color={colors.inkSoft} name="camera-outline" size={18} />
                <Text style={[typography.title, { color: colors.ink }]}>Chọn ảnh bill khác</Text>
              </Pressable>
              {xacNhanBo ? (
                <View accessibilityLiveRegion="polite" style={styles.xacNhan}>
                  <Text style={[typography.body, { color: colors.ink }]}>{`Bỏ ${reading.lines.length} món đang gõ để làm bill mới?`}</Text>
                  <View style={styles.hangXacNhan}>
                    <RudiButton compact full={false} label="Bỏ, làm bill mới" onPress={() => { if (contextId !== null) boNhapBill(contextId); nhapTay(); }} tone="warn" variant="outline" />
                    <RudiButton compact full={false} label="Giữ lại" onPress={() => setXacNhanBo(false)} tone="split" variant="ghost" />
                  </View>
                </View>
              ) : (
                <Pressable accessibilityRole="button" disabled={ban} onPress={() => setXacNhanBo(true)} style={({ pressed }) => [styles.butChi, { borderBottomColor: colors.lineStrong, opacity: pressed ? 0.7 : 1 }]}>
                  <Ionicons color={colors.inkSoft} name="document-outline" size={18} />
                  <Text style={[typography.title, { color: colors.ink }]}>Bắt đầu bill mới</Text>
                </Pressable>
              )}
            </>
          ) : (
            <Pressable accessibilityRole="button" disabled={ban} onPress={nhapTay} style={({ pressed }) => [styles.butChi, { borderBottomColor: colors.lineStrong, opacity: pressed ? 0.7 : 1 }]}>
              <Ionicons color={colors.inkSoft} name="pencil" size={18} />
              <Text style={[typography.title, { color: colors.ink }]}>Nhập tay</Text>
            </Pressable>
          )}
          <NapGiay tieuDe="Cách chia">
            <Text style={[typography.body, { color: colors.ink }]}>Rủ Đi đọc từng món trên hoá đơn. Bước sau, cả nhóm chọn ai dùng món nào; máy chủ chia mỗi món cho đúng những người đó, lẻ đồng dồn về một người, và tổng luôn khớp hoá đơn.</Text>
          </NapGiay>
        </>
      ) : null}

      {buoc.ten === "xem-anh" ? (
        <>
          {/* The heading gives way, so Nếp's M2 stays on the screen (QA UI-055). */}
          <View style={styles.hangDau}>
            <View style={styles.flex}>
              <Heading title="Ảnh này đúng bill chứ?" subtitle="Rủ Đi sẽ đọc từng món từ ảnh này." />
            </View>
            <NepDien khoanhKhac="M2" suKien={buoc.uri} />
          </View>
          <KhungAnh tilt={-1}>
            <Image accessibilityLabel="Ảnh bill đã chọn" contentFit="cover" source={{ uri: buoc.uri }} style={[styles.anh, { backgroundColor: colors.line }]} />
          </KhungAnh>
          <ChuThichLe icon="lock-closed-outline">Chưa gửi gì cho tới khi bạn bấm dùng.</ChuThichLe>
          {docAnhHong === "khong-bat" ? null : (
            <RudiButton disabled={ban} icon="scan-outline" label={docAnhHong === "thu-lai" ? "Đọc lại ảnh này" : "Dùng ảnh này"} loading={ban} onPress={() => void docAnh(buoc.uri, buoc.bytes)} tone="split" />
          )}
          {/* Another photo is read by the same switched-off reader. */}
          {docAnhHong === "khong-bat" ? null : (
            <RudiButton disabled={ban} icon="images-outline" label="Chọn ảnh khác" onPress={() => void chonAnh()} tone="split" variant="outline" />
          )}
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
              const vanLoi = dongLoi?.id === line.id && (dongLoi.o === "ten" ? line.name.trim() === "" : line.lineTotalVnd <= 0);
              // A name that will wrap keeps no leader: a dotted stub was left
              // floating by the amount under «NướcSuốiChaiLớn…» (finish review).
              const dan = ten.length <= 26 && !ten.split(/\s+/).some((tu) => tu.length > 16);
              return (
                <View key={line.id} style={styles.dong}>
                  <Pressable
                    accessibilityLabel={`${mo ? "Gấp" : "Sửa"} ${ten}`}
                    accessibilityRole="button"
                    aria-expanded={mo}
                    onPress={() => doiMo(line.id)}
                    style={({ pressed }) => [styles.dongDau, pressed && styles.bam]}
                  >
                    <View style={styles.tenMon}>
                      <Text style={[typography.label, { color: vanLoi ? colors.warn : colors.ink }]}>{ten}</Text>
                      {cauSoPhan(line.quantity) !== null ? (
                        <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauSoPhan(line.quantity)}</Text>
                      ) : null}
                      {nhan !== null ? (
                        <Text style={[typography.caption, { color: nhan.canKiem ? colors.warn : colors.inkSoft }]}>{nhan.chu}</Text>
                      ) : null}
                    </View>
                    {/* The leader between a dish and its sum, as on a printed bill. */}
                    {dan ? <View style={[styles.chamDan, { borderBottomColor: colors.lineStrong }]} /> : <View style={styles.flex} />}
                    <Money size="label" vnd={line.lineTotalVnd} />
                    <Ionicons color={colors.inkFaint} name={mo ? "chevron-up" : "chevron-down"} size={18} />
                  </Pressable>
                  {mo ? (
                    <View style={styles.sua}>
                      <ONhapMuc
                        accessibilityLabel={`Ô tên món ${i + 1}`}
                        error={dongLoi?.id === line.id && dongLoi.o === "ten" && line.name.trim() === "" ? "Đặt tên cho món này." : null}
                        label="Món"
                        oRef={dongLoi?.id === line.id && dongLoi.o === "ten" ? oLoiRef : undefined}
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
                            error={
                              oNhap(line.id, "tien") !== undefined
                                ? loiO("tien", oNhap(line.id, "tien") ?? "")
                                : dongLoi?.id === line.id && dongLoi.o === "tien" && line.lineTotalVnd <= 0
                                  ? "Gõ thành tiền của cả dòng."
                                  : null
                            }
                            helper={goiYTien(line.quantity, line.lineTotalVnd)}
                            keyboardType="number-pad"
                            label="Thành tiền (đồng)"
                            oRef={dongLoi?.id === line.id && dongLoi.o === "tien" ? oLoiRef : undefined}
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
                      <RudiButton accessibilityLabel={`Bỏ món ${line.name.trim() || "này"}`} compact full={false} icon="trash-outline" label="Bỏ món này" onPress={() => setReading((r) => removeLine(r, line.id))} style={styles.boMon} tone="warn" variant="ghost" />
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
                // Rows on the paper, split by hairlines: the dish on the table is
                // said in words, not by a teal block (teal is for the number).
                <View key={line.id} style={[styles.dongGan, { borderBottomColor: colors.line }]}>
                  <Pressable
                    accessibilityLabel={`Sửa người dùng ${line.name}`}
                    accessibilityRole="button"
                    aria-expanded={mo}
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
                        {/* «Chia đều» is said here, in words: as a stamp on the
                            table's card it hid a plate (B4 finish review). */}
                        {dangDung.length === 0
                          ? "Chưa chọn người"
                          : dangDung.length === roster.length && roster.length > 1
                            ? `Chia đều cả ${roster.length} người`
                            : `${dangDung.length} người`}
                        {trenBan ? " · đang trên bàn" : ""}
                        {/* Open, the figures below say who; folded, the names do. */}
                        {dangDung.length > 0 && !mo ? ` · ${dangDung.map((p) => p.name).join(", ")}` : ""}
                      </Text>
                    </View>
                    <Money size="label" tone="split" vnd={line.lineTotalVnd} />
                    <Ionicons color={colors.inkFaint} name={mo ? "chevron-up" : "chevron-down"} size={18} />
                  </Pressable>
                  {mo && roster.length >= BAN_DAI_TU ? (
                    // A big group's seats are the long table above (this dish is
                    // on it): twenty more figures here repeated them one by one.
                    <View style={[styles.sua, styles.hangChip]}>
                      <Chip label="Cả nhóm" onPress={() => setAssignment((a) => datNguoi(a, line.id, rosterIds, rosterIds))} tone="split" />
                      <Chip label="Bỏ hết" onPress={() => setAssignment((a) => datNguoi(a, line.id, [], rosterIds))} tone="split" />
                      {truoc !== null ? (
                        <Chip label="Như món trên" onPress={() => setAssignment((a) => datNguoi(a, line.id, truoc, rosterIds))} tone="split" />
                      ) : null}
                    </View>
                  ) : mo ? (
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
          {/* One bill, its shares printed as a strip: yours is torn off and
              stands a paper higher; everyone else's are still on the strip,
              perforated between them, and the strip ends in its footing -- the
              shares added up against the bill. The payer's stamp comes down on
              their row when the chip on the right picks them. */}
          {(() => {
            const hang = hangKetQua(buoc.chia, roster);
            const cuaBan = hang.find((h) => h.id === phien.person_id);
            const conLai = hang.filter((h) => h.id !== phien.person_id);
            const cong = dongCong(buoc.chia);
            const dongPhan = (h: (typeof hang)[number], laToi: boolean) => (
              <View style={styles.hangCuong}>
                <Avatar name={h.ten} personId={h.id} size={laToi ? 36 : 28} />
                {/* The stamp sits under the name: beside it, it pressed «Phần
                    của bạn» into a word a line at 390. */}
                <View style={styles.tenPhan}>
                  <Text style={[laToi ? typography.title : typography.body, { color: colors.ink }]}>{laToi ? "Phần của bạn" : h.ten}</Text>
                  {laToi ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{h.ten}</Text> : null}
                  {h.id === payerId ? <Stamp dong key={payerId} label="Đã trả bill" style={styles.dauTra} tilt={-3} tone="split" /> : null}
                </View>
                <Money size={laToi ? "money" : "body"} tone="split" vnd={buoc.chia.allocations[h.id] ?? 0} />
              </View>
            );
            return (
              <View style={styles.cuong}>
                {cuaBan ? (
                  // No coloured side stripe: torn off and a paper higher
                  // already says it is yours (B4 finish review).
                  <CuongPhieu noi>
                    {dongPhan(cuaBan, true)}
                  </CuongPhieu>
                ) : null}
                <HoaDonGiay rangTren testID="dai-phan">
                  {conLai.map((h, i) => (
                    <View key={h.id} style={styles.oPhan}>
                      {i > 0 ? <VachCat /> : null}
                      {dongPhan(h, false)}
                    </View>
                  ))}
                  <View style={[styles.vachCong, { borderColor: colors.ink }]} />
                  <DongHoaDon
                    dam
                    phai={<Money size="body" tone={cong.lechVnd === 0 ? "ink" : "warn"} vnd={cong.tongVnd} />}
                    // How many shares took a rounding đồng (each exactly one,
                    // the allocator's largest remainder) is said once, here:
                    // printed under each of them it read six times in eight.
                    phu={
                      cong.lechVnd === 0
                        ? `Đúng bằng tổng bill: chia lẻ không làm mất hay thừa đồng nào.${buoc.chia.roundingGainers.length > 0 ? ` ${buoc.chia.roundingGainers.length} phần được làm tròn lên 1đ để cộng lại đủ.` : ""}`
                        : `Lệch tổng bill ${dinhDangTien(Math.abs(cong.lechVnd))}: Rủ Đi sẽ không ghi cách chia này. Quay lại bước gán món để chia lại.`
                    }
                    testID="dong-cong"
                    trai={`Cộng ${cong.soPhan} phần`}
                  />
                </HoaDonGiay>
              </View>
            );
          })()}
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
          {/* What happened is said first, over the page: under twenty rows it
              was the last thing on the screen (B4 finish review). */}
          <Heading
            title={`Đã ghi: ${buoc.tenKhoan}`}
            subtitle={`${dinhDangTien(buoc.tongVnd)}, ${tenCua(roster, buoc.nguoiTraId)} đã trả. Mỗi người phần của mình như đã chia; quyết toán tính lại từ sổ.`}
          />
          {/* The bill goes into the book: every share is copied onto a ledger
              page, and Nếp brings the stamp down on it; the seal lands on the
              same beat (300 + 130 ms, `NHIP_DAU`). */}
          <View style={styles.hangSo}>
            <NepDien khoanhKhac="M3" suKien={buoc.expenseVersionId} />
            <TrangSo style={styles.flex} testID="trang-so-da-ghi">
              <DongSo dau trai={`Sổ chi · ${buoc.tenKhoan}`} />
              {buoc.hang.map((h) => (
                // Names in ink, amounts teal: the ledger page reads like the
                // strip of the step before (B4 finish review).
                <DongSo key={h.id} nhan={h.id === buoc.nguoiTraId ? "Đã trả" : undefined} phai={h.tien} trai={h.ten} />
              ))}
              <DauLon co="vua" dong key={buoc.expenseVersionId} nhan="Đã ghi sổ" style={styles.dauGhi} tre={300} />
            </TrangSo>
          </View>
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
  chan: { gap: 8 },
  xacNhan: { gap: 8, paddingVertical: 4 },
  hangXacNhan: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  dauHoaDon: { gap: 2, alignItems: "center" },
  dong: { paddingVertical: 2 },
  dongGan: { borderBottomWidth: StyleSheet.hairlineWidth, paddingVertical: 4 },
  hangChip: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
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
  hangCuong: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 44 },
  tenPhan: { flex: 1, minWidth: 0, alignItems: "flex-start" },
  dauTra: { marginTop: 6 },
  // Removing a dish is a quiet, destructive end-of-row action, not the
  // centred teal call it was (it read like the step's next move).
  boMon: { alignSelf: "flex-end" },
  oPhan: { gap: 8 },
  // The footing's double rule, the way a till closes a column before the sum.
  vachCong: { height: 4, borderTopWidth: 1, borderBottomWidth: 1, marginTop: 4 },
  hangSo: { flexDirection: "row", alignItems: "flex-start", gap: 10 },
  dauGhi: { marginTop: 10 },
});
