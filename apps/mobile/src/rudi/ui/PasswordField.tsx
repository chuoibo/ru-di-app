import { useEffect, useState } from "react";

import { IconButton } from "../ui";
import { ONhapMuc, type ONhapMucProps } from "./ONhapMuc";

/** Visibility is a local reading choice; clearing or locking masks the field. */
export function PasswordField(props: Omit<ONhapMucProps, "secureTextEntry" | "trailing">) {
  const [visible, setVisible] = useState(false);
  useEffect(() => { if (props.value === "" || props.editable === false) setVisible(false); }, [props.value, props.editable]);
  const shown = visible && props.editable !== false;
  return <ONhapMuc
    {...props}
    secureTextEntry={!shown}
    trailing={<IconButton
      accessibilityLabel={shown ? "Ẩn mật khẩu" : "Hiện mật khẩu"}
      disabled={props.editable === false}
      icon={shown ? "eye-off-outline" : "eye-outline"}
      onPress={() => setVisible((old) => !old)}
      quiet
    />}
  />;
}
