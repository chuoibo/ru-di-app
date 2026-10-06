package expo.modules.rudichatcrypto

import java.io.BufferedInputStream
import java.io.DataInputStream
import java.io.DataOutputStream
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import java.io.RandomAccessFile
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.Mac
import javax.crypto.spec.GCMParameterSpec
import javax.crypto.spec.SecretKeySpec
import org.json.JSONArray
import org.json.JSONObject

/**
 * A room's sealed record on this device (ADR-0057 §4.4): what was decrypted,
 * what this device sent, and the cursor past it.
 *
 * - An append-only log of AES-GCM frames, `[u32 length][12-byte IV][ciphertext]`,
 *   under a key derived from the device's wrapping key (itself held only
 *   under the Keystore). Next to the MLS state in noBackupFilesDir: never in a
 *   backup.
 * - The AAD binds actor, device, room and the frame's index, so frames cannot
 *   be moved between rooms or reordered.
 * - One append is one frame, written and fsynced before the answer. A torn
 *   last frame (the app died mid-write) is dropped: what was answered was
 *   whole.
 * - Every [COMPACT_AT] frames the log is rewritten as one frame, atomically,
 *   unless that frame would pass [MAX_FRAME]: then it simply keeps growing.
 */
internal class RoomLog(private val dir: File, private val scope: String, wrappingKey: ByteArray) {
  private val key: SecretKeySpec
  private val random = SecureRandom()

  /** Per room: frames so far and the byte length they end at. */
  private val known = HashMap<String, Pair<Int, Long>>()

  init {
    dir.mkdirs()
    val mac = Mac.getInstance("HmacSHA256")
    mac.init(SecretKeySpec(wrappingKey, "HmacSHA256"))
    val derived = mac.doFinal("rudi-chat-room-log-v1".toByteArray())
    key = SecretKeySpec(derived, "AES")
    derived.fill(0)
  }

  private fun file(room: String): File {
    if (!ROOM.matches(room)) throw IllegalArgumentException("room")
    return File(dir, "$room.log")
  }

  private fun aad(room: String, index: Int) = "$scope|$room|$index".toByteArray()

  private fun seal(room: String, index: Int, plain: ByteArray): ByteArray {
    val iv = ByteArray(12).also { random.nextBytes(it) }
    val cipher = Cipher.getInstance("AES/GCM/NoPadding")
    cipher.init(Cipher.ENCRYPT_MODE, key, GCMParameterSpec(128, iv))
    cipher.updateAAD(aad(room, index))
    return iv + cipher.doFinal(plain)
  }

  private fun open(room: String, index: Int, frame: ByteArray): ByteArray {
    val cipher = Cipher.getInstance("AES/GCM/NoPadding")
    cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(128, frame, 0, 12))
    cipher.updateAAD(aad(room, index))
    return cipher.doFinal(frame, 12, frame.size - 12)
  }

  /**
   * The frames that are whole (and, when [decrypt], open): their contents and
   * where they end. Stops at the first torn or unreadable frame.
   */
  private fun scan(room: String, decrypt: Boolean): Pair<List<JSONObject>, Pair<Int, Long>> {
    val f = file(room)
    val frames = ArrayList<JSONObject>()
    if (!f.exists()) return frames to (0 to 0L)
    val size = f.length()
    var offset = 0L
    var index = 0
    DataInputStream(BufferedInputStream(FileInputStream(f))).use { input ->
      while (offset + 4 <= size) {
        val length = input.readInt()
        if (length < 28 || length > MAX_FRAME || offset + 4 + length > size) break
        val frame = ByteArray(length)
        input.readFully(frame)
        if (decrypt) {
          val plain = try { open(room, index, frame) } catch (e: Exception) { break }
          frames.add(JSONObject(String(plain, Charsets.UTF_8)))
          plain.fill(0)
        }
        offset += 4 + length
        index += 1
      }
    }
    return frames to (index to offset)
  }

  private fun merge(frames: List<JSONObject>): JSONObject {
    var cursor = 0L
    val records = JSONArray()
    for (frame in frames) {
      if (!frame.isNull("cursor")) cursor = frame.getLong("cursor")
      val ban = frame.getJSONArray("ban")
      for (i in 0 until ban.length()) records.put(ban.get(i))
    }
    return JSONObject().put("cursor", cursor).put("ban", records)
  }

  /** The room's record as `{cursor, ban}` JSON, or null when there is none. */
  fun read(room: String): String? {
    val (frames, end) = scan(room, true)
    known[room] = end
    return if (frames.isEmpty()) null else merge(frames).toString()
  }

  /** Appends records (a JSON array) and moves the cursor when not null: one durable write. */
  fun append(room: String, cursor: Long?, records: String) {
    if (records.length > MAX_FRAME / 2) throw IllegalArgumentException("records")
    val ban = JSONArray(records)
    val f = file(room)
    val (count, end) = known[room] ?: scan(room, false).second
    if (f.exists() && f.length() > end) RandomAccessFile(f, "rw").use { it.setLength(end) }
    if (count > 0 && count % COMPACT_AT == 0 && compact(room, cursor, ban)) return
    val plain = JSONObject().put("cursor", cursor ?: JSONObject.NULL).put("ban", ban).toString().toByteArray()
    val frame = seal(room, count, plain)
    plain.fill(0)
    FileOutputStream(f, true).use { out ->
      DataOutputStream(out).writeInt(frame.size)
      out.write(frame)
      out.fd.sync()
    }
    known[room] = (count + 1) to (end + 4 + frame.size)
  }

  /** Rewrites the room as one frame holding everything plus the new records; false when too big. */
  private fun compact(room: String, cursor: Long?, ban: JSONArray): Boolean {
    val all = merge(scan(room, true).first)
    if (cursor != null) all.put("cursor", cursor)
    val records = all.getJSONArray("ban")
    for (i in 0 until ban.length()) records.put(ban.get(i))
    val plain = all.toString().toByteArray()
    if (plain.size + 28 > MAX_FRAME) {
      plain.fill(0)
      return false
    }
    val frame = seal(room, 0, plain)
    plain.fill(0)
    val target = file(room)
    val temp = File(dir, "$room.log.tmp")
    FileOutputStream(temp).use { out ->
      DataOutputStream(out).writeInt(frame.size)
      out.write(frame)
      out.fd.sync()
    }
    if (!temp.renameTo(target)) throw IllegalStateException("room_log_write_failed")
    known[room] = 1 to (4L + frame.size)
    return true
  }

  companion object {
    private val ROOM = Regex("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
    private const val MAX_FRAME = 16 * 1024 * 1024
    private const val COMPACT_AT = 256
  }
}
