/** Demo content is available only through explicitly named fixture routes. */
export function chatRoute(id: string | undefined, signedIn: boolean, fixtureIds: readonly string[]): "live" | "fixture" | "login" | "messages" {
  if (!id) return "messages";
  if (signedIn) return "live";
  return fixtureIds.includes(id) ? "fixture" : "login";
}
