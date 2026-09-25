import { useState } from "react";
import { Alert, Pressable, ScrollView, StyleSheet, Text, TextInput, View } from "react-native";

import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Nep } from "../ui/art/Nep";
import { RudiButton } from "../ui";
import { anhDanhMuc, MediaSlot } from "../ui/MediaSlot";
import { Sheet } from "../ui/Sheet";
import { cauPhienDiKem, gomPhien, nepDuocHoi } from "./hoi";
import { cauNguCanh } from "./phieu";
import { useNep } from "./NepProvider";
import { useNepAnh } from "./useNepAnh";
import { useNepHoi } from "./useNepHoi";

/**
 * The panel Nếp talks in.
 *
 * It reuses `ui/Sheet.tsx` rather than a route, because the assistant is not a
 * place in the app: Back, Escape and a drag on the handle all close it, and the
 * screen underneath never navigates.
 *
 * The first block is «what Nếp can see», printed from the context slip the open
 * screen declared. That is not decoration. `CLAUDE.md` forbids the assistant
 * reading chat, taste or history on its own, so the honest way to be
 * context-aware is to show the person the whole of what was shared and nothing
 * more. If the line looks thin, that is the point: it is the real payload.
 *
 * The block's second line counts the turns of this panel session that go with
 * the next question (ADR-0036 §2.5, §2.7): the preview is built from the very
 * list `goiNep` sends, not re-derived from the screen. The session lives in
 * `useNepHoi`'s state and ends when the panel closes. The answer comes back to
 * this person alone and is shown here, never in any room (§2.8).
 */

