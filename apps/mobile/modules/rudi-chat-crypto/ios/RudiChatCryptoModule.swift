import CryptoKit
import ExpoModulesCore
import Foundation
import Security

/// MLS for chat v2 on iOS (ADR-0057), the same contract as the Android module:
/// one device identity at a time, every call on one serial queue, and every
/// change sealed and persisted before its answer reaches JS.
public class RudiChatCryptoModule: Module {
  /// The lifecycle methods JS may reach through `call`. Never "seal": a
  /// wrapping key JS chose would hand JS the whole MLS state (security review
  /// 05/10). Sealing happens here alone, under the Keychain-held key.
  private static let jsMethods: Set<String> = [
    "generation", "enrollment", "conversations", "key_package", "join_group", "epoch", "roster",
    "stage_add", "stage_remove", "stage_rekey", "pending_commit", "acknowledge_commit",
    "acknowledge_sent", "abandon_send", "abandon_commit", "forget", "settle_received", "ai_card_digest", "seal_media", "open_media",
  ]
  private let queue = DispatchQueue(label: "rudi.chat.crypto")
  private var handle: OpaquePointer?
  private var vault: Vault?
  private var rooms: RoomLog?
  private var generation: Int64 = -1

  struct CryptoError: CodedError {
    let code: String
    let description: String
  }

  private func text(_ raw: UnsafeMutablePointer<CChar>?) throws -> String {
    guard let raw = raw else { throw CryptoError(code: "ERR_CHAT_CRYPTO", description: "null") }
    let answer = String(cString: raw)
    rudi_chat_crypto_string_free(raw)
    if answer.hasPrefix("{\"error\""),
       let json = try? JSONSerialization.jsonObject(with: Data(answer.utf8)) as? [String: Any],
       let code = json["error"] as? String {
      throw CryptoError(code: "ERR_CHAT_CRYPTO_" + code.uppercased(), description: code)
    }
    return answer
  }

  private func live() throws -> OpaquePointer {
    guard let h = handle else { throw CryptoError(code: "ERR_CHAT_CRYPTO_CLOSED", description: "closed") }
    return h
  }

  private func call(_ method: String, _ args: String) throws -> String {
    return try text(rudi_chat_crypto_call(try live(), method, args))
  }

  private func persistIfChanged() throws {
    guard let v = vault else { return }
    let now = (try JSONSerialization.jsonObject(with: Data(try call("generation", "{}").utf8)) as? [String: Any])?["generation"] as? Int64 ?? -1
    if now == generation { return }
    let args = try String(data: JSONSerialization.data(withJSONObject: ["wrapping_key": try v.wrappingKey()]), encoding: .utf8)!
    let sealed = try JSONSerialization.jsonObject(with: Data(try call("seal", args).utf8)) as! [String: Any]
    let state = String(data: try JSONSerialization.data(withJSONObject: sealed["sealed"]!), encoding: .utf8)!
    let anchor = String(data: try JSONSerialization.data(withJSONObject: sealed["anchor"]!), encoding: .utf8)!
    try v.persist(sealed: state, anchor: anchor)
    generation = now
  }

  private func serial<T>(_ block: () throws -> T) throws -> T {
    return try queue.sync { try block() }
  }

  private func mutating<T>(_ block: () throws -> T) throws -> T {
    return try serial {
      let result = try block()
      try persistIfChanged()
      return result
    }
  }

