# API 仕様書

ベースURL: `http://localhost:8080`

## CORS

`FRONTEND_ORIGIN` に指定した1つのオリジンからのみ、ブラウザで呼べます。

```bash
FRONTEND_ORIGIN=http://localhost:3000 go run .
```

未設定なら CORS ヘッダーを出しません（フロントを同一オリジンで配信する構成）。
プリフライト（OPTIONS）は認証より手前で 204 を返します。ブラウザは
プリフライトに認証情報を載せないので、認証に通すと必ず 401 で落ちるためです。

## 保存先

`DATABASE_URL` を設定すると Postgres に保存します。未設定ならメモリ上で動き、
プロセスを落とすと全部消えます（デモ用。残したいものを置く場所ではありません）。

スキーマは起動時に自動で適用されます。全ての文が `IF NOT EXISTS` なので、
毎回動かしても害はありません。

## 認証

`Authorization: Bearer <token>` でユーザーを識別します。
トークンは登録・ログインで受け取ります。

```
Authorization: Bearer 4f3c...（64桁の16進）
```

パスワードは argon2id で、セッショントークンは SHA-256 で保存されます。
どちらも平文では保存されません。

`X-User-ID` ヘッダーの直指定と、パスワード無しの `POST /v1/auth/sessions` は
**既定で無効**です。`ALLOW_DEV_AUTH=true` を立てたときだけ通ります。
どちらもパスワードを迂回できるので、開発機以外では立てないでください。

---

## 共通レスポンス形式

### 成功

```json
{
  "data": { ... }
}
```

### 一覧

```json
{
  "data": [ ... ]
}
```

### エラー

```json
{
  "error": {
    "code": "VALIDATION_ERROR"
  }
}
```

### エラーコード一覧

| HTTP | Code | 意味 |
|------|------|------|
| 400 | VALIDATION_ERROR | 入力値不正 |
| 401 | UNAUTHORIZED | 未認証 |
| 403 | FORBIDDEN | 権限不足 |
| 404 | NOT_FOUND | リソースなし |
| 409 | CONFLICT | 重複・競合 |
| 500 | INTERNAL_ERROR | サーバー内部エラー |

---

## 認証 API

### `POST /v1/auth/register` — 登録

Request:
```json
{ "email": "zav@example.com", "password": "correcthorse", "displayName": "ザビエル" }
```

Response `200`:
```json
{
  "data": {
    "token": "4f3c...",
    "user": { "id": "1786575858797889066", "displayName": "ザビエル" }
  }
}
```

`displayName` を省くとメールの `@` より前を使います。

- `400` — メールが不正、またはパスワードが8文字未満
- `409` — そのメールは登録済み

### `POST /v1/auth/login` — ログイン

Request: `{ "email": "...", "password": "..." }`

Response `200`: 登録と同じ形

`401` — パスワードが違う場合と、そのメールが存在しない場合の**両方**。
どちらか判別できると、誰がここにアカウントを持っているか調べられてしまうためです。

### `POST /v1/auth/logout` — ログアウト

Headers: `Authorization: Bearer <token>`

Response `204`: セッションを破棄

### `GET /v1/auth/me` — 自分を引く

Response `200`: `{ "data": { "id": "...", "displayName": "...", "email": "..." } }`

### `PATCH /v1/auth/me` — 表示名を変える

Request: `{ "displayName": "..." }`

Response `200`: 更新後のユーザー。パスワードは不要です — ラベルを変えるだけなので。

- `400` — 空の名前

### `DELETE /v1/auth/me` — アカウントを削除する

Request: `{ "currentPassword": "..." }`

Response `204`: アカウントと、それが持つ全てのもの（予定・タスク・プロジェクト・
時間割・プリント・自分が持ち主のコロニーとその参加者・共有アイテム・全セッション）
を削除します。**元に戻せません。**

- `401` — パスワードが違う

### `POST /v1/auth/change-password` — ログイン中にパスワードを変える

Request: `{ "currentPassword": "...", "newPassword": "..." }`

Response `200`: `{ "data": { "token": "新しいトークン" } }`

