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
import { useFocusEffect } from "expo-router";
import { useCallback, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import { docHoSoToi, suaHoSoToi, type HoSoToi, type Phien } from "../../../phien";
import { typography, useRudiTheme } from "../../theme";
import { Chip, Field, Inline, RudiButton } from "../../ui";
import { Avatar } from "../../ui/Avatar";
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
  const { colors } = useRudiTheme();
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
        <Field accessibilityLabel="Ô tên hiển thị" label="Tên" maxLength={200} onChangeText={setTen} value={ten} />
        {loiLuu ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loiLuu}</Text> : null}
        <Field
          accessibilityLabel="Ô giới thiệu"
          label="Giới thiệu"
          maxLength={500}
          multiline
          onChangeText={setBio}
          placeholder="Vài chữ về bạn"
          value={bio}
        />
        <Field
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

  return (
    <View style={styles.card}>
      <View style={styles.dau}>
        <Avatar name={hoSo.display_name} ring size={64} />
        <View style={styles.dauChu}>
          <Text style={[typography.h1, { color: colors.ink }]}>{hoSo.display_name}</Text>
          <Text style={[typography.caption, { color: colors.inkFaint }]}>
            Đăng nhập bằng {hoSo.login_methods.map((m) => NHAN_CUA[m] ?? m).join(", ") || "lời mời"}
          </Text>
        </View>
      </View>
      {hoSo.bio ? <Text style={[typography.body, { color: colors.inkSoft }]}>{hoSo.bio}</Text> : null}
      <Inline gap={7} wrap>
        {hoSo.city ? <Chip icon="location-outline" label={hoSo.city} /> : null}
        <Chip icon="calendar-outline" label={`Thành viên từ ${new Date(hoSo.created_at).getFullYear()}`} />
      </Inline>
      {/* The counts as one sentence: a footprint, not a scoreboard. */}
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{soDem}</Text>
      <RudiButton
        compact
        full={false}
        icon="create-outline"
        label="Chỉnh hồ sơ"
        onPress={() => moSua(hoSo)}
        variant="outline"
      />
    </View>
  );
}

const styles = StyleSheet.create({
  card: { gap: 10 },
  form: { gap: 12 },
  dau: { flexDirection: "row", alignItems: "center", gap: 14 },
  dauChu: { flex: 1, gap: 2 },
});
