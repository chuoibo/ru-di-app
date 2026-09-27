import { Redirect } from "expo-router";
import { useEffect, useState } from "react";
import { Text, View } from "react-native";

import { TRA_LOI_DAU, buocTraLoi, type TraLoiSong } from "../../src/rudi/ai/tra-loi-song";
import type { SuKienSSE } from "../../src/rudi/ai/sse";
import type { AiInvocation } from "../../src/rudi/chat/ai-invocations";
import type { Tin } from "../../src/rudi/chat/tin-song";
import { CUA_FIXTURE_DEV } from "../../src/rudi/cua-fixture";
import { NepPhien } from "../../src/rudi/nep/NepPhien";
import { HangTraLoiAiDangViet } from "../../src/rudi/screens/chat/TraLoiAiDangViet";
import { typography, useRudiTheme } from "../../src/rudi/theme";
import { Chip, Heading, Inline, RudiScreen, SectionHeader, TopBar } from "../../src/rudi/ui";

/**
 * Lab board for slice 11: the streamed answer in each of its states, drawn by
 * the components that ship (`NepPhien`, `HangTraLoiAiDangViet`) from states
 * built by the same reducer the screens run (`buocTraLoi`). Invented words,
 * no server. «Chạy thử» replays a whole answer at the server's pace (16 runes
 * every 25 ms) so the growing text can be watched. Never in a store build.
 */
const CAU_HOI = "Tối nay đi đâu ngắm đèn?";
const TRA_LOI =
  "Hồ Xuân Hương lên đèn từ khoảng 18 giờ. Đi bộ một vòng mất tầm bốn mươi phút, nên ra sớm cho kịp trời còn chút sáng.";

const su = (id: string, loai: SuKienSSE["loai"], data: unknown): SuKienSSE => ({ id, loai, data });
const gop = (ds: SuKienSSE[]) => ds.reduce<TraLoiSong>(buocTraLoi, TRA_LOI_DAU);

/** The answer cut into the pieces the server would release: 16 runes, never mid-word. */
function catNhip(chu: string): string[] {
  const out: string[] = [];
  let tu = 0;
  const rune = [...chu];
  while (tu < rune.length) {
    let den = Math.min(rune.length, tu + 16);
    while (den < rune.length && rune[den - 1] !== " ") den++;
    out.push(rune.slice(tu, den).join(""));
    tu = den;
  }
  return out;
}

const NHIP = catNhip(TRA_LOI);
const DANG_NGHI = gop([su("1-0", "trang_thai", { cau: "dang_nghi" })]);
const DANG_VIET_THAT = NHIP.slice(0, 4).reduce<TraLoiSong>((s, t, i) => buocTraLoi(s, su(`2-${i}`, "delta", { p: 0, text: t })), DANG_NGHI);

const HOI_NHOM: AiInvocation = {
  id: "lab-inv",
  status: "running",
  code: null,
  message_id: null,
  trigger_message_id: "lab-trigger",
  so_tin_doc: 6,
  created_at: "2026-09-27T12:00:00Z",
  updated_at: "2026-09-27T12:00:00Z",
};
const TIN_NHAC: Tin = {
  id: "lab-trigger",
  context_id: "lab",
  author_id: "lab-toi",
  kind: "text",
  body: "@Rủ Đi tối nay ăn gì gần hồ?",
  image_url: null,
  card: null,
  created_at: "2026-09-27T12:00:00Z",
  cursor: "lab",
};

