# `pkg/fts` - Full-Text Search Module

## 1. 責務 (Responsibility)
ローカル文書（Excel, PPTX, PDF, CSV, Text, HTML）を On-the-fly で構造化テキストに分解し、Bleve による形態素・N-gram インデックスの高速構築・差分更新・構造化メタデータ付き全文検索を提供する。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **CLI/UI層非依存**:
  - `pkg/cli` などの上位層をインポートしてはならない。
- **標準出力汚染の禁止**:
  - `fmt.Println` などの `stdout` 直接出力は厳禁。進捗や警告は `stderr` または返却オブジェクトのメトリクス（`BuildResult`）で行うこと。
- **決定論的なディレクトリ枝刈り (Pruning)**:
  - `.` で始まる隠しディレクトリ（`.git`, `.trash`, `.obsidian` 等）はデフォルトで `filepath.SkipDir` スキップする。
  - `ExcludeDirs` に指定されたディレクトリは即座に `filepath.SkipDir` スキップする。
  - `IncludeDirs` が指定されている場合、対象外の無関係なサブディレクトリは探索せずに即座に枝刈りする（明示指定された隠しディレクトリは許可）。
- **耐障害性と Goroutine 安全性**:
  - 単一ファイルのパース失敗やタイムアウト時も走査全体を中断してはならない（警告ログを残しスキップ）。
  - パース処理のタイムアウト監視用チャネルはバッファサイズ 1 以上とし、Goroutine リークを防止する。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **Bleve (Pure Go) の採用**:
  - *Why*: Lucene や Tantivy のような外部プロセス／CGO依存を排除し、単一バイナリ配布と高速起動を実現するため。
  - *Trade-off*: メモリ使用量が C++ エンジンより大きくなりやすいが、バッチ書き込み (`Batch`) とファイル単位タイムアウト制御で緩和。
- **On-the-fly チャンク化**:
  - *Why*: 事前中間テキストファイルをディスクに書き出さずメモリ上で即時インデックス化することで、ディスクI/Oとクリーンアップ負荷を最小化。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (FTS-01 〜 FTS-06)
- 設計: `doc/design.md` (FTS-F01 〜 FTS-F06, IPO設計)