  public func definition() -> ModuleDefinition {
    Name("RudiChatCrypto")

    AsyncFunction("open") { (actor: String, device: String) -> Bool in
      try self.serial {
        if let h = self.handle { rudi_chat_crypto_client_free(h); self.handle = nil }
        let v = try Vault(actor: actor, device: device)
        var resumed = false
        if v.exists() {
          let key = try v.wrappingKey()
          for anchor in v.anchors() {
            if let h = rudi_chat_crypto_client_resume(try v.sealedState(), key, anchor) {
              self.handle = h
              break
            }
          }
          guard self.handle != nil else { throw CryptoError(code: "ERR_CHAT_CRYPTO_CHECKPOINT", description: "checkpoint") }
          resumed = true
        } else {
          guard let h = rudi_chat_crypto_client_new(actor, device) else {
            throw CryptoError(code: "ERR_CHAT_CRYPTO_INVALID", description: "invalid")
          }
          self.handle = h
        }
        self.vault = v
        guard let wrapping = Data(base64Encoded: try v.wrappingKey()) else {
          throw CryptoError(code: "ERR_CHAT_CRYPTO_KEY", description: "key")
        }
        self.rooms = try RoomLog(dir: v.roomsDir, scope: "\(actor)|\(device)", wrappingKey: wrapping)
        self.generation = -1
        try self.persistIfChanged()
        return resumed
      }
    }

    AsyncFunction("identity") { () -> String in try self.serial { try self.text(rudi_chat_crypto_identity(try self.live())) } }

    AsyncFunction("createGroup") { (conversation: String) -> String in
      try self.mutating { try self.text(rudi_chat_crypto_create_group(try self.live(), conversation)) }
    }

    AsyncFunction("encrypt") { (conversation: String, logical: String, operation: String) -> String in
      try self.mutating { try self.text(rudi_chat_crypto_encrypt(try self.live(), conversation, logical, operation)) }
    }

    AsyncFunction("receive") { (envelope: String, roster: String?) -> String in
      try self.mutating { try self.text(rudi_chat_crypto_receive(try self.live(), envelope, roster)) }
    }

    AsyncFunction("call") { (method: String, args: String) -> String in
      guard Self.jsMethods.contains(method) else {
        throw CryptoError(code: "ERR_CHAT_CRYPTO_METHOD", description: "method_not_allowed")
      }
      return try self.mutating { try self.call(method, args) }
    }

    AsyncFunction("roomRead") { (room: String) -> String? in
      try self.serial {
        guard let rooms = self.rooms else { throw CryptoError(code: "ERR_CHAT_CRYPTO_CLOSED", description: "closed") }
        return try rooms.read(room)
      }
    }

    AsyncFunction("roomAppend") { (room: String, cursor: Double?, records: String) in
      try self.serial {
        guard let rooms = self.rooms else { throw CryptoError(code: "ERR_CHAT_CRYPTO_CLOSED", description: "closed") }
        try rooms.append(room, cursor: cursor.map { Int64($0) }, records: records)
      }
    }

    AsyncFunction("erase") { () in
      try self.serial {
        if let h = self.handle { rudi_chat_crypto_client_free(h); self.handle = nil }
        self.vault?.erase()
        self.vault = nil
        self.rooms = nil
      }
    }
  }
}

/// The sealed state in Application Support (excluded from backup); the wrapping
/// key and the {current, next} anchor in the Keychain, this device only.
final class Vault {
  private let dir: URL
  private let service: String