パスワード再設定と同じ理由で、**他の全セッションを破棄**します。ただし今使っている
セッションまで切ってしまうと不便なので、代わりに新しいトークンを発行して返します。

- `401` — 現在のパスワードが違う
- `400` — 新しいパスワードが8文字未満

### `POST /v1/auth/change-email` — メールアドレスを変える

Request: `{ "currentPassword": "...", "newEmail": "..." }`

Response `200`: 更新後のユーザー

- `401` — 現在のパスワードが違う
- `409` — そのメールは別のアカウントで使用済み
- `400` — メールの形式が不正

### `POST /v1/auth/password-reset` — 再設定を申し込む

Request: `{ "email": "..." }`

Response `204`: **登録の有無にかかわらず 204 を返します。**
区別できると、誰がここにアカウントを持っているか調べる道具になるためです。

再設定コードはメールで届きます。`SMTP_HOST` を設定してください。
**未設定だとサーバーのログに出るだけで誰にも届きません**（起動時にその旨が出ます）。

送信は必ず暗号化されます。STARTTLS を出さないサーバーには送らずに失敗します。

### `POST /v1/auth/password-reset/confirm` — 新しいパスワードを設定する

Request: `{ "token": "...", "password": "..." }`

Response `204`: 成功。**そのユーザーの既存セッションは全て破棄されます**
（再設定する人は、他人に使われているから再設定している可能性があるため）。

- `400 INVALID_TOKEN` — トークンが不正・期限切れ・使用済み
- `400 VALIDATION_ERROR` — パスワードが8文字未満

トークンは30分で失効し、一度しか使えません。保存はハッシュのみです。

### `POST /v1/auth/sessions` — セッション作成（旧・開発用）

Request:
```json
{
  "userId": "user-1",
  "name": "Alice"
}
```

Response `201`:
```json
{
  "data": {
    "id": "1784626018489794067",
    "userId": "user-1",
    "token": "sess-user-1",
    "name": "Alice"
  }
}
```

### `DELETE /v1/auth/sessions/{token}` — セッション削除（ログアウト）

Response `204`: 成功
Response `404`: トークンが見つからない

---

## Event API

### `GET /v1/events` — イベント一覧

Headers: `X-User-ID: user-1`

Response `200`:
```json
{
  "data": [
    {
      "id": "1784626018489794067",
      "title": "学校行事",
      "description": "体育館集合",
      "startAt": "2026-07-25T09:00:00Z",
      "endAt": "2026-07-25T12:00:00Z",
      "allDay": false,
      "version": 1
    }
  ]
}
```

### `POST /v1/events` — イベント作成

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

Request:
```json
{
  "title": "学校行事",
  "description": "体育館集合",
  "startAt": "2026-07-25T09:00:00Z",
  "endAt": "2026-07-25T12:00:00Z",
  "allDay": false
}
```

Response `201`: 作成されたイベント（GET一覧と同じ形式）

### `GET /v1/events/{eventId}` — イベント取得

Headers: `X-User-ID: user-1`

Response `200`: イベントオブジェクト
Response `404`: 存在しない

### `PATCH /v1/events/{eventId}` — イベント更新

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

Request（全てのフィールドが省略可能）:
```json
{
  "title": "更新後のタイトル",
  "description": "更新後の説明",
  "startAt": "2026-07-26T09:00:00Z",
  "endAt": "2026-07-26T12:00:00Z",
  "allDay": true,
  "version": 1
}
```

`version` は必須。DBの値と一致しない場合は `409 Conflict`。

Response `200`: 更新後のイベント
Response `409`: バージョン競合

### `DELETE /v1/events/{eventId}` — イベント削除

Headers: `X-User-ID: user-1`

Response `204`: 成功
Response `404`: 存在しない

---

## 時間割 API

### `GET /v1/timetable-entries` — 時間割一覧

Headers: `X-User-ID: user-1`

Response `200`:
```json
{
  "data": [
    {
      "id": "1784626018489794067",
      "dayOfWeek": 1,
      "period": 2,
      "subject": "数学",
      "room": "3年1組",
      "teacher": "",
      "version": 1
    }
  ]
}
```

`dayOfWeek`: 1=月曜, 2=火曜, ... 7=日曜

