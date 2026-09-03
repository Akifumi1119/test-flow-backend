# Task Management Backend

Go + Gin + PostgreSQL で構築されたTaskFlow(タスク管理アプリ)のバックエンド API サーバーです。

## 技術スタック

| カテゴリ          | 技術                               |
| ----------------- | ---------------------------------- |
| 言語              | Go                                 |
| Webフレームワーク | Gin                                |
| ORM               | GORM                               |
| データベース      | PostgreSQL                         |
| 認証              | JWT (Access Token) + Refresh Token |

## ディレクトリ構成

```
backend/
├── cmd/
│   └── server/
│       └── main.go        # エントリーポイント・ルーティング定義
├── internal/
│   ├── db/
│   │   └── db.go          # DB接続・AutoMigrate
│   ├── handler/
│   │   ├── auth.go        # 認証ハンドラー
│   │   ├── user.go        # ユーザーハンドラー
│   │   ├── project.go     # プロジェクトハンドラー
│   │   ├── task.go        # タスクハンドラー
│   │   └── comment.go     # コメントハンドラー
│   ├── middleware/
│   │   └── auth.go        # JWT認証ミドルウェア
│   └── model/
│       ├── user.go
│       ├── project.go
│       ├── project_member.go
│       ├── task.go
│       ├── comment.go
│       └── refresh_token.go
├── go.mod
└── go.sum
```

## セットアップ

### 前提条件

- Go 1.22+
- PostgreSQL

### 環境変数

| 変数名        | デフォルト値      | 説明                  |
| ------------- | ----------------- | --------------------- |
| `DB_HOST`     | `localhost`       | DBホスト              |
| `DB_PORT`     | `5432`            | DBポート              |
| `DB_USER`     | `postgres`        | DBユーザー            |
| `DB_PASSWORD` | ``                | DBパスワード          |
| `DB_NAME`     | `task_management` | DB名                  |
| `JWT_SECRET`  | `secret`          | JWTの署名シークレット |
| `PORT`        | `8080`            | サーバーポート        |

> 本番環境では `JWT_SECRET` を必ず強力なランダム文字列に変更してください。

### 起動手順

```bash
# 依存関係をインストール
go mod download

# サーバー起動 (ポート8080)
go run ./cmd/server/main.go
```

起動時に GORM の `AutoMigrate` が実行され、必要なテーブルが自動作成されます。

### 認証不要

| メソッド | パス               | 説明                         |
| -------- | ------------------ | ---------------------------- |
| GET      | `/api/health`      | ヘルスチェック               |
| POST     | `/api/users`       | ユーザー登録                 |
| POST     | `/api/login`       | ログイン                     |
| GET      | `/api/users/check` | メールアドレスでユーザー確認 |

#### GET `/api/users/check` レスポンス仕様

クエリパラメータ: `email`（必須）、`project_id`（必須）

| `exists` | 意味                                                   | 返却フィールド   |
| -------- | ------------------------------------------------------ | ---------------- |
| `1`      | ユーザーが存在し、対象プロジェクトに未参加（招待可能） | `exists`, `name` |
| `2`      | そのメールアドレスのユーザーが存在しない               | `exists`         |
| `3`      | ユーザーが存在し、対象プロジェクトに参加済み           | `exists`         |

### 認証必要 (`Authorization: Bearer <token>`)

| メソッド | パス                          | 説明                                                             |
| -------- | ----------------------------- | ---------------------------------------------------------------- |
| POST     | `/api/logout`                 | ログアウト                                                       |
| GET      | `/api/users/:id`              | ユーザー取得                                                     |
| PUT      | `/api/users/:id`              | ユーザー更新                                                     |
| DELETE   | `/api/users/:id`              | ユーザー削除（自分自身のみ）                                     |
| PUT      | `/api/users/:id/password`     | パスワード変更（自分自身のみ）                                   |
| GET      | `/api/projects/:id`           | プロジェクト取得                                                 |
| POST     | `/api/projects`               | プロジェクト作成                                                 |
| PUT      | `/api/projects`               | プロジェクト更新                                                 |
| DELETE   | `/api/projects/:id`           | プロジェクト削除                                                 |
| GET      | `/api/projects/:id/members`   | プロジェクトメンバー一覧                                         |
| GET      | `/api/projects/:id/authority` | 権限確認                                                         |
| GET      | `/api/tasks`                  | タスク一覧（`status` / `user_id` / `created_by` で絞り込み可能） |
| GET      | `/api/tasks/:task_id`         | タスク取得                                                       |
| POST     | `/api/tasks`                  | タスク作成                                                       |
| PUT      | `/api/tasks/:task_id`         | タスク更新                                                       |
| DELETE   | `/api/tasks/:task_id`         | タスク削除                                                       |
| POST     | `/api/comments/:task_id`      | コメント作成                                                     |
| PUT      | `/api/comments/:comment_id`   | コメント更新                                                     |
| DELETE   | `/api/comments/:comment_id`   | コメント削除                                                     |

## CORS

`https://akifumi1119.github.io` からのリクエストを許可しています。

## 認証フロー

1. `POST /api/users` でユーザー登録
2. `POST /api/login` で Access Token (有効期限24時間) と Refresh Token (有効期限7日) を取得
3. 認証が必要なエンドポイントには `Authorization: Bearer <access_token>` ヘッダーを付与
4. `POST /api/logout` でログアウト (Refresh Token を削除)
