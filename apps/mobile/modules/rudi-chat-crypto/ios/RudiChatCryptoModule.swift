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
    "acknowledge_sent", "abandon_send", "abandon_commit", "forget", "seal_media", "open_media",
  ]
  private let queue = DispatchQueue(label: "rudi.chat.crypto")
  private var handle: OpaquePointer?
  private var vault: Vault?
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

    AsyncFunction("erase") { () in
      try self.serial {
        if let h = self.handle { rudi_chat_crypto_client_free(h); self.handle = nil }
        self.vault?.erase()
        self.vault = nil
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
