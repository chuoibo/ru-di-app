/**
 * MLS for chat v2 (ADR-0057) through the native module. `null` where the
 * module is not in the binary (web, Expo Go, an old build): the app says E2EE
 * needs the current app and never falls back to plaintext.
 */
import { requireOptionalNativeModule } from "expo-modules-core";

export type ChatCryptoNative = {
  open(actorId: string, deviceId: string): Promise<boolean>;
  identity(): Promise<string>;
  createGroup(conversationId: string): Promise<string>;
  encrypt(conversationId: string, logicalSendId: string, operationJson: string): Promise<string>;
  receive(envelopeJson: string, rosterJson: string | null): Promise<string>;
  call(method: string, argsJson: string): Promise<string>;
  erase(): Promise<void>;
};

export const ChatCryptoModule: ChatCryptoNative | null = requireOptionalNativeModule<ChatCryptoNative>("RudiChatCrypto");
