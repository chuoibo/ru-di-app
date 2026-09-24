/**
 * «Thêm vào kèo» from a place (M4): pick one of the group's outings and the
 * place becomes a stop on it (next full hour, labelled with its name). The
 * server refuses a place it does not know before writing anything.
 *
 * UI v2: the place being added is named at the top so the task is never lost
 * between screens; each outing is a row with its date mark, and the one
 * action per row says what it does.
 */
import { useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { Canh } from "../../ui/art/Canh";
import { useCallback, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc } from "../../../api";
import type { Phien } from "../../../phien";
import { nhanKhoangNgay, type BuoiDi } from "../../../screens/len-plan/buoi-di";
import { docChiTiet } from "../../kham-pha/dia-diem";
import { cauSoChang, docKeoCuaNhom, gioTiepTheo, luuLichTrinh, themChang } from "../../keo/keo";
import { dauLich, homNay, nhanNhip, nhipKeo } from "../../keo/nhip-keo";
import { henHaiBan } from "../../keo/hen-hai-ban";
import { laPair, tenCuocTroChuyen } from "../../nhan-rieng/nhan-rieng";
import { displayFace, typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; keo: BuoiDi[]; hen: { keo: BuoiDi; voi: string }[]; ten: string }
  | { pha: "hong"; loi: string };

function thamSoChuoi(v: unknown): string {
  if (typeof v === "string") return v;
  return "";
}

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

export function PickOutingLiveScreen({ phien }: { phien: Phien }) {
  const router = useRouter();
  const params = useLocalSearchParams<{ place?: string }>();
  const { colors } = useRudiTheme();
  const placeId = thamSoChuoi(params.place);
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangGhi, setDangGhi] = useState<string | null>(null);
  const [loi, setLoi] = useState<string | null>(null);
  const contextId = phien.context_id;
  const today = homNay();

  // The pairs' upcoming plans too: a couple's plan lives in their pair, which
  // is never the current group, so «Thêm vào kèo» answered «Nhóm chưa có kèo
  // nào» the day after they agreed on Saturday (QA 23/09).
  const doi = (phien.contexts ?? []).filter((n) => laPair(n) && n.my_state === "active" && n.id !== contextId);
  const khoaDoi = doi.map((d) => d.id).join(",");
  const nap = useCallback(async () => {
    if (!placeId) return;
    try {
      const [keo, place, cuaDoi] = await Promise.all([
        contextId === null ? Promise.resolve([] as BuoiDi[]) : docKeoCuaNhom(contextId, phien.person_id),
        docChiTiet(placeId),
        Promise.allSettled(doi.map((d) => docKeoCuaNhom(d.id, phien.person_id))),
      ]);
      const theoDoi = new Map<string, BuoiDi[]>();
      cuaDoi.forEach((k, i) => {
        if (k.status === "fulfilled") theoDoi.set(doi[i].id, k.value);
      });
      const hen = henHaiBan(doi.map((d) => ({ id: d.id, tenNguoiKia: tenCuocTroChuyen(d) })), theoDoi, homNay()).map((h) => ({ keo: h.keo, voi: h.tenNguoiKia }));
      setTrang({ pha: "xong", keo, hen, ten: place.name });
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    }
    // `doi` is rebuilt every render; its ids are what the read depends on.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [contextId, placeId, phien.person_id, khoaDoi]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  const them = async (keo: BuoiDi, ten: string) => {
    setDangGhi(keo.id);
    setLoi(null);
    try {
      await luuLichTrinh(
        keo,
        themChang(keo.stops, { at: gioTiepTheo(), label: ten, place_name: ten, place_id: placeId }),
        phien.person_id,
        newAttempt(),
      );
      router.replace((keo.context_id === contextId ? `/outings/${keo.id}` : `/outings/${keo.id}?ctx=${keo.context_id}`) as never);
    } catch (error) {
      setLoi(loiRaChu(error));
    } finally {
      setDangGhi(null);
    }
  };

  return (
    <RudiScreen testID="pick-outing-screen">
      <TopBar title="Thêm vào kèo" />
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup>
          <SkeletonRow leading={44} />
          <SkeletonRow leading={44} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? (
        <ErrorState body={trang.loi} onRetry={() => void nap()} secondary={{ label: "Quay về", onPress: () => router.back() }} title="Chưa đọc được kèo" />
      ) : null}
      {trang.pha === "xong" ? (
        <>
          <Heading title={trang.ten} subtitle="Chọn kèo để thêm làm một chặng. Giờ đặt tạm là giờ tròn kế tiếp, sửa được trong kèo." />
          {loi !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
          {trang.keo.length === 0 && trang.hen.length === 0 ? (
            <EmptyState
              action={{ label: "Tạo kèo", onPress: () => router.push("/outings/new") }}
              body="Tạo kèo trước ở Lên plan, rồi quay lại thêm địa điểm này."
              kind="first-use"
              layout="inline"
              illustration={<Canh id="chua-co-keo" width={168} />} title="Chưa có kèo nào để thêm vào"
            />
          ) : null}
          {trang.hen.length > 0 ? <SectionHeader title="Hẹn của hai bạn" /> : null}
          {trang.hen.map(({ keo: k, voi }) => (
            <HangKeo dangGhi={dangGhi} k={k} key={k.id} onThem={() => void them(k, trang.ten)} today={today} voi={voi} />
          ))}
          {trang.hen.length > 0 && trang.keo.length > 0 ? <SectionHeader title="Kèo của nhóm" /> : null}
          {trang.keo.map((k) => {
            const dau = dauLich(k.starts_on);
            const nhan = nhanNhip(nhipKeo(k.starts_on, k.ends_on, today));
            return (
              <View key={k.id} style={[styles.hang, { borderBottomColor: colors.line }]}>
                <View style={styles.dauLich}>
                  <Text style={[styles.ngay, { color: colors.ink }]}>{dau?.ngay ?? "?"}</Text>
                  <Text style={[typography.caption, { color: colors.inkSoft }]}>{dau?.thang ?? ""}</Text>
                </View>
                <View style={styles.hangChu}>
                  <Text numberOfLines={2} style={[typography.title, { color: colors.ink }]}>{k.title}</Text>
                  <Text numberOfLines={1} style={[typography.caption, { color: colors.inkSoft }]}>
                    {nhanKhoangNgay(k.starts_on, k.ends_on)} · {cauSoChang(k.stops.length)}
                    {nhan ? ` · ${nhan}` : ""}
                  </Text>
                </View>
                <RudiButton
                  compact
                  disabled={dangGhi !== null}
                  full={false}
                  label="Thêm vào"
                  loading={dangGhi === k.id}
                  onPress={() => void them(k, trang.ten)}
                />
              </View>
            );
          })}
        </>
      ) : null}
    </RudiScreen>
  );
}

function HangKeo({ k, voi, today, dangGhi, onThem }: { k: BuoiDi; voi: string; today: string; dangGhi: string | null; onThem: () => void }) {
  const { colors } = useRudiTheme();
  const dau = dauLich(k.starts_on);
  const nhan = nhanNhip(nhipKeo(k.starts_on, k.ends_on, today));
  return (
    <View style={[styles.hang, { borderBottomColor: colors.line }]}>
      <View style={styles.dauLich}>
        <Text style={[styles.ngay, { color: colors.ink }]}>{dau?.ngay ?? "?"}</Text>
        <Text style={[typography.caption, { color: colors.inkSoft }]}>{dau?.thang ?? ""}</Text>
      </View>
      <View style={styles.hangChu}>
        <Text numberOfLines={2} style={[typography.title, { color: colors.ink }]}>{k.title}</Text>
        <Text numberOfLines={1} style={[typography.caption, { color: colors.inkSoft }]}>
          với {voi} · {cauSoChang(k.stops.length)}
          {nhan ? ` · ${nhan}` : ""}
        </Text>
      </View>
      <RudiButton compact disabled={dangGhi !== null} full={false} label="Thêm vào" loading={dangGhi === k.id} onPress={onThem} />
    </View>
  );
}

const styles = StyleSheet.create({
  hang: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 64, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  dauLich: { width: 48, alignItems: "center" },
  ngay: { fontFamily: displayFace.extraBold, fontSize: 24, lineHeight: 28, fontVariant: ["tabular-nums"] },
  hangChu: { flex: 1, gap: 3 },
});
