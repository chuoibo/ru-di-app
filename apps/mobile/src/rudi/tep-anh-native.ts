/**
 * Installs how the app reads a local photo into bytes for an upload
 * (`datCachDocTepAnh` in `api.ts`). A module of its own because it imports the
 * native file system, which the node tests of `api.ts` cannot load; the root
 * layout imports it once for the side effect.
 */
import { File } from "expo-file-system";

import { datCachDocTepAnh } from "../api";

datCachDocTepAnh((uri) => new File(uri).bytes());
