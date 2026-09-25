/**
 * Tạo kèo on a real session (M4): title, dates, headcount, budget a person,
 * validated by App B's `kiemTraTaoBuoiDi` and written with one Attempt.
 * Dates default to today; headcount defaults to the group's size (a real
 * number, not an invented one); the budget is the person's to type.
 *
 * ## An invitation, not a form to file (UI v2, đợt 4)
 *
 * The fields sit directly on the paper in the order a person says an
 * invitation out loud -- what, when, how many, how much -- with the group
 * named at the top so nobody creates into the wrong one. A small preview
 * under the fields reads the invitation back as it will appear on the plan.
 * There is one create button, and it waits on the server before anything is
 * called created. No native date picker ships in this build, so the date
 * fields say their format beside them instead of after a failed submit.
 */
import { Redirect, useRouter } from "expo-router";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { ApiError, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../../api";
import type { Phien } from "../../../phien";
import { kiemTraTaoBuoiDi, nhanKhoangNgay } from "../../../screens/len-plan/buoi-di";
import { homNayIso, taoKeo, kiemTraChangMoi } from "../../keo/keo";
import { typography, useRudiTheme } from "../../theme";
import { Field, Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { CauRu } from "../../ui/CauRu";
import { ChonNgayLich } from "../../ui/ChonNgayLich";
import { ChuThichLe } from "../../ui/ChuThichLe";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { PressScale } from "../../ui/PressScale";
import { StampButton } from "../../ui/StampButton";
import { TheVe } from "../../ui/TheVe";
import { Washi } from "../../ui/Washi";
import { dinhDangTienVnd } from "../../../screens/chat/ke-hoach";
import { docAnhChupChat } from "../../chat/thay-doi";
import { docTheAi } from "../../chat/tin-song";
import type { ChangGui } from "../../../screens/len-plan/buoi-di";
import { docKeoTuChat, taoKeoTuChat } from "../../chat/ai-invocations";
import { HangChang } from "./HangChang";
import { ngayKieuViet, ngayVeISO } from "../../chat/to-hen-chung";

const MUC_NGAN_SACH = [
  { nhan: "200 nghìn", dong: 200000 },
  { nhan: "300 nghìn", dong: 300000 },
  { nhan: "500 nghìn", dong: 500000 },
  { nhan: "1 triệu", dong: 1000000 },
] as const;

/** What the digits typed mean in đồng, or nothing while they are not a whole number. */
function tienDaGo(chu: string): string | null {
  const t = chu.trim();
  if (!/^[0-9]+$/.test(t)) return null;
  return dinhDangTienVnd(Number(t));
}

function nhomHienTai(phien: Phien): { ten: string; soNguoi: string } {
  const nhom = phien.contexts?.find((n) => n.id === phien.context_id);
  if (nhom === undefined) return { ten: "nhóm của bạn", soNguoi: "" };
  return { ten: nhom.display_name, soNguoi: String(nhom.member_count) };
}

export function CreateOutingLiveScreen({ phien, sourceMessageId }: { phien: Phien; sourceMessageId?: string }) {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { colors, radius } = useRudiTheme();
  const nhom = nhomHienTai(phien);
  const [title, setTitle] = useState("");
  // The boxes hold the spelling a person types and reads; ISO lives on the wire.
  const [startsOn, setStartsOn] = useState(ngayKieuViet(homNayIso()));
  const [endsOn, setEndsOn] = useState(ngayKieuViet(homNayIso()));
  const [headcount, setHeadcount] = useState(nhom.soNguoi);
  const [nganSach, setNganSach] = useState("");
  const [loi, setLoi] = useState<string | null>(null);
  const [loiNgay, setLoiNgay] = useState<{ batDau: string | null; ketThuc: string | null } | null>(null);
  const [dangTao, setDangTao] = useState(false);
  const attempt = useRef<Attempt | null>(null);
  const attemptBody = useRef<string | null>(null);
  const [sourceLoading, setSourceLoading] = useState(Boolean(sourceMessageId));
  const [sourceFailed, setSourceFailed] = useState(false);
  const [stops, setStops] = useState<ChangGui[]>([]);
  const [reviewTime, setReviewTime] = useState(false);
  const [existingId, setExistingId] = useState<string | null>(null);
  const creating = useRef(false);

  useEffect(() => {
    if (!sourceMessageId || !phien.context_id) return;
    let active = true;
    void docAnhChupChat(phien.context_id, phien.person_id, [{ sequence: 0, revision: 0, type: "message", entity_id: sourceMessageId }]).then((snapshot) => {
      if (!active) return;
      const message = snapshot.messages.find((item) => item.id === sourceMessageId);
      const card = docTheAi(message?.card);
      if (card.loai !== "itinerary") throw new Error("Unavailable plan");
      setTitle(card.the.tieuDe);
      setStops(card.the.chang.map((stop) => ({ at: stop.gio, label: stop.diaDiem.ten, place_name: stop.diaDiem.ten, place_id: stop.diaDiem.id })));
      setReviewTime(card.the.chang.some((stop) => !/^([01][0-9]|2[0-3]):[0-5][0-9]$/.test(stop.gio)));
    }).catch(() => {
      if (active) { setSourceFailed(true); setLoi("Không đọc được tờ hẹn gốc. Quay lại chat để mở lại, hoặc viết một kèo mới."); }
    }).finally(() => { if (active) setSourceLoading(false); });
    return () => { active = false; };
  }, [sourceMessageId, phien.context_id, phien.person_id]);

  if (phien.context_id === null) return <Redirect href="/(tabs)/plan" />;
  const contextId = phien.context_id;

  const tao = async () => {
    if (creating.current || sourceLoading || sourceFailed) return;
    // The boxes hold what a person typed; the contract holds ISO. A date the
    // calendar does not have is refused here, by the box it was typed in --
    // sending it raw asked the server to explain a typo it cannot see.
    const batDau = ngayVeISO(startsOn);
    const ketThuc = ngayVeISO(endsOn);
    if (!batDau || !ketThuc) {
      setLoiNgay({ batDau: batDau ? null : "Ngày không có thật. Dạng ngày/tháng/năm.", ketThuc: ketThuc ? null : "Ngày không có thật. Dạng ngày/tháng/năm." });
      return;
    }
    setLoiNgay(null);
    const kq = kiemTraTaoBuoiDi({
      title,
      starts_on: batDau,
      ends_on: ketThuc,
      headcount,
      nganSach,
    });
    if (!kq.ok) {
      setLoi(kq.loi);
      return;
    }
    if (reviewTime) {
      for (const stop of stops) { const check = kiemTraChangMoi(stop.at, stop.label); if (!check.ok) { setLoi(check.loi); return; } }
    }
    const bodyKey = JSON.stringify(kq.body);
    if (attempt.current === null || attemptBody.current !== bodyKey) { attempt.current = newAttempt(); attemptBody.current = bodyKey; }
    creating.current = true;
    setDangTao(true);
    setLoi(null);
    try {
      const id = sourceMessageId
        ? (await taoKeoTuChat(contextId, phien.person_id, sourceMessageId, kq.body, reviewTime ? stops : undefined)).outing_id
        : (await taoKeo(contextId, phien.person_id, kq.body, attempt.current!)).id;
      // `vua=tao`: the outing opens on the moment it was made (M5).
      router.replace(`/outings/${id}?vua=tao` as never);
    } catch (error) {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
      if (sourceMessageId && error instanceof ApiError && error.code === "plan_already_promoted") {
        try { setExistingId((await docKeoTuChat(contextId, phien.person_id, sourceMessageId)).outing_id); } catch { /* Preserve the refusal; never create a second outing. */ }
      }
    } finally {
      creating.current = false;
      setDangTao(false);
    }
  };

  const tien = tienDaGo(nganSach);
  const xemTruoc = title.trim() !== "";

  const soNguoi = Number(headcount.trim());
  const doiSoNguoi = (buoc: number) => {
    const hienTai = Number.isInteger(soNguoi) && soNguoi > 0 ? soNguoi : 1;
    setHeadcount(String(Math.max(1, hienTai + buoc)));
  };
  const ngayDi = ngayVeISO(startsOn) ?? startsOn;
  const ngayVe = ngayVeISO(endsOn) ?? endsOn;

  return (
    // The one decision of this screen stays above the keyboard and the gesture
    // bar however tall the form grows: at font 1.3 the button sat below the
    // fold behind the IME and the live board could not reach it (2026-09-07).
    <RudiScreen
      bottomInset={Math.max(insets.bottom, 16) + 40}
      contentStyle={styles.screen}
      footer={
        sourceMessageId ? (
          <RudiButton disabled={dangTao || sourceLoading || sourceFailed || existingId !== null} label="Xác nhận và tạo kèo" loading={dangTao || sourceLoading} onPress={() => void tao()} />
        ) : (
          // The invitation is sealed with a stamp, not filed with a button (ADR-0037 D1).
          <StampButton disabled={dangTao || sourceLoading || sourceFailed || existingId !== null} label="Tạo kèo" loading={dangTao} onPress={() => void tao()} size="vua" tilt={-1} />
        )
      }
      footerInset={Math.max(insets.bottom, 12) + 4}
      testID="create-outing-screen"
    >
      <TopBar title={sourceMessageId ? "Sửa tờ hẹn" : "Kèo mới"} />
      <Heading title={sourceMessageId ? "Từ nét chì, thành lời hẹn." : "Hội mình đi đâu?"} subtitle={sourceMessageId ? `Bạn xác nhận tờ hẹn này cho ${nhom.ten}.` : undefined} />
      {/* The invitation said out loud, with its blanks: what, when, how many,
          how much (plan S3). The words are the labels; each blank is its own field. */}
      <View style={[styles.thiep, { backgroundColor: colors.card, borderColor: colors.lineStrong, borderRadius: radius.small }]}>
        <Washi style={styles.washi} />
        <CauRu
          co="lon"
          mau={`Rủ ${nhom.ten} đi {ten}`}
          moTa={`Rủ ${nhom.ten} đi …`}
          o={{
            ten: (
              <ONhapMuc
                accessibilityLabel="Ô tên kèo"
                co="lon"
                khungStyle={styles.oTen}
                onChangeText={(t) => {
                  setTitle(t);
                  if (loi !== null) setLoi(null);
                }}
                placeholder="Đà Lạt cuối tuần"
                value={title}
              />
            ),
          }}
          testID="cau-ru-keo"
        />
        <View style={styles.hangLich}>
          <Text style={[typography.h2, { color: colors.ink }]}>từ</Text>
          <ChonNgayLich giaTri={startsOn} nhan="Ngày đi" onChange={(v) => { setStartsOn(v); if (loiNgay) setLoiNgay(null); }} testID="ngay-di" />
          <Text style={[typography.h2, { color: colors.ink }]}>tới</Text>
          <ChonNgayLich giaTri={endsOn} nhan="Ngày về" onChange={(v) => { setEndsOn(v); if (loiNgay) setLoiNgay(null); }} testID="ngay-ve" />
        </View>
        {loiNgay?.batDau || loiNgay?.ketThuc ? (
          <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{loiNgay.batDau ?? loiNgay.ketThuc}</Text>
        ) : null}
        <View style={styles.hangSo}>
          <Pressable accessibilityLabel="Bớt một người" accessibilityRole="button" hitSlop={4} onPress={() => doiSoNguoi(-1)} style={[styles.nutSo, { borderColor: colors.lineStrong }]}>
            <Ionicons color={colors.ink} name="remove" size={20} />
          </Pressable>
          <ONhapMuc accessibilityLabel="Ô số người" co="lon" keyboardType="number-pad" khungStyle={styles.oSo} onChangeText={setHeadcount} style={styles.giua} value={headcount} />
          <Pressable accessibilityLabel="Thêm một người" accessibilityRole="button" hitSlop={4} onPress={() => doiSoNguoi(1)} style={[styles.nutSo, { borderColor: colors.lineStrong }]}>
            <Ionicons color={colors.ink} name="add" size={20} />
          </Pressable>
          <Text style={[typography.h2, { color: colors.ink }]}>người,</Text>
        </View>
        {nhom.soNguoi ? <ChuThichLe icon="people-outline">{nhom.ten} hiện có {nhom.soNguoi} người; bớt đi nếu chỉ một phần đi.</ChuThichLe> : null}
        <Text style={[typography.h2, { color: colors.ink }]}>mỗi người khoảng</Text>
        {/* Four envelopes, thin to thick, or the amount typed. */}
        <View style={styles.hangPhongBi}>
          {MUC_NGAN_SACH.map((m, i) => {
            const chon = nganSach === String(m.dong);
            return (
              <PressScale
                accessibilityLabel={m.nhan}
                accessibilityRole="radio"
                accessibilityState={{ selected: chon }}
                haptic="select"
                key={m.dong}
                onPress={() => setNganSach(String(m.dong))}
                style={[styles.phongBi, { backgroundColor: chon ? colors.accentSoft : colors.paper, borderColor: chon ? colors.accent : colors.lineStrong, borderWidth: chon ? 2 : 1 }]}
              >
                <View style={[styles.napPhongBi, { borderColor: colors.lineStrong }]} />
                <View style={[styles.dayPhongBi, { height: 2 + i * 2, backgroundColor: colors.lineStrong }]} />
                <Text numberOfLines={1} style={[typography.label, { color: colors.ink }]}>{m.nhan}</Text>
              </PressScale>
            );
          })}
        </View>
        <ONhapMuc accessibilityLabel="Ô ngân sách một người" keyboardType="number-pad" label="hoặc gõ số đồng" onChangeText={setNganSach} placeholder="250000" value={nganSach} />
        {tien !== null ? <ChuThichLe icon="wallet-outline">{`= ${tien} một người, số tham chiếu chứ không phải mức trần.`}</ChuThichLe> : null}
      </View>
      {xemTruoc ? (
        // The invitation read back as the ticket the plan tab will print.
        <View style={styles.khoi} testID="xem-truoc-keo">
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Lời rủ sẽ hiện trên Lên plan</Text>
          <TheVe
            cuong={
              <>
                <Text style={[typography.stamp, styles.giua, { color: colors.inkSoft }]}>Vé</Text>
                <Text style={[typography.title, styles.giua, { color: colors.ink }]}>{headcount.trim() || "?"}</Text>
                <Text style={[typography.caption, styles.giua, { color: colors.inkSoft }]}>người</Text>
              </>
            }
          >
            <Text numberOfLines={2} style={[typography.title, { color: colors.ink }]}>{title.trim()}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{nhanKhoangNgay(ngayDi, ngayVe)}</Text>
            {tien !== null ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{tien} một người</Text> : null}
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{nhom.ten}</Text>
          </TheVe>
        </View>
      ) : null}
      {stops.length ? <View style={styles.khoi}>
        <Text style={[typography.title, { color: colors.ink }]}>Các chặng trong tờ hẹn</Text>
        <Text style={[typography.caption, { color: colors.inkSoft }]}>Các chặng gốc sẽ theo sang kèo. Hội có thể sửa giờ, đổi chỗ và sắp xếp lại trong lịch trình.</Text>
        {reviewTime ? <Text style={[typography.caption, { color: colors.inkSoft }]}>AI chưa ghi giờ theo dạng 24 giờ. Điền giờ cho mỗi chặng để hội cùng theo được.</Text> : null}
        {stops.map((stop, index) => reviewTime
          ? <Field key={`${stop.place_id}-${index}`} label={`Giờ · ${stop.label}`} value={stop.at} placeholder="18:30" onChangeText={(at) => setStops((held) => held.map((old, i) => i === index ? { ...old, at } : old))} />
          : <HangChang key={`${stop.place_id}-${index}`} gio={stop.at} tieuDe={stop.label} phac cuoi={index === stops.length - 1} />)}
      </View> : null}
      {existingId ? <View style={styles.khoi}>
        <Text style={[typography.caption, { color: colors.inkSoft }]}>Tờ hẹn này đã thành kèo. Mọi người tiếp tục sửa trên cùng một lịch trình.</Text>
        <RudiButton label="Mở kèo đã tạo" variant="outline" onPress={() => router.replace(`/outings/${existingId}` as never)} />
      </View> : null}
      {sourceFailed ? <RudiButton label="Viết kèo mới" variant="outline" onPress={() => router.replace({ pathname: "/outings/new", params: { contextId } } as never)} /> : null}
      {loi !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  screen: { maxWidth: 640 },
  khoi: { gap: 8 },
  // The invitation card: a sheet of the coral paper's card, taped at the top.
  thiep: { borderWidth: 1, padding: 16, paddingTop: 22, gap: 12 },
  washi: { position: "absolute", top: -10, alignSelf: "center", width: 96 },
  oTen: { minWidth: 180, flexGrow: 1 },
  hangLich: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 10 },
  hangSo: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 8 },
  nutSo: { width: 44, height: 44, borderRadius: 22, borderWidth: 1, alignItems: "center", justifyContent: "center" },
  oSo: { width: 64 },
  giua: { textAlign: "center" },
  hangPhongBi: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  // An envelope per budget: thicker as the amount grows.
  phongBi: { flexGrow: 1, flexBasis: 72, minHeight: 64, borderRadius: 4, alignItems: "center", justifyContent: "flex-end", paddingBottom: 14, paddingTop: 20, overflow: "hidden" },
  napPhongBi: { position: "absolute", top: -14, width: 40, height: 28, borderWidth: 1, transform: [{ rotate: "45deg" }] },
  dayPhongBi: { position: "absolute", bottom: 0, left: 0, right: 0, opacity: 0.5 },
});
