/* JNI shim over the C ABI. Strings cross as UTF-8 byte arrays, never through
 * GetStringUTFChars: JNI's modified UTF-8 encodes an emoji as two surrogates,
 * which the Rust side rightly refuses as invalid UTF-8. */
#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include "rudi_chat_crypto.h"

static char *copy_bytes(JNIEnv *env, jbyteArray bytes) {
  if (bytes == NULL) return NULL;
  jsize n = (*env)->GetArrayLength(env, bytes);
  char *out = malloc((size_t)n + 1);
  if (out == NULL) return NULL;
  (*env)->GetByteArrayRegion(env, bytes, 0, n, (jbyte *)out);
  out[n] = '\0';
  return out;
}

static jbyteArray answer(JNIEnv *env, char *text) {
  if (text == NULL) return NULL;
  jsize n = (jsize)strlen(text);
  jbyteArray out = (*env)->NewByteArray(env, n);
  if (out != NULL) (*env)->SetByteArrayRegion(env, out, 0, n, (const jbyte *)text);
  rudi_chat_crypto_string_free(text);
  return out;
}

#define HANDLE(h) ((ClientHandle *)(intptr_t)(h))

JNIEXPORT jlong JNICALL Java_expo_modules_rudichatcrypto_Native_newClient(JNIEnv *env, jclass c, jbyteArray actor, jbyteArray device) {
  char *a = copy_bytes(env, actor), *d = copy_bytes(env, device);
  ClientHandle *h = (a && d) ? rudi_chat_crypto_client_new(a, d) : NULL;
  free(a); free(d);
  return (jlong)(intptr_t)h;
}

JNIEXPORT jlong JNICALL Java_expo_modules_rudichatcrypto_Native_resume(JNIEnv *env, jclass c, jbyteArray sealed, jbyteArray key, jbyteArray anchor) {
  char *s = copy_bytes(env, sealed), *k = copy_bytes(env, key), *a = copy_bytes(env, anchor);
  ClientHandle *h = (s && k && a) ? rudi_chat_crypto_client_resume(s, k, a) : NULL;
  if (k) { memset(k, 0, strlen(k)); }
  free(s); free(k); free(a);
  return (jlong)(intptr_t)h;
}

JNIEXPORT void JNICALL Java_expo_modules_rudichatcrypto_Native_free(JNIEnv *env, jclass c, jlong h) {
  rudi_chat_crypto_client_free(HANDLE(h));
}

JNIEXPORT jbyteArray JNICALL Java_expo_modules_rudichatcrypto_Native_identity(JNIEnv *env, jclass c, jlong h) {
  return answer(env, rudi_chat_crypto_identity(HANDLE(h)));
}

JNIEXPORT jbyteArray JNICALL Java_expo_modules_rudichatcrypto_Native_createGroup(JNIEnv *env, jclass c, jlong h, jbyteArray conversation) {
  char *conv = copy_bytes(env, conversation);
  jbyteArray out = conv ? answer(env, rudi_chat_crypto_create_group(HANDLE(h), conv)) : NULL;
  free(conv);
  return out;
}

JNIEXPORT jbyteArray JNICALL Java_expo_modules_rudichatcrypto_Native_encrypt(JNIEnv *env, jclass c, jlong h, jbyteArray conversation, jbyteArray logical, jbyteArray operation) {
  char *conv = copy_bytes(env, conversation), *l = copy_bytes(env, logical), *op = copy_bytes(env, operation);
  jbyteArray out = (conv && l && op) ? answer(env, rudi_chat_crypto_encrypt(HANDLE(h), conv, l, op)) : NULL;
  if (op) { memset(op, 0, strlen(op)); }
  free(conv); free(l); free(op);
  return out;
}

JNIEXPORT jbyteArray JNICALL Java_expo_modules_rudichatcrypto_Native_receive(JNIEnv *env, jclass c, jlong h, jbyteArray envelope, jbyteArray roster) {
  char *e = copy_bytes(env, envelope), *r = copy_bytes(env, roster);
  jbyteArray out = e ? answer(env, rudi_chat_crypto_receive(HANDLE(h), e, r)) : NULL;
  free(e); free(r);
  return out;
}

JNIEXPORT jbyteArray JNICALL Java_expo_modules_rudichatcrypto_Native_call(JNIEnv *env, jclass c, jlong h, jbyteArray method, jbyteArray args) {
  char *m = copy_bytes(env, method), *a = copy_bytes(env, args);
  jbyteArray out = (m && a) ? answer(env, rudi_chat_crypto_call(HANDLE(h), m, a)) : NULL;
  if (a) { memset(a, 0, strlen(a)); }
  free(m); free(a);
  return out;
}