### `POST /v1/timetable-entries` — 時間割登録

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

Request:
```json
{
  "dayOfWeek": 1,
  "period": 2,
  "subject": "数学",
  "room": "3年1組",
  "teacher": ""
}
```

Response `201`: 作成されたエントリ

### `GET /v1/timetable-entries/{entryId}` — 時間割取得

Headers: `X-User-ID: user-1`

Response `200`: エントリオブジェクト
Response `404`: 存在しない

### `PATCH /v1/timetable-entries/{entryId}` — 時間割更新

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

Request（全てのフィールドが省略可能）:
```json
{
  "subject": "物理",
  "room": "理科室",
  "version": 1
}
```

Response `200`: 更新後のエントリ
Response `409`: バージョン競合

### `DELETE /v1/timetable-entries/{entryId}` — 時間割削除

Headers: `X-User-ID: user-1`

Response `204`: 成功

---

## Colony API（コロニー/グループ）

### `GET /v1/colonies` — コロニー一覧

自分がメンバーであるコロニーを返す（作成したものも、参加しただけのものも含む）。

Headers: `X-User-ID: user-1`

Response `200`:
```json
{
  "data": [
    {
      "id": "1784626018489794067",
      "name": "3年1組",
      "description": "クラス共有",
      "ownerUserId": "user-1",
      "inviteCode": "00000001"
    }
  ]
}
```

### `POST /v1/colonies` — コロニー作成

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

Request:
```json
{
  "name": "3年1組",
  "description": "クラス共有"
}
```

Response `201`:
```json
{
  "data": {
    "id": "1784626018489794067",
    "name": "3年1組",
    "description": "クラス共有",
    "ownerUserId": "user-1",
    "inviteCode": "00000001"
  }
}
```

`inviteCode` は作成時に一度だけ返却されます。

### `GET /v1/colonies/{colonyId}` — コロニー取得

Headers: `X-User-ID: user-1`

Response `200`: コロニーオブジェクト
Response `404`: 存在しない / メンバーでない

### `PATCH /v1/colonies/{colonyId}` — コロニー更新

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

Request（全てのフィールドが省略可能）: `{ "name": "...", "description": "..." }`

Response `200`: 更新後のコロニー

### `DELETE /v1/colonies/{colonyId}` — コロニー削除

Headers: `X-User-ID: user-1`

Response `204`: 成功
Response `403`: 作成者ではない

### `POST /v1/colonies/join` — 招待コードで参加

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

招待される側はコロニー ID を知らないので、コードだけで参加できる。

Request:
```json
{ "inviteCode": "00000001" }
```

Response `200`: 参加したコロニー（ID と名前が分かる）
Response `400`: `inviteCode` が空
Response `404`: そのコードのコロニーが無い
Response `409`: 既に参加している

### `POST /v1/colonies/{colonyId}/join` — コロニー参加（ID 指定）

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

Request: `{ "inviteCode": "00000001" }`

Response `200`: `{"ok": true}`
Response `403`: 招待コードが違う

### `POST /v1/colonies/{colonyId}/leave` — コロニー退出

Headers: `X-User-ID: user-1`

Response `200`: `{"ok": true}`

### `GET /v1/colonies/{colonyId}/members` — メンバー一覧

Response `200`:
```json
{
  "data": [
    { "colonyId": "...", "userId": "user-1", "displayName": "ザビエル", "role": "OWNER", "joinedAt": "..." },
    { "colonyId": "...", "userId": "user-2", "displayName": "べつの人", "role": "MEMBER", "joinedAt": "..." }
  ]
}
```

### `GET /v1/colonies/{colonyId}/feed` — 共有アイテム一覧

Response `200`: SharedItem の配列

---

## SharedItem API（共有アイテム / Echo）

### `POST /v1/colonies/{colonyId}/shared-items` — アイテム共有

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

Request:
```json
{
  "sourceType": "TASK",
  "sourceId": "task-1"
}
```

Response `201`:
```json
{
  "data": {
    "id": "1784626018489794067",
    "colonyId": "...",
    "sourceType": "TASK",
    "sourceId": "task-1",
    "createdBy": "user-1",
    "titleSnapshot": ""
  }
}
```

