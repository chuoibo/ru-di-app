/**
 * The paper puppet drawn with Skia (ADR-0037 D5, D10): every piece of paper
 * is a fixed path in its own frame (`boPhanRoi`), and each frame of a
 * performance only moves the frames -- one affine per part, solved on the UI
 * thread from the shared clock (`khopTai` -> `giaiRoi` -> `maTranBoPhan`). No
 * path is rebuilt while Nếp moves, and the JS thread is not involved.
 *
 * The order is `tuTheRoi`'s, part for part (sheet, eyes, brow and mouth, legs,
 * arms, prop, pins), and `tests/nep-roi.test.mjs` proves the same matrices put
 * every part where `tuTheRoi` draws it, so the SVG frame and this canvas are
 * one drawing.
 *
 * Only reachable through `luoiSkia(() => import("./skia/NepRoiSkia"))`.
 */
import { Canvas, Group } from "@shopify/react-native-skia";
import { useMemo } from "react";
import { useDerivedValue, type SharedValue } from "react-native-reanimated";

import { KHUNG_NEP, type BieuCamNep, type GapNep } from "../../art/nep";
import { khopTai, vatCuaTietMuc, type TietMuc } from "../../art/nep-dien";
import { boPhanRoi, giaiRoi, maTranBoPhan, maTranGapTo, type MaTranBoPhan, type TuTheRoi } from "../../art/nep-roi";
import { nhanMaTran, type MaTran } from "../../art/net";
import { useRudiTheme } from "../../theme";
import { LopSkia } from "./VeSkia";

export interface NepRoiVeProps {
  tm: TietMuc;
  /** The performance clock, ms. */
  t: SharedValue<number>;
  /** The drawing's side, dp (the 96-box scaled to it). */
  width: number;
  chiTiet?: boolean;
  gap?: GapNep;
}

const MAT: readonly BieuCamNep[] = ["binh-than", "hao-hung", "hoi", "quyet", "met", "nhuong", "giu-kin"];

/** An affine as Skia's row-major 3 × 3. */
function skia3(m: MaTran): number[] {
  "worklet";
  return [m[0], m[2], m[4], m[1], m[3], m[5], 0, 0, 1];
}

function useMaTran(m: SharedValue<MaTranBoPhan>, k: keyof MaTranBoPhan) {
  return useDerivedValue(() => skia3(m.value[k]));
}

function MatTheoBieuCam({ i, tt, lop, colors }: { i: number; tt: SharedValue<TuTheRoi>; lop: Parameters<typeof LopSkia>[0]["lop"]; colors: Parameters<typeof LopSkia>[0]["colors"] }) {
  const hien = useDerivedValue<number>(() => (tt.value.bieuCam === MAT[i] ? 1 : 0));
  return <LopSkia colors={colors} lop={lop} opacity={hien} />;
}

