/**
 * Chọn điểm đến (M10): which city Khám phá is showing.
 *
 * The catalogue spans fifteen destinations, so the app has to ask. The list is
 * the server's; the choice is this phone's (AsyncStorage), because it is
 * browsing state rather than a fact about the person.
 *
 * «Gần tôi» is not on this screen yet: reading the device's position needs a
 * native module and a permission, and that arrives with its own build and its
 * own explanation screen (ADR-0018). What is here is the honest half -- pick a
 * city by name -- and it works with no permission at all.
 *
 * A large catalogue preview and equal list rows; only the persisted city
 * carries the selected state. The featured card shows the destination's real
 * cover photo (with the credit that came with it) through MediaSlot, or its
 * drawn scene with no label. Every list row draws the same line sketch in the
 * same frame, with room for a two-line subtitle, so 33 rows scan at one height
 * (finish review 21/10).
 */
import { Canh } from "../../ui/art/Canh";
import { Ionicons } from "@expo/vector-icons";
import { useRouter } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import {
  docDiemDen,
  docDiemDenDaChon,
  dongPhuDiemDen,
  luuDiemDen,
  type DiemDen,
} from "../../kham-pha/diem-den";
import { AssetIllustration } from "../../asset-pack/react-native/AssetIllustration";
import { canhDiemDen } from "../../kham-pha/canh-ve";
import { KHUNG_THANH_PHO, hinhThanhPho } from "../../art/thanh-pho";
import { VeLop } from "../../ui/art/VeLop";
import { MediaSlot } from "../../ui/MediaSlot";
import { bongGiay, typography, useRudiTheme } from "../../theme";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { DauLon } from "../../ui/DauLon";
import { RudiScreen, SearchField, TopBar } from "../../ui";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { giuState } from "../../../ui/a11y";
import { luiVeVe } from "../../lui-ve";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; ds: DiemDen[] }
  | { pha: "hong"; loi: string };

function khongDau(s: string): string {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/gi, "d").toLowerCase();
}

/** Regional subtitle when the catalogue sends a province with `province === name` and `blurb === null`. */
const MO_TA_VUNG: Readonly<Record<string, string>> = {
  "d-tinh-1": "Đồng bằng sông Hồng · Phố cổ & hồ",
  "d-tinh-4": "Đông Bắc · Núi đá & thác bản",
  "d-tinh-8": "Đông Bắc · Sông núi & hồ sinh thái",
  "d-tinh-11": "Tây Bắc · Thung lũng & đèo cao",
  "d-tinh-12": "Tây Bắc · Đỉnh mây & bản làng",
  "d-tinh-14": "Tây Bắc · Cao nguyên & đồi chè",
  "d-tinh-15": "Tây Bắc · Sa Pa & ruộng bậc thang",
  "d-tinh-19": "Trung du Bắc Bộ · Đồi chè & hồ núi",
  "d-tinh-20": "Đông Bắc · Cửa khẩu & núi đá vôi",
  "d-tinh-22": "Đông Bắc · Vịnh Hạ Long & đảo biển",
  "d-tinh-25": "Trung du Bắc Bộ · Đền Hùng & đồi cọ",
  "d-tinh-31": "Duyên hải Bắc Bộ · Phố cảng & đảo Cát Bà",
  "d-tinh-33": "Đồng bằng sông Hồng · Phố Hiến & làng cổ",
  "d-tinh-37": "Đồng bằng sông Hồng · Tràng An & non nước",
  "d-tinh-38": "Bắc Trung Bộ · Biển Sầm Sơn & Pù Luông",
  "d-tinh-40": "Bắc Trung Bộ · Biển Cửa Lò & miền Tây xứ Nghệ",
  "d-tinh-42": "Bắc Trung Bộ · Biển Thiên Cầm & núi Hồng",
  "d-tinh-44": "Bắc Trung Bộ · Di tích & biển Cửa Việt",
  "d-tinh-46": "Miền Trung · Cố đô & sông Hương",
  "d-tinh-48": "Miền Trung · Cầu sông Hàn & bán đảo Sơn Trà",
  "d-tinh-51": "Duyên hải Nam Trung Bộ · Đảo Lý Sơn & biển xanh",
  "d-tinh-52": "Tây Nguyên & duyên hải · Biển hồ & ghềnh đá",
  "d-tinh-56": "Nam Trung Bộ · Vịnh Nha Trang & đảo biển",
  "d-tinh-66": "Tây Nguyên · Buôn Ma Thuột & thủ phủ cà phê",
  "d-tinh-68": "Tây Nguyên · Đà Lạt, đồi thông & sương mù",
  "d-tinh-75": "Đông Nam Bộ · Rừng Nam Cát Tiên & hồ Trị An",
  "d-tinh-79": "Đông Nam Bộ · Nhịp sống đô thị & ẩm thực",
  "d-tinh-80": "Đông Nam Bộ · Núi Bà Đen & hồ Dầu Tiếng",
  "d-tinh-82": "Đồng bằng sông Cửu Long · Mùa sen & làng hoa",
  "d-tinh-86": "Đồng bằng sông Cửu Long · Cù lao & miệt vườn",
  "d-tinh-91": "Đồng bằng sông Cửu Long · Thất Sơn & đảo Phú Quốc",
  "d-tinh-92": "Đồng bằng sông Cửu Long · Bến Ninh Kiều & chợ nổi",
  "d-tinh-96": "Đồng bằng sông Cửu Long · Đất Mũi & rừng ngập mặn",
};