Response `409`: 同じ `(colonyId, sourceType, sourceId)` の組み合わせが既に存在

### `DELETE /v1/colonies/{colonyId}/shared-items/{sharedItemId}` — 共有解除

Headers: `X-User-ID: user-1`

Response `204`: 成功

---

## Task API

### `GET /v1/tasks` — タスク一覧

期限の近い順。期限なしは後ろに回ります。

Response `200`:
```json
{
  "data": [
    {
      "id": "1784626018489794067",
      "title": "看板デザイン案の提出",
      "description": "クラス企画用",
      "dueAt": "2026-08-20",
      "status": "OPEN",
      "projectId": "1784626018489794000",
      "version": 1
    }
  ]
}
```

`dueAt` は日付のみ（`YYYY-MM-DD`）。タスクは「その日まで」であって時刻を持たないため。
`projectId` は無所属なら `null`。

### `POST /v1/tasks` — タスク作成

Request:
```json
{
  "title": "看板デザイン案の提出",
  "description": "クラス企画用",
  "dueAt": "2026-08-20",
  "status": "OPEN",
  "projectId": "..."
}
```

`title` は必須。`status` を省くと `OPEN`。
`400`: `title` が空 / `status` が `OPEN`・`DONE` 以外 / `dueAt` が `YYYY-MM-DD` でない /
`projectId` が存在しない

### `GET /v1/tasks/{taskId}` — タスク取得

`403`: 他人のタスク

### `PATCH /v1/tasks/{taskId}` — タスク更新

Request（全てのフィールドが省略可能）:
```json
{ "title": "...", "description": "...", "dueAt": "2026-08-20",
  "status": "DONE", "projectId": null, "version": 1 }
```

`projectId` に `null` を送ると無所属になります。省略した場合は現状維持です。
`409`: `version` が食い違う

### `DELETE /v1/tasks/{taskId}` — タスク削除

Response `204`

---

## Project API

### `GET /v1/projects` — プロジェクト一覧

Response `200`:
```json
{
  "data": [
    { "id": "...", "name": "文化祭 実行委員", "description": "11月の準備",
      "version": 1, "createdAt": "2026-08-12T12:00:00Z" }
  ]
}
```

### `POST /v1/projects` — プロジェクト作成

Request: `{ "name": "文化祭 実行委員", "description": "任意" }`

`name` は必須。

### `GET /v1/projects/{projectId}` — 取得
### `PATCH /v1/projects/{projectId}` — 更新（`version` 必須）
### `DELETE /v1/projects/{projectId}` — 削除

削除しても所属していたタスクは消えません。`projectId` が外れて無所属になります。
まとめ方をやめただけで中身の作業まで失うのは割に合わないためです。

---

## AnalysisJob API（AI解析ジョブ）

### `POST /v1/uploads/jobs/{jobId}/analyse` — 画像を送って解析する

Headers: `Authorization: Bearer <token>`

Body: 画像のバイト列をそのまま（最大 10MB）

Response `200`:
```json
{
  "data": {
    "job": { "id": "...", "status": "review_required", "resultSummary": "2件のタスクと1件の予定" },
    "candidates": [
      { "type": "task",  "title": "数学プリント p.24", "date": "2026-08-20" },
      { "type": "event", "title": "保護者会", "date": "2026-08-25", "time": "10:00" }
    ]
  }
}
```

候補はここで返るだけで、カレンダーには入りません。人が確認してから登録します。
読み取れなかった行は捨てます。推測で日付を埋めることはしません。

`status` は `queued` → `processing` → `review_required`、失敗すると `failed`。
`failed` のときは `resultSummary` に理由が入り、候補は空のままです。
解析モデル（`OLLAMA_BASE_URL`）に繋がらないときも `failed` になります。

- `403` — 他人のジョブ
- `413` — 10MB 超

### `GET /v1/uploads/jobs/{jobId}/candidates` — 候補一覧

### `GET /v1/prints` — 読み取ったプリント一覧

解析に送った画像は保存され、あとから見返せます。

### `GET /v1/prints/{printId}/image` — 画像を取り出す

