# 0001. Go製 Agent-Native CLI アーキテクチャの採用およびシングルバイナリ配布

* Status: Accepted
* Date: 2026-08-11

## Context (背景)

LLM / AI エージェントがローカルドキュメント（PDF, PowerPoint, Excel, CSV, HTML, テキスト, 画像等）を高速・決定論的に閲覧・検索・加工する環境において、従来の構成には以下の課題があった：

1. **配布と可搬性の難しさ**: 外部ランタイムや多数のサードパーティライブラリに依存する構成では、事前の環境構築なしに社内PCやコンテナ、CI/CD環境へ即座にツールを展開することが困難であった。
2. **起動オーバーヘッド (Cold Start)**: ツール呼び出しごとのプロセス起動やライブラリ読み込み待ちがミリ秒〜秒単位の遅延を生んでいた。
3. **MCP (Model Context Protocol) のトークン消費**: MCP サーバー方式ではツール定義スキーマが常時 AI のプロンプトコンテキストを消費し、接続オーバーヘッドも発生していた。

## Decision (決定事項)

本ツールを **Go言語製 Agent-Native CLI (`doctools-cli.exe`)** として設計・構築することを決定した。

1. **Agent-Native CLI 設計の徹底**:
   - `stdout`（構造化 JSON データ）と `stderr`（ログ・ヒント）を完全分離。
   - 非対話的自動実行を保証する `--force` フラグの標準化。
   - 全データ系コマンドでの一貫した `--json` 出力。
2. **3-Layer Introspection (AI 自律能力探索)**:
   - **Layer 1**: 人間・AI 向けの `--help`
   - **Layer 2**: `doctools-cli agent-context --json` による全コマンド構文・型スキーマの機械可読な一括提供
   - **Layer 3**: 高度なコンテキスト注入用 `AGENTS.md` / `SKILL.md` 指示プロンプト
3. **静的シングルバイナリ配布**:
   - `blevesearch/bleve` (全文検索), `qax-os/excelize` (Excel), `pdfcpu` (PDF), `go-ole` (Windows Office COM 連携) を内包し、外部依存ゼロの単一バイナリとして配布可能とする。

## Consequences (影響と結果)

### メリット
- バイナリを1つ配置するだけで、ゼロセットアップで即座に動作する。
- 数ミリ秒の起動速度により、AI エージェントのツール実行レイテンシが劇的に低減。
- `agent-context` により、エージェントが必要最小限のトークンでツールの全機能を自己探索可能になった。

### デメリット / 制約
- Windows Office COM を利用する高度な描画（Fit-to-Page画像化等）は、Windows環境かつ Microsoft Office がインストールされている環境に限定される。