export default function TraLoiSongLab() {
  const { colors } = useRudiTheme();
  const [giam, datGiam] = useState(false);
  const [chay, datChay] = useState<TraLoiSong | null>(null);
  useEffect(() => {
    if (chay === null || chay.pha === "xong") return;
    const i = chay.pha === "dang_viet" ? Number((chay.idCuoi ?? "0-0").split("-")[1]) + 1 : 0;
    const h = setTimeout(() => {
      datChay((s) =>
        s === null
          ? s
          : i < NHIP.length
            ? buocTraLoi(s, su(`9-${i}`, "delta", { p: 0, text: NHIP[i] }))
            : buocTraLoi(s, su(`9-${i}`, "xong", { text: TRA_LOI, chips: ["Mở bản đồ"], nguon: [] })),
      );
    }, i === 0 ? 1200 : 25);
    return () => clearTimeout(h);
  }, [chay]);
  if (!CUA_FIXTURE_DEV) return <Redirect href="/welcome" />;

  const khung = { borderRadius: 14, borderWidth: 1, borderColor: colors.line, backgroundColor: colors.paper, padding: 16 } as const;
  const ten = () => "Bạn";
  return (
    <RudiScreen>
      <TopBar title="Chữ AI hiện dần" />
      <Heading title="Dữ liệu tổng hợp" subtitle="Ba trạng thái của một câu trả lời, vẽ bằng đúng thành phần app dùng." />
      <Inline gap={8} wrap>
        <Chip label="Reduce Motion" onPress={() => datGiam((g) => !g)} selected={giam} accessibilityLabel="Reduce Motion" />
        <Chip label="Chạy thử" onPress={() => datChay(DANG_NGHI)} selected={chay !== null} accessibilityLabel="Chạy thử" />
      </Inline>

      {chay ? (
        <>
          <SectionHeader title="Bảng Nếp · chạy thử" />
          <View style={khung} testID="lab-nep-chay">
            <NepPhien cauDangHoi={chay.pha === "xong" ? null : CAU_HOI} chips={chay.pha === "xong" ? ["Mở bản đồ"] : []} dangDo={null} dangHoi={chay.pha !== "xong"} giamChuyenDong={giam} luot={chay.pha === "xong" ? [{ vai: "toi", chu: CAU_HOI }, { vai: "nep", chu: TRA_LOI }] : []} onChip={() => {}} song={chay.pha === "xong" ? null : chay} />
          </View>
        </>
      ) : null}

      <SectionHeader title="Bảng Nếp · đang nghĩ" />
      <View style={khung} testID="lab-nep-dang-nghi">
        <NepPhien cauDangHoi={CAU_HOI} chips={[]} dangDo={null} dangHoi giamChuyenDong={giam} luot={[]} onChip={() => {}} song={DANG_NGHI} />
      </View>

      <SectionHeader title="Bảng Nếp · đang viết" />
      <View style={khung} testID="lab-nep-dang-viet">
        <NepPhien cauDangHoi={CAU_HOI} chips={[]} dangDo={null} dangHoi giamChuyenDong={giam} luot={[]} onChip={() => {}} song={DANG_VIET_THAT} />
      </View>

      <SectionHeader title="Bảng Nếp · xong" />
      <View style={khung} testID="lab-nep-xong">
        <NepPhien cauDangHoi={null} chips={["Mở bản đồ", "Gợi ý quán gần hồ"]} dangDo={null} dangHoi={false} giamChuyenDong={giam} luot={[{ vai: "toi", chu: CAU_HOI }, { vai: "nep", chu: TRA_LOI }]} onChip={() => {}} song={null} />
      </View>

      <SectionHeader title="Luồng nhóm · người hỏi" />
      <Text style={{ ...typography.note, color: colors.inkFaint }}>Đang đọc, rồi đang viết. Khi thẻ thật tới, hàng này nhường chỗ.</Text>
      <View style={{ gap: 8 }} testID="lab-nhom">
        <HangTraLoiAiDangViet daCoThe={() => false} giamChuyenDong={giam} request={HOI_NHOM} tenNguoi={ten} traLoi={TRA_LOI_DAU} trigger={TIN_NHAC} />
        <HangTraLoiAiDangViet daCoThe={() => false} giamChuyenDong={giam} request={HOI_NHOM} tenNguoi={ten} traLoi={DANG_VIET_THAT} trigger={TIN_NHAC} />
      </View>
    </RudiScreen>
  );
}
