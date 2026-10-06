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
 * A large catalogue preview and medium illustrated rows; only the persisted
 * city carries the selected state. Local artwork is always labelled.
 */
import { Canh } from "../../ui/art/Canh";
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
import { AssetIllustration, type IllustrationName } from "../../asset-pack/react-native/AssetIllustration";
import { KHUNG_THANH_PHO, hinhThanhPho } from "../../art/thanh-pho";
import { VeLop } from "../../ui/art/VeLop";
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

/** These scenes illustrate the catalogue theme; they are not venue photos. */
const CANH_DIEM_DEN: Readonly<Record<string, IllustrationName>> = {
  "d-da-lat": "scene-hills",
  "d-ha-noi": "scene-street",
  "d-hoi-an": "scene-lanterns",
  "d-da-nang": "scene-coast",
  "d-nha-trang": "scene-coast",
  "d-phu-quoc": "scene-coast",
  "d-vung-tau": "scene-coast",
  "d-quy-nhon": "scene-coast",
  "d-mui-ne": "scene-coast",
};

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
  const noiNoiBat = tim.trim() === "" ? (loc.find((d) => d.id === dangChon) ?? loc[0])?.id : null;
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
            const noiBat = noiNoiBat === d.id;
            const canh = CANH_DIEM_DEN[d.id];
            const rongCanh = noiBat ? rongAnh : 96;
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
                <View style={noiBat ? styles.canhDan : styles.canhHang}>
                  <View style={[styles.canhFrame, { width: rongCanh, backgroundColor: colors.paper }]}>
                    {canh ? (
                      <AssetIllustration name={canh} width={rongCanh} height={rongCanh * 2 / 3} fit="contain" />
                    ) : (
                      <VeLop height={Math.round(rongCanh * KHUNG_THANH_PHO.h / KHUNG_THANH_PHO.w)} khungH={KHUNG_THANH_PHO.h} khungW={KHUNG_THANH_PHO.w} lop={hinhThanhPho(d.id)} width={rongCanh} />
                    )}
                  </View>
                  <Text style={[typography.caption, { color: colors.inkFaint }]}>Minh hoạ</Text>
                </View>
                <View style={[styles.theChu, !noiBat && styles.chuHang]}>
                  <Text style={[noiBat ? typography.h2 : typography.title, { color: colors.ink }]}>{d.name}</Text>
                  {dongPhuDiemDen(d) === "" ? null : (
                    <Text style={[typography.caption, { color: colors.inkSoft }]}>{dongPhuDiemDen(d)}</Text>
                  )}
                  {d.blurb === null ? null : (
                    <Text numberOfLines={2} style={[typography.note, { color: colors.inkSoft }]}>
                      {d.blurb}
                    </Text>
                  )}
                  {chonRoi ? <DauLon co="nho" nhan="Đang xem" style={styles.dauBuuDien} tilt={-8} tone="ink" /> : null}
                  {dangLuu === d.id ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Đang lưu…</Text> : null}
                </View>
              </Pressable>
            );
          })}
        </View>
      ) : null}
    </RudiScreen>
  );
}

const KHE = 10;

const styles = StyleSheet.create({
  content: { maxWidth: 560, gap: 14 },
  heading: { gap: 6 },
  luoi: { gap: KHE },
  hang: { flexDirection: "row", alignItems: "center" },
  canhDan: { alignItems: "center", gap: 2 },
  canhHang: { alignItems: "center", width: 96, gap: 2 },
  canhFrame: { borderRadius: 3, overflow: "hidden", minHeight: 64, justifyContent: "center" },
  chuHang: { flex: 1, minWidth: 0 },
  the: { borderRadius: 6, padding: 8, gap: 10, minHeight: 96, minWidth: 48 },
  theChu: { gap: 2, paddingHorizontal: 2, paddingBottom: 2 },
  dauBuuDien: { alignSelf: "flex-start", marginTop: 8 },
  bam: { opacity: 0.7 },
});