  init(actor: String, device: String) throws {
    let base = try FileManager.default.url(for: .applicationSupportDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
    var dir = base.appendingPathComponent("rudi-chat/\(actor)/\(device)", isDirectory: true)
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    var values = URLResourceValues()
    values.isExcludedFromBackup = true
    try dir.setResourceValues(values)
    self.dir = dir
    self.service = "rudi-chat.\(actor).\(device)"
  }

  private var stateURL: URL { dir.appendingPathComponent("state.json") }

  /// Where this device identity's room records live (RoomLog), beside its state.
  var roomsDir: URL { dir.appendingPathComponent("rooms", isDirectory: true) }

  func exists() -> Bool { FileManager.default.fileExists(atPath: stateURL.path) && read("anchor") != nil && read("key") != nil }

  func sealedState() throws -> String { try String(contentsOf: stateURL, encoding: .utf8) }

  func wrappingKey() throws -> String {
    if let key = read("key") { return key }
    var raw = [UInt8](repeating: 0, count: 32)
    guard SecRandomCopyBytes(kSecRandomDefault, raw.count, &raw) == errSecSuccess else {
      throw RudiChatCryptoModule.CryptoError(code: "ERR_CHAT_CRYPTO_KEY", description: "random")
    }
    let key = Data(raw).base64EncodedString()
    raw.withUnsafeMutableBufferPointer { $0.update(repeating: 0) }
    try write("key", key)
    return key
  }

  func anchors() -> [String] {
    guard let raw = read("anchor"), let json = try? JSONSerialization.jsonObject(with: Data(raw.utf8)) as? [String: String] else { return [] }
    return [json["current"], json["next"]].compactMap { $0 }.filter { !$0.isEmpty }
  }

  func persist(sealed: String, anchor: String) throws {
    let current = anchors().first ?? anchor
    try write("anchor", try String(data: JSONSerialization.data(withJSONObject: ["current": current, "next": anchor]), encoding: .utf8)!)
    try Data(sealed.utf8).write(to: stateURL, options: [.atomic, .completeFileProtectionUntilFirstUserAuthentication])
    try write("anchor", try String(data: JSONSerialization.data(withJSONObject: ["current": anchor]), encoding: .utf8)!)
  }

  func erase() {
    try? FileManager.default.removeItem(at: roomsDir)
    try? FileManager.default.removeItem(at: stateURL)
    for account in ["anchor", "key"] {
      SecItemDelete([kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: account] as CFDictionary)
    }
  }

  private func read(_ account: String) -> String? {
    var out: CFTypeRef?
    let query: [CFString: Any] = [kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: account,
                                  kSecReturnData: true, kSecMatchLimit: kSecMatchLimitOne]
    guard SecItemCopyMatching(query as CFDictionary, &out) == errSecSuccess, let data = out as? Data else { return nil }
    return String(data: data, encoding: .utf8)
  }

  private func write(_ account: String, _ value: String) throws {
    let base: [CFString: Any] = [kSecClass: kSecClassGenericPassword, kSecAttrService: service, kSecAttrAccount: account]
    let attributes: [CFString: Any] = [kSecValueData: Data(value.utf8),
                                       kSecAttrAccessible: kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly]
    var status = SecItemUpdate(base as CFDictionary, attributes as CFDictionary)
    if status == errSecItemNotFound {
      status = SecItemAdd(base.merging(attributes) { $1 } as CFDictionary, nil)
    }
    guard status == errSecSuccess else {
      throw RudiChatCryptoModule.CryptoError(code: "ERR_CHAT_CRYPTO_KEYCHAIN", description: "\(status)")
    }
  }
}

/// A room's sealed record, the same format as Android's RoomLog.kt: an
/// append-only log of `[u32 big-endian length][12-byte nonce][ciphertext+tag]`
/// AES-GCM frames under HMAC-SHA256(wrapping key, "rudi-chat-room-log-v1"),
/// AAD `actor|device|room|index`. A torn last frame is dropped; every
/// [compactAt] frames the log is rewritten as one, atomically, unless too big.
final class RoomLog {
  private static let room = try! NSRegularExpression(pattern: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
  private static let maxFrame = 16 * 1024 * 1024
  private static let compactAt = 256
  private let dir: URL
  private let scope: String
  private let key: SymmetricKey
  private var known: [String: (count: Int, end: UInt64)] = [:]

  init(dir: URL, scope: String, wrappingKey: Data) throws {
    try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
    self.dir = dir
    self.scope = scope
    let code = HMAC<SHA256>.authenticationCode(for: Data("rudi-chat-room-log-v1".utf8), using: SymmetricKey(data: wrappingKey))
    self.key = SymmetricKey(data: Data(code))
  }

  private func file(_ room: String) throws -> URL {
    let range = NSRange(room.startIndex..., in: room)
    guard Self.room.firstMatch(in: room, range: range) != nil else {
      throw RudiChatCryptoModule.CryptoError(code: "ERR_CHAT_CRYPTO_INVALID", description: "room")
    }
    return dir.appendingPathComponent("\(room).log")
  }

  private func aad(_ room: String, _ index: Int) -> Data { Data("\(scope)|\(room)|\(index)".utf8) }

  private func scan(_ room: String, decrypt: Bool) throws -> (frames: [[String: Any]], count: Int, end: UInt64) {
    let url = try file(room)
    guard let data = try? Data(contentsOf: url) else { return ([], 0, 0) }
    var frames: [[String: Any]] = []
    var offset = 0
    var index = 0
    while offset + 4 <= data.count {
      let length = data[offset..<offset + 4].reduce(0) { ($0 << 8) | Int($1) }
      if length < 28 || length > Self.maxFrame || offset + 4 + length > data.count { break }
      if decrypt {
        let frame = data[offset + 4..<offset + 4 + length]
        guard let box = try? AES.GCM.SealedBox(combined: frame),
              let plain = try? AES.GCM.open(box, using: key, authenticating: aad(room, index)),
              let json = try? JSONSerialization.jsonObject(with: plain) as? [String: Any] else { break }
        frames.append(json)
      }
      offset += 4 + length
      index += 1
    }
    return (frames, index, UInt64(offset))
  }

  private func merge(_ frames: [[String: Any]]) -> [String: Any] {
    var cursor: Int64 = 0
    var records: [Any] = []
    for frame in frames {
      if let c = frame["cursor"] as? NSNumber { cursor = c.int64Value }
      records.append(contentsOf: frame["ban"] as? [Any] ?? [])
    }
    return ["cursor": cursor, "ban": records]
  }

  private func frame(_ room: String, _ index: Int, _ object: [String: Any]) throws -> Data {
    let plain = try JSONSerialization.data(withJSONObject: object)
    let sealed = try AES.GCM.seal(plain, using: key, authenticating: aad(room, index))
    guard let combined = sealed.combined else { throw RudiChatCryptoModule.CryptoError(code: "ERR_CHAT_CRYPTO", description: "seal") }
    var length = UInt32(combined.count).bigEndian
    return Data(bytes: &length, count: 4) + combined
  }

  func read(_ room: String) throws -> String? {
    let found = try scan(room, decrypt: true)
    known[room] = (found.count, found.end)
    if found.frames.isEmpty { return nil }
    return String(data: try JSONSerialization.data(withJSONObject: merge(found.frames)), encoding: .utf8)
  }

  func append(_ room: String, cursor: Int64?, records: String) throws {
    guard records.utf8.count <= Self.maxFrame / 2,
          let ban = try JSONSerialization.jsonObject(with: Data(records.utf8)) as? [Any] else {
      throw RudiChatCryptoModule.CryptoError(code: "ERR_CHAT_CRYPTO_INVALID", description: "records")
    }
    let url = try file(room)
    let at = try known[room] ?? { let s = try scan(room, decrypt: false); return (s.count, s.end) }()
    if !FileManager.default.fileExists(atPath: url.path) {
      FileManager.default.createFile(atPath: url.path, contents: nil, attributes: [.protectionKey: FileProtectionType.completeUntilFirstUserAuthentication])
    }
    let handle = try FileHandle(forWritingTo: url)
    defer { try? handle.close() }
    if try handle.seekToEnd() > at.end { try handle.truncate(atOffset: at.end) }
    if at.count > 0 && at.count % Self.compactAt == 0 {
      var all = merge(try scan(room, decrypt: true).frames)
      if let cursor = cursor { all["cursor"] = cursor }
      all["ban"] = (all["ban"] as? [Any] ?? []) + ban
      let one = try frame(room, 0, all)
      if one.count <= Self.maxFrame {
        let temp = dir.appendingPathComponent("\(room).log.tmp")
        try one.write(to: temp, options: [.completeFileProtectionUntilFirstUserAuthentication])
        _ = try FileManager.default.replaceItemAt(url, withItemAt: temp)
        known[room] = (1, UInt64(one.count))
        return
      }
    }
    let next = try frame(room, at.count, ["cursor": cursor.map { NSNumber(value: $0) } ?? NSNull(), "ban": ban])
    try handle.seekToEnd()
    try handle.write(contentsOf: next)
    try handle.synchronize()
    known[room] = (at.count + 1, at.end + UInt64(next.count))
  }
}
