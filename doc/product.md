# 製品概要

## 製品の目的
Gemini-CLI、Claude、Antigravity などの AI エージェントが、ローカル環境の各種ドキュメント（Excel, PowerPoint, PDF, テキスト等）を高速かつ決定論的に操作・参照・検索できるようにするための **Go言語製 Agent-Native CLI ツール (`doctools-cli.exe`)**。

## コアコンセプト
- **"Agent-Native CLI"**: MCP (Model Context Protocol) の持つトークンオーバーヘッドや接続待ちを完全に排除し、CLI から即座に、かつパースしやすい JSON 形式等で結果を返す特注CLI。
- **シングルバイナリ・ゼロ依存**: Windows 環境で Python やサードパーティライブラリを一切インストールすることなく、単一の可搬バイナリ `doctools-cli.exe` を配置するだけで動作。
- **Context-Aware / Handle-Based**: 大容量データは標準出力に撒き散らさず、ローカルファイルへの出力パスを返却することで、AI エージェントのコンテキスト消費を最小化。

## ターゲットユーザー
- AI エージェント（Gemini-CLI, Antigravity, Claude, Codex 等）を活用して、ローカルの大量のドキュメント（仕様書、データシート、マニュアル、プレゼン資料等）を参照しながら開発や分析を行うエンジニア。
- AI エージェントに信頼性の高いドキュメント操作・検索能力を与えたい開発者。

## 主要機能
- **DocTools (ドキュメント操作 CLI)**:
  - **Excel**: シート一覧取得、シートCSV抽出、セル単位差分比較 (Diff)、安全ガードレール付きパッチ適用 (Patch)、座標付きMarkdown抽出、キーワード検索、シートFit-to-Page画像化 (`go-ole` 連携)。
  - **PowerPoint**: テキスト抽出、複数ファイル結合 (純Go)、スライド画像化 (`go-ole` 連携)。
  - **PDF**: ページ分割・結合・テキスト抽出 (`pdfcpu`)、埋め込み画像抽出 (`pdfcpu`)、全画面スライド画像化 (MuPDF CGO 静的リンク)。
  - **CSV / Text / Image / HTML**: 高速なテキスト・画像・エンコーディング処理。
- **Agent Introspection (3-Layer Introspection)**:
  - **Layer 1 (`--help`)**: 人間および初見エージェント向けヘルプ。
  - **Layer 2 (`doctools-cli agent-context`)**: 全コマンドの構造・型・引数スキーマを記述した機械判読可能な JSON 出力。
  - **Layer 3 (`SKILL.md`)**: エージェント向けコンパニオンスキル。
- **PageIndex (推論ベースRAG支援)**:
  - **Structure Extraction & Node Retrieval**: ドキュメント構造解析およびピンポイント取得。
  - **LLM Summary Generation**: 公式 Google GenAI SDK (`google.golang.org/genai`) を用いたサマリー構築。
- **Search (全文検索)**:
  - **Bleve Search Engine**: 各種ドキュメント (Excel, PPT, PDF, CSV, Text) を On-the-fly で Markdown/CSV テキストへ変換・チャンク分割し、ファイルパスやシート名・ページ番号などのメタデータ付きでインデックス登録。スコア、前後ハイライトスニペット、AI向けナビゲーション構造 (`target`) を返却する爆速全文検索。
- **DocSearch GUI (人間向けデスクトップ全文検索ランチャー＆WebUI)**:
  - **独立バイナリ (`docsearch-gui.exe`)**: Agent-Native CLI の純度を保つため別バイナリとして分離。`pkg/fts` の Bleve インデックスを 100% 互換で共有。
  - **自己デタッチ起動**: コマンドラインから起動された際、自身をバックグラウンド子プロセスとして即座にデタッチ起動し、プロンプトを瞬時（約20ms）に解放。
  - **タスクトレイ常駐**: タスクバーを占有せずタスクトレイ（通知領域）に常駐し、右クリックメニュー（検索画面を開く、設定、終了）を提供。
  - **Ctrl連打直接ブラウザ起動**: Win32 低レベルキーフック (`WH_KEYBOARD_LL`) による Ctrl 連打（400ms以内）検知で、直接ブラウザ検索画面をオープン。
  - **同一タブ完全制御**: SSE (Server-Sent Events) と Win32 ウィンドウ最前面化連携により、Firefox や Chrome 等での新規タブ乱立を 100% 防止し、既存タブを前面化フォーカス。
  - **日本語自動フレーズ検索**: ユーザーが入力した日本語単語（例:「テスト戦略」）を内部的に `"..."` フレーズクエリに自動変換し、文字種境界での意図しない OR 分割を防止。
  - **検索結果 WebUI**: ブラウザ上で検索結果の閲覧、検索条件の修正、複数インデックスの動的切り替え・実在ヘルス判定、ファイル/フォルダの直接起動。
  - **Query Expansion & 検索履歴サジェスト**: Gemini と連携した同義語・表記揺れタグ展開、および過去の検索履歴（`history.json`）に基づくインクリメンタルサジェスト。

## ビジネス・開発目標
AI エージェントがローカル知識ベースを高速かつ確実に探索できる環境を提供し、応答速度の劇的向上（ミリ秒単位の起動）とトークン消費削減を達成する。
また、Python 依存やインストール手順を完全に排除し、社内やチーム内での配布・共有を極限まで容易にする。