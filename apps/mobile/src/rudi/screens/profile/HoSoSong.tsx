/**
 * The profile card for a real session (M2): what `GET /people/me` says.
 *
 * Name, bio and city are the person's own words; the five numbers are the
 * server's counts, each from the table that owns it (friends, active groups,
 * outings of those groups, distinct stops checked in at, memories authored).
 * Nothing on this card is derived on the phone, and nothing comes from the
 * fixture -- which is the whole reason it exists: the fixture hero showed
 * «Minh Anh · Cấp 12» to whoever signed in.
 *
 * Editing goes through `PATCH /people/me` and the card re-reads the server's
 * answer rather than trusting the form.
 *
 * UI v2 (đợt 7): a footprint, not a trophy page -- initial, name, one line,
 * and the five counts as one sentence on the paper. The form sits on the
 * page too, with the error next to the field it is about.
 */
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import { docHoSoToi, doiTenTrongPhien, suaHoSoToi, type HoSoToi, type Phien } from "../../../phien";
import { useRudiSession } from "../../session";
import { bongGiay, typography, useRudiTheme } from "../../theme";
import { RudiButton } from "../../ui";
import { DauLon } from "../../ui/DauLon";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { Ionicons } from "@expo/vector-icons";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; hoSo: HoSoToi }
  | { pha: "hong"; loi: string };

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

const NHAN_CUA: Record<string, string> = { phone: "số điện thoại", google: "Google" };

export function HoSoSong({ phien }: { phien: Phien }) {
  const { datPhien } = useRudiSession();
  const { colors, dark } = useRudiTheme();
  const router = useRouter();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangSua, setDangSua] = useState(false);
  const [ten, setTen] = useState("");
  const [bio, setBio] = useState("");
  const [city, setCity] = useState("");
  const [dangLuu, setDangLuu] = useState(false);
  const [loiLuu, setLoiLuu] = useState<string | null>(null);

  const nap = useCallback(async () => {
    try {
      setTrang({ pha: "xong", hoSo: await docHoSoToi(phien.person_id) });
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [phien.person_id]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  const moSua = (hoSo: HoSoToi) => {
    setTen(hoSo.display_name);
    setBio(hoSo.bio ?? "");
    setCity(hoSo.city ?? "");
    setLoiLuu(null);
    setDangSua(true);
  };

  const luu = async () => {
    if (ten.trim() === "") {
      setLoiLuu("Tên hiển thị không được rỗng.");
      return;
    }
    setDangLuu(true);
    try {
      const hoSo = await suaHoSoToi(phien.person_id, { display_name: ten.trim(), bio, city });
      setTrang({ pha: "xong", hoSo });
      // Keep the session's greeting in step with the server (QA 23/09).
      datPhien(await doiTenTrongPhien(phien, hoSo.display_name));
      setDangSua(false);
    } catch (error) {
      setLoiLuu(loiRaChu(error));
    } finally {
      setDangLuu(false);
    }
  };

  if (trang.pha === "dang-doc") {
    return (
      <SkeletonGroup>
        <SkeletonRow leading={64} />
      </SkeletonGroup>
    );
  }
  if (trang.pha === "hong") {
    return <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được hồ sơ" />;
  }
  const { hoSo } = trang;

  if (dangSua) {
    return (
      <View style={styles.form}>
        <Text style={[typography.h2, { color: colors.ink }]}>Chỉnh hồ sơ</Text>
        {/* Edited on the passport page itself: its lines are pen lines. */}
        <ONhapMuc accessibilityLabel="Ô tên hiển thị" label="Tên" maxLength={200} onChangeText={setTen} value={ten} />
        {loiLuu ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loiLuu}</Text> : null}
        <ONhapMuc
          accessibilityLabel="Ô giới thiệu"
          label="Giới thiệu"
          maxLength={500}
          multiline
          onChangeText={setBio}
          placeholder="Vài chữ về bạn"
          value={bio}
        />
        <ONhapMuc
          accessibilityLabel="Ô thành phố"
          label="Thành phố"
          maxLength={120}
          onChangeText={setCity}
          placeholder="Bạn hay ở đâu"
          value={city}
        />
        <RudiButton disabled={dangLuu} label="Lưu hồ sơ" loading={dangLuu} onPress={() => void luu()} />
        <RudiButton disabled={dangLuu} label="Huỷ" onPress={() => setDangSua(false)} variant="ghost" />
      </View>
    );
  }

  const soDem = [
    `${hoSo.counts.friends} bạn bè`,
    `${hoSo.counts.contexts} nhóm`,
    `${hoSo.counts.outings} kèo`,
    `${hoSo.counts.places_checked_in} nơi đã tới`,
    `${hoSo.counts.memories} kỷ niệm`,
  ].join(" · ");

  const namVao = new Date(hoSo.created_at).getFullYear();
  // A passport (ADR-0037 D1, plan S6): the cloth cover band with its title,
  // then the data page -- photo, name, city, the year stamped in -- and the
  // footprint as one sentence under it, not a scoreboard.
  return (
    <View style={[styles.hoChieu, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]}>
      <View style={[styles.bia, { backgroundColor: colors.cover }]}>
        <Text style={[typography.stamp, { color: colors.coverInk }]}>Hộ chiếu Rủ Đi</Text>
        <Ionicons color={colors.coverInkSoft} name="compass-outline" size={18} />
      </View>
      <View style={styles.trang}>
        <View style={styles.dau}>
          {/* The picture is changed where it is picked, compressed and uploaded
              (Cài đặt); tapping it here is the way there, not a second uploader. */}
          <Pressable
            accessibilityLabel="Đổi ảnh đại diện"
            accessibilityRole="button"
            onPress={() => router.push("/settings" as never)}
            style={({ pressed }) => [styles.anhHoChieu, { borderColor: colors.lineStrong }, pressed && styles.bam]}
          >
            <AvatarNguoi name={hoSo.display_name} personId={phien.person_id} ring size={64} />
          </Pressable>
          <View style={styles.dauChu}>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Họ tên</Text>
            <Text style={[typography.h1, { color: colors.ink }]}>{hoSo.display_name}</Text>
            {hoSo.city ? (
              <>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>Thành phố</Text>
                <Text style={[typography.label, { color: colors.ink }]}>{hoSo.city}</Text>
              </>
            ) : null}
          </View>
        </View>
        {hoSo.bio ? <Text style={[typography.body, { color: colors.inkSoft }]}>{hoSo.bio}</Text> : null}
        <View style={styles.hangDau}>
          <DauLon co="nho" nhan={`Tham gia ${namVao}`} tilt={-6} tone="ink" />
          <Text style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>
            Đăng nhập bằng {hoSo.login_methods.map((m) => NHAN_CUA[m] ?? m).join(", ") || "lời mời"}
          </Text>
        </View>
        <Text style={[typography.caption, { color: colors.inkSoft }]}>{soDem}</Text>
        <RudiButton compact full={false} icon="create-outline" label="Chỉnh hồ sơ" onPress={() => moSua(hoSo)} variant="outline" />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  hoChieu: { borderWidth: 1, borderRadius: 8, overflow: "hidden" },
  bia: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", paddingHorizontal: 14, paddingVertical: 8 },
  trang: { gap: 10, padding: 14 },
  anhHoChieu: { borderWidth: 1, padding: 4, borderRadius: 4 },
  hangDau: { flexDirection: "row", alignItems: "center", gap: 12, flexWrap: "wrap" },
  form: { gap: 12 },
  dau: { flexDirection: "row", alignItems: "center", gap: 14 },
  dauChu: { flex: 1, gap: 2 },
  bam: { opacity: 0.7 },
});
