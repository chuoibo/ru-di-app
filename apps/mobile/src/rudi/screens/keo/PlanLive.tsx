/**
 * Lên plan on a real session (M4): the current group's outings from the
 * server, and the door to make one. The fixture build keeps the fixture
 * trip, which is what the default Maestro table drives.
 *
 * ## The next appointment leads (UI v2, đợt 4)
 *
 * The tab used to be a list of identical cards, newest-created first. What
 * the person opens it for is one question -- when is the next one, and what
 * still needs deciding -- so the soonest upcoming outing is drawn large with
 * its date as a calendar mark, the ones after it are rows, and what has
 * already happened sits under «Đã qua» in a quieter voice. «Còn N ngày» is
 * computed from the server's dates (`nhip-keo.ts`), never a caption.
 */
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import type { Phien } from "../../../phien";
import { nhanKhoangNgay, type BuoiDi } from "../../../screens/len-plan/buoi-di";
import { henHaiBan, type HenHaiBan } from "../../keo/hen-hai-ban";
import { cauSoChang, docKeoCuaNhom } from "../../keo/keo";
import { laPair, tenCuocTroChuyen } from "../../nhan-rieng/nhan-rieng";
import { useNepNguCanh } from "../../nep/NepProvider";
import { chiaKeo, dauLich, homNay, nhanNhip, nhipKeo } from "../../keo/nhip-keo";
import { displayFace, typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen, SectionHeader } from "../../ui";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { Money } from "../../ui/Money";
import { RouteLine } from "../../ui/RouteLine";
import { SkeletonCard, SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { Stamp } from "../../ui/Stamp";

type Trang = { pha: "dang-doc" } | { pha: "xong"; keo: BuoiDi[] } | { pha: "hong"; loi: string };

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

function tenNhom(phien: Phien): string {
  const nhom = phien.contexts?.find((n) => n.id === phien.context_id);
  if (nhom === undefined) return "nhóm của bạn";
  return nhom.display_name;
}

/** The day and month of a date, as a mark on the page. */
function DauLich({ iso, lon = false, mo = false }: { iso: string; lon?: boolean; mo?: boolean }) {
  const { colors } = useRudiTheme();
  const dau = dauLich(iso);
  const muc = mo ? colors.inkFaint : colors.ink;
  return (
    <View style={[styles.dauLich, lon && styles.dauLichLon]}>
      <Text style={[lon ? styles.ngayLon : styles.ngay, { color: muc }]}>{dau?.ngay ?? "?"}</Text>
      <Text style={[typography.caption, { color: mo ? colors.inkFaint : colors.inkSoft }]}>{dau?.thang ?? ""}</Text>
    </View>
  );
}

function KeoDan({ keo, today, onOpen }: { keo: BuoiDi; today: string; onOpen: () => void }) {
  const { colors, radius } = useRudiTheme();
  const nhip = nhipKeo(keo.starts_on, keo.ends_on, today);
  const nhan = nhanNhip(nhip);
  return (
    <Pressable
      accessibilityLabel={`Mở kèo ${keo.title}`}
      accessibilityRole="button"
      onPress={onOpen}
      style={({ pressed }) => [styles.dan, { backgroundColor: colors.accentSoft, borderRadius: radius.base }, pressed && styles.bam]}
    >
      <DauLich iso={keo.starts_on} lon />
      <View style={styles.danChu}>
        {nhan ? <Stamp label={nhan} tilt={-2} tone={nhip.kieu === "dang-dien-ra" || nhip.kieu === "hom-nay" ? "split" : "accent"} /> : null}
        <Text style={[typography.h2, { color: colors.ink }]}>{keo.title}</Text>
        <Text style={[typography.body, { color: colors.inkSoft }]}>
          {nhanKhoangNgay(keo.starts_on, keo.ends_on)} · {keo.headcount} người
        </Text>
        <View style={styles.tienRow}>
          <Money size="label" vnd={keo.budget_per_person_vnd} />
          <Text style={[typography.caption, { color: colors.inkSoft }]}>một người · {cauSoChang(keo.stops.length)}</Text>
        </View>
      </View>
    </Pressable>
  );
}

function HangKeo({ keo, today, mo = false, voi, onOpen }: { keo: BuoiDi; today: string; mo?: boolean; voi?: string; onOpen: () => void }) {
  const { colors } = useRudiTheme();
  const nhan = nhanNhip(nhipKeo(keo.starts_on, keo.ends_on, today));
  return (
    <Pressable
      accessibilityLabel={`Mở kèo ${keo.title}`}
      accessibilityRole="button"
      onPress={onOpen}
      style={({ pressed }) => [styles.hang, { borderBottomColor: colors.line }, pressed && styles.bam]}
    >
      <DauLich iso={keo.starts_on} mo={mo} />
      <View style={styles.hangChu}>
        <Text numberOfLines={2} style={[typography.title, { color: mo ? colors.inkSoft : colors.ink }]}>{keo.title}</Text>
        <Text numberOfLines={1} style={[typography.caption, { color: colors.inkFaint }]}>
          {voi ? `với ${voi} · ` : ""}{nhanKhoangNgay(keo.starts_on, keo.ends_on)} · {voi ? "" : `${keo.headcount} người · `}{cauSoChang(keo.stops.length)}
          {nhan ? ` · ${nhan}` : ""}
        </Text>
      </View>
    </Pressable>
  );
}

/**
 * The outings of every pair the person is active in, read beside the current
 * group's: a pair is never the current group, so a plan two people agreed on
 * would otherwise never reach this tab. A pair that fails to read is left out
 * rather than failing the tab -- it is a second list, not the page.
 */
function HenCuaHaiBan({ phien, today }: { phien: Phien; today: string }) {
  const router = useRouter();
  const [hen, setHen] = useState<HenHaiBan<BuoiDi>[]>([]);
  const doi = (phien.contexts ?? []).filter((n) => laPair(n) && n.my_state === "active" && n.id !== phien.context_id);
  const khoaDoi = doi.map((d) => d.id).join(",");

  useFocusEffect(
    useCallback(() => {
      let conDung = true;
      void Promise.allSettled(doi.map((d) => docKeoCuaNhom(d.id, phien.person_id))).then((ket) => {
        if (!conDung) return;
        const theoDoi = new Map<string, BuoiDi[]>();
        ket.forEach((k, i) => {
          if (k.status === "fulfilled") theoDoi.set(doi[i].id, k.value);
        });
        setHen(henHaiBan(doi.map((d) => ({ id: d.id, tenNguoiKia: tenCuocTroChuyen(d) })), theoDoi, today));
      });
      return () => {
        conDung = false;
      };
      // `doi` is rebuilt every render; its ids are what the read depends on.
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [khoaDoi, phien.person_id, today]),
  );

  if (hen.length === 0) return null;
  return (
    <View testID="hen-hai-ban">
      <SectionHeader title="Hẹn của hai bạn" />
      {hen.map((h) => (
        <HangKeo
          key={h.keo.id}
          keo={h.keo}
          onOpen={() => router.push(`/outings/${h.keo.id}?ctx=${h.contextId}` as never)}
          today={today}
          voi={h.tenNguoiKia}
        />
      ))}
    </View>
  );
}

export function PlanLiveScreen({ phien }: { phien: Phien }) {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const contextId = phien.context_id;
  const today = homNay();
  useNepNguCanh({ man: "plan", tieuDe: "Lên plan", goiY: ["Kèo nào sắp tới?", "Gợi ý một buổi cuối tuần"] });

  const nap = useCallback(async () => {
    if (contextId === null) return;
    try {
      const keo = await docKeoCuaNhom(contextId, phien.person_id);
      setTrang({ pha: "xong", keo });
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [contextId, phien.person_id]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  if (contextId === null) {
    return (
      <RudiScreen bottomInset="tab" onRefresh={nap} testID="plan-screen">
        <Heading title="Lên plan" subtitle="Kèo của nhóm nằm ở đây khi bạn vào một nhóm." />
        <HenCuaHaiBan phien={phien} today={today} />
        <RudiButton label="Tới Tin nhắn" onPress={() => router.push("/(tabs)/messages" as never)} variant="outline" />
      </RudiScreen>
    );
  }

  const moKeo = (k: BuoiDi) => router.push(`/outings/${k.id}` as never);
  const chia = trang.pha === "xong" ? chiaKeo(trang.keo, today) : null;
  const [dan, ...sauDo] = chia?.sapToi ?? [];

  return (
    <RudiScreen bottomInset="tab" onRefresh={nap} testID="plan-screen">
      <View style={styles.dau}>
        <View style={styles.flex}>
          <Heading title="Lên plan" subtitle={`Kèo của ${tenNhom(phien)}`} />
        </View>
        {/* The tab bar's own stamp is the create door; this one is the shortcut
            for a person already looking at the list. Hidden while the list is
            empty, where the empty state carries the same words. */}
        {trang.pha === "xong" && trang.keo.length > 0 ? (
          <RudiButton compact full={false} icon="add" label="Tạo kèo" onPress={() => router.push("/outings/new")} variant="outline" />
        ) : null}
      </View>
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup style={styles.khung}>
          <SkeletonCard lines={2} />
          <SkeletonRow leading={44} />
          <SkeletonRow leading={44} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được kèo" /> : null}
      {trang.pha === "xong" && trang.keo.length === 0 ? (
        <EmptyState
          action={{ label: "Tạo kèo", onPress: () => router.push("/outings/new") }}
          body="Rủ một buổi đầu tiên: ngày, số người, ngân sách. Chặng và địa điểm thêm sau."
          illustration={<RouteLine color={colors.inkFaint} dashed height={72} stops={3} width={200} />}
          kind="first-use"
          title="Chưa có kèo nào"
        />
      ) : null}
      {chia && dan ? (
        <>
          <KeoDan keo={dan} onOpen={() => moKeo(dan)} today={today} />
          {sauDo.length > 0 ? (
            <View>
              <SectionHeader title="Sau đó" />
              {sauDo.map((k) => (
                <HangKeo key={k.id} keo={k} onOpen={() => moKeo(k)} today={today} />
              ))}
            </View>
          ) : null}
        </>
      ) : null}
      <HenCuaHaiBan phien={phien} today={today} />
      {chia && chia.daQua.length > 0 ? (
        <View>
          <SectionHeader title="Đã qua" />
          {chia.daQua.map((k) => (
            <HangKeo key={k.id} keo={k} mo onOpen={() => moKeo(k)} today={today} />
          ))}
        </View>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  dau: { flexDirection: "row", alignItems: "flex-start", justifyContent: "space-between", gap: 12 },
  khung: { gap: 12 },
  dan: { flexDirection: "row", alignItems: "flex-start", gap: 14, padding: 16 },
  danChu: { flex: 1, gap: 6 },
  tienRow: { flexDirection: "row", alignItems: "baseline", gap: 8, flexWrap: "wrap" },
  dauLich: { width: 52, alignItems: "center", paddingTop: 2 },
  dauLichLon: { width: 64 },
  ngay: { fontFamily: displayFace.extraBold, fontSize: 24, lineHeight: 28, fontVariant: ["tabular-nums"] },
  ngayLon: { fontFamily: displayFace.extraBold, fontSize: 36, lineHeight: 40, letterSpacing: -1, fontVariant: ["tabular-nums"] },
  hang: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 64, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  hangChu: { flex: 1, gap: 3 },
  bam: { opacity: 0.7 },
});
