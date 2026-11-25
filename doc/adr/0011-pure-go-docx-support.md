# 0011. 純 Go による Word (.docx) 構造化テキスト抽出・検索およびシングルバイナリ維持

* Status: Accepted
* Date: 2026-11-25
* Deciders: SDD Review Lead, Architecture Team
* Consulted: Grill Master, Review Manager, Spec Reviewer

## Context (背景)

AI エージェントが業務や開発現場で遭遇するドキュメントのうち、Word ドキュメント（`.docx`, `.doc`）は仕様書やマニュアル、報告書等で極めて頻繁に利用されている。
ユーザーおよび AI エージェントから、MCP サーバーの `read` / `search` アクション、CLI コマンド、および全文検索（FTS）において Word ドキュメントの内容を高速・決定論的に参照したいという強い要望が寄せられた。

Word ドキュメントのサポートにあたり、以下の技術的課題とトレードオフを検討する必要があった：

1. **ファイル形式の選定（レガシー `.doc` vs 現代の `.docx`）**:
   - 古い Word 97-2003 形式（`.doc`）は OLE2 Compound File Binary Format (CFBF) という複雑なバイナリ構造を持ち、純 Go での完全なパースは極めて困難。Windows COM (`Word.Application`) への依存や外部バイナリ（Antiword / LibreOffice 等）のインストールが必要となり、プラットフォーム移植性やシングルバイナリ原則を損なう。
   - 一方、Word 2007 以降の標準形式（`.docx`）は Office Open XML (OOXML) であり、実体は ZIP アーカイブ内に XML ファイル（`word/document.xml` 等）が格納された構造である。
2. **外部ライブラリ導入 vs 純 Go 内製**:
   - 既存の PowerPoint（`pkg/pptx`）では、Go 標準ライブラリ（`archive/zip`, `encoding/xml`）を活用した純 Go 実装（`ExtractTextPureGo`）を採用しており、外部モジュール依存ゼロで高速・安定動作を実現している。
3. **テキスト構造化の粒度（段落・見出し・表の扱い）**:
   - 単なるプレーンテキストのベタ書き抽出では、AI エージェントがドキュメントの文脈や章節構造を理解しづらく、検索時のヒット位置（Location）特定も困難になる。また、Word 内の表（Table）構造が崩れると表形式データの理解が大幅に阻害される。

## Decision (決定事項)

1. **対象形式を `.docx` (Office Open XML) に限定し、レガシー `.doc` はスコープ外とする**:
   - 外部 COM や追加ツールに依存せず、すべてのプラットフォームで同一に動作する純 Go 実装を担保するため、対象を `.docx` に絞る。
2. **外部ライブラリを追加せず、Go 標準ライブラリによる純 Go パーサー (`pkg/docx`) を新設する**:
   - `archive/zip` と `encoding/xml` を用い、`word/document.xml` をストリーミング/トークン解析してテキスト抽出を行う。
   - 外部依存モジュールを増やさず、ゼロ依存・シングルバイナリ配布（Single Executable Binary）の原則を厳格に維持する。
3. **見出し（Heading）階層および表（Table）の Markdown 構造化抽出**:
   - 段落スタイル（`Heading 1`, `Heading 2`, `<w:outlineLvl>` 等）を解析し、Markdown 見出し（`#`, `##`...）として整形する。
   - 表（`<w:tbl>`）を検知し、セル（`<w:tc>`）を行・列ごとに解析して Markdown テーブル（`| ... |`）形式に変換する。
4. **統合ディスパッチャ（MCP `doctools_run`）への統合**:
   - `command: "read"`: `handleReadDocx` を追加し、Smart Dynamic Response（4,000文字閾値判定、超過時は自動でローカルファイル保存とハンドル返却）を適用する。
   - `command: "search"`: `handleSearchDocx` を追加し、見出し・段落・表単位でピンポイント検索を行い、Location（例: `Heading: 概要 > Paragraph: 3`）と前後ハイライトスニペットを返却する。
5. **CLI サブコマンドおよび FTS 全文検索の統合**:
   - CLI: 既存の `excel`, `pptx`, `pdf` と統一された拡張子ベースの命名規則に従い、`doctools-cli docx extract-text <file>` を提供する。
   - FTS: `pkg/fts` のファイル解析パイプラインに `.docx` を追加し、見出し・段落・表単位の階層チャンクとして Bleve にインデックス登録する。
6. **キャプチャ（`capture`）機能はスコープ外**:
   - Word ドキュメントはリフロー型であり、固定スライドや物理ページを持つ PPTX/PDF と異なり、純 Go での確実なページ画像化が困難であるため、今回は画像キャプチャ機能の対象外としテキスト抽出・検索に集中する。

## Consequences (影響と結果)

### メリット
- **完全ゼロ依存・シングルバイナリの維持**: 追加の外部モジュールや CGO・COM・外部インストーラなしでビルド・動作可能。
- **Agent-Native な構造化理解**: 見出し階層と Markdown テーブルの保持により、LLM の認知負荷を最小化し、高精度なピンポイント検索を実現。
- **一貫したインターフェース体験**: MCP (`read`/`search`)、CLI (`docx extract-text`)、FTS (`fts query`) のすべてから同一のパース結果を活用可能。

### 影響・トレードオフ
- レガシーなバイナリ `.doc`（Word 97-2003）ファイルはサポートされない（必要に応じてユーザーが `.docx` に再保存して利用）。
- ページ番号単位の分割・画像化（`capture`）は提供されない（見出しや段落単位の論理構造で扱う）。
