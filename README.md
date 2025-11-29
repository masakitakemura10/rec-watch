# RecWatch

**RecWatch** は、macOSの画面収録などで作成された動画ファイルを、自動で監視・変換するCLIツールです。
フォルダに動画を保存するだけで、自動的に1080pのMP4（H.264）に変換し、完了を通知します。

> [!NOTE]
> 本ツールは **macOS** での利用を前提としています。

## 特徴

-   **監視モード (Watch Mode)**: 指定したフォルダを常時監視。録画を停止するだけで勝手にMP4化されます。
-   **デスクトップ通知**: 変換完了時にMacの通知センターでお知らせします（通知音付き）。
    -   **クリックで再生**: 通知をクリックすると、変換されたMP4ファイルがデフォルトのプレイヤー（QuickTime Playerなど）で即座に開きます。
-   **スマートな変換**:
    -   アスペクト比を維持しつつ1080pにリサイズ＆黒帯追加（パディング）。
    -   変換元のファイルは自動でゴミ箱へ（設定で変更可能）。
    -   **ファイル名自動整理**: 録画日時（`YYYY-MM-DD_HH-MM-SS.mp4`）に自動リネーム。
-   **高速処理**: CPUコア数に応じた並列処理で、大量のファイルもサクサク変換。

## セキュリティとプライバシー

RecWatchは、ユーザーのプライバシーとセキュリティを第一に設計されています。

-   **完全ローカル動作**: すべての処理はご自身のMac内（ローカル）で完結します。動画データやログが外部サーバーに送信されることは一切ありません。
-   **安全なファイル削除**: 変換後の元ファイルは「削除（rm）」ではなく「ゴミ箱への移動」を行います。万が一の場合でも、ゴミ箱から簡単に復元可能です。
-   **オープンソース**: ソースコードは全て公開されており、不審な挙動がないことを誰でも確認できます。

## インストール

### 1. FFmpegのインストール
このツールは内部で `ffmpeg` を使用します。
```bash
brew install ffmpeg
```

### 2. terminal-notifierのインストール (推奨)
変換完了通知をクリックしてファイルを開くために必要です。
```bash
brew install terminal-notifier
```

### 3. ツールのインストール
Go環境がある場合:
```bash
go install github.com/mt4110/rec-watch@latest
```
または、リポジトリをクローンしてビルド:
```bash
git clone https://github.com/mt4110/rec-watch.git
cd rec-watch
go build -o rec-watch main.go
sudo mv rec-watch /usr/local/bin/
```

## 使い方

### 監視モード (おすすめ)
指定したディレクトリを常時監視し、新しいファイルが追加されると自動で変換します。
```bash
rec-watch --watch ~/Desktop/ScreenRecordings
```

### 一括変換モード
カレントディレクトリ、または指定したディレクトリ以下の動画ファイルを一括変換します。
```bash
# カレントディレクトリ
rec-watch

# 指定ディレクトリ
rec-watch ~/Movies/ScreenRecordings
```

### オプション一覧
```bash
Flags:
      --watch               指定したディレクトリを監視して自動変換する
      --notify              変換完了時にデスクトップ通知を送る (default true)
      --dest string         出力先ディレクトリ (default "./out")
      --no-trash            変換元のファイルをゴミ箱に移動しない
      --concurrent int      並列実行数 (default CPUコア数-1)
      --crf int             CRF値 (品質) (default 22)
      --preset string       エンコードプリセット (default "faster")
      --keywords strings    ファイル名に含まれるキーワードでフィルタ
```

---

## 自動実行（常駐化） on macOS

PC起動時に自動で `RecWatch` を立ち上げる設定です。

1.  **plistファイルの作成**
    `~/Library/LaunchAgents/com.user.recwatch.plist` を作成します。
    (`YOUR_USERNAME` はご自身のユーザー名に書き換えてください)

    ```xml
    <?xml version="1.0" encoding="UTF-8"?>
    <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
    <plist version="1.0">
    <dict>
        <key>Label</key>
        <string>com.user.recwatch</string>
        <key>ProgramArguments</key>
        <array>
            <string>/usr/local/bin/rec-watch</string>
            <string>--watch</string>
            <string>/Users/YOUR_USERNAME/Desktop/ScreenRecordings</string>
        </array>
        <key>RunAtLoad</key>
        <true/>
        <key>KeepAlive</key>
        <true/>
        <key>StandardOutPath</key>
        <string>/Users/YOUR_USERNAME/Library/Logs/rec-watch.log</string>
        <key>StandardErrorPath</key>
        <string>/Users/YOUR_USERNAME/Library/Logs/rec-watch.log</string>
    </dict>
    </plist>
    ```

2.  **有効化**
    ```bash
    launchctl load ~/Library/LaunchAgents/com.user.recwatch.plist
    ```

## トラブルシューティング

### 通知が表示されない場合

`terminal-notifier` をインストールしても通知が表示されない場合は、以下を確認してください。

1.  **通知設定の確認**:
    - macOSの「システム設定」>「通知」を開きます。
    - アプリケーション一覧から `terminal-notifier` (または `rec-watch`) を探し、通知が許可されているか確認してください。
    - **「集中モード」（おやすみモードなど）がオンになっていないか確認してください。** オンになっていると通知が届かない場合があります。

2.  **通知のテスト**:
    ターミナルで以下のコマンドを実行して、通知が表示されるか確認できます。
    ```bash
    terminal-notifier -title "テスト" -message "これはテストです" -sound default
    ```
    これで表示されない場合は、`terminal-notifier` 自体の問題か、macOSの設定の問題です。

3.  **古い通知の削除**:
    通知センターに古い通知が溜まっていると、新しい通知が表示されない（隠れている）場合があります。通知センターを開いて確認してみてください。
