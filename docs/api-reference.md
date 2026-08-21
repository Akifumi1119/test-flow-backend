# API リファレンス

作成日: 2026-08-20

ベースURL: `http://localhost:8080`

認証が必要なAPIは `Authorization: Bearer {token}` ヘッダーを付与してください。

---

## 認証

### POST /api/login
ログイン

**リクエスト**
| フィールド | 型     | 必須 | 説明           |
|-----------|--------|------|--------------|
| email     | string | ○   | メールアドレス   |
| password  | string | ○   | パスワード      |

```json
{
  "email": "a-doi@example.com",
  "password": "password123"
}
```

**レスポンス（200 OK）**
| フィールド      | 型     | 説明            |
|--------------|--------|---------------|
| token        | string | JWTアクセストークン |
| refresh_token| string | リフレッシュトークン  |
| user_id      | number | ユーザーID        |
| name         | string | ユーザー名        |

```json
{
  "token": "eyJ...",
  "refresh_token": "abc123...",
  "user_id": 1,
  "name": "a-doi"
}
```

---

### POST /api/users
ユーザー登録

**リクエスト**
| フィールド  | 型     | 必須 | 説明           |
|-----------|--------|------|--------------|
| email     | string | ○   | メールアドレス   |
| password  | string | ○   | パスワード      |
| name      | string | ○   | ユーザー名      |

```json
{
  "email": "a-doi@example.com",
  "password": "password123",
  "name": "a-doi"
}
```

**レスポンス（200 OK）**
```json
{
  "message": "登録成功しました。"
}
```

**エラー**
| 条件                   | ステータス | メッセージ              |
|----------------------|-----------|----------------------|
| メールアドレス重複        | 409       | `登録失敗しました。`     |
| バリデーションエラー      | 400       | `登録失敗しました。`     |

---

### GET /api/users/check
ユーザー存在確認（メンバー招待時に使用）

**クエリパラメータ**
| パラメータ    | 型     | 必須 | 説明              |
|------------|--------|------|-----------------|
| email      | string | ○   | 確認するメールアドレス |
| project_id | number | ○   | プロジェクトID      |

**レスポンス（200 OK）**
| フィールド | 型     | 説明                                      |
|---------|--------|------------------------------------------|
| exists  | number | `1`: ユーザー存在・未参加 / `2`: ユーザー未存在 / `3`: 既に参加済み |

```json
{ "exists": 1 }
```

---

### POST /api/logout
ログアウト ※認証必要

**レスポンス（200 OK）**
```json
{
  "message": "ログアウトしました。"
}
```

---

## プロジェクト

### GET /api/projects/:id
プロジェクト一覧取得 ※認証必要

**パスパラメータ**
| パラメータ | 型     | 説明       |
|---------|--------|----------|
| id      | number | ユーザーID  |

**レスポンス（200 OK）**
| フィールド             | 型      | 説明               |
|--------------------|---------|------------------|
| project_id         | number  | プロジェクトID        |
| project_name       | string  | プロジェクト名        |
| task_per_complete  | number  | タスク完了率（%）      |
| task_per_incomplete| number  | タスク未完了率（%）     |

```json
[
  {
    "project_id": 1,
    "project_name": "プロジェクトA",
    "task_per_complete": 60.0,
    "task_per_incomplete": 40.0
  }
]
```

---

### POST /api/projects
プロジェクト作成 ※認証必要

**リクエスト**
| フィールド       | 型     | 必須 | 説明          |
|--------------|--------|------|-------------|
| user_id      | number | ○   | ユーザーID     |
| project_name | string | ○   | プロジェクト名   |

```json
{
  "user_id": 1,
  "project_name": "プロジェクトA"
}
```

**レスポンス（200 OK）**
```json
{
  "message": "success",
  "projects": [
    { "project_id": 1, "project_name": "プロジェクトA" }
  ]
}
```

---

### GET /api/projects/:id/members
プロジェクトメンバー一覧取得 ※認証必要

**パスパラメータ**
| パラメータ | 型     | 説明           |
|---------|--------|--------------|
| id      | number | プロジェクトID   |

**レスポンス（200 OK）**
```json
[
  { "user_id": 1, "name": "a-doi" }
]
```

---

### GET /api/projects/:id/authority
ユーザーの権限取得 ※認証必要

**パスパラメータ**
| パラメータ | 型     | 説明           |
|---------|--------|--------------|
| id      | number | プロジェクトID   |

**クエリパラメータ**
| パラメータ  | 型     | 必須 | 説明      |
|----------|--------|------|---------|
| user_id  | number | ○   | ユーザーID |

**レスポンス（200 OK）**
| フィールド    | 型     | 説明                           |
|-----------|--------|------------------------------|
| authority | number | `1`: 一般 / `3`: マネージャー        |

```json
{ "authority": 3 }
```

---

### PUT /api/projects
プロジェクト更新 ※認証必要