export default function NepRoiSkia({ tm, t, width, chiTiet = true, gap = "trang" }: NepRoiVeProps) {
  const { colors } = useRudiTheme();
  const bp = useMemo(() => boPhanRoi({ chiTiet, gap }), [chiTiet, gap]);
  const vats = useMemo(() => vatCuaTietMuc(tm), [tm]);
  const tt = useDerivedValue(() => khopTai(tm, t.value));
  const m = useDerivedValue(() => maTranBoPhan(tt.value, giaiRoi(tt.value, gap)));
  const than = useMaTran(m, "than");
  const matTrai = useMaTran(m, "matTrai");
  const matPhai = useMaTran(m, "matPhai");
  const miengHoi = useMaTran(m, "miengHoi");
  const duiGan = useMaTran(m, "duiGan");
  const cangGan = useMaTran(m, "cangGan");
  const banChanGan = useMaTran(m, "banChanGan");
  const duiXa = useMaTran(m, "duiXa");
  const cangXa = useMaTran(m, "cangXa");
  const banChanXa = useMaTran(m, "banChanXa");
  const canhTayXa = useMaTran(m, "canhTayXa");
  const cangTayXa = useMaTran(m, "cangTayXa");
  const canhTayGan = useMaTran(m, "canhTayGan");
  const cangTayGan = useMaTran(m, "cangTayGan");
  const vat = useMaTran(m, "vat");
  const vatGap = useDerivedValue(() => skia3(nhanMaTran(m.value.vat, maTranGapTo(tt.value.vat?.gap ?? 0))));
  const ghimVaiXa = useMaTran(m, "ghimVaiXa");
  const ghimHongXa = useMaTran(m, "ghimHongXa");
  const ghimHongGan = useMaTran(m, "ghimHongGan");
  const ghimVaiGan = useMaTran(m, "ghimVaiGan");
  const laHoi = useDerivedValue<number>(() => (tt.value.bieuCam === "hoi" ? 1 : 0));
  const hienVat = useDerivedValue(() => tt.value.vat?.hien ?? 0);
  const vatDangCam = useDerivedValue<string>(() => tt.value.vat?.id ?? "");
  return (
    <Canvas style={{ width, height: width }}>
      <Group transform={[{ scale: width / KHUNG_NEP }]}>
        <Group matrix={than}>
          <LopSkia colors={colors} lop={bp.than} />
        </Group>
        <Group matrix={matTrai}>
          <LopSkia colors={colors} lop={bp.mat} />
        </Group>
        <Group matrix={matPhai}>
          <LopSkia colors={colors} lop={bp.mat} />
        </Group>
        <Group matrix={than}>
          {MAT.map((b, i) => (
            <MatTheoBieuCam colors={colors} i={i} key={b} lop={bp.mat7[b]} tt={tt} />
          ))}
        </Group>
        <Group matrix={miengHoi}>
          <LopSkia colors={colors} lop={bp.miengHoi} opacity={laHoi} />
        </Group>
        <Group matrix={duiGan}>
          <LopSkia colors={colors} lop={bp.dui} />
        </Group>
        <Group matrix={cangGan}>
          <LopSkia colors={colors} lop={bp.cang} />
        </Group>
        <Group matrix={banChanGan}>
          <LopSkia colors={colors} lop={bp.banChanGan} />
        </Group>
        <Group matrix={duiXa}>
          <LopSkia colors={colors} lop={bp.dui} />
        </Group>
        <Group matrix={cangXa}>
          <LopSkia colors={colors} lop={bp.cang} />
        </Group>
        <Group matrix={banChanXa}>
          <LopSkia colors={colors} lop={bp.banChanXa} />
        </Group>
        <Group matrix={canhTayXa}>
          <LopSkia colors={colors} lop={bp.canhTay} />
        </Group>
        <Group matrix={cangTayXa}>
          <LopSkia colors={colors} lop={bp.cangTay} />
        </Group>
        <Group matrix={canhTayGan}>
          <LopSkia colors={colors} lop={bp.canhTay} />
        </Group>
        <Group matrix={cangTayGan}>
          <LopSkia colors={colors} lop={bp.cangTay} />
        </Group>
        {vats.map((id) => (
          <VatSkia bp={bp} colors={colors} hien={hienVat} id={id} key={id} m={vat} mGap={vatGap} dangCam={vatDangCam} />
        ))}
        <Group matrix={ghimVaiXa}>
          <LopSkia colors={colors} lop={bp.ghim} />
        </Group>
        <Group matrix={ghimHongXa}>
          <LopSkia colors={colors} lop={bp.ghim} />
        </Group>
        <Group matrix={ghimHongGan}>
          <LopSkia colors={colors} lop={bp.ghim} />
        </Group>
        <Group matrix={ghimVaiGan}>
          <LopSkia colors={colors} lop={bp.ghim} />
        </Group>
      </Group>
    </Canvas>
  );
}

function VatSkia({
  id,
  bp,
  colors,
  m,
  mGap,
  hien,
  dangCam,
}: {
  id: ReturnType<typeof vatCuaTietMuc>[number];
  bp: ReturnType<typeof boPhanRoi>;
  colors: Parameters<typeof LopSkia>[0]["colors"];
  m: SharedValue<number[]>;
  mGap: SharedValue<number[]>;
  hien: SharedValue<number>;
  dangCam: SharedValue<string>;
}) {
  const v = bp.vat[id];
  const doHien = useDerivedValue<number>(() => (dangCam.value === id ? hien.value : 0));
  return (
    <Group opacity={doHien}>
      <Group matrix={m}>
        <LopSkia colors={colors} lop={v.la} />
      </Group>
      {v.gapLai ? (
        <Group matrix={mGap}>
          <LopSkia colors={colors} lop={v.gapLai} />
        </Group>
      ) : null}
    </Group>
  );
}
