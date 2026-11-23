# 0010. PageIndex の決定論的構造ツリー探索 (Retrieval) 特化と GenAI SDK 依存の排除

* Status: Accepted
* Date: 2026-11-23
* Deciders: SDD Review Lead, Architecture Team
* Consulted: Review Manager, Spec Reviewer, Spec Verifier

## Context (背景)

長大なドキュメントの構造化ナビゲーションを提供する PageIndex 機能において、当初は各セクションの要約（Summary）を自動付与するために `google.golang.org/genai` (Gemini API) を利用した `pageindex build` コマンドの提供が構想されていた（`PAGEINDEX-01`）。

しかし、プロジェクトの根本方針（`doc/product.md`）である「決定論的（Deterministic）、ローカル完結、ゼロコンフィグ、超高速な Agent-Native ツール」に照らし合わせた結果、以下の深刻な課題が浮き彫りとなった：

1. **外部 API 依存によるゼロコンフィグ原則の崩壊**:
   - 外部 LLM API を呼び出すために API キー設定や環境変数管理が必要となり、インストール後即座にローカルドキュメントを処理できる利便性が損なわれる。
2. **非決定論性と障害耐性の低下**:
   - ネットワーク障害、API レート制限（HTTP 429）、外部サービスダウン、トークンコスト、レスポンスの非決定論性がツールの信頼性を阻害する。
3. **エージェント・ネイティブ設計との責務重複**:
   - 要約や内容理解は `doctools` を呼び出す上位 AI エージェント（クライアント側）が自身のコンテキストやモデルを用いて行うべき推論タスクであり、ローカルデータ抽出・構造化 CLI の責務を超える。

## Decision (決定事項)

1. **PageIndex を「決定論的ツリー探索（Retrieval: `tree`, `content`）」に完全特化する**:
   - `doctools pageindex tree` による階層構造ナビゲーションと、`doctools pageindex content` によるノード単位のピンポイント抽出のみを正式スコープとする。
2. **`pageindex build` コマンドおよび `google.golang.org/genai` 依存を完全排除する**:
   - CLI サブコマンド `pageindex build` は提供しない。
   - `go.mod` および Steering（`product.md`, `tech.md`, `structure.md`）、要件定義書（`requirements.md`）、基本設計書（`design.md`）、タスク台帳（`tasks.md`）から GenAI SDK 依存・要約ツリー生成仕様を完全に削除する。
3. **インデックスメタデータの生成**:
   - 将来的に PageIndex ツリーを自動生成する場合は、LLM 要約ではなく、ドキュメントのネイティブ見出し（PDF Bookmarks, PPTX タイトル等）から純粋にローカル・決定論的に抽出する方針とする。

## Consequences (影響と結果)

### メリット
- **完全ローカル・ゼロコンフィグの達成**: 外部通信や API キー設定が一切不要で、クローズド・オフライン環境でも 100% 安定動作。
- **軽量・高速性**: 外部ライブラリ肥大化（GenAI SDK / gRPC 依存等）を防ぎ、単一バイナリサイズと起動時間を最小化。
- **アーキテクチャの整合性**: 上位 AI エージェント（呼び出し側）と CLI（ローカル高速ツール）の責務分離が明確化。

### 影響・トレードオフ
- `doctools` 単体での LLM 要約自動生成は行わない（クライアント側の AI エージェントが `content` 取得後に必要に応じて自身で要約を行う）。