**リクエスト**
| フィールド         | 型        | 必須 | 説明                       |
|---------------|-----------|------|--------------------------|
| project_id    | number    | ○   | プロジェクトID                |
| manager       | number    |      | 新マネージャーのユーザーID（変更時のみ）  |
| rename_project| string    |      | 新プロジェクト名（変更時のみ）         |
| members       | string[]  |      | 追加するメンバーのメールアドレス一覧     |

```json
{
  "project_id": 1,
  "manager": 2,
  "rename_project": "プロジェクトB",
  "members": ["b-yamada@example.com"]
}
```

**レスポンス（200 OK）**
```json
{ "message": "success" }
```

---

### DELETE /api/projects/:id
プロジェクト削除 ※認証必要

プロジェクト削除時は、紐づく `tasks`・`comments`・`project_members` も同一トランザクションで削除されます。

**パスパラメータ**
| パラメータ | 型     | 説明           |
|---------|--------|--------------|
| id      | number | プロジェクトID   |

**レスポンス（200 OK）**
```json
{ "message": "success" }
```

---

## タスク

### GET /api/tasks
タスク一覧取得 ※認証必要

**クエリパラメータ**
| パラメータ     | 型     | 必須 | 説明           |
|------------|--------|------|--------------|
| project_id | number | ○   | プロジェクトID   |

**レスポンス（200 OK）**
| フィールド     | 型        | 説明              |
|-----------|----------|-----------------|
| task_id   | number   | タスクID           |
| title     | string   | タスク名           |
| status    | number   | ステータス          |
| created_by| string   | 作成者名           |
| user_name | string   | 担当者名           |
| created_at| timestamp| 作成日時           |

```json
[
  {
    "task_id": 1,
    "title": "test task",
    "status": 1,
    "created_by": "a-doi",
    "user_name": "b-yamada",
    "created_at": "2026-08-01T10:00:00+09:00"
  }
]
```

---

### GET /api/tasks/:task_id
タスク詳細取得 ※認証必要

**パスパラメータ**
| パラメータ   | 型     | 説明     |
|----------|--------|--------|
| task_id  | number | タスクID |

**レスポンス（200 OK）**
| フィールド        | 型           | 説明                  |
|-------------|-------------|---------------------|
| task_id     | number      | タスクID               |
| title       | string      | タスク名               |
| status      | number      | ステータス              |
| content     | string      | タスク説明              |
| created_by  | string      | 作成者名               |
| created_by_id| number     | 作成者のユーザーID        |
| user_name   | string      | 担当者名               |
| created_at  | timestamp   | 作成日時               |
| comments    | object[]    | コメント一覧            |

**comments の各項目**
| フィールド        | 型        | 説明                 |
|-------------|----------|---------------------|
| comment_id  | number   | コメントID            |
| content     | string   | コメント内容           |
| created_by  | string   | コメント作成者名        |
| created_by_id| number  | コメント作成者のユーザーID |
| created_at  | timestamp| コメント日時           |

```json
{
  "task_id": 1,
  "title": "test task",
  "status": 1,
  "content": "タスクの説明",
  "created_by": "a-doi",
  "created_by_id": 1,
  "user_name": "b-yamada",
  "created_at": "2026-08-01T10:00:00+09:00",
  "comments": [
    {
      "comment_id": 1,
      "content": "test text",
      "created_by": "a-doi",
      "created_by_id": 1,
      "created_at": "2026-08-20T15:30:45+09:00"
    }
  ]
}
```

---

### POST /api/tasks
タスク作成 ※認証必要

**リクエスト**
| フィールド     | 型     | 必須 | 説明              |
|-----------|--------|------|-----------------|
| created_by| number | ○   | 作成者のユーザーID    |
| project_id| number | ○   | プロジェクトID      |
| title     | string | ○   | タスク名           |
| content   | string |      | タスク説明          |
| user_name | number |      | 担当者のユーザーID    |

```json
{
  "created_by": 1,
  "project_id": 1,
  "title": "test task",
  "content": "タスクの説明",
  "user_name": 2
}
```

**レスポンス（200 OK）**
| フィールド     | 型        | 説明           |
|-----------|----------|--------------|
| task_id   | number   | タスクID        |
| title     | string   | タスク名        |
| content   | string   | タスク説明       |
| created_by| string   | 作成者名        |
| user_name | string   | 担当者名        |
| created_at| timestamp| 作成日時        |

```json
{
  "task_id": 1,
  "title": "test task",
  "content": "タスクの説明",
  "created_by": "a-doi",
  "user_name": "b-yamada",
  "created_at": "2026-08-20T10:00:00+09:00"
}
```

---

### PUT /api/tasks/:task_id
タスク更新 ※認証必要

