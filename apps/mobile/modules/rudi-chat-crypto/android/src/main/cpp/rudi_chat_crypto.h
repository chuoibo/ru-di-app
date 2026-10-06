/* The C ABI of packages/chat-crypto-ffi (ADR-0057), shared by the Android JNI
 * shim and the iOS Swift module. JSON in, JSON out; every returned string is
 * freed with rudi_chat_crypto_string_free; a handle with _client_free. */
#ifndef RUDI_CHAT_CRYPTO_H
#define RUDI_CHAT_CRYPTO_H
#ifdef __cplusplus
extern "C" {
#endif
typedef struct ClientHandle ClientHandle;
ClientHandle *rudi_chat_crypto_client_new(const char *actor_id, const char *device_id);
ClientHandle *rudi_chat_crypto_client_resume(const char *sealed_json, const char *wrapping_key_b64, const char *anchor_json);
void rudi_chat_crypto_client_free(ClientHandle *handle);
void rudi_chat_crypto_string_free(char *text);
char *rudi_chat_crypto_identity(ClientHandle *handle);
char *rudi_chat_crypto_create_group(ClientHandle *handle, const char *conversation_id);
char *rudi_chat_crypto_encrypt(ClientHandle *handle, const char *conversation_id, const char *logical_send_id, const char *operation_json);
char *rudi_chat_crypto_receive(ClientHandle *handle, const char *envelope_json, const char *verified_roster_json);
char *rudi_chat_crypto_call(ClientHandle *handle, const char *method, const char *args_json);
#ifdef __cplusplus
}
#endif
#endif
