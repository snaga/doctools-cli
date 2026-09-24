# `pkg/models` - Common Data Models Layer

## 1. 責務 (Responsibility)
CLI 全体および各パッケージで共通利用される標準 JSON レスポンスエンベロープ（`Response`, `SuccessResponse`, `ErrorResponse`）の定義。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **ゼロ外部依存 (Zero Dependencies)**:
  - 他のどの `pkg/*` パッケージやサードパーティライブラリにも依存してはならない（Go 標準型のみで構成）。
- **JSON スキーマの完全決定性**:
  - `Status`: `"success"` または `"error"`
  - `Data`: 正常時のペイロード構造体またはマップ
  - `ErrorCode`, `Message`, `Hint`: 異常時に AI エージェントが次のアクションを判断するための構造化フィールド

## 3. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (NFR-02: Structured JSON Output)
- 設計: `doc/design.md` (CLI-F02)
