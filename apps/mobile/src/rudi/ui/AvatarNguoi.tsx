import { useSyncExternalStore } from "react";

import { baoAnhHong, dangKyAnhDaiDien, nguonAvatar, nhipAnhDaiDien } from "../nguoi/anh-dai-dien";
import { useRudiSession } from "../session";
import { Avatar, type AvatarProps } from "./Avatar";

export interface AvatarNguoiProps extends Omit<AvatarProps, "source" | "onError"> {
  /** The person drawn. Null (a stranger's request, a fixture) means initials only. */
  personId: string | null | undefined;
}

/**
 * A real person's avatar on a live screen: their uploaded photograph when the
 * server lets this reader see it, their initials otherwise.
 *
 * The server decides who may see whom (`view_person_avatar`); this frame only
 * asks, remembers a refusal, and hears about uploads -- see `anh-dai-dien.ts`.
 * Fixture screens keep the plain `Avatar`: their people have no server id, and
 * a photograph there would be a picture with no provenance (ADR-0020 §2.5).
 */
export function AvatarNguoi({ personId, ...rest }: AvatarNguoiProps) {
  const { phien } = useRudiSession();
  // Re-render when any upload or refusal is announced.
  useSyncExternalStore(dangKyAnhDaiDien, nhipAnhDaiDien, nhipAnhDaiDien);
  const actorId = phien?.person_id ?? null;
  const source = nguonAvatar(personId, actorId);
  const onError = source && personId && actorId ? () => baoAnhHong(personId, actorId) : undefined;
  // The person's own ink rides along (ADR-0037 D6): until 25/09 the id stopped
  // here, so every live avatar kept the screen's tone.
  return <Avatar {...rest} onError={onError} personId={personId} source={source} />;
}
