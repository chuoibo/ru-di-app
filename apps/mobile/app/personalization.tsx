import { PersonalizationScreen } from "../src/rudi/screens/Onboarding";
import { CanPhien } from "../src/rudi/ui/CuaDangNhap";

/** Taste belongs to an account: no session, the sign-in door first. */
export default function PersonalizationRoute() {
  return (
    <CanPhien>
      <PersonalizationScreen />
    </CanPhien>
  );
}
