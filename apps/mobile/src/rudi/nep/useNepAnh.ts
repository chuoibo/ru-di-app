import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AppState } from "react-native";

import { ApiError, BASE_URL, newAttempt } from "../../api";
import { conPhaiHoi, docTrangThai, duongFile, nguonAnhNep, nhipHoiMs, xinAnh, type TrangThaiMedia } from "./media";

/**
 * Một lượt Nếp vẽ ảnh, từ lúc xin tới lúc có bytes.
 *
 * Đo thật: 99 giây một tấm. Nên đây là hàng đợi chứ không phải lời gọi, và hook
 * này chỉ làm ba việc: xin một `job_id`, hỏi lại theo nhịp tăng dần, rồi dừng.
 *
 * Ba luật, và cả ba đều là luật phủ định:
 *   - không hỏi khi app ở nền. Một màn bị che vẫn gõ cửa máy chủ mỗi hai giây
 *     là cách nhanh nhất để một tính năng hiếm dùng thành một hoá đơn thường xuyên;
 *   - không nhận kết quả của lượt cũ. Người dùng đổi ý và xin bức khác thì bức
 *     trước không được phép ghi đè bức sau, nên mỗi lượt mang một số thứ tự;
 *   - không im lặng khi hỏng. Hết hạn mức, mô tả không dùng được, máy chủ chưa
 *     bật tính năng — cả ba đều phải thành một câu người đọc.
 */
export interface LuotVeAnh {
  trangThai: TrangThaiMedia | "chua-bat-dau";
  jobId: string | null;
  duongAnh: string | null;
  /** What `MediaSlot` loads: the file route plus the caller's headers. */
  nguonAnh: { uri: string; headers: Record<string, string> } | null;
  loi: string | null;
  dangCho: boolean;
  /** `man` is the open screen, so the server can refuse a money screen. */
  nhoVe(moTa: string, man?: string): Promise<void>;
  dep(): void;
}

export function useNepAnh(actorId: string | null): LuotVeAnh {
  const [trangThai, datTrangThai] = useState<TrangThaiMedia | "chua-bat-dau">("chua-bat-dau");
  const [jobId, datJobId] = useState<string | null>(null);
  const [duongAnh, datDuongAnh] = useState<string | null>(null);
  const [loi, datLoi] = useState<string | null>(null);

  // Số thứ tự lượt: mọi kết quả về muộn của lượt cũ đều bị bỏ.
  const doi = useRef(0);
  const hen = useRef<ReturnType<typeof setTimeout> | null>(null);

  const dungHoi = useCallback(() => {
    if (hen.current) clearTimeout(hen.current);
    hen.current = null;
  }, []);

  useEffect(() => dungHoi, [dungHoi]);

  const dep = useCallback(() => {
    doi.current += 1;
    dungHoi();
    datTrangThai("chua-bat-dau");
    datJobId(null);
    datDuongAnh(null);
    datLoi(null);
  }, [dungHoi]);

  const nhoVe = useCallback(
    async (moTa: string, man?: string) => {
      if (!actorId) {
        datLoi("Bản trải nghiệm chưa vẽ được. Đăng nhập rồi thử lại nhé.");
        return;
      }
      const luot = ++doi.current;
      dungHoi();
      datLoi(null);
      datDuongAnh(null);
      datTrangThai("dang-cho");

      let job: string;
      try {
        job = (await xinAnh(actorId, newAttempt(), moTa, { man })).job_id;
      } catch (error) {
        if (luot !== doi.current) return;
        datTrangThai("hong");
        // `translatedAsActor` đã đổi mã máy chủ thành câu người đọc bằng
        // `MEDIA_REFUSALS`, nên `message` ở đây đã là câu để hiện lên màn.
        datLoi(
          error instanceof ApiError ? error.message : "Nếp chưa vẽ được lúc này. Thử lại sau nhé.",
        );
        return;
      }
      if (luot !== doi.current) return;
      datJobId(job);

      let lanThu = 0;
      const hoi = async () => {
        if (luot !== doi.current) return;
        if (AppState.currentState !== "active") {
          hen.current = setTimeout(hoi, nhipHoiMs(lanThu));
          return;
        }
        try {
          const tin = await docTrangThai(actorId, job);
          if (luot !== doi.current) return;
          datTrangThai(tin.trang_thai);
          if (tin.trang_thai === "xong") {
            datDuongAnh(duongFile(BASE_URL, job));
            return;
          }
          if (tin.trang_thai === "hong") {
            datLoi(tin.loi || "Nếp vẽ hỏng mất rồi. Thử lại nhé.");
            return;
          }
          if (!conPhaiHoi(tin.trang_thai)) return;
        } catch {
          // Một lần hỏi trượt không phải là một lượt vẽ hỏng: mạng chập thì
          // hỏi lại, còn job vẫn đang chạy trên máy chủ.
        }
        lanThu += 1;
        hen.current = setTimeout(hoi, nhipHoiMs(lanThu));
      };
      hen.current = setTimeout(hoi, nhipHoiMs(0));
    },
    [actorId, dungHoi],
  );

  // Memoised so the image source keeps its identity across renders; a new
  // object each render could make the image view refetch the file.
  const nguonAnh = useMemo(
    () => (duongAnh !== null && jobId !== null && actorId ? nguonAnhNep(BASE_URL, jobId, actorId) : null),
    [duongAnh, jobId, actorId],
  );

  return {
    trangThai,
    jobId,
    duongAnh,
    nguonAnh,
    loi,
    dangCho: trangThai === "dang-cho" || trangThai === "dang-chay",
    nhoVe,
    dep,
  };
}
