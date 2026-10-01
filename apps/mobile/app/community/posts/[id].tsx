import { PostDetail } from "../../../src/rudi/community/PostDetail";
import { CanPhien } from "../../../src/rudi/ui/CuaDangNhap";

/** Signed out, a link here goes through the sign-in door and comes back (QA UI-137). */
export default function Route() {
  return (
    <CanPhien>
      <PostDetail />
    </CanPhien>
  );
}
