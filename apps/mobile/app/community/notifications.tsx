import { Notifications } from "../../src/rudi/community/Notifications";
import { CanPhien } from "../../src/rudi/ui/CuaDangNhap";

/** Signed out, a link here goes through the sign-in door and comes back (QA UI-137). */
export default function Route() {
  return (
    <CanPhien>
      <Notifications />
    </CanPhien>
  );
}
