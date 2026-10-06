// api.js — fetch ラッパ。
// - Bearer トークン付与
// - エラーJSON {"error":{"code","message"}} の code を日本語メッセージへ変換
// - AbortController によるタイムアウト
// すべて相対パス（/api/...）で通信する（Discord CSP 制約）。

const DEFAULT_TIMEOUT_MS = 15000;

let accessToken = null;

// SDK 認証後に app.js から呼ぶ。トークンはメモリ上のみで保持（localStorage 禁止）。
export function setToken(token) {
  accessToken = token;
}

// エラーコード → 日本語メッセージ。バックエンドの snake_case code に対応。
const ERROR_MESSAGES = {
  unauthorized: '認証に失敗しました。お手数ですが再読み込みしてください。',
  forbidden: 'このサーバーの辞書を編集する権限がありません。サーバーのメンバーであることを確認してください。',
  upstream_rate_limited: 'Discord側が混み合っています。しばらく待ってから、もう一度お試しください。',
  rate_limited: '操作が多すぎます。少し時間をおいてから、もう一度お試しください。',
  invalid_word: '単語または読みが正しくありません。1〜64文字で、改行や制御文字は使えません。',
  word_limit: '登録できる単語数の上限（500語）に達しました。不要な単語を削除してください。',
  invalid_user_info: '音声設定の値が正しくありません。',
  // クライアント内部で使う疑似コード
  timeout: '通信がタイムアウトしました。接続を確認して、もう一度お試しください。',
  network: '通信に失敗しました。ネットワーク接続を確認してください。',
  server_error: 'サーバーでエラーが発生しました。しばらくしてからお試しください。',
  bad_response: 'サーバーから予期しない応答がありました。',
};

// code と（あれば）サーバー由来 message から表示用メッセージを決める。
export function messageForCode(code, serverMessage) {
  const base = ERROR_MESSAGES[code];
  if (base) {
    // invalid_user_info はサーバーが具体的な理由を返すため併記。
    if (code === 'invalid_user_info' && serverMessage) {
      return base + '（' + serverMessage + '）';
    }
    return base;
  }
  if (serverMessage) return serverMessage;
  return ERROR_MESSAGES.server_error;
}

// API 呼び出しで投げられる例外。app.js 側で code を見て分岐できる。
export class ApiError extends Error {
  constructor(code, message, status, serverMessage) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.status = status || 0;
    this.serverMessage = serverMessage || '';
  }
}

async function request(method, path, { body, auth = true, timeout = DEFAULT_TIMEOUT_MS } = {}) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);

  const headers = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (auth && accessToken) headers.Authorization = 'Bearer ' + accessToken;

  let res;
  try {
    res = await fetch(path, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal: controller.signal,
      cache: 'no-store',
    });
  } catch (e) {
    clearTimeout(timer);
    if (e && e.name === 'AbortError') {
      throw new ApiError('timeout', ERROR_MESSAGES.timeout, 0);
    }
    throw new ApiError('network', ERROR_MESSAGES.network, 0);
  }
  clearTimeout(timer);

  // レスポンス本文を JSON として読む（空でも許容）。
  let data = null;
  let text = '';
  try {
    text = await res.text();
  } catch (_) {
    text = '';
  }
  if (text) {
    try {
      data = JSON.parse(text);
    } catch (_) {
      if (res.ok) {
        throw new ApiError('bad_response', ERROR_MESSAGES.bad_response, res.status);
      }
      data = null;
    }
  }

  if (!res.ok) {
    const code = (data && data.error && data.error.code) || 'server_error';
    const serverMsg = (data && data.error && data.error.message) || '';
    throw new ApiError(code, messageForCode(code, serverMsg), res.status, serverMsg);
  }
  return data;
}

// ---- エンドポイント別ヘルパ ----

// 認証不要
export const getConfig = () => request('GET', '/api/config', { auth: false });
export const postToken = (code) => request('POST', '/api/token', { auth: false, body: { code } });

// 認証必須
export const getMe = () => request('GET', '/api/me');
export const getVoices = () => request('GET', '/api/voices');
export const getMyVoice = () => request('GET', '/api/me/voice');
export const putMyVoice = (userInfo) => request('PUT', '/api/me/voice', { body: userInfo });
export const postRandom = () => request('POST', '/api/me/voice/random');

// 辞書（要 member）
export const getWords = (gid) =>
  request('GET', '/api/guilds/' + encodeURIComponent(gid) + '/words');
export const postWord = (gid, word, reading) =>
  request('POST', '/api/guilds/' + encodeURIComponent(gid) + '/words', { body: { word, reading } });
export const deleteWord = (gid, word) =>
  request('DELETE', '/api/guilds/' + encodeURIComponent(gid) + '/words/' + encodeURIComponent(word));