/**
 * The featured card's drawing where there is no photograph: the destination's
 * own painted scene when one paints that region (`canhDiemDen`), else its line
 * sketch. Never another city's landmark (Hội An's lanterns stood for Hà Nội).
 */
function tranhCanh(d: DiemDen, rongCanh: number) {
  const canh = canhDiemDen(d.id);
  return canh ? (
    <AssetIllustration name={canh} width={rongCanh} height={rongCanh * 2 / 3} fit="contain" />
  ) : (
    <ThanhPhoNet id={d.id} width={rongCanh} />
  );
}

/** One drawn register for every list thumbnail: the destination's line sketch, in the same frame. */
function ThanhPhoNet({ id, width }: { id: string; width: number }) {
  return <VeLop height={Math.round(width * KHUNG_THANH_PHO.h / KHUNG_THANH_PHO.w)} khungH={KHUNG_THANH_PHO.h} khungW={KHUNG_THANH_PHO.w} lop={hinhThanhPho(id)} width={width} />;
}

export function DiemDenScreen() {
  const router = useRouter();
  const { colors, dark } = useRudiTheme();
  const [rongLuoi, setRongLuoi] = useState(0);
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [tim, setTim] = useState("");
  const [dangChon, setDangChon] = useState<string | null>(null);
  const [dangLuu, setDangLuu] = useState<string | null>(null);
  const [loiLuu, setLoiLuu] = useState<string | null>(null);
  const luuLock = useRef(false);

  const nap = useCallback(async () => {
    setTrang({ pha: "dang-doc" });
    try {
      const [ds, daChon] = await Promise.all([docDiemDen(), docDiemDenDaChon()]);
      setDangChon(daChon);
      setTrang({ pha: "xong", ds: ds.diemDen });
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    }
  }, []);

  useEffect(() => {
    void nap();
  }, [nap]);

  const chon = async (diemDen: DiemDen) => {
    if (luuLock.current) return;
    luuLock.current = true;
    setDangLuu(diemDen.id);
    setLoiLuu(null);
    try {
      await luuDiemDen(diemDen.id);
      // The destination is already persisted before returning. Reordering the
      // native postcard children in the same frame as the route pop can make
      // Fabric mount a clipped child twice; the next visit reads the new key.
      luiVeVe(router as never, "/explore");
    } catch {
      setLoiLuu("Chưa lưu được điểm đến trên máy này. Chọn lại để thử lần nữa.");
    } finally {
      luuLock.current = false;
      setDangLuu(null);
    }
  };

  const loc =
    trang.pha === "xong"
      ? trang.ds.filter((d) => {
          const q = khongDau(tim.trim());
          if (q === "") return true;
          return khongDau(`${d.name} ${d.province ?? ""}`).includes(q);
        })
      : [];
  // With no saved choice the API's first catalogue row is its default. It can
  // lead the sheet without writing a choice or falsely marking it selected.
  const noiNoiBat = tim.trim() === "" ? (loc.find((d) => d.id === dangChon) ?? loc[0]) : null;
  const rongAnh = Math.max(120, Math.min(544, rongLuoi - 16));

  return (
    <RudiScreen contentStyle={styles.content} testID="diem-den-screen">
      <TopBar title="Đi đâu?" />
      <View style={styles.heading}>
        <Text style={[typography.h1, { color: colors.ink }]}>Bạn muốn đi đâu?</Text>
        <Text style={[typography.body, { color: colors.inkSoft }]}>Chọn một thành phố để khám phá.</Text>
      </View>
      <SearchField
        onChangeText={setTim}
        placeholder="Tìm thành phố hoặc tỉnh"
        value={tim}
      />
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup>
          <SkeletonRow leading={0} />
          <SkeletonRow leading={0} />
          <SkeletonRow leading={0} />
          <SkeletonRow leading={0} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách nơi đến" /> : null}
      {trang.pha === "xong" && loc.length === 0 ? (
        <EmptyState
          action={{ label: "Xóa ô tìm", onPress: () => setTim("") }}
          // The count comes from the list: a number written into the sentence
          // said «mười lăm» long after the catalogue had 48 (2026-09-23).
          body={`Rủ Đi mới biết ${trang.ds.length.toLocaleString("vi-VN")} nơi. Thử tên khác, hoặc xoá ô tìm để xem hết.`}
          kind="no-results"
          layout="inline"
          illustration={<Canh id="bo-loc-che-het" width={168} />} title="Chưa có nơi nào khớp"
        />
      ) : null}
      <CauTaiCho cau={loiLuu} />
      {loc.length > 0 ? (
        // One selected postcard, then compact rows with the catalogue city marks.
        <View onLayout={(e) => setRongLuoi(Math.round(e.nativeEvent.layout.width))} style={styles.luoi}>
          {[...loc].sort((a, b) => Number(b.id === dangChon) - Number(a.id === dangChon)).map((d) => {
            const chonRoi = dangChon === d.id;
            const noiBat = noiNoiBat?.id === d.id;
            const dongPhu = dongPhuDiemDen(d) || MO_TA_VUNG[d.id] || "";
            const rongCanh = noiBat ? rongAnh : RONG_THU;
            return (
              <Pressable
                accessibilityLabel={`Chọn ${d.name}`}
                accessibilityRole="button"
                disabled={dangLuu !== null}
                {...giuState(chonRoi)}
                accessibilityState={{ selected: chonRoi, disabled: dangLuu !== null, busy: dangLuu === d.id }}
                key={d.id}
                onPress={() => void chon(d)}
                style={({ pressed }) => [
                  styles.the,
                  !noiBat && styles.hang,
                  { backgroundColor: colors.card, borderColor: chonRoi ? colors.accent : colors.line, borderWidth: chonRoi ? 2 : 1 },
                  noiBat && bongGiay(1, dark),
                  pressed && styles.bam,
                ]}
              >
                {noiBat ? (
                  // The featured card is the one place a photograph is shown, with its credit under it.
                  <View style={styles.canhDan}>
                    {d.anhBia !== null ? (
                      <View style={{ width: rongCanh }}>
                        <MediaSlot alt={`Ảnh ${d.name}`} fallback={tranhCanh(d, rongCanh)} nguon={{ loai: "danh-muc", anh: d.anhBia }} ratio={3 / 2} />
                      </View>
                    ) : (
                      <View style={[styles.canhFrame, { width: rongCanh, backgroundColor: colors.paper, borderColor: colors.line }]}>{tranhCanh(d, rongCanh)}</View>
                    )}
                  </View>
                ) : (
                  // Every list row draws the same register in the same frame, photo or not:
                  // a thumbnail cannot carry a credit whole, so the photograph waits for the
                  // featured card, where it is shown with its credit (ADR-0017 §2.5).
                  <View style={[styles.canhFrame, styles.thuHang, { backgroundColor: colors.paper, borderColor: colors.line }]}>
                    <ThanhPhoNet id={d.id} width={RONG_THU} />
                  </View>
                )}
                <View style={[styles.theChu, !noiBat && styles.chuHang]}>
                  <Text style={[noiBat ? typography.h2 : typography.title, { color: colors.ink }]}>{d.name}</Text>
                  {dongPhu === "" ? null : (
                    // Never clipped: a row reserves two lines so rows keep one height, and longer copy (320 dp, large text) wraps on.
                    <Text style={[typography.caption, { color: colors.inkSoft }, !noiBat && { minHeight: typography.caption.lineHeight * 2 }]}>{dongPhu}</Text>
                  )}
                  {/* A row has room for one quiet line: the subtitle, or the blurb when there is none. */}
                  {d.blurb === null || (!noiBat && dongPhu !== "") ? null : (
                    <Text numberOfLines={noiBat ? 2 : undefined} style={[typography.note, { color: colors.inkSoft }, !noiBat && { minHeight: typography.note.lineHeight * 2 }]}>
                      {d.blurb}
                    </Text>
                  )}
                  {chonRoi ? <DauLon co="nho" nhan="Đang xem" style={styles.dauBuuDien} tilt={-8} tone="ink" /> : null}
                  {dangLuu === d.id ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Đang lưu…</Text> : null}
                </View>
                {!noiBat ? (
                  <Ionicons
                    color={chonRoi ? colors.accent : colors.inkFaint}
                    importantForAccessibility="no"
                    name={chonRoi ? "checkmark-circle" : "chevron-forward"}
                    size={18}
                  />
                ) : null}
              </Pressable>
            );
          })}
        </View>
      ) : null}
    </RudiScreen>
  );
}

const KHE = 10;
/** The list thumbnail: one size for every row (the city sketch's 288:120 frame at 96 wide). */
const RONG_THU = 96;

const styles = StyleSheet.create({
  content: { maxWidth: 560, gap: 14 },
  heading: { gap: 6 },
  luoi: { gap: KHE },
  hang: { flexDirection: "row", alignItems: "center" },
  canhDan: { alignItems: "center", gap: 2 },
  thuHang: { width: RONG_THU, height: 64 },
  canhFrame: { borderRadius: 4, borderWidth: StyleSheet.hairlineWidth, overflow: "hidden", minHeight: 64, justifyContent: "center" },
  chuHang: { flex: 1, minWidth: 0, gap: 3 },
  the: { borderRadius: 8, padding: 10, gap: 12, minHeight: 84, minWidth: 48 },
  theChu: { gap: 3, paddingHorizontal: 2, paddingBottom: 2 },
  dauBuuDien: { alignSelf: "flex-start", marginTop: 8 },
  bam: { opacity: 0.7 },
});
