/** Web MapLibre GL. Native override: BanDo.native.tsx. */

import { createElement, useEffect, useRef, useState } from "react";
import { StyleSheet, View } from "react-native";
import { GeoJSONSource, Map, Marker, NavigationControl, Popup, type MapLayerMouseEvent, type MapMouseEvent } from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";

import { TAM_DA_LAT } from "./toa-do-mau";
import { DEM_KHOP, hopGioi, hopHanhTrinh, muiTenDoan, tapHop, type BanDoProps, type MocBanDo } from "./kieu-ban-do";

function veMoc(moc: MocBanDo, mauMoc: readonly string[], mauInk: string, mauChon: string, mauVien: string): HTMLElement {
  const el = document.createElement("button");
  el.type = "button";
  // Named chips in ManHinhHanhTrinh carry the stop title; pins are numbered so
  // clickLabel is not ambiguous and VoiceOver does not hear each stop twice.
  el.setAttribute("aria-label", `Mốc ${moc.so}`);
  el.setAttribute("role", "button");
  const mau = moc.chon ? mauChon : (mauMoc[(moc.so - 1) % mauMoc.length] ?? mauChon);
  const co = moc.chon ? 52 : 48;
  el.style.cssText = [
    `width:${co}px`,
    `height:${co}px`,
    "border-radius:999px",
    `background:${mau}`,
    `color:${mauInk}`,
    `border:2px solid ${mauVien}`,
    "display:flex",
    "flex-direction:column",
    "align-items:center",
    "justify-content:center",
    "font:700 13px/1 system-ui,sans-serif",
    "padding:0",
    "cursor:pointer",
  ].join(";");
  const so = document.createElement("span");
  so.textContent = String(moc.so);
  el.appendChild(so);
  return el;
}

