import type { ReactNode } from "react";
import type { StyleProp, ViewStyle } from "react-native";

import { EmptyState } from "./EmptyState";
import { Canh } from "./art/Canh";

export interface ErrorStateProps {
  /** What failed, in the product's words; never a raw server code. */
  title?: string;
  /** Why (when known and useful) and what happens to what the person typed. */
  body?: string;
  onRetry: () => void;
  retrying?: boolean;
  /** «Về danh sách», «Dùng lại bản nháp»… */
  secondary?: { label: string; onPress: () => void };
  illustration?: ReactNode;
  layout?: "full" | "inline";
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/**
 * Failure that keeps what the person typed. The retry is an outline button so
 * it never competes with the screen's primary action, and the body says out
 * loud that nothing was lost -- twelve screens used to say «Thử lại» with no
 * sentence about the form they were about to keep or discard.
 */
export function ErrorState({
  title = "Chưa đọc được từ máy chủ",
  body = "Kiểm tra mạng rồi thử lại. Những gì bạn đã nhập vẫn còn nguyên.",
  onRetry,
  retrying,
  secondary,
  illustration,
  layout = "inline",
  style,
  testID,
}: ErrorStateProps) {
  return (
    <EmptyState
      kind="failure"
      title={title}
      body={body}
      action={{ label: "Thử lại", onPress: onRetry, loading: retrying }}
      secondary={secondary}
      // The failure scene by default, in ONE place: about twenty screens draw
      // this component, and the alternative is twenty screens each remembering
      // to pass a picture. It is deliberately a props-only scene -- `hinhCanh`
      // refuses the figure for it -- because «Luật Nếp Đứng Xa Tiền» keeps the
      // character away from errors, conflicts and money, and several of these
      // twenty are the ledger's own.
      illustration={illustration ?? <Canh id="chua-doc-duoc" width={168} />}
      layout={layout}
      style={style}
      testID={testID ?? "error-state"}
    />
  );
}
