# `pkg/text` - Text Processing Module

## 1. 責務 (Responsibility)
テキストファイル（ソースコード、ログ、Markdown等）の自動文字コード判定（`chardet`）、エンコーディング変換（Shift_JIS ⇄ UTF-8等）、行単位の先頭／末尾読み込み（`head` / `tail`）、正規表現検索（`grep`）、およびクリップボード文字列操作の提供。

## 2. 依存制約と不変条件 (Invariants & Boundaries)
- **ストリーム処理による大容量テキスト耐性**:
  - `ReadHead` / `ReadTail` / `Grep` はファイル全体を一括ロードせず、`bufio.Scanner` 等による行ストリーム処理を行い、メモリ消費を最小化する。
- **文字コードの透過的変換**:
  - `chardet` により検出したエンコーディングをもとに、UTF-8 以外のテキストも化けることなく安全に行単位処理を行う。

## 3. 技術選定の理由とトレードオフ (Rationale)
- **saintfish/chardet & golang.org/x/text の採用**:
  - *Why*: 日本語レガシーシステムで頻出する CP932 / Shift_JIS / EUC-JP を確実にサポートし、外部 iconv ライブラリなしで完結させるため。

## 4. 関連仕様書 (Related Specs)
- 要件: `doc/requirements.md` (TEXT-01 〜 TEXT-06)
- 設計: `doc/design.md` (TEXT-F01 〜 TEXT-F06)