export function BanDo({
  mocs,
  doan,
  mauMoc,
  mauMocInk,
  mauMocChon,
  mauDuong,
  mauDuongMo,
  mauVien,
  mauVienDuong,
  mauNen,
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
}: BanDoProps) {
  const cbs = useRef({ onUserMove, onChonMoc, onChonDoan, onNen, onGhim, mauMoc, mauMocInk, mauMocChon, mauVien, mauVienDuong, mauDuong, mauDuongMo });
  cbs.current = { onUserMove, onChonMoc, onChonDoan, onNen, onGhim, mauMoc, mauMocInk, mauMocChon, mauVien, mauVienDuong, mauDuong, mauDuongMo };
  const mapRef = useRef<Map | null>(null);
  const markers = useRef<Marker[]>([]);
  const arrows = useRef<Marker[]>([]);
  const chooser = useRef<Popup | null>(null);
  const loaded = useRef(false);
  const [host, setHost] = useState<HTMLDivElement | null>(null);
  const [san, setSan] = useState(false);

  useEffect(() => {
    const el = host;
    if (!el) return;
    const map = new Map({
      container: el,
      style: kieu.startsWith("{") ? JSON.parse(kieu) : kieu,
      center: [TAM_DA_LAT.lng, TAM_DA_LAT.lat],
      zoom: 12,
      attributionControl: { compact: true },
    });
    map.addControl(new NavigationControl({ showCompass: true, showZoom: false, visualizePitch: true }), "top-right");
    map.on("load", () => {
      loaded.current = true;
      const mau = cbs.current;
      if (!map.getSource("hanh-trinh-duong")) {
        map.addSource("hanh-trinh-duong", { type: "geojson", data: tapHop([]) });
        // Casing first, then the line: an ink route on paper, not a road.
        map.addLayer({
          id: "hanh-trinh-duong-vien",
          type: "line",
          filter: ["!=", ["get", "uocLuong"], 1],
          source: "hanh-trinh-duong",
          layout: { "line-cap": "round", "line-join": "round" },
          paint: {
            "line-color": mau.mauVienDuong,
            "line-width": ["case", ["==", ["get", "chon"], 1], 11, 8],
            "line-opacity": 0.9,
          },
        });
        map.addLayer({
          id: "hanh-trinh-duong",
          type: "line",
          filter: ["!=", ["get", "uocLuong"], 1],
          source: "hanh-trinh-duong",
          layout: { "line-cap": "round", "line-join": "round" },
          paint: {
            "line-color": ["case", ["==", ["get", "chon"], 1], mau.mauDuong, mau.mauDuongMo],
            "line-width": ["case", ["==", ["get", "chon"], 1], 6, 4],
          },
        });
        map.addLayer({
          id: "hanh-trinh-net-noi",
          type: "line",
          source: "hanh-trinh-duong",
          filter: ["==", ["get", "uocLuong"], 1],
          paint: { "line-color": mau.mauDuongMo, "line-width": 2, "line-dasharray": [2, 3] },
        });
        map.addLayer({
          id: "hanh-trinh-duong-hit",
          type: "line",
          source: "hanh-trinh-duong",
          paint: { "line-color": mau.mauDuong, "line-width": 28, "line-opacity": 0 },
        });
      }
      map.resize();
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
    const ro = typeof ResizeObserver !== "undefined" ? new ResizeObserver(() => map.resize()) : null;
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
  }, [host, kieu]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    arrows.current.forEach((m) => m.remove());
    arrows.current = muiTenDoan(doan).map((arrow) => {
      const el = document.createElement("div"); el.style.cssText = "width:18px;height:20px";
      const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg"); svg.setAttribute("viewBox", "0 0 18 20");
      for (const [color, width] of [[mauVienDuong, "6"], [mauDuong, "3"]]) { const path = document.createElementNS("http://www.w3.org/2000/svg", "path"); path.setAttribute("d", "M3 14 L9 5 L15 14"); path.setAttribute("fill", "none"); path.setAttribute("stroke", color); path.setAttribute("stroke-width", width); svg.appendChild(path); }
      el.appendChild(svg); return new Marker({ element: el, rotation: arrow.heading }).setLngLat([arrow.lng, arrow.lat]).addTo(map);
    });
    const ve = () => {
      const src = map.getSource("hanh-trinh-duong");
      if (src && "setData" in src) (src as GeoJSONSource).setData(tapHop(doan));
    };
    if (loaded.current && map.isStyleLoaded()) ve();
    else map.once("load", ve);
  }, [doan, san]);

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
      const el = veMoc(moc, cbs.current.mauMoc, cbs.current.mauMocInk, cbs.current.mauMocChon, cbs.current.mauVien);
      if (group.length > 1) {
        el.textContent = group.length <= 3 ? group.map((m) => m.so).join(" · ") : `${group.length} điểm`;
        el.style.minWidth = "52px";
        el.style.width = "auto";
        el.style.padding = "0 8px";
        el.setAttribute("aria-label", `${group.length} điểm gần nhau: ${group.map((m) => m.so).join(", ")}`);
      }
      el.addEventListener("click", (ev) => {
        ev.stopPropagation();
        chooser.current?.remove();
        if (group.length === 1) { cbs.current.onChonMoc(moc.id); return; }
        const list = document.createElement("div");
        list.setAttribute("role", "group");
        list.setAttribute("aria-label", "Chọn điểm hẹn gần nhau");
        // A popup cannot leave the map: `.maplibregl-map` clips at its box, so
        // anything past that edge is invisible AND unclickable. Measured at
        // 390x844 the map box is only 250px tall, so cap the list to what fits
        // inside it instead of to a constant that happens to fit on a laptop.
        const hopBanDo = map.getContainer().getBoundingClientRect();
        const caoToiDa = Math.max(120, Math.round(hopBanDo.height) - 56);
        list.style.cssText = `display:flex;flex-direction:column;max-height:${caoToiDa}px;overflow:auto;background:${mauNen};padding:8px`;
        for (const stop of group) {
          const button = document.createElement("button");
          button.type = "button";
          button.textContent = `${stop.so} · ${stop.gio} · ${stop.tieuDe}`;
          button.style.cssText = `min-height:48px;text-align:left;padding:8px;background:${mauNen};color:${cbs.current.mauDuong};border:0;font:inherit;cursor:pointer`;
          button.addEventListener("click", (event) => { event.stopPropagation(); chooser.current?.remove(); cbs.current.onChonMoc(stop.id); });
          list.appendChild(button);
        }
        chooser.current = new Popup({ closeButton: true, maxWidth: "280px", focusAfterOpen: true }).setLngLat([moc.lng, moc.lat]).setDOMContent(list).addTo(map);
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
        const content = chooser.current.getElement().querySelector<HTMLElement>(".maplibregl-popup-content");
        if (content) { content.style.background = mauNen; content.style.color = cbs.current.mauDuong; }
      });
      // A round chip marks its point at its centre; native does the same.
      return new Marker({ element: el, anchor: "center" }).setLngLat([moc.lng, moc.lat]).addTo(map);
    });
    };
    draw();
    map.on("moveend", draw);
    return () => { map.off("moveend", draw); chooser.current?.remove(); };
  }, [mocs, san, mauNen]);

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
      { padding, duration },
    );
    // Fit Journey is the only yank; selection uses easeTo, pan stays free.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mocs is read at the tick of fitDem
  }, [fitDem, cameraKey, padding, san]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !toi) return;
    // Same reason as native: the panel owns the bottom of the map.
    map.easeTo({ center: [toi.lng, toi.lat], zoom: Math.max(map.getZoom(), 14), duration, padding });
  }, [toi?.dem, padding, san]);

  return (
    <View collapsable={false} style={[styles.fill, { backgroundColor: mauNen }]} testID="ban-do-hanh-trinh">
      {createElement("div", {
        id: "ban-do-hanh-trinh",
        ref: (node: HTMLDivElement | null) => {
          setHost(node);
        },
        style: { position: "absolute", top: 0, right: 0, bottom: 0, left: 0, backgroundColor: mauNen },
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1, minHeight: 220, position: "relative" },
});
