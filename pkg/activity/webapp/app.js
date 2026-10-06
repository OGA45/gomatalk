// app.js — Discord Embedded App SDK の初期化、状態管理、描画、イベント処理。
import { DiscordSDK } from './vendor/discord-embedded-app-sdk.mjs';
import * as api from './api.js';

// ------------------------------------------------------------------
// 定数
// ------------------------------------------------------------------

// パラメータ定義（表示順）。key は user_info の小文字キーに一致。
const PARAMS = ['speed', 'tone', 'intone', 'threshold', 'allpass', 'volume'];
// VOICEVOX 互換エンジンで反映されない（disabled にする）パラメータ。
const VOICEVOX_DISABLED = new Set(['tone', 'threshold', 'allpass', 'volume']);
const VOICEVOX_LIKE = new Set(['voicevox', 'aivisspeech']);
const SLIDER_STEP = 0.1;
const WORD_MAX = 64;

// エンジン表示順（optgroup 並び順）。
const ENGINE_ORDER = ['openjtalk', 'local', 'voicevox', 'aivisspeech', 'voiceroid', 'aquestalk'];

// 制御文字（C0 制御 + DEL）。改行・タブを含む。
const CONTROL_CHARS = /[\u0000-\u001f\u007f]/;

// ------------------------------------------------------------------
// 状態
// ------------------------------------------------------------------
const state = {
  sdk: null,
  guildId: null,
  user: null,
  voices: [], // [{name, engine}]
  engines: {}, // {engineId: label}
  params: { default: {}, voiceroid: {}, aivisspeech: {} },
  engineByVoice: {}, // voiceName -> engineId
  userInfo: null, // 現在フォームの値（保存前含む）
  savedInfo: null, // サーバーに保存済みの基準値（dirty 判定用）
  words: [], // [{word, reading}]
  wordSearch: '',
  pendingDelete: null, // 2段階削除確認中の word
  saving: false,
};

// ------------------------------------------------------------------
// DOM 参照ヘルパ
// ------------------------------------------------------------------
const $ = (id) => document.getElementById(id);

// ------------------------------------------------------------------
// ローディング / 致命的エラー
// ------------------------------------------------------------------
function setLoadingStage(text) {
  const el = $('loading-text');
  if (el) el.textContent = text;
}

function showFatal(message) {
  $('loading').hidden = true;
  $('app').hidden = true;
  $('fatal-message').textContent = message;
  $('fatal').hidden = false;
}

function showApp() {
  $('loading').hidden = true;
  $('fatal').hidden = true;
  $('app').hidden = false;
}

// ------------------------------------------------------------------
// トースト
// ------------------------------------------------------------------
let toastTimer = null;
function showToast(message, kind /* 'success' | 'error' | 'info' */) {
  const el = $('toast');
  el.textContent = '';
  const icon = document.createElement('span');
  icon.className = 'toast-icon';
  icon.setAttribute('aria-hidden', 'true');
  icon.textContent = kind === 'error' ? '!' : kind === 'success' ? '✓' : 'i';
  const span = document.createElement('span');
  span.className = 'toast-text';
  span.textContent = message; // ユーザー由来文字列も安全（textContent）
  el.appendChild(icon);
  el.appendChild(span);
  el.className = 'toast toast-' + (kind || 'info') + ' toast-show';
  el.hidden = false;
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(
    () => {
      el.className = 'toast';
      el.hidden = true;
    },
    kind === 'error' ? 6000 : 3500
  );
}

