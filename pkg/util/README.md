# `pkg/util` - Shared Utilities & Archiving Layer

## 1. 責務 (Responsibility)
CLI 実行時の共通エラーハンドリング（`ExitWithError` / `ExitWithAppError`）、標準 JSON レスポンス整形出力ヘルパー（`PrintJSONResponse`）、および ZIP アーカイブの圧縮・展開（`Zip` / `Unzip`）機能の提供。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **stderr / stdout の厳格な分離**:
  - 正常レスポンスは `os.Stdout` にのみ出力（`PrintJSONResponse`）。
  - エラーレスポンス（`models.ErrorResponse`）およびエラーメッセージは必ず `os.Stderr` にのみ出力（`ExitWithError`）。
- **Zip Slip 脆弱性の完全防御**:
  - `Unzip` 展開処理時、アーカイブ内のファイルパスに `../` 等が含まれている場合に指定展開先ディレクトリ外へ脱出する攻撃（Zip Slip）を厳格に検証・遮断する。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **Go 標準 `archive/zip` の採用**:
  - *Why*: 外部 zip コマンドや C ライブラリに依存せず、安全なパストラバーサル検証ロジックを組み込んだ可搬なアーカイブ操作を実現するため。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (UTIL-01, UTIL-02, NFR-02)
- 設計: `doc/design.md` (UTIL-F01, UTIL-F02, CLI-F02)
