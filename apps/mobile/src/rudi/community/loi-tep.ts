/** Only app-authored media messages may reach the attachment UI. */
export class LoiTep extends Error {}

export function cauLoiTep(error: unknown): string {
  return error instanceof LoiTep
    ? error.message
    : "Chưa chuẩn bị được tệp. Thử lại hoặc chọn tệp khác nhé.";
}