// ------------------------------------------------------------------
// 起動フロー
// ------------------------------------------------------------------
async function boot() {
  try {
    setLoadingStage('Discordに接続中…');
    const cfg = await api.getConfig();
    if (!cfg || !cfg.client_id) throw new Error('client_id を取得できませんでした');

    const sdk = new DiscordSDK(cfg.client_id);
    state.sdk = sdk;
    // ready()（Discordクライアントとのハンドシェイク）にはタイムアウトが
    // 無く、失敗すると無限スピナーになるためウォッチドッグを張る。
    // authorize（同意ダイアログ）はユーザー操作待ちなので対象外。
    const readyWatchdog = setTimeout(() => {
      showFatal('Discordクライアントとの接続がタイムアウトしました。アクティビティを一度閉じて、開き直してください。');
    }, 30000);
    await sdk.ready();
    clearTimeout(readyWatchdog);

    setLoadingStage('認証中…');
    const { code } = await sdk.commands.authorize({
      client_id: cfg.client_id,
      response_type: 'code',
      state: '',
      prompt: 'none',
      scope: ['identify', 'guilds'],
    });

    const tokenRes = await api.postToken(code);
    if (!tokenRes || !tokenRes.access_token) throw new Error('アクセストークンを取得できませんでした');
    api.setToken(tokenRes.access_token);

    await sdk.commands.authenticate({ access_token: tokenRes.access_token });

    state.guildId = sdk.guildId || null;

    setLoadingStage('データを読み込み中…');
    // 音声設定に必須の3つは並列取得。いずれか失敗で致命的。
    const [me, voices, myVoice] = await Promise.all([
      api.getMe(),
      api.getVoices(),
      api.getMyVoice(),
    ]);

    applyMe(me);
    applyVoices(voices);
    applyUserInfo(myVoice && myVoice.user_info);

    buildUI();
    showApp();

    // 辞書は非必須。ギルドがあれば別途読み込み（失敗しても致命的にしない）。
    if (state.guildId) {
      loadWords();
    } else {
      renderDictMode();
    }
  } catch (e) {
    const msg = e && e.message ? e.message : 'アプリの初期化に失敗しました。';
    showFatal(msg);
    // トークンそのものは含めない安全なログ。
    console.error('boot failed:', e && e.code ? e.code : msg);
  }
}

// ------------------------------------------------------------------
// データ適用
// ------------------------------------------------------------------
function applyMe(me) {
  const u = (me && me.user) || {};
  state.user = u;
  const name = u.global_name || u.username || 'ユーザー';
  const el = $('username');
  el.textContent = name; // ユーザー名は textContent（XSS 防止）
  el.title = name;
}

function applyVoices(v) {
  state.voices = Array.isArray(v && v.voices) ? v.voices : [];
  state.engines = (v && v.engines) || {};
  const p = (v && v.params) || {};
  state.params.default = p.default || {};
  state.params.voiceroid = p.voiceroid || p.default || {};
  state.params.aivisspeech = p.aivisspeech || p.default || {};
  state.engineByVoice = {};
  for (const item of state.voices) {
    if (item && item.name) state.engineByVoice[item.name] = item.engine || '';
  }
}

function applyUserInfo(ui) {
  const src = ui || {};
  const norm = {
    voice: typeof src.voice === 'string' ? src.voice : 'normal',
    speed: num(src.speed, 1.0),
    tone: num(src.tone, 0.0),
    intone: num(src.intone, 1.0),
    threshold: num(src.threshold, 0.5),
    allpass: num(src.allpass, 0.0),
    volume: num(src.volume, 1.0),
  };
  state.userInfo = norm;
  state.savedInfo = Object.assign({}, norm);
}

function num(v, fallback) {
  const n = typeof v === 'number' ? v : parseFloat(v);
  return Number.isFinite(n) ? n : fallback;
}

// ------------------------------------------------------------------
// UI 構築（初回のみ静的部品にデータを流し込む）
// ------------------------------------------------------------------
function buildUI() {
  buildVoiceSelect();
  bindTabs();
  bindVoiceControls();
  bindDictControls();
  applyParamConstraints(); // レンジ・disabled を設定
  renderParamValues(); // 現在値を反映
  renderDirty();
}

