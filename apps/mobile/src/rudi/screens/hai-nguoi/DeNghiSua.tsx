import { useEffect, useMemo, useState } from "react";
import { ScrollView, StyleSheet, Text, View } from "react-native";

import { docDanhMuc } from "../../kham-pha/dia-diem";
import { homNay } from "../../keo/nhip-keo";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { ngayDocDuoc } from "../../to-giay/ngay";
import { chuanGio, loiGio, loiThuTu, loiViec, ngayChonDuoc, ngayNgan } from "../../to-giay/sua-to";
import { type NoiDungTo, type ToGiay, khacGi, phienBan } from "../../to-giay/to-giay";
import { useTenCho } from "../../to-giay/useTenCho";
import { Chip, Heading, ListRow, RudiButton } from "../../ui";
import { Field } from "../../ui/Field";
import { Sheet } from "../../ui/Sheet";

/**
 * «Đề nghị sửa»: a new version, not an edit (spec §3.3 rule 3). The sheet
 * starts from the current version's words, and before the person sends it
 * shows exactly what changed («Giờ chỗ chính: 18:30 → 19:00») from `khacGi`,
 * the same function the receiving side reads it with. Sending it means the
 * proposer has agreed to it; the other person now answers.
 *
 * On a person's own draft (`nhap`) the same sheet edits the draft: nothing is
 * sent and no version is made, so it says «Sửa bản phác / Lưu bản phác» --
 * «Gửi phiên bản 2» on a draft promised a send that never happened (QA 23/09).
 *
 * The day is chosen among chips of this week and the next, not typed as ISO;
 * the hours are checked as the server will check them (`to-giay/sua-to.ts`);
 * and the place a stop points at is kept, shown, and can be changed from the
 * catalogue -- it used to be dropped silently whenever the line was edited.
 */
