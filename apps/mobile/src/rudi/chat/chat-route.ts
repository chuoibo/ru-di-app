/** Where `/groups/{id}/chat` goes: no id is the list, no session is the sign-in door. */
export function chatRoute(id: string | undefined, signedIn: boolean): "live" | "login" | "messages" {
  if (!id) return "messages";
  return signedIn ? "live" : "login";
}
