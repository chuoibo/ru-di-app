import { Redirect } from "expo-router";
import { useState } from "react";
import { Text, View } from "react-native";

import type { BoiCanh } from "../../src/rudi/ai/boi-canh";
import { TRA_LOI_DAU, buocTraLoi, type TraLoiSong } from "../../src/rudi/ai/tra-loi-song";
import type { SuKienSSE } from "../../src/rudi/ai/sse";
import type { AiInvocation, ChatCapabilities } from "../../src/rudi/chat/ai-invocations";
import type { Tin } from "../../src/rudi/chat/tin-song";
import { CUA_FIXTURE_DEV } from "../../src/rudi/cua-fixture";
import { ChipBoiCanh } from "../../src/rudi/screens/chat/ChipBoiCanh";
import { KhaySticker } from "../../src/rudi/screens/chat/KhaySticker";
import { CongCuChat, type KhayChat } from "../../src/rudi/screens/chat/SoHen";
import { HangTraLoiAiDangViet } from "../../src/rudi/screens/chat/TraLoiAiDangViet";
import { HangLoiDeNghi, HangToGiay } from "../../src/rudi/screens/hai-nguoi/HangToGiay";
import { typography, useRudiTheme } from "../../src/rudi/theme";
import { Chip, Heading, Inline, RudiScreen, SectionHeader, TopBar } from "../../src/rudi/ui";

/**
 * Lab board for the two classes of a two-person chat (owner model 2026-09-28):
 * a friends' pair («đám bạn», the same tools as a group, worded for two) and
 * a couple («cặp đôi», `cap_doi`: both turned «Một đôi» on, which adds the
 * pinned paper row, the fifth tray tool «Tờ giấy» and the stickers for two).
 *
 * Every surface is the component the chat screen ships, fed with invented
 * data («Linh», «Minh»); no server, no session. The tray is drawn full width
 * under the scroll, where `GroupChatLive` puts it, so the five-tool row wraps
 * (or not) at the real window width. Never in a store build: gated exactly as
 * the other lab pages are, by `CUA_FIXTURE_DEV`.
 */
const TOI = "lab-linh";
const MINH = "lab-minh";
/** The reader is Linh, so her own message is quoted as «Bạn», as in the chat. */
const ten = (id: string | null) => (id === MINH ? "Minh" : "Bạn");

/** A server that can run `/plan` now: the tray offers «Hỏi Rủ Đi AI». */
const SAN_SANG: ChatCapabilities = {
  protocol: "legacy",
  realtime: { available: true },
  ai: {
    plan: { available: true, reason: null },
    chia_bill: { available: true, reason: null },
    hoi: { available: true, reason: null },
    share_scope: "caller_attached",
    mention: true,
  },
  media: { image: true, sticker: true, voice: false },
};

const GOI: BoiCanh = {
  ban: 1,
  nguon: "chat-nhom",
  tongLuot: 3,
  daCat: false,
  luot: [
    { id: "lab-1", vai: "ban", biDanh: "Minh", loai: "chu", luc: "2026-09-28T11:02:00Z", chu: "Cuối tuần này rảnh không?" },
    { id: "lab-2", vai: "toi", loai: "chu", luc: "2026-09-28T11:03:00Z", chu: "Rảnh chiều thứ bảy." },
    { id: "lab-3", vai: "ban", biDanh: "Minh", loai: "sticker", luc: "2026-09-28T11:04:00Z", chu: "Sticker" },
  ],
};

const HOI: AiInvocation = {
  id: "lab-hai-inv",
  status: "running",
  code: null,
  message_id: null,
  trigger_message_id: "lab-hai-trigger",
  so_tin_doc: 3,
  created_at: "2026-09-28T11:05:00Z",
  updated_at: "2026-09-28T11:05:00Z",
};
const TIN_NHO: Tin = {
  id: "lab-hai-trigger",
  context_id: "lab-hai",
  author_id: TOI,
  kind: "text",
  body: "@Rủ Đi chiều thứ bảy hai mình đi đâu gần hồ?",
  image_url: null,
  card: null,
  created_at: "2026-09-28T11:05:00Z",
  cursor: "lab",
};
const SU: SuKienSSE[] = [
  { id: "1-0", loai: "trang_thai", data: { cau: "dang_nghi" } },
  { id: "2-0", loai: "delta", data: { p: 0, text: "Chiều thứ bảy hồ lộng gió, " } },
  { id: "2-1", loai: "delta", data: { p: 0, text: "hai bạn đi bộ một vòng hồ. " } },
  { id: "2-2", loai: "delta", data: { p: 0, text: "Rồi ghé quán " } },
];
const DANG_VIET = SU.reduce<TraLoiSong>(buocTraLoi, TRA_LOI_DAU);

type LopKhay = "ban" | "doi" | null;

