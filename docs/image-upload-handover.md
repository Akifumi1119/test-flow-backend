# 画像アップロード機能 引き継ぎメモ

## 概要

タスク作成・コメント作成時に画像を複数枚添付できるようになりました。
画像はCloudinary（外部ストレージ）に保存」され、URLがレスポンスで返されます。

---

## 変更されたエンドポイント

### 1. タスク作成 `POST /api/tasks`

#### リクエスト形式の変更

**変更前：** `Content-Type: application/json`
**変更後：** `Content-Type: multipart/form-data`

#### リクエストフィールド

| フィールド名 | 型 | 必須 | 説明 |
|---|---|---|---|
| `created_by` | number | ✅ | 作成者のユーザーID |
| `project_id` | number | ✅ | プロジェクトID |
| `title` | string | ✅ | タスクタイトル |
| `content` | string | - | タスク本文 |
| `priority` | string | - | `"urgent"` / `"high"` / `"medium"` / `"low"` / `""` |
| `user_name` | number | - | 担当者のユーザーID（0または省略で未割り当て） |
| `images` | File[] | - | 添付画像（複数可、省略可） |

#### レスポンス（変更点のみ）

`images` フィールドが追加されました。

```json
{
  "task_id": 1,
  "title": "タスク名",
  "content": "内容",
  "priority": null,
  "created_by": "山田太郎",
  "user_name": "",
  "created_at": "2026-09-04T10:00:00Z",
  "images": [
    "https://res.cloudinary.com/xxx/image/upload/v000/task-management/abc.jpg",
    "https://res.cloudinary.com/xxx/image/upload/v000/task-management/def.jpg"
  ]
}
```

画像なしの場合は空配列 `[]` が返ります。

---

### 2. コメント作成 `POST /api/comments/:task_id`

#### リクエスト形式の変更

**変更前：** `Content-Type: application/json`
**変更後：** `Content-Type: multipart/form-data`

#### リクエストフィールド

| フィールド名 | 型 | 必須 | 説明 |
|---|---|---|---|
| `user_id` | number | ✅ | 投稿者のユーザーID |
| `task_id` | number | ✅ | タスクID |
| `comment` | string | ✅ | コメント本文 |
| `images` | File[] | - | 添付画像（複数可、省略可） |

#### レスポンス（変更点のみ）

`images` フィールドが追加されました。

```json
{
  "task_id": 1,
  "title": "タスク名",
  "comment_id": 5,
  "comment": "コメント内容",
  "created_by": "山田太郎",
  "created_at": "2026-09-04T10:00:00Z",
  "images": [
    "https://res.cloudinary.com/xxx/image/upload/v000/task-management/ghi.jpg"
  ]
}
```

---

### 3. タスク一覧取得 `GET /api/tasks`

各タスクに `images` フィールドが追加されました。

#### クエリパラメータ

| パラメータ名 | 型 | 必須 | 説明 |
|---|---|---|---|
| `project_id` | number | ✅ | プロジェクトID |
| `status` | number | - | ステータスで絞り込み |
| `user_id` | number | - | 担当者IDで絞り込み |
| `created_by` | number | - | 作成者IDで絞り込み |

#### レスポンス

```json
[
  {
    "task_id": 1,
    "title": "タスク名",
    "status": 1,
    "priority": null,
    "created_by": "山田太郎",
    "user_name": "",
    "created_at": "2026-09-04T10:00:00Z",
    "images": [
      "https://res.cloudinary.com/xxx/image/upload/v000/task-management/abc.jpg"
    ]
  }
]
```

---

### 4. タスク詳細取得 `GET /api/tasks/:task_id`

タスク本体とコメントそれぞれに `images` フィールドが追加されました。

```json
{
  "task_id": 1,
  "title": "タスク名",
  "status": 1,
  "priority": null,
  "content": "内容",
  "created_by": "山田太郎",
  "created_by_id": 1,
  "user_name": "",
  "created_at": "2026-09-04T10:00:00Z",
  "images": [
    "https://res.cloudinary.com/xxx/image/upload/v000/task-management/abc.jpg"
  ],
  "comments": [
    {
      "comment_id": 5,
      "content": "コメント内容",
      "created_by": "鈴木花子",
      "created_by_id": 2,
      "created_at": "2026-09-04T11:00:00Z",
      "images": [
        "https://res.cloudinary.com/xxx/image/upload/v000/task-management/ghi.jpg"
      ]
    }
  ]
}
```

---

## 実装例（JavaScript）

### 画像ありでタスク作成

```js
const formData = new FormData();
formData.append("created_by", 1);
formData.append("project_id", 1);
formData.append("title", "タスク名");
formData.append("content", "内容");
formData.append("priority", "high");

// 画像を複数追加（省略可）
files.forEach((file) => {
  formData.append("images", file);
});

const res = await fetch("/api/tasks", {
  method: "POST",
  headers: { Authorization: `Bearer ${token}` },
  body: formData,
  // Content-Type は FormData 使用時に自動でセットされるため指定不要
});
```

### 画像なしでタスク作成（従来相当）

```js
const formData = new FormData();
formData.append("created_by", 1);
formData.append("project_id", 1);
formData.append("title", "タスク名");

const res = await fetch("/api/tasks", {
  method: "POST",
  headers: { Authorization: `Bearer ${token}` },
  body: formData,
});
```

### タスク一覧取得（画像URLを含む）

```js
const res = await fetch(`/api/tasks?project_id=${projectId}`, {
  headers: { Authorization: `Bearer ${token}` },
});
const tasks = await res.json();

// 各タスクの images に画像URLの配列が入っている（画像なしは []）
tasks.forEach((task) => {
  console.log(task.images); // ["https://res.cloudinary.com/..."]
});
```

### タスク詳細取得（タスク・コメント両方の画像URLを含む）

```js
const res = await fetch(`/api/tasks/${taskId}`, {
  headers: { Authorization: `Bearer ${token}` },
});
const task = await res.json();

console.log(task.images);             // タスクの添付画像
console.log(task.comments[0].images); // コメントの添付画像
```

---

## 注意事項

- `Content-Type: application/json` でリクエストするとエラーになります。必ず `multipart/form-data` を使用してください。
- `images` フィールドは省略可能です。画像なしで送信しても従来通り動作します。
- 返却される画像URLはそのまま `<img src="...">` に使用できます。
- タスク・コメント削除時は添付画像も自動的に削除されます（フロントエンド側での対応不要）。
