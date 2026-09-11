/** Journey view: map, fit, summary, optimize, and a bottom card that does not steal pan. */

import { useEffect, useMemo, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { useRouter } from "expo-router";

import { typography, useRudiTheme, mauMocHanhTrinh, mauSoMoc } from "../theme";
import { RudiButton } from "../ui";
import { BanDo } from "./BanDo";
import { gioTru, lamGiauDoan, layDoanDuong } from "./duong";
import type { DoanDuongHanhTrinh, HanhTrinh, HoatDongHanhTrinh } from "./mo-hinh";
import { chuKhoangCach, chuThoiGian, tomTatHanhTrinh } from "./tom-tat";
import type { MocBanDo } from "./kieu-ban-do";

export function ManHinhHanhTrinh({
  hanh,
  selectedActivityId,
  selectedSegmentId,
  fitDem,
  toiDem,
  onUserMove,
  onChonMoc,
  onChonDoan,
  onNen,
  onKhop,
  onToiUu,
  dangToiUu = false,
}: {
  hanh: HanhTrinh;
  selectedActivityId: string | null;
  selectedSegmentId: string | null;
  fitDem: number;
  toiDem: number;
  onUserMove: () => void;
  onChonMoc: (id: string) => void;
  onChonDoan: (id: string) => void;
  onNen: () => void;
  onKhop: () => void;
  onToiUu?: () => void;
  dangToiUu?: boolean;
}) {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const mau = mauMocHanhTrinh(colors);
  const [doan, setDoan] = useState<DoanDuongHanhTrinh[]>(hanh.routeSegments);
  const cache = useRef(new Map<string, Awaited<ReturnType<typeof layDoanDuong>>>());

  useEffect(() => {
    setDoan(hanh.routeSegments);
    let song = true;
    void lamGiauDoan(hanh.routeSegments, hanh.activities, { cache: cache.current }).then((giau) => {
      if (song) setDoan(giau);
    });
    return () => {
      song = false;
    };
  }, [hanh]);

  const mocs: MocBanDo[] = useMemo(
    () =>
      hanh.activities
        .filter((a): a is HoatDongHanhTrinh & { lat: number; lng: number; so: number } => a.lat !== null && a.lng !== null && a.so !== null)
        .map((a) => ({
          id: a.id,
          so: a.so,
          lat: a.lat,
          lng: a.lng,
          tieuDe: a.tieuDe,
          gio: a.gio,
          chon: a.id === selectedActivityId,
        })),
    [hanh.activities, selectedActivityId],
  );

  const tom = tomTatHanhTrinh(hanh.activities, doan.length > 0 ? doan : hanh.routeSegments);
  const mocChon = hanh.activities.find((a) => a.id === selectedActivityId) ?? null;
  const doanChon = doan.find((d) => d.id === selectedSegmentId) ?? null;
  const toi = (() => {
    if (!mocChon || mocChon.lat === null || mocChon.lng === null || toiDem === 0) return null;
    return { lat: mocChon.lat, lng: mocChon.lng, dem: toiDem };
  })();

  const thieu = tom.soChang - tom.soChangCoViTri;
  const tomChu = [
    tom.soChangCoViTri === 0 ? "Chưa có chặng nào có vị trí trên bản đồ" : `${tom.soChangCoViTri} chặng`,
    tom.met !== null ? chuKhoangCach(tom.met) : null,
    tom.giay !== null ? chuThoiGian(tom.giay) : null,
    tom.hieuSuat !== null ? `${tom.hieuSuat}%` : null,
  ]
    .filter((s): s is string => s !== null)
    .join(" · ");

  const mauMoc = [mauSoMoc(colors, 1), mauSoMoc(colors, 2), mauSoMoc(colors, 3)];

  return (
    <View style={styles.khung}>
      <BanDo
        doan={doan.map((d) => ({ id: d.id, polyline: d.polyline, chon: d.id === selectedSegmentId }))}
        fitDem={fitDem}
        mauDuong={mau.duong}
        mauDuongMo={mau.duongMo}
        mauMoc={mauMoc}
        mauMocChon={mau.mocChon}
        mauMocInk={mau.mocInk}
        mauNen={mau.the}
        mauVien={mau.vien}
        mocs={mocs}
        onChonDoan={onChonDoan}
        onChonMoc={onChonMoc}
        onNen={onNen}
        onUserMove={onUserMove}
        toi={toi}
      />
      <View pointerEvents="box-none" style={StyleSheet.absoluteFill}>
        <View pointerEvents="box-none" style={styles.hangNut}>
          <RudiButton accessibilityLabel="Khớp hành trình" compact full={false} icon="scan-outline" label="Khớp hành trình" onPress={onKhop} variant="outline" />
          {mocs.length > 0 ? (
            <View style={styles.hangMoc}>
              {mocs.map((m) => (
                <Pressable
                  accessibilityLabel={m.tieuDe}
                  accessibilityRole="button"
                  accessibilityState={{ selected: m.chon }}
                  key={m.id}
                  onPress={() => (m.chon ? onNen() : onChonMoc(m.id))}
                  style={[
                    styles.mocChip,
                    {
                      backgroundColor: m.chon ? mau.mocChon : (mauMoc[(m.so - 1) % mauMoc.length] ?? mau.mocChon),
                      borderColor: mau.vien,
                    },
                  ]}
                >
                  <Text style={[styles.mocSo, { color: mau.mocInk }]}>{m.so}</Text>
                </Pressable>
              ))}
            </View>
          ) : null}
        </View>
        <View pointerEvents="none" style={styles.dan} />
        <View style={[styles.the, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.base }]}>
          {mocChon ? (
            <TheMoc
              moc={mocChon}
              truoc={changTruoc(hanh.activities, mocChon.id)}
              onChiTiet={mocChon.placeId ? () => router.push(`/places/${mocChon.placeId}` as never) : undefined}
            />
          ) : doanChon ? (
            <TheDoan doan={doanChon} activities={hanh.activities} />
          ) : (
            <>
              <Text style={[typography.label, { color: colors.ink }]}>{tomChu}</Text>
              {thieu > 0 && tom.soChangCoViTri > 0 ? (
                <Text style={[typography.caption, { color: colors.inkSoft }]}>{thieu} chặng chưa có vị trí trên bản đồ</Text>
              ) : null}
              {onToiUu && tom.soChangCoViTri >= 2 ? (
                <RudiButton accessibilityLabel="Tối ưu lộ trình" compact disabled={dangToiUu} label="Tối ưu lộ trình" loading={dangToiUu} onPress={onToiUu} />
              ) : null}
            </>
          )}
        </View>
      </View>
    </View>
  );
}