export function NepBang({ open, onClose }: { open: boolean; onClose(): void }) {
  const { phieu } = useNep();
  const { colors } = useRudiTheme();
  const { cheDo, nguon } = useRudiSession();
  const buc = useNepAnh(nguon.kieu === "live" ? nguon.actorId : null);
  const [nhap, datNhap] = useState("");
  const phien = useNepHoi(nguon.kieu === "live" ? nguon.actorId : null, phieu, open);
  const duocHoi = nepDuocHoi(phieu);
  // Exactly the turns the next send carries, so the count cannot drift from the payload.
  const soLuotDiKem = gomPhien(phien.luot, nhap.trim()).length;
  const guiCau = async () => {
    const cau = nhap.trim();
    if (!cau || phien.dangHoi || !duocHoi) return;
    if (await phien.hoi(cau)) datNhap("");
  };

  const goiY = phieu?.goiY ?? [];

  return (
    <Sheet accessibilityLabel="Nếp" onClose={onClose} open={open} testID="nep-bang">
      <View style={styles.dau}>
        <Nep gap="trang" pose="gop-y" size={48} />
        <View style={styles.dauChu}>
          <Text style={[typography.title, { color: colors.ink }]}>Nếp</Text>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            {cheDo === "live" ? "Trợ lý riêng của bạn" : "Chế độ trải nghiệm"}
          </Text>
        </View>
      </View>

      <View style={[styles.the, { backgroundColor: colors.aiSoft, borderColor: colors.ai }]}>
        {/* `aiInk` is ink ON the solid ai colour (white in light mode); on `aiSoft`
            it vanished. `ai` reads on `aiSoft` in both themes. */}
        <Text style={[typography.label, { color: colors.ai }]}>Mình đang thấy</Text>
        <Text style={[typography.body, { color: colors.ink }]} testID="nep-ngu-canh">
          {cauNguCanh(phieu)}
        </Text>
        <Text style={[typography.caption, { color: colors.inkSoft }]} testID="nep-phien-di-kem">
          {cauPhienDiKem(soLuotDiKem)}
        </Text>
      </View>

      {goiY.length > 0 ? (
        <ScrollView contentContainerStyle={styles.goiY} horizontal showsHorizontalScrollIndicator={false}>
          {goiY.map((g) => (
            <Pressable
              accessibilityRole="button"
              key={g}
              onPress={() => datNhap(g)}
              style={[styles.chip, { borderColor: colors.lineStrong, backgroundColor: colors.paper }]}
            >
              <Text style={[typography.caption, { color: colors.ink }]}>{g}</Text>
            </Pressable>
          ))}
        </ScrollView>
      ) : null}

      {phien.luot.length > 0 ? (
        <View style={styles.phien} testID="nep-phien">
          {phien.luot.map((l, i) => (
            <Text
              // The session only ever grows at the end, so the index is stable.
              key={i}
              style={[
                typography.body,
                l.vai === "toi" ? styles.cauHoi : null,
                { color: l.vai === "toi" ? colors.inkSoft : colors.ink },
              ]}
              testID={l.vai === "nep" ? "nep-tra-loi" : undefined}
            >
              {l.chu}
            </Text>
          ))}
        </View>
      ) : null}

      {phien.dangHoi ? (
        <Text style={[typography.body, styles.loi, { color: colors.inkSoft }]} testID="nep-dang-nghi">
          Nếp đang nghĩ…
        </Text>
      ) : null}

      {phien.loi ? (
        <Text style={[typography.body, styles.loi, { color: colors.ink }]} testID="nep-loi-hoi">
          {phien.loi}
        </Text>
      ) : null}

      {!duocHoi ? (
        <Text style={[typography.body, styles.loi, { color: colors.inkSoft }]} testID="nep-im-man-tien">
          Ở màn tiền Nếp không trả lời. Ra màn khác rồi hỏi nhé.
        </Text>
      ) : null}

      {buc.dangCho ? (
        <Text style={[typography.body, styles.loi, { color: colors.inkSoft }]} testID="nep-dang-ve">
          Mình đang vẽ, tầm hai phút. Bạn cứ làm việc khác, xong mình báo.
        </Text>
      ) : null}

      {buc.loi ? (
        <Text style={[typography.body, styles.loi, { color: colors.ink }]} testID="nep-loi-ve">
          {buc.loi}
        </Text>
      ) : null}

      {buc.duongAnh ? (
        <Pressable accessibilityRole="button" onPress={buc.dep} style={styles.khungAnh}>
          {/* Qua `MediaSlot` chứ không phải một `<Image>` trần: ADR-0017 §2.5 nói
              một tấm ảnh không bao giờ đi mà thiếu xuất xứ, và ảnh này CÓ xuất
              xứ đáng nói. Nó do máy vẽ, và mọi ảnh Gemini sinh ra đều mang dấu
              SynthID không tắt được, nên người xem đáng được biết cả hai điều
              đó ngay cạnh bức tranh chứ không phải trong một trang trợ giúp. */}
          <MediaSlot
            alt="Bức Nếp vừa vẽ"
            contentFit="contain"
            nguon={{
              loai: "danh-muc",
              anh: anhDanhMuc(
                { uri: buc.duongAnh },
                { author: "Nếp", license: "AI vẽ, có dấu SynthID" },
              ),
            }}
            ratio={1}
            testID="nep-anh-ve"
          />
        </Pressable>
      ) : null}

      <View style={[styles.soan, { borderColor: colors.line }]}>
        <TextInput
          accessibilityLabel="Hỏi Nếp"
          onChangeText={datNhap}
          onSubmitEditing={() => void guiCau()}
          placeholder="Hỏi Nếp một câu"
          placeholderTextColor={colors.inkFaint}
          returnKeyType="send"
          style={[typography.body, styles.o, { color: colors.ink }]}
          testID="nep-o-nhap"
          value={nhap}
        />
        {/* The kit button, not a hand-rolled one: it resolves its own ink from
            the tone (`colors[`${tone}Ink`]`), and `tests/rudi-khong-hex.test.mjs`
            allows no file but `theme.ts` to spell a colour. */}
        {/* Vẽ là tool vòng 2 trong bảng quyền: nó GHI và nó tốn một lượt quota
            thật, nên hỏi mỗi lần, không nhớ câu trả lời trước. */}
        <RudiButton
          compact
          disabled={!nhap.trim() || buc.dangCho}
          full={false}
          label="Vẽ"
          loading={buc.dangCho}
          onPress={() => {
            const moTa = nhap.trim();
            if (!moTa) return;
            Alert.alert(
              "Nhờ Nếp vẽ?",
              `Mình sẽ vẽ «${moTa}». Mất tầm hai phút.`,
              [
                { text: "Thôi", style: "cancel" },
                { text: "Vẽ đi", onPress: () => void buc.nhoVe(moTa) },
              ],
            );
          }}
          tone="ai"
          variant="outline"
        />
        <RudiButton
          compact
          disabled={!nhap.trim() || phien.dangHoi || !duocHoi}
          full={false}
          label="Gửi"
          loading={phien.dangHoi}
          onPress={() => void guiCau()}
          tone="ai"
        />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({
  dau: { flexDirection: "row", alignItems: "center", gap: 12, marginBottom: 16 },
  dauChu: { flex: 1 },
  the: { borderRadius: 14, borderWidth: StyleSheet.hairlineWidth, padding: 12, gap: 4 },
  goiY: { gap: 8, paddingVertical: 12 },
  chip: { borderRadius: 999, borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 12, paddingVertical: 8 },
  loi: { marginTop: 12 },
  phien: { gap: 8, marginTop: 12 },
  cauHoi: { alignSelf: "flex-end", textAlign: "right" },
  khungAnh: { marginTop: 12, borderRadius: 14, overflow: "hidden" },
  soan: { flexDirection: "row", alignItems: "center", gap: 8, marginTop: 16, borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 12 },
  // Two compact buttons share this row with the input. Buttons default to
  // `full` (width 100%, no shrink), which squeezed the input to zero width on
  // Android (QA 23/09); they opt out above, and the input keeps a floor.
  o: { flex: 1, minWidth: 120, paddingVertical: 10 },
});
