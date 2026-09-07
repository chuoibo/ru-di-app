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
import { useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../../api";
import type { Phien } from "../../../phien";
import { kiemTraTaoBuoiDi, nhanKhoangNgay } from "../../../screens/len-plan/buoi-di";
import { homNayIso, taoKeo } from "../../keo/keo";
import { typography, useRudiTheme } from "../../theme";
import { Chip, Field, Heading, Inline, RudiButton, RudiScreen, TopBar } from "../../ui";
import { dinhDangTienVnd } from "../../../screens/chat/ke-hoach";

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

export function CreateOutingLiveScreen({ phien }: { phien: Phien }) {
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

  if (phien.context_id === null) return <Redirect href="/(tabs)/plan" />;
  const contextId = phien.context_id;

  const tao = async () => {
    const kq = kiemTraTaoBuoiDi({ title, starts_on: startsOn, ends_on: endsOn, headcount, nganSach });
    if (!kq.ok) {
      setLoi(kq.loi);
      return;
    }
    if (attempt.current === null) attempt.current = newAttempt();
    setDangTao(true);
    setLoi(null);
    try {
      const keo = await taoKeo(contextId, phien.person_id, kq.body, attempt.current);
      router.replace(`/outings/${keo.id}` as never);
    } catch (error) {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
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
      footer={<RudiButton disabled={dangTao} label="Tạo kèo" loading={dangTao} onPress={() => void tao()} />}
      footerInset={Math.max(insets.bottom, 12) + 4}
      testID="create-outing-screen"
    >
      <TopBar title="Kèo mới" />
      <Heading title="Hội mình đi đâu?" subtitle={`Rủ ${nhom.ten}. Chặng và địa điểm thêm sau, trong kèo.`} />
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
      {loi !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  screen: { maxWidth: 640 },
  flex: { flex: 1 },
  khoi: { gap: 8 },
  hang: { flexDirection: "row", gap: 10 },
  xemTruoc: { gap: 4, padding: 14, borderWidth: 1, borderStyle: "dashed" },
});
