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
import { StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../../api";
import type { Phien } from "../../../phien";
import { kiemTraTaoBuoiDi, nhanKhoangNgay } from "../../../screens/len-plan/buoi-di";
import { homNayIso, taoKeo, kiemTraChangMoi } from "../../keo/keo";
import { typography, useRudiTheme } from "../../theme";
import { Chip, Field, Heading, Inline, RudiButton, RudiScreen, TopBar } from "../../ui";
import { dinhDangTienVnd } from "../../../screens/chat/ke-hoach";
import { docAnhChupChat } from "../../chat/thay-doi";
import { docTheAi } from "../../chat/tin-song";
import type { ChangGui } from "../../../screens/len-plan/buoi-di";
import { docKeoTuChat, taoKeoTuChat } from "../../chat/ai-invocations";
import { HangChang } from "./HangChang";

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
  const [startsOn, setStartsOn] = useState(homNayIso());
  const [endsOn, setEndsOn] = useState(homNayIso());
  const [headcount, setHeadcount] = useState(nhom.soNguoi);
  const [nganSach, setNganSach] = useState("");
  const [loi, setLoi] = useState<string | null>(null);
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
    const kq = kiemTraTaoBuoiDi({ title, starts_on: startsOn, ends_on: endsOn, headcount, nganSach });
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
      router.replace(`/outings/${id}` as never);
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

  return (
    // The one decision of this screen stays above the keyboard and the gesture
    // bar however tall the form grows: at font 1.3 the button sat below the
    // fold behind the IME and the live board could not reach it (2026-09-07).
    <RudiScreen
      bottomInset={Math.max(insets.bottom, 16) + 40}
      contentStyle={styles.screen}
      footer={<RudiButton disabled={dangTao || sourceLoading || sourceFailed || existingId !== null} label={sourceMessageId ? "Xác nhận và tạo kèo" : "Tạo kèo"} loading={dangTao || sourceLoading} onPress={() => void tao()} />}
      footerInset={Math.max(insets.bottom, 12) + 4}
      testID="create-outing-screen"
    >
      <TopBar title={sourceMessageId ? "Sửa tờ hẹn" : "Kèo mới"} />
      <Heading title={sourceMessageId ? "Từ nét chì, thành lời hẹn." : "Hội mình đi đâu?"} subtitle={sourceMessageId ? `Bạn xác nhận tờ hẹn này cho ${nhom.ten}.` : `Rủ ${nhom.ten}. Chặng và địa điểm thêm sau, trong kèo.`} />
      <View style={styles.khoi}>
        <Field
          accessibilityLabel="Ô tên kèo"
          icon="flag-outline"
          label="Tên kèo"
          onChangeText={(t) => {
            setTitle(t);
            if (loi !== null) setLoi(null);
          }}
          placeholder="Ví dụ: Đà Lạt cuối tuần"
          value={title}
        />
      </View>
      <View style={styles.khoi}>
        <View style={styles.hang}>
          <View style={styles.flex}>
            <Field accessibilityLabel="Ô ngày đi" icon="calendar-outline" keyboardType="numbers-and-punctuation" label="Ngày đi" onChangeText={setStartsOn} value={startsOn} />
          </View>
          <View style={styles.flex}>
            <Field accessibilityLabel="Ô ngày về" icon="calendar-outline" keyboardType="numbers-and-punctuation" label="Ngày về" onChangeText={setEndsOn} value={endsOn} />
          </View>
        </View>
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Dạng năm-tháng-ngày, ví dụ 2026-09-20. Đi về trong ngày thì để hai ô giống nhau.</Text>
      </View>
      <View style={styles.khoi}>
        <Field accessibilityLabel="Ô số người" icon="people-outline" keyboardType="number-pad" label="Số người" onChangeText={setHeadcount} value={headcount} />
        {nhom.soNguoi ? <Text style={[typography.caption, { color: colors.inkFaint }]}>{nhom.ten} hiện có {nhom.soNguoi} người; sửa nếu chỉ một phần đi.</Text> : null}
      </View>
      <View style={styles.khoi}>
        <Field
          accessibilityLabel="Ô ngân sách một người"
          icon="wallet-outline"
          keyboardType="number-pad"
          label="Ngân sách một người (đồng)"
          onChangeText={setNganSach}
          placeholder="Ví dụ: 250000"
          value={nganSach}
        />
        <Inline gap={6} wrap>
          {MUC_NGAN_SACH.map((m) => (
            <Chip key={m.dong} label={m.nhan} onPress={() => setNganSach(String(m.dong))} selected={nganSach === String(m.dong)} />
          ))}
        </Inline>
        {tien !== null ? <Text style={[typography.caption, { color: colors.inkSoft }]}>= {tien} một người, số tham chiếu chứ không phải mức trần.</Text> : null}
      </View>
      {xemTruoc ? (
        // The invitation read back, the way the plan tab will print it.
        <View style={[styles.xemTruoc, { borderColor: colors.line, borderRadius: radius.small }]}>
          <Text style={[typography.caption, { color: colors.inkFaint }]}>Lời rủ sẽ hiện trên Lên plan</Text>
          <Text style={[typography.title, { color: colors.ink }]}>{title.trim()}</Text>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            {nhanKhoangNgay(startsOn, endsOn)}
            {headcount.trim() ? ` · ${headcount.trim()} người` : ""}
            {tien !== null ? ` · ${tien} một người` : ""}
          </Text>
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
  flex: { flex: 1 },
  khoi: { gap: 8 },
  stop: { gap: 8, paddingVertical: 12, borderTopWidth: StyleSheet.hairlineWidth },
  hang: { flexDirection: "row", gap: 10 },
  xemTruoc: { gap: 4, padding: 14, borderWidth: 1, borderStyle: "dashed" },
});
