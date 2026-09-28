import type { RouteChoice, RouteID } from "./achievement-routes";

/** Keep a chosen future ending visible while still showing other exits. */
export function choicesForRoute<T extends Pick<RouteChoice, "id" | "route_id" | "eligible" | "earned">>(
  choices: T[], routeId: RouteID, selectedEndingId: string | null,
): T[] {
  return choices.filter((choice) => choice.route_id === routeId).sort((a, b) => {
    if (a.id === selectedEndingId) return -1;
    if (b.id === selectedEndingId) return 1;
    if (a.earned !== b.earned) return a.earned ? 1 : -1;
    if (a.eligible !== b.eligible) return a.eligible ? -1 : 1;
    return 0;
  });
}

/** Display is a deliberate selection, limited to three server-earned IDs. */
export function toggleDisplayedBadge(current: string[], badgeId: string, earned: string[]): string[] {
  if (current.includes(badgeId)) return current.filter((id) => id !== badgeId);
  if (!earned.includes(badgeId) || current.length >= 3) return current;
  return [...current, badgeId];
}