function buildVoiceSelect() {
  const sel = $('voice-select');
  sel.textContent = '';
  // エンジンごとにグループ化。
  const byEngine = new Map();
  for (const v of state.voices) {
    if (!v || !v.name) continue;
    const eng = v.engine || 'other';
    if (!byEngine.has(eng)) byEngine.set(eng, []);
    byEngine.get(eng).push(v);
  }
  // 既知エンジン順 → 未知エンジンは末尾。
  const engineIds = [];
  for (const e of ENGINE_ORDER) if (byEngine.has(e)) engineIds.push(e);
  for (const e of byEngine.keys()) if (!engineIds.includes(e)) engineIds.push(e);

  for (const eng of engineIds) {
    const group = document.createElement('optgroup');
    group.label = state.engines[eng] || eng;
    for (const v of byEngine.get(eng)) {
      const opt = document.createElement('option');
      opt.value = v.name;
      opt.textContent = v.name; // 声名は textContent
      group.appendChild(opt);
    }
    sel.appendChild(group);
  }

  // 現在値を選択。保存済みの声が一覧に無い（エンジン停止中など）ときは、
  // 別の声にすり替えて保存させないよう「利用できない声」として表示だけする。
  const saved = state.userInfo && state.userInfo.voice;
  if (saved && hasVoice(saved)) {
    sel.value = saved;
  } else if (saved) {
    const group = document.createElement('optgroup');
    group.label = '現在利用できない声';
    const opt = document.createElement('option');
    opt.value = saved;
    opt.textContent = saved + '（現在利用できません）';
    opt.disabled = true;
    group.appendChild(opt);
    sel.insertBefore(group, sel.firstChild);
    sel.value = saved;
  } else if (sel.options.length > 0) {
    sel.value = sel.options[0].value;
    state.userInfo.voice = sel.value;
  }
}

function hasVoice(name) {
  return Object.prototype.hasOwnProperty.call(state.engineByVoice, name);
}

// 現在選択中の声のエンジンに応じたレンジ集合を返す。
function currentEngine() {
  return state.engineByVoice[state.userInfo.voice] || '';
}
function currentRangeSet() {
  const engine = currentEngine();
  if (engine === 'voiceroid') return state.params.voiceroid;
  if (engine === 'aivisspeech') return state.params.aivisspeech;
  return state.params.default;
}

// ------------------------------------------------------------------
// パラメータ：レンジ・disabled・値
// ------------------------------------------------------------------
function applyParamConstraints() {
  const engine = currentEngine();
  const ranges = currentRangeSet();
  let clamped = false;

  for (const key of PARAMS) {
    const range = $('p-' + key + '-range');
    const number = $('p-' + key + '-num');
    const r = ranges[key] || state.params.default[key] || [0, 1];
    const min = r[0];
    const max = r[1];

    range.min = String(min);
    range.max = String(max);
    range.step = String(SLIDER_STEP);
    number.min = String(min);
    number.max = String(max);
    number.step = String(SLIDER_STEP);

    // レンジ外の現在値はクランプ。
    const before = state.userInfo[key];
    const after = clamp(before, min, max);
    if (after !== before) {
      state.userInfo[key] = after;
      clamped = true;
    }

    // VOICEVOX / AivisSpeech：未反映パラメータを無効化。
    const disabled = VOICEVOX_LIKE.has(engine) && VOICEVOX_DISABLED.has(key);
    range.disabled = disabled;
    number.disabled = disabled;
    const row = document.querySelector('.param-row[data-param="' + key + '"]');
    if (row) row.classList.toggle('is-disabled', disabled);
  }

  // 注記表示。
  const notice = $('engine-notice');
  if (engine === 'voicevox') {
    notice.textContent = 'VOICEVOXでは「速度」と「イントネーション」のみ反映されます。他の項目は無効です。';
    notice.hidden = false;
  } else if (engine === 'aivisspeech') {
    notice.textContent = 'AivisSpeechでは「速度」と「イントネーション」（感情の強さ、0〜2）のみ反映されます。他の項目は無効です。';
    notice.hidden = false;
  } else {
    notice.hidden = true;
    notice.textContent = '';
  }

  if (clamped && (engine === 'voiceroid' || engine === 'aivisspeech')) {
    showToast((state.engines[engine] || engine) + 'の範囲に調整しました', 'info');
  }
}

function renderParamValues() {
  for (const key of PARAMS) {
    const val = state.userInfo[key];
    $('p-' + key + '-range').value = String(val);
    $('p-' + key + '-num').value = String(val);
  }
}

function clamp(v, min, max) {
  // レンジ内はサーバー値をそのまま保持（0.1丸めはしない）。dirty判定は
  // isDirty 側の許容差(1e-9)で浮動小数のブレを吸収する。
  if (!Number.isFinite(v)) return min;
  if (v < min) return min;
  if (v > max) return max;
  return v;
}

