/**
 * Thả khoảnh khắc on a real session (M6): one photo, one caption, into the
 * group the session is in. The photo is shown before anything is sent; the
 * upload is App B's two requests (bytes, then the memory under its own
 * Attempt); a failure keeps the draft on screen instead of clearing it.
 *
 * UI v2 (đợt 7): the picture is shown whole (contain, never a forced square
 * crop), the caption sits under it like a line on an Instax, and the group
 * it goes to is named beside the button. The draft survives a failed send.
 */
import { useLocalSearchParams, useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Keyboard, Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc, type Attempt } from "../../../api";
import type { Phien } from "../../../phien";
import { boAnh, chonAnh, nenVaDung, type GiaiDoanTaiAnh, type TempPhoto } from "../../ky-niem/chon-anh";
import { boNhapDang, docNhapDang, ghiNhapDang, khoaNhapDang } from "../../ky-niem/nhap-dang";
import { tiLeKhung } from "../../ky-niem/ti-le";
import { laPair } from "../../nhan-rieng/nhan-rieng";
import { CAPTION_DAI_NHAT, dangAnhLenTuong } from "../../ky-niem/ky-niem";
import { bongGiay, typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { MemoryPhoto } from "../../ui/MemoryPhoto";
import { RudiIcon } from "../../ui/RudiIcon";
import { ONhapMuc } from "../../ui/ONhapMuc";

function loiRaChu(error: unknown): string {
  if (error instanceof ApiError) return error.message;
  if (error instanceof Error && error.message !== "") return error.message;
  return thongDiepNguoiDoc(0, null);
}

function cauGiaiDoan(giaiDoan: GiaiDoanTaiAnh | null): string | null {
  if (giaiDoan === "chuan-bi-anh") return "Đang chuẩn bị ảnh…";
  if (giaiDoan === "dang-gui") return "Đang tải ảnh lên…";
  return null;
}

export function ShareMomentLiveScreen({ phien }: { phien: Phien }) {
  const router = useRouter();
  const params = useLocalSearchParams<{ place?: string; ten?: string }>();
  // Mở từ màn một địa điểm thì ảnh được gắn vào chỗ ấy (M12, ADR-0017 §2.4).
  // Chỉ id đi lên máy chủ; TÊN chỗ ở đây thuần tuý để nói cho người đăng biết
  // họ đang gắn vào đâu, máy chủ tra tên của chính nó.
  const placeId = typeof params.place === "string" && params.place !== "" ? params.place : null;
  const tenCho = typeof params.ten === "string" && params.ten !== "" ? params.ten : null;
  const { colors, dark } = useRudiTheme();
  const contextId = phien.context_id;
  const nhom = phien.contexts?.find((n) => n.id === contextId);
  const laDoi = laPair(nhom);
  const tenNhom = laDoi ? `kỷ niệm của bạn và ${nhom?.display_name || "người ấy"}` : nhom?.display_name ?? "nhóm hiện tại";
  // QA UI-097: what was being posted waits here when the person steps away.
  const khoaNhap = khoaNhapDang("khoanh-khac", phien.person_id, contextId);
  const [nhapCu] = useState(() => docNhapDang(khoaNhap));
  const [anh, setAnh] = useState<TempPhoto | null>(nhapCu?.anh ?? null);
  const [caption, setCaption] = useState(nhapCu?.chu ?? "");
  const [conNhapCu, setConNhapCu] = useState(nhapCu !== null);
  const [giaiDoan, setGiaiDoan] = useState<GiaiDoanTaiAnh | null>(null);
  const [ban, setBan] = useState(false);
  const [thongBao, setThongBao] = useState<string | null>(null);
  const attempts = useRef<Record<string, Attempt>>({});
  const posting = useRef(false);
  // Leaving keeps the pick and the words as a draft; sharing, «Bỏ ảnh» or
  // «Bỏ bản nháp» is what lets the picked file go.
  const daGuiRef = useRef(false);
  useEffect(() => {
    if (!daGuiRef.current) ghiNhapDang(khoaNhap, { anh, chu: caption });
  }, [khoaNhap, anh, caption]);
  const boBanNhap = () => {
    if (anh !== null) void boAnh(anh);
    setAnh(null);
    setCaption("");
    setConNhapCu(false);
    boNhapDang(khoaNhap);
  };

  if (contextId === null) {
    return (
      <RudiScreen material="quiet" testID="share-moment-screen">
        <TopBar title="Thả khoảnh khắc" />
        <Heading title="Vào một nhóm trước" subtitle="Khoảnh khắc đăng vào tường của một nhóm; chưa có nhóm thì chưa có tường." />
        <RudiButton label="Tới Tin nhắn" onPress={() => router.push("/(tabs)/messages" as never)} variant="outline" />
      </RudiScreen>
    );
  }
  const ctx = contextId;

  const chon = async () => {
    setThongBao(null);
    try {
      const daChon = await chonAnh();
      if (daChon === null) return;
      if (anh !== null) await boAnh(anh);
      setAnh(daChon);
    } catch (error) {
      setThongBao(loiRaChu(error));
    }
  };

  const chiaSe = async () => {
    if (anh === null || posting.current) return;
    Keyboard.dismiss();
    posting.current = true;
    setBan(true);
    setThongBao(null);
    try {
      await nenVaDung(anh, (nen) => dangAnhLenTuong(ctx, nen, caption.trim() === "" ? null : caption.trim(), phien.person_id, attempts.current, placeId), setGiaiDoan);
      daGuiRef.current = true;
      boNhapDang(khoaNhap);
      router.replace(`/groups/${ctx}/wall` as never);
    } catch (error) {
      // Compression discards its own copy, while the picked original survives
      // a refused upload. Keep the preview and caption for the same retry.
      setThongBao(`${loiRaChu(error)} Ảnh và lời viết vẫn ở đây; bạn thử lại nhé.`);
    } finally {
      setGiaiDoan(null);
      setBan(false);
      posting.current = false;
    }
  };

  const conLai = CAPTION_DAI_NHAT - caption.length;
  const cauTrangThai = cauGiaiDoan(giaiDoan);

  return (
    <RudiScreen
      material="quiet"
      cot="form"
      avoidKeyboard
      contentStyle={styles.screen}
      // The one action stays on screen however tall the photo is: under the
      // print and the caption it was pushed past the fold (QA 23/09).
      footer={
        <View style={styles.footer}>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>{laDoi ? `Chỉ hai bạn · ${nhom?.display_name || "Người ấy"}` : `Thành viên nhóm · ${tenNhom}`}</Text>
          {thongBao !== null ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{thongBao}</Text> : null}
          <RudiButton disabled={ban || anh === null} lyDo={!ban && anh === null ? "Chọn ảnh trước." : undefined} icon="paper-plane-outline" label={thongBao !== null && anh !== null ? "Thử gửi lại" : "Chia sẻ"} loading={ban} onPress={() => void chiaSe()} />
          {cauTrangThai !== null ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkFaint }]}>{cauTrangThai}</Text> : null}
        </View>
      }
      footerInset={12}
      testID="share-moment-screen"
    >
      <TopBar title="Thả khoảnh khắc" />
      {conNhapCu && (anh !== null || caption.trim() !== "") ? (
        <View style={styles.nhapCu} testID="tha-khoanh-khac-nhap-cu">
          <Text style={[typography.note, styles.flexNhap, { color: colors.inkSoft }]}>Bản nháp lần trước còn đây.</Text>
          <RudiButton compact full={false} label="Bỏ bản nháp" onPress={boBanNhap} tone="warn" variant="ghost" />
        </View>
      ) : null}
      {placeId === null ? null : (
        <Text style={[typography.caption, { color: colors.inkSoft }]}>
          {`Gắn vào ${tenCho ?? "địa điểm bạn vừa mở"}: ảnh sẽ hiện ở màn chỗ đó, cho người trong nhóm.`}
        </Text>
      )}
      {/* The print: the picture whole on paper, the caption written by hand on
          its white margin (ADR-0037 D1, plan S6). An empty print is itself
          the way to pick a photo. */}
      <View style={[styles.instax, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(2, dark)]}>
        {anh === null ? (
          <Pressable accessibilityLabel="Chưa chọn ảnh, chạm để chọn" accessibilityRole="button" disabled={ban} onPress={() => void chon()} style={[styles.khungTrong, { backgroundColor: colors.ground, borderColor: colors.lineStrong }]}>
            <RudiIcon color={colors.accent} name="camera-outline" size={32} />
            <Text style={[typography.h2, { color: colors.ink }]}>Chọn một tấm ảnh</Text>
            <Text style={[typography.caption, { color: colors.inkSoft, textAlign: "center" }]}>Một cuộc đi, một điều muốn giữ lại.</Text>
          </Pressable>
        ) : (
          // The bounded paper slot shows the entire image, including extreme ratios.
          <MemoryPhoto label="Ảnh đã chọn" ratio={tiLeKhung(anh)} source={{ uri: anh.uri }} />
        )}
        <ONhapMuc
          accessibilityLabel="Ô câu chú thích"
          helper={`${conLai} ký tự còn lại`}
          maxLength={CAPTION_DAI_NHAT}
          multiline
          numberOfLines={3}
          onChangeText={setCaption}
          placeholder="Viết một câu về tấm ảnh này…"
          value={caption}
        />
      </View>
      {anh !== null ? <View style={styles.doiAnh}>
        <RudiButton compact full={false} disabled={ban} icon="images-outline" label="Đổi ảnh" onPress={() => void chon()} variant="ghost" />
        <RudiButton compact full={false} disabled={ban} label="Bỏ ảnh" onPress={() => { void boAnh(anh); setAnh(null); setThongBao(null); }} variant="ghost" />
      </View> : null}
      <Text style={[typography.note, { color: colors.inkSoft }]}>Ảnh được xoá vị trí và thông tin máy chụp trước khi lưu. Thành viên có thể chọn ảnh vào sổ chuyến đi của mình.</Text>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  nhapCu: { flexDirection: "row", alignItems: "center", gap: 8 },
  flexNhap: { flex: 1 },
  screen: { maxWidth: 640 },
  // An instax print: narrow sides, a deep bottom margin for the words.
  instax: { gap: 14, padding: 12, paddingBottom: 22, borderWidth: 1, borderRadius: 3 },
  khungTrong: { alignItems: "center", justifyContent: "center", gap: 10, padding: 24, minHeight: 180, borderWidth: 1, borderStyle: "dashed" },
  doiAnh: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  footer: { gap: 10, paddingTop: 10 },
});