export function DeNghiSua({ to, open, onClose, onGui, testID }: { to: ToGiay; open: boolean; onClose: () => void; onGui: (content: NoiDungTo, lyDo: string | null) => void; testID?: string }) {
  const { colors, space } = useRudiTheme();
  const { phien } = useRudiSession();
  const nhap = to.state === "nhap";
  const pb = phienBan(to);
  const chinh = pb?.content.chang[0], tiep = pb?.content.chang[1];
  const [ngay, setNgay] = useState(pb?.content.ngay ?? "");
  const [gio1, setGio1] = useState(chinh?.gio ?? "");
  const [viec1, setViec1] = useState(chinh?.viec ?? "");
  const [cho1, setCho1] = useState<string | null>(chinh?.place_id ?? null);
  const [gio2, setGio2] = useState(tiep?.gio ?? "");
  const [viec2, setViec2] = useState(tiep?.viec ?? "");
  const [cho2, setCho2] = useState<string | null>(tiep?.place_id ?? null);
  const [lyDo, setLyDo] = useState("");
  const [chonCho, setChonCho] = useState<0 | 1 | null>(null);
  const [danhMuc, setDanhMuc] = useState<{ id: string; name: string }[]>([]);

  useEffect(() => {
    if (!open) return;
    setNgay(pb?.content.ngay ?? "");
    setGio1(chinh?.gio ?? "");
    setViec1(chinh?.viec ?? "");
    setCho1(chinh?.place_id ?? null);
    setGio2(tiep?.gio ?? "");
    setViec2(tiep?.viec ?? "");
    setCho2(tiep?.place_id ?? null);
    setLyDo("");
    setChonCho(null);
  }, [open, pb?.version]); // eslint-disable-line react-hooks/exhaustive-deps

  // The catalogue, read the first time a place is being chosen.
  useEffect(() => {
    if (chonCho === null || danhMuc.length > 0) return;
    let song = true;
    void docDanhMuc({ personId: phien?.person_id ?? null })
      .then((dm) => {
        if (song) setDanhMuc(dm.places.map((p) => ({ id: p.id, name: p.name })));
      })
      .catch(() => undefined);
    return () => {
      song = false;
    };
  }, [chonCho, danhMuc.length, phien?.person_id]);

  const tenCho = useTenCho([cho1, cho2, chinh?.place_id, tiep?.place_id]);
  const tenCua = (id: string) => tenCho[id] ?? danhMuc.find((p) => p.id === id)?.name;

  const noiDung = useMemo<NoiDungTo>(() => {
    const chang = [];
    if (gio1.trim() || viec1.trim()) chang.push({ gio: chuanGio(gio1), viec: viec1.trim(), place_id: cho1, can_kiem: true });
    if (gio2.trim() || viec2.trim()) chang.push({ gio: chuanGio(gio2), viec: viec2.trim(), place_id: cho2, can_kiem: true });
    return { ngay: ngay.trim(), chang };
  }, [ngay, gio1, viec1, cho1, gio2, viec2, cho2]);

  const loi1 = loiGio(chuanGio(gio1), true);
  const loi2 = loiGio(chuanGio(gio2), viec2.trim() !== "") ?? loiThuTu(chuanGio(gio1), chuanGio(gio2));
  const doi = pb
    ? khacGi({ ...pb, version: pb.version + 1, content: noiDung, ly_do: lyDo.trim() || null }, pb, tenCua)
    : [];
  const loiViec1 = loiViec(viec1, gio1);
  const loiViec2 = loiViec(viec2, gio2);
  const guiDuoc = doi.length > 0 && noiDung.chang.length > 0 && noiDung.chang.every((c) => c.viec.length > 0) && !loi1 && !loi2 && !loiViec1 && !loiViec2;
  const ngayDuoc = ngayChonDuoc(to.tuan, homNay(), ngay);

  const oCho = (i: 0 | 1, id: string | null, dat: (v: string | null) => void) => (
    <View style={styles.cho}>
      {id ? (
        <Text numberOfLines={1} style={[typography.caption, styles.tenCho, { color: colors.inkSoft }]}>
          Ở {tenCua(id) ?? "một chỗ trong danh mục"}
        </Text>
      ) : null}
      <RudiButton
        compact
        full={false}
        label={id ? "Đổi chỗ" : "Chọn chỗ"}
        onPress={() => setChonCho(chonCho === i ? null : i)}
        variant="ghost"
      />
      {id ? <RudiButton compact full={false} label="Bỏ chỗ" onPress={() => dat(null)} variant="ghost" /> : null}
    </View>
  );

  return (
    <Sheet accessibilityLabel={nhap ? "Sửa bản phác" : "Đề nghị sửa tờ giấy"} onClose={onClose} open={open} testID={testID ?? "de-nghi-sua"}>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading
          size="h2"
          subtitle={
            nhap
              ? "Chỉ bạn thấy. Lưu xong vẫn là bản phác, gửi ở tờ giấy."
              : `Sửa gì thì thành phiên bản ${to.version + 1}. Người ấy sẽ thấy đúng chỗ đổi.`
          }
          title={nhap ? "Sửa bản phác" : "Đề nghị sửa"}
        />
        <View style={styles.khoi}>
          <Text style={[typography.label, { color: colors.ink }]}>Ngày</Text>
          <ScrollView contentContainerStyle={styles.hangChip} horizontal showsHorizontalScrollIndicator={false} testID="de-nghi-sua-ngay">
            {ngayDuoc.map((d) => (
              <Chip accessibilityLabel={ngayDocDuoc(d)} key={d} label={ngayNgan(d)} onPress={() => setNgay(d)} selected={d === ngay} />
            ))}
          </ScrollView>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>{ngayDocDuoc(ngay)}</Text>
        </View>
        <View style={styles.hang}>
          <View style={styles.gio}>
            <Field
              accessibilityLabel="Giờ chỗ chính"
              error={gio1.trim() ? loi1 : null}
              keyboardType="numbers-and-punctuation"
              label="Giờ"
              maxLength={5}
              onBlur={() => setGio1(chuanGio(gio1))}
              onChangeText={setGio1}
              placeholder="hh:mm"
              testID="de-nghi-sua-gio"
              value={gio1}
            />
          </View>
          <View style={styles.viec}>
            <Field error={loiViec1} label="Chỗ chính" onChangeText={setViec1} placeholder="Ăn tối, một quán chưa đi" testID="de-nghi-sua-viec" value={viec1} />
          </View>
        </View>
        {oCho(0, cho1, setCho1)}
        <View style={styles.hang}>
          <View style={styles.gio}>
            <Field
              accessibilityLabel="Giờ đi tiếp"
              error={gio2.trim() ? loi2 : null}
              keyboardType="numbers-and-punctuation"
              label="Giờ"
              maxLength={5}
              onBlur={() => setGio2(chuanGio(gio2))}
              onChangeText={setGio2}
              placeholder="hh:mm"
              value={gio2}
            />
          </View>
          <View style={styles.viec}>
            <Field error={loiViec2} label="Đi tiếp (tuỳ chọn)" onChangeText={setViec2} placeholder="Đi bộ, rồi chè" testID="de-nghi-sua-viec-2" value={viec2} />
          </View>
        </View>
        {viec2.trim() || cho2 ? oCho(1, cho2, setCho2) : null}
        {chonCho !== null ? (
          <View style={[styles.danhMuc, { borderColor: colors.line }]} testID="de-nghi-sua-danh-muc">
            <Text style={[typography.caption, { color: colors.inkSoft }]}>
              {chonCho === 0 ? "Chỗ cho chặng chính" : "Chỗ cho chặng đi tiếp"} · từ Khám phá
            </Text>
            {danhMuc.length === 0 ? <Text style={[typography.caption, { color: colors.inkFaint }]}>Đang đọc danh mục…</Text> : null}
            {danhMuc.slice(0, 12).map((p) => (
              <ListRow
                icon="location-outline"
                key={p.id}
                onPress={() => {
                  (chonCho === 0 ? setCho1 : setCho2)(p.id);
                  setChonCho(null);
                }}
                title={p.name}
              />
            ))}
          </View>
        ) : null}
        <Field label="Vì sao đổi (tuỳ chọn)" multiline onChangeText={setLyDo} placeholder="Tối thứ Bảy mưa." value={lyDo} />
        {doi.length > 0 ? (
          <View style={[styles.doi, { borderLeftColor: colors.lineStrong }]} testID="de-nghi-sua-khac-gi">
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{nhap ? "Bản phác sẽ đổi:" : "Người ấy sẽ thấy:"}</Text>
            {doi.map((d) => (
              <Text key={d} style={[typography.label, { color: colors.ink }]}>· {d}</Text>
            ))}
          </View>
        ) : (
          <Text style={[typography.caption, { color: colors.inkSoft }]}>{nhap ? "Chưa đổi gì." : "Chưa đổi gì. Sửa một dòng ở trên thì gửi được."}</Text>
        )}
        <RudiButton
          disabled={!guiDuoc}
          label={nhap ? "Lưu bản phác" : `Gửi phiên bản ${to.version + 1}`}
          onPress={() => onGui(noiDung, lyDo.trim() || null)}
        />
        <RudiButton label="Thôi" onPress={onClose} variant="ghost" />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({
  noiDung: { paddingBottom: 8 },
  khoi: { gap: 7 },
  hangChip: { gap: 8, paddingVertical: 2 },
  hang: { flexDirection: "row", gap: 10, alignItems: "flex-start" },
  gio: { width: 104 },
  viec: { flex: 1 },
  cho: { flexDirection: "row", alignItems: "center", gap: 4, flexWrap: "wrap", marginTop: -6 },
  tenCho: { flexShrink: 1 },
  danhMuc: { borderWidth: StyleSheet.hairlineWidth, borderRadius: 12, padding: 10, gap: 2 },
  doi: { borderLeftWidth: StyleSheet.hairlineWidth, paddingLeft: 12, gap: 2 },
});