// ------------------------------------------------------------------
// dirty 表示
// ------------------------------------------------------------------
function isDirty() {
  const a = state.userInfo;
  const b = state.savedInfo;
  if (!a || !b) return false;
  if (a.voice !== b.voice) return true;
  for (const key of PARAMS) {
    if (Math.abs(a[key] - b[key]) > 1e-9) return true;
  }
  return false;
}

function renderDirty() {
  const dirty = isDirty();
  const btn = $('save-btn');
  btn.classList.toggle('is-dirty', dirty && !state.saving);
  $('dirty-hint').hidden = !dirty;
}

// ------------------------------------------------------------------
// タブ
// ------------------------------------------------------------------
function bindTabs() {
  const tabs = [$('tab-voice'), $('tab-dict')];
  const panels = { 'tab-voice': $('panel-voice'), 'tab-dict': $('panel-dict') };

  function select(tab) {
    for (const t of tabs) {
      const active = t === tab;
      t.setAttribute('aria-selected', active ? 'true' : 'false');
      t.tabIndex = active ? 0 : -1;
      panels[t.id].hidden = !active;
    }
    tab.focus();
  }

  tabs.forEach((tab, i) => {
    tab.addEventListener('click', () => select(tab));
    tab.addEventListener('keydown', (e) => {
      if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
        e.preventDefault();
        select(tabs[(i + 1) % tabs.length]);
      } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
        e.preventDefault();
        select(tabs[(i - 1 + tabs.length) % tabs.length]);
      } else if (e.key === 'Home') {
        e.preventDefault();
        select(tabs[0]);
      } else if (e.key === 'End') {
        e.preventDefault();
        select(tabs[tabs.length - 1]);
      }
    });
  });
}

// ------------------------------------------------------------------
// 音声コントロールのイベント
// ------------------------------------------------------------------
function bindVoiceControls() {
  const sel = $('voice-select');
  sel.addEventListener('change', () => {
    state.userInfo.voice = sel.value;
    applyParamConstraints();
    renderParamValues();
    renderDirty();
  });

  for (const key of PARAMS) {
    const range = $('p-' + key + '-range');
    const number = $('p-' + key + '-num');

    range.addEventListener('input', () => {
      const v = clamp(parseFloat(range.value), Number(range.min), Number(range.max));
      state.userInfo[key] = v;
      number.value = String(v);
      renderDirty();
    });

    // number は入力途中の空文字を許容し、確定時（change/blur）にクランプ。
    number.addEventListener('input', () => {
      const raw = parseFloat(number.value);
      if (Number.isFinite(raw)) {
        const v = clamp(raw, Number(number.min), Number(number.max));
        state.userInfo[key] = v;
        range.value = String(v);
        renderDirty();
      }
    });
    number.addEventListener('change', () => {
      const v = clamp(parseFloat(number.value), Number(number.min), Number(number.max));
      state.userInfo[key] = v;
      number.value = String(v);
      range.value = String(v);
      renderDirty();
    });
  }

  $('save-btn').addEventListener('click', onSave);
  $('random-btn').addEventListener('click', onRandom);
}

async function onSave() {
  if (state.saving) return;
  setSaving(true, '保存中…');
  try {
    const res = await api.putMyVoice(buildUserInfoPayload());
    if (res && res.user_info) applyUserInfo(res.user_info);
    buildVoiceSelect();
    applyParamConstraints();
    renderParamValues();
    showToast('音声設定を保存しました', 'success');
  } catch (e) {
    showToast(e.message || '保存に失敗しました', 'error');
  } finally {
    setSaving(false, '保存');
    renderDirty();
  }
}