**リクエスト**
| フィールド          | 型     | 必須 | 説明              |
|----------------|--------|------|-----------------|
| user_id        | number | ○   | ユーザーID         |
| task_id        | number | ○   | タスクID           |
| title          | string | ○   | タスク名           |
| content        | string |      | タスク説明（空文字可）   |
| status         | number | ○   | ステータス          |
| assignee_user_id| number|      | 担当者のユーザーID    |

```json
{
  "user_id": 1,
  "task_id": 1,
  "title": "test task",
  "content": "",
  "status": 2,
  "assignee_user_id": 2
}
```

**レスポンス（200 OK）**
| フィールド     | 型        | 説明           |
|-----------|----------|--------------|
| task_id   | number   | タスクID        |
| title     | string   | タスク名        |
| content   | string   | タスク説明       |
| status    | number   | ステータス       |
| created_by| string   | 作成者名        |
| user_name | string   | 担当者名        |
| created_at| timestamp| 作成日時        |
| updated_at| timestamp| 更新日時        |

```json
{
  "task_id": 1,
  "title": "test task",
  "content": "",
  "status": 2,
  "created_by": "a-doi",
  "user_name": "b-yamada",
  "created_at": "2026-08-01T10:00:00+09:00",
  "updated_at": "2026-08-20T12:00:00+09:00"
}
```

---

### DELETE /api/tasks/:task_id
タスク削除 ※認証必要

タスク削除時は、紐づく `comments` も同一トランザクションで削除されます。

**リクエスト**
| フィールド  | 型     | 必須 | 説明      |
|---------|--------|------|---------|
| user_id | number | ○   | ユーザーID |
| task_id | number | ○   | タスクID  |

```json
{
  "user_id": 1,
  "task_id": 1
}
```

**レスポンス（200 OK）**
```json
{ "task_id": 1 }
```

**エラー**
| 条件               | ステータス | メッセージ                      |
|------------------|-----------|-------------------------------|
| タスクが存在しない     | 400       | `タスクが存在しません。`           |
| ユーザーが存在しない   | 401       | `トークン切れです。`              |

---

## コメント

### POST /api/comments/:task_id
コメント作成 ※認証必要

**リクエスト**
| フィールド  | 型     | 必須 | 説明                   |
|---------|--------|------|----------------------|
| user_id | number | ○   | コメント作成者のユーザーID |
| task_id | number | ○   | タスクID               |
| comment | string | ○   | コメント内容             |

```json
{
  "user_id": 1,
  "task_id": 1,
  "comment": "test text"
}
```

**レスポンス（200 OK）**
| フィールド     | 型        | 説明                  |
|-----------|----------|---------------------|
| task_id   | number   | コメントされたタスクID      |
| title     | string   | コメントされたタスク名      |
| comment_id| number   | 作成されたコメントID       |
| comment   | string   | コメント内容            |
| created_by| string   | コメント作成者名          |
| created_at| timestamp| コメント日時            |

```json
{
  "task_id": 1,
  "title": "test task",
  "comment_id": 1,
  "comment": "test text",
  "created_by": "a-doi",
  "created_at": "2026-08-20T15:30:45+09:00"
}
```

---

### PUT /api/comments/:comment_id
コメント更新 ※認証必要

**リクエスト**
| フィールド     | 型     | 必須 | 説明                   |
|-----------|--------|------|----------------------|
| user_id   | number | ○   | ユーザーID              |
| comment_id| number | ○   | コメントID              |
| comment   | string | ○   | 更新後のコメント内容      |

```json
{
  "user_id": 1,
  "comment_id": 1,
  "comment": "edit text"
}
```

**レスポンス（200 OK）**
| フィールド     | 型        | 説明             |
|-----------|----------|----------------|
| comment_id| number   | 更新されたコメントID  |
| comment   | string   | 更新されたコメント内容 |
| updated_at| timestamp| 最終更新日時       |

```json
{
  "comment_id": 1,
  "comment": "edit text",
  "updated_at": "2026-08-20T11:30:45+09:00"
}
```

---

### DELETE /api/comments/:comment_id
コメント削除 ※認証必要

**リクエスト**
| フィールド     | 型     | 必須 | 説明      |
|-----------|--------|------|---------|
| user_id   | number | ○   | ユーザーID |
| comment_id| number | ○   | コメントID |

```json
{
  "user_id": 1,
  "comment_id": 1
}
```

**レスポンス（200 OK）**
```json
{ "comment_id": 1 }
```

**エラー**
| 条件               | ステータス | メッセージ                      |
|------------------|-----------|-------------------------------|
| コメントが存在しない   | 400       | `コメントが存在しません。`          |
| ユーザーが存在しない   | 401       | `トークン切れです。`              |

---

## 共通エラー

| ステータス | メッセージ                      | 説明             |
|-----------|-------------------------------|----------------|
| 400       | `タスクが存在しません。`           | 対象リソースが存在しない |
| 401       | `トークン切れです。`              | 認証失敗          |
| 500       | `システムエラーが発生しました。`    | サーバー内部エラー    |
