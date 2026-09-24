# `pkg/cli` - Agent-Native CLI Command Layer

## 1. 責務 (Responsibility)
AI エージェントおよび人間のための Cobra コマンドルーティング、引数・フラグのバリデーション、および Layer 2 Introspection（`agent-context`）の提供。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **Agent-Native CLI 3原則の死守**:
  1. **非対話的実行**: ユーザー入力を待つ `Prompt` / `Scan` / 対話ダイアログの記述は全面禁止。ファイル上書き等の破壊的操作には `--force` フラグを要求する。
  2. **完全な標準出力分離**: 正常データは `--json` 有無に関わらず `stdout` にのみ出力し、ログ・警告・プログレスはすべて `stderr` に出力する。
  3. **自己説明性 (Self-Describing)**: すべてのサブコマンドは正確なフラグ定義と `Long` 説明文を持ち、`agent-context` で完全な JSON スキーマとして動的抽出可能でなければならない。
- **ビジネスロジックの排除**:
  - コマンドハンドラ（`Run` / `RunE`）は引数のパース・正規化と各 `pkg/*` サービスの呼び出しに専念し、ドメインロジックをここに直接記述してはならない。
- **フラグリセット**:
  - グローバル変数として保持されるフラグ値は、テスト時や再実行時の状態汚染を防ぐため、実行後に適切にリセットされること。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **spf13/cobra & pflag の採用**:
  - *Why*: Go コマンドラインツールの事実上の標準であり、サブコマンドツリー構造の構築、型安全なフラグパース、`agent-context` 生成に必要なリフレクション／メタデータ探索が容易なため。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (NFR-01, NFR-02, NFR-03)
- 設計: `doc/design.md` (CLI-F01, CLI-F02, INTRO-F01)