export default function HaiLopChatLab() {
  const { colors } = useRudiTheme();
  const [lop, datLop] = useState<LopKhay>(null);
  const [panel, datPanel] = useState<KhayChat>(null);
  const [kemTin, datKemTin] = useState(true);
  const [sticker, datSticker] = useState<LopKhay>(null);
  const [giam, datGiam] = useState(false);
  if (!CUA_FIXTURE_DEV) return <Redirect href="/welcome" />;

  const khung = { borderRadius: 14, borderWidth: 1, borderColor: colors.line, backgroundColor: colors.paper, overflow: "hidden" as const };
  const ghiChu = (chu: string) => <Text style={{ ...typography.note, color: colors.inkFaint }}>{chu}</Text>;
  const moKhay = (l: Exclude<LopKhay, null>) => {
    datLop(l);
    datPanel("tools");
  };
  return (
    <View style={{ flex: 1, backgroundColor: colors.ground }}>
      <RudiScreen>
        <TopBar title="Hai lớp chat hai người" />
        <Heading title="Dữ liệu tổng hợp" subtitle="Đám bạn và cặp đôi, vẽ bằng đúng thành phần của màn chat." />

        <SectionHeader title="Khay công cụ" />
        {ghiChu("Khay mở ở đáy màn, rộng bằng cửa sổ như trong chat thật.")}
        <Inline gap={8} wrap>
          <Chip label="Khay đám bạn" onPress={() => moKhay("ban")} selected={lop === "ban"} accessibilityLabel="Khay đám bạn" />
          <Chip label="Khay cặp đôi" onPress={() => moKhay("doi")} selected={lop === "doi"} accessibilityLabel="Khay cặp đôi" />
          <Chip label="Bảng Tờ hẹn" onPress={() => { datLop((l) => l ?? "ban"); datPanel("plan"); }} selected={panel === "plan"} accessibilityLabel="Bảng Tờ hẹn" />
          <Chip label="Bảng bình chọn" onPress={() => { datLop((l) => l ?? "ban"); datPanel("poll"); }} selected={panel === "poll"} accessibilityLabel="Bảng bình chọn" />
        </Inline>

        <SectionHeader title="Hàng mời · đám bạn" />
        {ghiChu("Minh đề nghị, Linh chưa trả lời: hàng mỏng dưới tiêu đề chat.")}
        <View style={khung} testID="lab-hang-loi-moi">
          <HangLoiDeNghi deNghi={{ purpose: "lap_so", ten: "Minh" }} onPress={() => {}} />
          <HangLoiDeNghi deNghi={{ purpose: "bat_doi", ten: "Minh" }} onPress={() => {}} />
        </View>

        <SectionHeader title="Hàng ghim Tờ giấy · cặp đôi" />
        {ghiChu("Tuần yên, rồi khi Minh có đề nghị đang chờ Linh.")}
        <View style={khung} testID="lab-hang-to-giay">
          <HangToGiay onPress={() => {}} tenNguoiKia="Minh" toMo={undefined} toiId={TOI} testID="lab-to-giay-yen" />
          <HangToGiay deNghiDenToi={{ purpose: "lap_so", ten: "Minh" }} onPress={() => {}} tenNguoiKia="Minh" toMo={undefined} toiId={TOI} testID="lab-to-giay-de-nghi" />
        </View>

        <SectionHeader title="Chip trên nút gửi · hai người" />
        {ghiChu("Sẵn sàng (bấm «Xem» mở bảng), rồi AI chưa sẵn sàng.")}
        <View style={{ gap: 8 }} testID="lab-chip">
          <ChipBoiCanh goi={GOI} haiNguoi kemTin={kemTin} onDoi={datKemTin} sanSang />
          <ChipBoiCanh goi={GOI} haiNguoi kemTin onDoi={() => {}} sanSang={false} />
          <ChipBoiCanh goi={{ ...GOI, luot: [], tongLuot: 0 }} haiNguoi kemTin onDoi={() => {}} sanSang />
        </View>

        <SectionHeader title="Câu trả lời đang tới" />
        <Inline gap={8} wrap>
          <Chip label="Reduce Motion" onPress={() => datGiam((g) => !g)} selected={giam} accessibilityLabel="Reduce Motion" />
        </Inline>
        {ghiChu("Linh hỏi: đang đọc, rồi đang viết.")}
        <View style={{ gap: 8 }} testID="lab-tra-loi">
          <HangTraLoiAiDangViet daCoThe={() => false} giamChuyenDong={giam} request={HOI} tenNguoi={ten} traLoi={TRA_LOI_DAU} trigger={TIN_NHO} />
          <HangTraLoiAiDangViet daCoThe={() => false} giamChuyenDong={giam} request={HOI} tenNguoi={ten} traLoi={DANG_VIET} trigger={TIN_NHO} />
        </View>

        <SectionHeader title="Khay sticker" />
        <Inline gap={8} wrap>
          <Chip label="Sticker đám bạn" onPress={() => datSticker("ban")} selected={sticker === "ban"} accessibilityLabel="Sticker đám bạn" />
          <Chip label="Sticker cặp đôi" onPress={() => datSticker("doi")} selected={sticker === "doi"} accessibilityLabel="Sticker cặp đôi" />
        </Inline>
      </RudiScreen>

      {lop !== null ? (
        <View testID={lop === "doi" ? "lab-khay-doi" : "lab-khay-ban"}>
          <CongCuChat
            busy={false}
            capabilities={SAN_SANG}
            contextId={`lab-hai-${lop}`}
            error={null}
            haiNguoi
            onHoiAi={() => {}}
            onImage={() => {}}
            onManual={() => {}}
            onPanel={(p) => {
              datPanel(p);
              if (p === null) datLop(null);
            }}
            onPoll={async () => true}
            onSticker={() => {}}
            onToGiay={lop === "doi" ? () => {} : undefined}
            panel={panel}
            personId={TOI}
          />
        </View>
      ) : null}
      <KhaySticker capDoi={sticker === "doi"} onChon={() => datSticker(null)} onClose={() => datSticker(null)} open={sticker !== null} />
    </View>
  );
}
