/**
 * Ngày trên tờ giấy, từ ISO của wire sang chữ người đọc.
 *
 * Bản fixture của Phase 2 ghi sẵn «Thứ Bảy 20/09» vì nó không có máy chủ. Wire
 * mang một ngày THẬT (`2026-09-19`) — phải thế, vì máy chủ so ngày ấy với hôm
 * nay để quyết «đã tới ngày đi chưa» — nên ai đó phải đổi nó thành chữ. Trên
 * máy thật, màn in thẳng «2026-09-19» ra giữa tờ giấy (đo 14/09).
 *
 * Không dùng `toLocaleDateString`: nó đọc locale của máy, nên cùng một tờ giấy
 * đọc ra hai thứ tiếng trên hai điện thoại của hai người đang nhìn cùng một
 * buổi tối. Bảng chữ nằm ở đây, một bản, tiếng Việt.
 */

const THU = ["Chủ nhật", "Thứ Hai", "Thứ Ba", "Thứ Tư", "Thứ Năm", "Thứ Sáu", "Thứ Bảy"] as const;

/**
 * «Thứ Bảy 19/09» từ «2026-09-19».
 *
 * Chuỗi không đọc được thì trả lại NGUYÊN VĂN, không trả chuỗi rỗng: một tờ
 * giấy mất ngày là một tờ giấy không dùng được, còn một ngày lạ mắt vẫn đọc
 * được và vẫn báo cho người đọc rằng có gì đó sai.
 */
export function ngayDocDuoc(iso: string): string {
  const khop = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso.trim());
  if (!khop) return iso;
  const [, nam, thang, ngay] = khop;
  // `Date.UTC` chứ không `new Date("...")`: bản sau đọc chuỗi ngày trần theo
  // giờ địa phương ở một số máy, nên một tờ giấy ngày 19 hiện ra là ngày 18
  // cho người ở múi giờ âm.
  const moc = new Date(Date.UTC(Number(nam), Number(thang) - 1, Number(ngay)));
  // `Date.UTC` KHÔNG ném với tháng 13 hay ngày 45 — nó cuộn sang tháng sau và
  // trả về một ngày hợp lệ, nên `isNaN` một mình đọc «2026-13-45» thành
  // «Thứ Bảy 14/02». Đối chiếu vòng về là phép kiểm duy nhất thật sự kiểm.
  if (
    moc.getUTCFullYear() !== Number(nam) ||
    moc.getUTCMonth() !== Number(thang) - 1 ||
    moc.getUTCDate() !== Number(ngay)
  ) {
    return iso;
  }
  return `${THU[moc.getUTCDay()]} ${ngay}/${thang}`;
}