function changTruoc(danh: readonly HoatDongHanhTrinh[], id: string): HoatDongHanhTrinh | null {
  const mapped = danh.filter((a) => a.lat !== null);
  const i = mapped.findIndex((a) => a.id === id);
  if (i <= 0) return null;
  return mapped[i - 1];
}

function TheMoc({
  moc,
  truoc,
  onChiTiet,
}: {
  moc: HoatDongHanhTrinh;
  truoc: HoatDongHanhTrinh | null;
  onChiTiet?: () => void;
}) {
  const { colors } = useRudiTheme();
  return (
    <View style={styles.khoiThe}>
      <Text style={[typography.caption, { color: colors.inkFaint }]}>{moc.gio}</Text>
      <Text style={[typography.h2, { color: colors.ink }]}>{moc.tieuDe}</Text>
      {moc.diaChi ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{moc.diaChi}</Text> : null}
      {truoc ? (
        <Text style={[typography.caption, { color: colors.inkSoft }]}>Từ {truoc.tieuDe}</Text>
      ) : null}
      {onChiTiet ? <RudiButton compact label="Xem chi tiết" onPress={onChiTiet} variant="outline" /> : null}
    </View>
  );
}

function TheDoan({ doan, activities }: { doan: DoanDuongHanhTrinh; activities: readonly HoatDongHanhTrinh[] }) {
  const { colors } = useRudiTheme();
  const from = activities.find((a) => a.id === doan.fromActivityId);
  const to = activities.find((a) => a.id === doan.toActivityId);
  const roi = to ? gioTru(to.gio, doan.durationSeconds) : null;
  return (
    <View style={styles.khoiThe}>
      <Text style={[typography.h2, { color: colors.ink }]}>
        {from?.tieuDe ?? "A"} → {to?.tieuDe ?? "B"}
      </Text>
      <Text style={[typography.body, { color: colors.inkSoft }]}>
        {chuThoiGian(doan.durationSeconds)} · {chuKhoangCach(doan.distanceMeters)}
      </Text>
      {roi && to ? (
        <Text style={[typography.caption, { color: colors.inkSoft }]}>
          Rời khoảng {roi} để tới lúc {to.gio}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  khung: { flex: 1, minHeight: 280 },
  hangNut: { paddingHorizontal: 12, paddingTop: 8, alignItems: "flex-start", gap: 8 },
  hangMoc: { flexDirection: "row", flexWrap: "wrap", gap: 6 },
  mocChip: { width: 28, height: 28, borderRadius: 999, borderWidth: 2, alignItems: "center", justifyContent: "center" },
  mocSo: { fontSize: 13, fontWeight: "700", lineHeight: 16 },
  dan: { flex: 1 },
  the: { margin: 12, padding: 14, gap: 8, borderWidth: StyleSheet.hairlineWidth },
  khoiThe: { gap: 6 },
});
