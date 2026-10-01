/** Web MapLibre GL. Native override: BanDo.native.tsx. */

import { createElement, useEffect, useMemo, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import { GeoJSONSource, Map, Marker, Popup, setWorkerUrl, type MapLayerMouseEvent, type MapMouseEvent } from "maplibre-gl";

/**
 * MapLibre's own words on the controls it draws, in Vietnamese: its English
 * defaults («Close popup», «Toggle attribution», «Map») reached a screen
 * reader on a Vietnamese page (QA UI-043). Only the strings a control this
 * map shows can say; the rest stay MapLibre's.
 */
const TIENG_BAN_DO = {
  "AttributionControl.ToggleAttribution": "Hiện hoặc ẩn nguồn bản đồ",
  "AttributionControl.MapFeedback": "Góp ý về dữ liệu bản đồ",
  "Map.Title": "Bản đồ hành trình",
  "Marker.Title": "Điểm trên bản đồ",
  "NavigationControl.ResetBearing": "Xoay bản đồ về hướng bắc",
  "NavigationControl.ZoomIn": "Phóng to",
  "NavigationControl.ZoomOut": "Thu nhỏ",
  "Popup.Close": "Đóng danh sách điểm gần nhau",
};
import "maplibre-gl/dist/maplibre-gl.css";

import { TAM_DA_LAT } from "./toa-do-mau";
import { chuNeo, DEM_KHOP, DUONG_TICK, giuaDoan, hinhTem, hopGioi, hopHanhTrinh, lopDuong, mocChum, muiTenDoan, tapHop, type BanDoProps, type MauBanDo, type MocBanDo } from "./kieu-ban-do";
import { typography, useRudiTheme } from "../theme";

const SVG_NS = "http://www.w3.org/2000/svg";

/**
 * maplibre-gl 6 derives its worker URL from `import.meta.url`, which inside
 * Metro's bundle is not an http URL: no worker starts and not one tile loads.
 * `tools/chep-maplibre-worker.mjs` puts the worker in `public/maplibre/`.
 */
export const MAPLIBRE_WORKER_URL = "/maplibre/maplibre-gl-worker.mjs";
setWorkerUrl(MAPLIBRE_WORKER_URL);

/**
 * One stamp pin as DOM: a paper stamp leaning a few degrees, its face and
 * edge from `hinhTem`, a drawn tick when the stop is reached and a small
 * stamp naming the day's anchor. The button itself is at least 48px square
 * and transparent, so the target is larger than the stamp it holds.
 */
function veMoc(moc: MocBanDo, mau: MauBanDo): HTMLElement {
  const tem = hinhTem(moc, mau);
  const hop = Math.max(48, tem.co);
  const el = document.createElement("button");
  el.type = "button";
  // Named chips in the day page carry the stop title; the pin says its number
  // and state, so a screen reader does not hear each stop name twice.
  el.setAttribute("aria-label", [`Mốc ${moc.so}`, moc.trangThai === "xong" ? "đã tới" : moc.trangThai === "hien-tai" ? "điểm tiếp theo" : null].filter(Boolean).join(", "));
  el.setAttribute("role", "button");
  el.style.cssText = [
    `width:${hop}px`, `height:${hop}px`, "padding:0", "border:0", "background:transparent",
    // No `position` here: `.maplibregl-marker` is absolute, and an inline
    // `relative` put every pin in document flow -- the second pin sat 48px
    // below its point (412px, 2026-09-29). Absolute still anchors the tick.
    "display:flex", "align-items:center", "justify-content:center", "cursor:pointer",
  ].join(";");
  const mat = document.createElement("span");
  mat.dataset.tem = "1";
  mat.dataset.nhip = moc.nhip ?? "";
  mat.style.cssText = [
    `min-width:${tem.co}px`, `height:${tem.co}px`, "box-sizing:border-box", "padding:0 6px",
    "border-radius:8px", `background:${tem.nen}`, `border:${tem.doVien}px solid ${tem.vien}`,
    `box-shadow:${moc.nhip === "cho" ? mau.bongCao : tem.bong}`,
    `transform:${dangNhip(moc.nhip, tem.nghieng)}`,
    // The press when the ink arrives: the stamp falls the last few px and
    // lands (ease-in), the paper shadow shrinking under it.
    "transition:transform 180ms cubic-bezier(0.5,0,0.75,0),box-shadow 180ms ease-in",
    "display:flex", "align-items:center", "justify-content:center",
    `color:${tem.so}`, "font:800 15px/1 system-ui,sans-serif", "font-variant-numeric:tabular-nums",
  ].join(";");
  mat.textContent = String(moc.so);
  el.appendChild(mat);
  if (tem.dauTick) {
    const svg = document.createElementNS(SVG_NS, "svg");
    svg.setAttribute("viewBox", "0 0 20 20");
    svg.setAttribute("aria-hidden", "true");
    svg.style.cssText = `position:absolute;right:${(hop - tem.co) / 2 - 5}px;bottom:${(hop - tem.co) / 2 - 5}px;width:20px;height:20px;border-radius:999px;background:${mau.giay};border:1.5px solid ${mau.vien};box-sizing:border-box`;
    const path = document.createElementNS(SVG_NS, "path");
    path.setAttribute("d", DUONG_TICK);
    path.setAttribute("fill", "none");
    path.setAttribute("stroke", mau.chu);
    path.setAttribute("stroke-width", "2.4");
    path.setAttribute("stroke-linecap", "round");
    path.setAttribute("stroke-linejoin", "round");
    svg.appendChild(path);
    el.appendChild(svg);
  }
  const neo = chuNeo(moc.neo);
  if (neo) {
    const nhan = document.createElement("span");
    nhan.setAttribute("aria-hidden", "true");
    nhan.textContent = neo;
    nhan.style.cssText = [
      "position:absolute", "top:100%", "left:50%", "transform:translate(-50%,2px) rotate(-2deg)",
      "white-space:nowrap", "padding:1px 6px", "border-radius:4px", `background:${mau.giay}`,
      `border:1.5px solid ${mau.muc}`, `color:${mau.muc}`, "font:800 12px/14px system-ui,sans-serif",
      "letter-spacing:0.6px", "pointer-events:none",
    ].join(";");
    el.appendChild(nhan);
  }
  return el;
}

/** A waiting stamp hovers 4px above the page; landed, it lies on it (native: TemNhip). */
function dangNhip(nhip: MocBanDo["nhip"], nghieng: number): string {
  return `translateY(${nhip === "cho" ? -4 : 0}px) rotate(${nghieng}deg)`;
}

export function BanDo({
  mocs,
  doan,
  mau,
  kieu,
  fitDem,
  cameraKey,
  fitPoints,
  padding = DEM_KHOP,
  duration = 0,
  onGhim,
  toi,
  onUserMove,
  onChonMoc,
  onChonDoan,
  onNen,
  onSan,
}: BanDoProps) {
  const { colors } = useRudiTheme();
  const cbs = useRef({ onUserMove, onChonMoc, onChonDoan, onNen, onGhim, onSan, mau });
  cbs.current = { onUserMove, onChonMoc, onChonDoan, onNen, onGhim, onSan, mau };
  const mauNen = mau.nen;
  const mapRef = useRef<Map | null>(null);
  const markers = useRef<Marker[]>([]);
  const arrows = useRef<Marker[]>([]);
  const chooser = useRef<Popup | null>(null);
  // Pins rebuild when a stop, its place or its state changes -- not when the
  // ink moves on (that only flips `ve` on the existing element, below).
  const khoaMoc = mocs.map((m) => [m.id, m.so, m.lat, m.lng, m.chon ? 1 : 0, m.trangThai, m.neo].join(",")).join("|");
  const loaded = useRef(false);
  const [host, setHost] = useState<HTMLDivElement | null>(null);
  const [san, setSan] = useState(false);
  const [webgl2, setWebgl2] = useState<boolean | null>(null);
  // The map's measured frame. The day page settles after the map mounts, so
  // the first fit ran on a taller frame and left the last stop under the page
  // (412px, 2026-09-29). Native refits on its measured viewport; so does this.
  const [khung, setKhung] = useState({ w: 0, h: 0 });
  // The fit padding a short map can afford: a quarter of each side at most.
  // Past that MapLibre has no room to fit and keeps the old camera, and since
  // B3 the map is exactly as tall as the room the day page leaves it.
  const demVua = useMemo(
    () =>
      khung.w > 0 && khung.h > 0
        ? { top: Math.min(padding.top, khung.h / 4), bottom: Math.min(padding.bottom, khung.h / 4), left: Math.min(padding.left, khung.w / 4), right: Math.min(padding.right, khung.w / 4) }
        : padding,
    [padding, khung.w, khung.h],
  );

  useEffect(() => {
    try {
      setWebgl2(Boolean(document.createElement("canvas").getContext("webgl2")));
    } catch {
      setWebgl2(false);
    }
  }, []);

  // No map to watch the ink on: nothing waits for it.
  useEffect(() => { if (webgl2 === false) cbs.current.onSan?.(); }, [webgl2]);

  useEffect(() => {
    const el = host;
    if (!el || !webgl2) return;
    let map: Map;
    try {
      map = new Map({
        container: el,
        style: kieu.startsWith("{") ? JSON.parse(kieu) : kieu,
        center: [TAM_DA_LAT.lng, TAM_DA_LAT.lat],
        zoom: 12,
        attributionControl: { compact: true },
        locale: TIENG_BAN_DO,
        // No rotation, so no compass control: the stock white button sat
        // over the paper like a browser widget (review, 2026-09-29), and a
        // turned map had no way back but that button.
        dragRotate: false,
        pitchWithRotate: false,
      });
      map.touchZoomRotate.disableRotation();
    } catch {
      setWebgl2(false);
      return;
    }
    map.on("load", () => {
      loaded.current = true;
      const lop = lopDuong(cbs.current.mau);
      if (!map.getSource("hanh-trinh-duong")) {
        map.addSource("hanh-trinh-duong", { type: "geojson", data: tapHop([]) });
        // Paper casing, then the ink line, then the pencil draft and the time
        // labels: an ink route drawn on the page, not one more road.
        map.addLayer({ id: "hanh-trinh-duong-vien", type: "line", source: "hanh-trinh-duong", ...lop.vien } as never);
        map.addLayer({ id: "hanh-trinh-duong", type: "line", source: "hanh-trinh-duong", ...lop.duong } as never);
        map.addLayer({ id: "hanh-trinh-net-noi", type: "line", source: "hanh-trinh-duong", ...lop.nhap } as never);
        map.addLayer({ id: "hanh-trinh-nhan", type: "symbol", source: "hanh-trinh-duong", ...lop.nhan } as never);
        map.addLayer({
          id: "hanh-trinh-duong-hit",
          type: "line",
          source: "hanh-trinh-duong",
          paint: { "line-color": cbs.current.mau.muc, "line-width": 28, "line-opacity": 0 },
        });
      }
      map.resize();
      cbs.current.onSan?.();
    });
    map.on("click", "hanh-trinh-duong-hit", (e: MapLayerMouseEvent) => {
      const id = e.features?.[0]?.properties?.id;
      if (typeof id === "string") cbs.current.onChonDoan(id);
    });
    map.on("click", (e: MapMouseEvent) => {
      try {
        if (!map.getLayer("hanh-trinh-duong-hit")) {
          cbs.current.onNen();
          return;
        }
        const feats = map.queryRenderedFeatures(e.point, { layers: ["hanh-trinh-duong-hit"] });
        if (feats.length === 0) cbs.current.onNen();
      } catch {
        cbs.current.onNen();
      }
    });
    map.on("contextmenu", (e) => cbs.current.onGhim?.({ lat: e.lngLat.lat, lng: e.lngLat.lng }));
    map.on("dragstart", () => cbs.current.onUserMove());
    map.on("zoomstart", (e) => {
      if (e.originalEvent) cbs.current.onUserMove();
    });
    map.on("rotatestart", (e) => {
      if (e.originalEvent) cbs.current.onUserMove();
    });
    mapRef.current = map;
    setSan(true);
    const ro = typeof ResizeObserver !== "undefined" ? new ResizeObserver(() => {
      map.resize();
      const w = Math.round(el.clientWidth); const h = Math.round(el.clientHeight);
      setKhung((truoc) => (truoc.w === w && truoc.h === h ? truoc : { w, h }));
    }) : null;
    ro?.observe(el);
    requestAnimationFrame(() => map.resize());
    return () => {
      ro?.disconnect();
      markers.current.forEach((m) => m.remove());
      markers.current = [];
      arrows.current.forEach((m) => m.remove());
      arrows.current = [];
      chooser.current?.remove();
      map.remove();
      mapRef.current = null;
      loaded.current = false;
      setSan(false);
    };
    // `kieu` is in the deps: a theme change swaps the basemap, and the map is
    // rebuilt with its sources rather than restyled underneath them.
  }, [host, kieu, webgl2]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    arrows.current.forEach((m) => m.remove());
    // Direction chevrons and the chosen leg's paper tag wait for the ink:
    // mid-drawing they would sit at the tip of a line still being written.
    const dangVe = mocs.some((m) => m.nhip);
    arrows.current = (dangVe ? [] : muiTenDoan(doan)).map((arrow) => {
      const el = document.createElement("div"); el.style.cssText = "width:18px;height:20px";
      const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg"); svg.setAttribute("viewBox", "0 0 18 20");
      for (const [color, width] of [[mau.giay, "6"], [mau.muc, "3"]]) { const path = document.createElementNS("http://www.w3.org/2000/svg", "path"); path.setAttribute("d", "M3 14 L9 5 L15 14"); path.setAttribute("fill", "none"); path.setAttribute("stroke", color); path.setAttribute("stroke-width", width); svg.appendChild(path); }
      el.appendChild(svg); return new Marker({ element: el, rotation: arrow.heading }).setLngLat([arrow.lng, arrow.lat]).addTo(map);
    });
    const ve = () => {
      const src = map.getSource("hanh-trinh-duong");
      if (src && "setData" in src) (src as GeoJSONSource).setData(tapHop(doan));
    };
    if (loaded.current && map.isStyleLoaded()) ve();
    else map.once("load", ve);
    const chon = dangVe ? undefined : doan.find((d) => d.chon && d.the?.length);
    const giua = chon ? giuaDoan(chon.polyline) : null;
    if (chon && giua) {
      const the = document.createElement("div");
      the.setAttribute("role", "note");
      the.style.cssText = [
        "pointer-events:none", "padding:6px 10px", "border-radius:8px", `background:${mau.giay}`,
        `border:1.5px solid ${mau.vien}`, `box-shadow:${mau.bongCao}`, `color:${mau.chu}`,
        "font:700 13px/18px system-ui,sans-serif", "font-variant-numeric:tabular-nums",
        "transform:translateY(-34px) rotate(-2deg)", "white-space:nowrap", "text-align:center",
      ].join(";");
      chon.the!.forEach((dong, i) => {
        const hang = document.createElement("div");
        hang.textContent = dong;
        if (i > 0) hang.style.cssText = `font-weight:600;color:${mau.netChi}`;
        the.appendChild(hang);
      });
      arrows.current.push(new Marker({ element: the, anchor: "center" }).setLngLat([giua.lng, giua.lat]).addTo(map));
    }
  }, [doan, san, mocs.some((m) => m.nhip)]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    const draw = () => {
    // A wheel zoom may settle while a popup button is being pressed. Keep
    // the popup mounted across marker projection updates so the release
    // still reaches that button; selection/background clicks close it.
    markers.current.forEach((m) => m.remove());
    const pending = [...mocs];
    const groups: MocBanDo[][] = [];
    while (pending.length) {
      const first = pending.shift()!;
      const pixel = map.project([first.lng, first.lat]);
      const close = pending.filter((m) => pixel.dist(map.project([m.lng, m.lat])) < 52);
      groups.push([first, ...close]);
      close.forEach((m) => pending.splice(pending.indexOf(m), 1));
    }
    markers.current = groups.map((group) => {
      const moc = group[0];
      const el = veMoc(group.length > 1 ? mocChum(group) : moc, cbs.current.mau);
      if (group.length > 1) {
        const mat = el.querySelector<HTMLElement>("[data-tem]");
        if (mat) mat.textContent = group.length <= 3 ? group.map((m) => m.so).join(" · ") : `${group.length} điểm`;
        el.style.width = "auto";
        el.setAttribute("aria-label", `${group.length} điểm gần nhau: ${group.map((m) => m.so).join(", ")}`);
      }
      el.addEventListener("click", (ev) => {
        ev.stopPropagation();
        chooser.current?.remove();
        if (group.length === 1) { cbs.current.onChonMoc(moc.id); return; }
        // A slip of the notebook's paper, not the stock MapLibre box (B3
        // finish review): pencil edge, paper height 2, a head with its own
        // 48dp close, one row per stop on a hairline, the stop number as the
        // same 26dp stamp the day page's stop bar draws.
        const m = cbs.current.mau;
        const giay = document.createElement("div");
        giay.style.cssText = `background:${m.giay};color:${m.chu};border:1.5px solid ${m.vien};border-radius:8px;box-shadow:${m.bongCao};overflow:hidden;min-width:220px`;
        const dau = document.createElement("div");
        dau.style.cssText = `display:flex;align-items:center;justify-content:space-between;padding-left:14px;border-bottom:1px solid ${m.vien}`;
        const tieuDe = document.createElement("span");
        tieuDe.textContent = `${group.length} điểm gần nhau`;
        tieuDe.style.cssText = "font-size:13px;line-height:18px;font-weight:700";
        const dong = document.createElement("button");
        dong.type = "button";
        dong.dataset.dongCum = "1";
        dong.setAttribute("aria-label", "Đóng danh sách điểm gần nhau");
        dong.style.cssText = `width:48px;height:48px;display:flex;align-items:center;justify-content:center;background:transparent;border:0;cursor:pointer;color:${m.chu}`;
        dong.innerHTML = '<svg width="18" height="18" viewBox="0 0 18 18" aria-hidden="true"><path d="M4 4l10 10M14 4L4 14" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>';
        dong.addEventListener("click", (event) => { event.stopPropagation(); chooser.current?.remove(); });
        dau.append(tieuDe, dong);
        // A popup cannot leave the map: `.maplibregl-map` clips at its box, so
        // anything past that edge is invisible AND unclickable. Measured at
        // 390x844 the map box is only 250px tall, so cap the list to what fits
        // inside it instead of to a constant that happens to fit on a laptop.
        const hopBanDo = map.getContainer().getBoundingClientRect();
        const caoToiDa = Math.max(96, Math.round(hopBanDo.height) - 56 - 48);
        // The choices are the group; the slip's close is not one of them.
        const list = document.createElement("div");
        list.setAttribute("role", "group");
        list.setAttribute("aria-label", "Chọn điểm hẹn gần nhau");
        list.style.cssText = `display:flex;flex-direction:column;max-height:${caoToiDa}px;overflow:auto`;
        group.forEach((stop, i) => {
          const button = document.createElement("button");
          button.type = "button";
          button.setAttribute("aria-label", `${stop.so}, ${stop.gio}, ${stop.tieuDe}`);
          button.style.cssText = `display:flex;align-items:center;gap:10px;min-height:48px;text-align:left;padding:8px 14px;background:transparent;color:${m.chu};border:0;${i > 0 ? `border-top:1px solid ${m.vien};` : ""}font:inherit;cursor:pointer`;
          const so = document.createElement("span");
          so.textContent = String(stop.so);
          so.style.cssText = `min-width:26px;height:26px;box-sizing:border-box;display:inline-flex;align-items:center;justify-content:center;border:1.5px solid ${m.muc};border-radius:6px;color:${m.muc};font-size:13px;font-weight:800;font-variant-numeric:tabular-nums;flex:none`;
          const chu = document.createElement("span");
          chu.style.cssText = "display:flex;flex-direction:column;min-width:0";
          const gio = document.createElement("span");
          gio.textContent = stop.gio;
          gio.style.cssText = "font-size:12px;line-height:16px;opacity:0.8;font-variant-numeric:tabular-nums";
          const ten = document.createElement("span");
          ten.textContent = stop.tieuDe;
          ten.style.cssText = "font-size:14px;line-height:19px;font-weight:600";
          chu.append(gio, ten);
          button.append(so, chu);
          button.addEventListener("click", (event) => { event.stopPropagation(); chooser.current?.remove(); cbs.current.onChonMoc(stop.id); });
          list.appendChild(button);
        });
        giay.append(dau, list);
        // `offset` clears the cluster's own stamp: anchored at its centre, the
        // box covered the stamp's lower edge.
        // Focus goes to the first stop, not to the close in the head: the
        // list is what the keyboard came for.
        chooser.current = new Popup({ closeButton: false, maxWidth: "300px", focusAfterOpen: false, offset: 30 }).setLngLat([moc.lng, moc.lat]).setDOMContent(giay).addTo(map);
        list.querySelector<HTMLElement>("button")?.focus();
        // Escape closes the list like the close button does, and focus goes
        // back to the cluster it came from -- found again by its stops, since
        // the pan below redraws every marker (QA UI-043).
        const nhom = group.map((m) => m.id).join(",");
        const dongBangEsc = (event: KeyboardEvent) => {
          if (event.key !== "Escape") return;
          event.preventDefault();
          event.stopImmediatePropagation();
          chooser.current?.remove();
          map.getContainer().querySelector<HTMLElement>(`[data-nhom="${CSS.escape(nhom)}"]`)?.focus();
        };
        document.addEventListener("keydown", dongBangEsc, true);
        chooser.current.once("close", () => document.removeEventListener("keydown", dongBangEsc, true));
        // Đo trên CI: khung bản đồ y=201..451, mục thứ BA của chooser ở y=465 —
        // NGOÀI bản đồ 14px. `.maplibregl-map` có `overflow: hidden` nên phần
        // thò ra bị xén, và hit-test ở đó trả về một nút của app nằm dưới.
        //
        // Ba lần vá bằng z-index đều vô ích, và chuỗi tổ tiên nói vì sao: popup
        // nằm trong `.maplibregl-map`, mà khung ấy lại nằm trong một
        // `DIV[position:relative, z-index:0]` của react-native-web. Một phần tử
        // như thế TẠO ngữ cảnh xếp lớp, nên mọi z-index bên trong bị nhốt lại và
        // không bao giờ so được với nhánh chứa nút kia. Không con số nào cứu
        // được; chỗ phải sửa là hình học.
        //
        // Nên: dời bản đồ đúng bằng phần tràn, để popup lọt hẳn vào trong khung.
        // `panBy([0, d])` dời nội dung LÊN d pixel, nên tràn đáy dùng dấu dương.
        // Chừa 26px ở đáy cho dải attribution và 8px ở đỉnh.
        const hopPopup = chooser.current.getElement().getBoundingClientRect();
        const khung = map.getContainer().getBoundingClientRect();
        const tranDuoi = hopPopup.bottom - (khung.bottom - 26);
        const tranTren = khung.top + 8 - hopPopup.top;
        const tranPhai = hopPopup.right - (khung.right - 8);
        const tranTrai = khung.left + 8 - hopPopup.left;
        const dichY = tranDuoi > 0 ? tranDuoi : tranTren > 0 ? -tranTren : 0;
        const dichX = tranPhai > 0 ? tranPhai : tranTrai > 0 ? -tranTrai : 0;
        // `moveend` chỉ vẽ lại marker và giữ nguyên popup, nên không có vòng lặp.
        if (dichX || dichY) map.panBy([dichX, dichY], { duration: 0 });
        // Lớp phòng thủ thứ hai: dải attribution nằm trong bản đồ và MapLibre xếp
        // nó trên popup. Nó phải LUÔN NHÌN THẤY -- đó là nghĩa vụ bản quyền -- nên
        // chỉ ngừng nhận con trỏ trong lúc chooser mở, rồi trả lại y như cũ.
        const controls = [...map.getContainer().querySelectorAll<HTMLElement>(".maplibregl-ctrl")];
        const truoc = controls.map((el) => el.style.pointerEvents);
        for (const el of controls) el.style.pointerEvents = "none";
        chooser.current.once("close", () => {
          controls.forEach((el, k) => { el.style.pointerEvents = truoc[k]; });
        });
        // The slip draws its own paper: MapLibre's white box and its tip go.
        const content = chooser.current.getElement().querySelector<HTMLElement>(".maplibregl-popup-content");
        if (content) { content.style.background = "transparent"; content.style.padding = "0"; content.style.boxShadow = "none"; }
        const mui = chooser.current.getElement().querySelector<HTMLElement>(".maplibregl-popup-tip");
        if (mui) mui.style.display = "none";
      });
      // A round chip marks its point at its centre; native does the same.
      el.dataset.moc = moc.id;
      el.dataset.nhom = group.map((m) => m.id).join(",");
      return new Marker({ element: el, anchor: "center" }).setLngLat([moc.lng, moc.lat]).addTo(map);
    });
    };
    draw();
    map.on("moveend", draw);
    return () => { map.off("moveend", draw); chooser.current?.remove(); };
  }, [khoaMoc, san, mau]);

  // The ink reaching a stamp only flips its state: the element stays, so the
  // CSS transition plays instead of a new pin appearing.
  useEffect(() => {
    for (const marker of markers.current) {
      const mat = marker.getElement().querySelector<HTMLElement>("[data-tem]");
      const nhom = marker.getElement().dataset.nhom?.split(",") ?? [];
      const trong = mocs.filter((m) => nhom.includes(m.id));
      const moc = trong.length > 1 ? mocChum(trong) : trong[0];
      if (!mat || !moc) continue;
      const tem = hinhTem(moc, mau);
      mat.dataset.nhip = moc.nhip ?? "";
      mat.style.transform = dangNhip(moc.nhip, tem.nghieng);
      mat.style.boxShadow = moc.nhip === "cho" ? mau.bongCao : tem.bong;
    }
  }, [mocs, mau]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !san || fitDem === 0) return;
    const hop = (fitPoints?.length ? hopGioi(fitPoints) : hopHanhTrinh(mocs, doan));
    if (!hop) {
      map.easeTo({ center: [TAM_DA_LAT.lng, TAM_DA_LAT.lat], zoom: 12, duration });
      return;
    }
    map.fitBounds(
      [
        [hop[0], hop[1]],
        [hop[2], hop[3]],
      ],
      { padding: demVua, duration },
    );
    // Fit Journey is the only yank; selection uses easeTo, pan stays free.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mocs is read at the tick of fitDem
  }, [fitDem, cameraKey, demVua, san, khung.w, khung.h]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !toi) return;
    // Same reasons as native: the panel owns the bottom of the map, and the
    // stop is shown with the stops before and after it.
    const hop = toi.ke?.length ? hopGioi([toi, ...toi.ke]) : null;
    if (hop) map.fitBounds([[hop[0], hop[1]], [hop[2], hop[3]]], { padding: demVua, duration });
    else map.easeTo({ center: [toi.lng, toi.lat], zoom: Math.max(map.getZoom(), 14), duration, padding: demVua });
  }, [toi?.dem, demVua, san]);

  return (
    <View collapsable={false} style={[styles.fill, { backgroundColor: mauNen }]} testID="ban-do-hanh-trinh">
      {webgl2 ? createElement("div", {
          id: "ban-do-hanh-trinh",
          ref: (node: HTMLDivElement | null) => {
            setHost(node);
          },
          style: { position: "absolute", top: 0, right: 0, bottom: 0, left: 0, backgroundColor: mauNen },
        }) : (
          <View accessibilityLabel="Bản đồ không khả dụng" style={styles.fallback}>
            <View style={[styles.routeLine, { backgroundColor: colors.line }]}>
              <View style={[styles.routeDot, { backgroundColor: colors.accent }]} />
              <View style={[styles.routeDot, { backgroundColor: colors.accent }]} />
            </View>
            <View style={styles.fallbackCopy}>
              <Text style={[typography.h2, { color: colors.ink }]}>
                {webgl2 === null ? "Đang mở hành trình" : "Đường đi vẫn ở đây"}
              </Text>
              <Text style={[typography.note, { color: colors.inkSoft }]}>
                {webgl2 === null
                  ? "Đang chuẩn bị bản đồ cho trang ngày của hội."
                  : "Thiết bị chưa hiển thị được bản đồ. Chọn mốc trong trang ngày bên dưới để xem giờ và địa điểm."}
              </Text>
              {webgl2 === false ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{mocs.length} mốc có vị trí</Text> : null}
            </View>
          </View>
        )}
    </View>
  );
}

const styles = StyleSheet.create({
  // As tall as the room it is given, never more: a 220 floor let the map hang
  // under the day page with its credit line (B3 probe, empty day at 390×844).
  fill: { flex: 1, minHeight: 0, position: "relative" },
  fallback: { flex: 1, flexDirection: "row", alignItems: "center", paddingHorizontal: 28, paddingVertical: 24, gap: 24 },
  fallbackCopy: { flex: 1, gap: 10, maxWidth: 400 },
  routeLine: { width: 2, height: 100, justifyContent: "space-between", alignItems: "center" },
  routeDot: { width: 12, height: 12, borderRadius: 6 },
});