async function onRandom() {
  if (state.saving) return;
  const btn = $('random-btn');
  btn.disabled = true;
  const orig = btn.textContent;
  btn.textContent = '変更中…';
  try {
    const res = await api.postRandom();
    if (res && res.user_info) applyUserInfo(res.user_info);
    buildVoiceSelect();
    applyParamConstraints();
    renderParamValues();
    showToast('ランダムな音声に変更して保存しました', 'success');
  } catch (e) {
    showToast(e.message || 'ランダム変更に失敗しました', 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = orig;
    renderDirty();
  }
}

function setSaving(on, label) {
  state.saving = on;
  const btn = $('save-btn');
  btn.disabled = on;
  btn.textContent = label;
  if (on) btn.classList.remove('is-dirty');
}

function buildUserInfoPayload() {
  const u = state.userInfo;
  return {
    voice: u.voice,
    speed: u.speed,
    tone: u.tone,
    intone: u.intone,
    threshold: u.threshold,
    allpass: u.allpass,
    volume: u.volume,
  };
}

// ------------------------------------------------------------------
// 辞書
// ------------------------------------------------------------------
function bindDictControls() {
  $('add-form').addEventListener('submit', onAddWord);
  $('word-input').addEventListener('input', clearAddError);
  $('reading-input').addEventListener('input', clearAddError);

  const search = $('search-input');
  search.addEventListener('input', () => {
    state.wordSearch = search.value;
    $('search-clear').hidden = search.value.length === 0;
    state.pendingDelete = null;
    renderWords();
  });
  $('search-clear').addEventListener('click', () => {
    search.value = '';
    state.wordSearch = '';
    $('search-clear').hidden = true;
    search.focus();
    renderWords();
  });
  $('noresults-clear').addEventListener('click', () => {
    search.value = '';
    state.wordSearch = '';
    $('search-clear').hidden = true;
    renderWords();
    search.focus();
  });
  $('dict-retry').addEventListener('click', loadWords);
}

// ギルド有無で辞書タブの表示を切替。
function renderDictMode() {
  const hasGuild = !!state.guildId;
  $('dict-dm').hidden = hasGuild;
  $('dict-main').hidden = !hasGuild;
}

async function loadWords() {
  renderDictMode();
  if (!state.guildId) return;
  $('dict-error').hidden = true;
  showWordSkeleton();
  try {
    const res = await api.getWords(state.guildId);
    state.words = Array.isArray(res && res.words) ? res.words : [];
    state.pendingDelete = null;
    renderWords();
  } catch (e) {
    $('word-list').textContent = '';
    $('dict-empty').hidden = true;
    $('dict-noresults').hidden = true;
    $('dict-error-msg').textContent = e.message || '辞書の読み込みに失敗しました';
    $('dict-error').hidden = false;
    $('word-count').textContent = '';
  }
}

// レイアウトシフト防止：実際の行と同じ高さのスケルトンを表示。
function showWordSkeleton() {
  const list = $('word-list');
  list.textContent = '';
  $('dict-empty').hidden = true;
  $('dict-noresults').hidden = true;
  for (let i = 0; i < 4; i++) {
    const li = document.createElement('li');
    li.className = 'word-item skeleton';
    li.setAttribute('aria-hidden', 'true');
    const bar = document.createElement('span');
    bar.className = 'skeleton-bar';
    li.appendChild(bar);
    list.appendChild(li);
  }
  $('word-count').textContent = '';
}

function filteredWords() {
  const q = state.wordSearch.trim().toLowerCase();
  if (!q) return state.words;
  return state.words.filter(
    (w) =>
      (w.word && w.word.toLowerCase().includes(q)) ||
      (w.reading && w.reading.toLowerCase().includes(q))
  );
}

function renderWords() {
  renderDictMode();
  if (!state.guildId) return;

  const list = $('word-list');
  const total = state.words.length;
  const shown = filteredWords();

  // 件数表示（「12 / 45件」）。
  $('word-count').textContent = state.wordSearch.trim()
    ? shown.length + ' / ' + total + '件'
    : total + '件';

  list.textContent = '';

  // 空状態・0件状態の出し分け。
  $('dict-empty').hidden = true;
  $('dict-noresults').hidden = true;
  if (total === 0) {
    $('dict-empty').hidden = false;
    return;
  }
  if (shown.length === 0) {
    $('noresults-title').textContent =
      '「' + state.wordSearch.trim() + '」に一致する単語はありません';
    $('dict-noresults').hidden = false;
    return;
  }

  for (const w of shown) {
    list.appendChild(renderWordItem(w));
  }
}

function renderWordItem(w) {
  const li = document.createElement('li');
  li.className = 'word-item';

  const info = document.createElement('div');
  info.className = 'word-info';
  const word = document.createElement('span');
  word.className = 'word-text';
  word.textContent = w.word; // ユーザー入力：textContent
  const arrow = document.createElement('span');
  arrow.className = 'word-arrow';
  arrow.setAttribute('aria-hidden', 'true');
  arrow.textContent = '→';
  const reading = document.createElement('span');
  reading.className = 'word-reading';
  reading.textContent = w.reading; // ユーザー入力：textContent
  info.appendChild(word);
  info.appendChild(arrow);
  info.appendChild(reading);

  const actions = document.createElement('div');
  actions.className = 'word-actions';

  if (state.pendingDelete === w.word) {
    // 2段階目：確認。
    const confirmText = document.createElement('span');
    confirmText.className = 'confirm-text';
    confirmText.textContent = '削除しますか？';
    const yes = document.createElement('button');
    yes.type = 'button';
    yes.className = 'btn btn-danger btn-sm';
    yes.textContent = 'はい';
    yes.addEventListener('click', () => onDeleteConfirmed(w.word));
    const no = document.createElement('button');
    no.type = 'button';
    no.className = 'btn btn-secondary btn-sm';
    no.textContent = 'キャンセル';
    no.addEventListener('click', () => {
      state.pendingDelete = null;
      renderWords();
    });
    actions.appendChild(confirmText);
    actions.appendChild(yes);
    actions.appendChild(no);
  } else {
    // 1段階目：削除ボタン。
    const del = document.createElement('button');
    del.type = 'button';
    del.className = 'btn btn-ghost btn-sm';
    del.textContent = '削除';
    del.setAttribute('aria-label', '単語「' + w.word + '」を削除');
    del.addEventListener('click', () => {
      state.pendingDelete = w.word;
      renderWords();
    });
    actions.appendChild(del);
  }

  li.appendChild(info);
  li.appendChild(actions);
  return li;
}

async function onDeleteConfirmed(word) {
  try {
    const res = await api.deleteWord(state.guildId, word);
    state.words = Array.isArray(res && res.words) ? res.words : [];
    state.pendingDelete = null;
    renderWords();
    showToast('単語を削除しました', 'success');
  } catch (e) {
    state.pendingDelete = null;
    renderWords();
    showToast(e.message || '削除に失敗しました', 'error');
  }
}

// クライアント側バリデーション（空・65文字以上・改行/制御文字）。
// 文字数はコードポイント単位（[...v].length）で数え、サーバーのrune数と揃える
// （.length はUTF-16単位のため絵文字等で過剰に厳しくなる）。
function validateWordField(value) {
  const v = value.trim();
  const len = [...v].length;
  if (len === 0) return '空欄です。入力してください。';
  if (len > WORD_MAX) return WORD_MAX + '文字以内で入力してください。';
  if (CONTROL_CHARS.test(v)) return '改行や制御文字は使えません。';
  return null;
}

function showAddError(msg) {
  const el = $('add-error');
  el.textContent = msg;
  el.hidden = false;
}
function clearAddError() {
  const el = $('add-error');
  if (!el.hidden) {
    el.hidden = true;
    el.textContent = '';
  }
}

async function onAddWord(e) {
  e.preventDefault();
  const wordEl = $('word-input');
  const readingEl = $('reading-input');
  const word = wordEl.value;
  const reading = readingEl.value;

  const wErr = validateWordField(word);
  if (wErr) {
    showAddError('単語：' + wErr);
    wordEl.focus();
    return;
  }
  const rErr = validateWordField(reading);
  if (rErr) {
    showAddError('読み：' + rErr);
    readingEl.focus();
    return;
  }

  const btn = $('add-btn');
  btn.disabled = true;
  const orig = btn.textContent;
  btn.textContent = '追加中…';
  try {
    const res = await api.postWord(state.guildId, word.trim(), reading.trim());
    state.words = Array.isArray(res && res.words) ? res.words : [];
    wordEl.value = '';
    readingEl.value = '';
    clearAddError();
    state.pendingDelete = null;
    renderWords();
    showToast('単語を追加しました', 'success');
    wordEl.focus();
  } catch (err) {
    showAddError(err.message || '追加に失敗しました');
  } finally {
    btn.disabled = false;
    btn.textContent = orig;
  }
}

// ------------------------------------------------------------------
// 起動
// ------------------------------------------------------------------
$('reload-btn').addEventListener('click', () => location.reload());
boot();
