import { useNavigation, useRouter } from "expo-router";
import { useEffect } from "react";

import { CreateSheet } from "../src/rudi/screens/Create";

/**
 * The create sheet belongs to the four-tab shell: from the FAB it opens over
 * the tab it was pressed on. Reached cold (a deep link, a notification) there
 * is nothing under the transparent route but grey, so the shell is put in
 * place first and the sheet re-opened over it.
 */
export default function CreateRoute() {
  const router = useRouter();
  const navigation = useNavigation();
  const lanh = !navigation.canGoBack();
  useEffect(() => {
    if (!lanh) return;
    router.replace("/explore" as never);
    const t = setTimeout(() => router.push("/create" as never), 0);
    return () => clearTimeout(t);
  }, [lanh, router]);
  if (lanh) return null;
  return <CreateSheet />;
}