Response `200`: 画像そのもの（`Content-Type` は登録時のもの、`Cache-Control: private`）

`403` — 他人のプリント / `404` — 保存されていない（`BLOB_DIR` が空のとき）

### `GET /v1/uploads/jobs/{jobId}` — ジョブ取得

### `POST /v1/uploads/jobs` — 解析ジョブ作成

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

Request:
```json
{
  "contentType": "image/png",
  "filename": "sample.png"
}
```

Response `201`:
```json
{
  "data": {
    "id": "1784626018489794067",
    "status": "queued",
    "filename": "sample.png",
    "contentType": "image/png",
    "userId": "user-1"
  }
}
```

### `GET /v1/uploads/jobs` — 解析ジョブ一覧

Headers: `X-User-ID: user-1`

Response `200`:
```json
{
  "data": [
    {
      "id": "...",
      "userId": "user-1",
      "contentType": "image/png",
      "filename": "sample.png",
      "status": "queued",
      "createdAt": "2026-07-21T18:26:56Z",
      "updatedAt": "2026-07-21T18:26:56Z",
      "resultSummary": ""
    }
  ]
}
```

---

## データモデル

### Event

| フィールド | 型 | 説明 |
|-----------|-----|------|
| id | string | 自動生成ID |
| title | string | タイトル |
| description | string | 説明 |
| startAt | string (RFC3339) | 開始日時 |
| endAt | string (RFC3339) | 終了日時 |
| allDay | bool | 終日フラグ |
| repeat | object \| null | 繰り返しの規則。単発なら null |
| exdates | string[] | 繰り返しから除いた日（`YYYY-MM-DD`） |
| version | int | 楽観ロック用バージョン |

### Repeat

| フィールド | 型 | 説明 |
|-----------|-----|------|
| freq | string | `daily` / `weekly` / `monthly` |
| until | string | 繰り返しの終わり（`YYYY-MM-DD`）。空なら終わりなし |

系列の展開は呼ぶ側の仕事です。サーバーは規則と除外日を保持するだけです。

`PATCH` で `repeat` を省くと現状維持、`null` を送ると繰り返しを解除します。

### Task

| フィールド | 型 | 説明 |
|-----------|-----|------|
| id | string | 自動生成ID |
| title | string | タイトル |
| description | string | 説明 |
| dueAt | string | 期限（`YYYY-MM-DD`）。無ければ空文字 |
| status | string | `OPEN` / `DONE` |
| projectId | string \| null | 所属プロジェクト。無所属は null |
| version | int | 楽観ロック用バージョン |

### Project

| フィールド | 型 | 説明 |
|-----------|-----|------|
| id | string | 自動生成ID |
| name | string | 名前 |
| description | string | 説明 |
| version | int | 楽観ロック用バージョン |

### TimetableEntry

| フィールド | 型 | 説明 |
|-----------|-----|------|
| id | string | 自動生成ID |
| dayOfWeek | int | 1=月曜〜7=日曜 |
| period | int | 時限（1〜） |
| subject | string | 科目名 |
| room | string | 教室 |
| teacher | string | 教員名 |
| version | int | 楽観ロック用バージョン |

### Colony

| フィールド | 型 | 説明 |
|-----------|-----|------|
| id | string | 自動生成ID |
| name | string | コロニー名 |
| description | string | 説明 |
| ownerUserId | string | 作成者 |
| inviteCode | string | 招待コード（作成時のみ） |

### SharedItem

| フィールド | 型 | 説明 |
|-----------|-----|------|
| id | string | 自動生成ID |
| colonyId | string | 所属コロニー |
| sourceType | string | 元データ種別 (TASK/EVENT) |
| sourceId | string | 元データID |
| createdBy | string | 共有者 |
| titleSnapshot | string | 共有時のタイトル |

### AnalysisJob

| フィールド | 型 | 説明 |
|-----------|-----|------|
| id | string | 自動生成ID |
| userId | string | 所有者 |
| contentType | string | MIMEタイプ |
| filename | string | ファイル名 |
| status | string | queued / processing / done / failed |
| resultSummary | string | 解析結果サマリ |
