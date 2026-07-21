# API 仕様書

ベースURL: `http://localhost:8080`

## 認証

X-User-ID ヘッダーもしくは Bearer Token でユーザーを識別します。

```
X-User-ID: user-1
Authorization: Bearer sess-user-1
```

事前に `POST /v1/auth/sessions` でセッションを作成し、返却された `token` を Authorization ヘッダーに指定します。
簡易実装のため、当面は `X-User-ID` ヘッダー直指定も利用可能です。

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

### `POST /v1/auth/sessions` — セッション作成

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

⚠️ 現在未実装（404 が返ります）

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

⚠️ 現在未実装（404 が返ります）

### `PATCH /v1/colonies/{colonyId}` — コロニー更新

⚠️ 現在未実装（404 が返ります）

### `DELETE /v1/colonies/{colonyId}` — コロニー削除

⚠️ 現在未実装（404 が返ります）

### `POST /v1/colonies/{colonyId}/join` — コロニー参加

Headers: `Content-Type: application/json`, `X-User-ID: user-1`

⚠️ 現在スタブ実装（常に `{"ok": true}` を返す）

### `POST /v1/colonies/{colonyId}/leave` — コロニー退出

Headers: `X-User-ID: user-1`

⚠️ 現在スタブ実装（常に `{"ok": true}` を返す）

### `GET /v1/colonies/{colonyId}/members` — メンバー一覧

⚠️ 現在スタブ実装（常に空配列を返す）

### `GET /v1/colonies/{colonyId}/feed` — 共有アイテム一覧

⚠️ 現在スタブ実装（常に空配列を返す）

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

## AnalysisJob API（AI解析ジョブ）

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
