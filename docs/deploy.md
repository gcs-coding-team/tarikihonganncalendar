# さくらのクラウドに置く

API・Postgres・Ollama を1台に載せる。docker compose 一式が揃っているので、
サーバーを1台立てて `.env` を埋めれば動く。

## 1. サーバーを選ぶ

必要なのはメモリ。`gemma3:4b` は動作中に **6GB 前後**を使い、Postgres と API で
さらに 1GB ほど要る。

| | 目安 |
|---|---|
| メモリ | **8GB 以上**（4GB だとモデルが載らない） |
| ディスク | 40GB 以上（モデル 3〜4GB、プリント画像、DB） |
| CPU | 4コア以上。多いほど1枚あたりが速い |

**GPU は無くても動く。** ただし CPU だけだと1枚に**数十秒から数分**かかる。
`OLLAMA_TIMEOUT_SECONDS=300` を既定にしてあるのはそのため。速さが要るなら GPU
付きのプランを見ることになるが、月額はかなり上がる。まずは CPU で1枚試して、
読めるかどうかを確かめるのが順番として正しい。

料金は変わるので、契約前にコントロールパネルで実際の月額を確認すること。

## 2. 立てる

Ubuntu を選び、Docker を入れる。

```bash
sudo apt update && sudo apt install -y docker.io docker-compose-v2 git
sudo usermod -aG docker $USER   # 入り直す
```

```bash
git clone https://github.com/gcs-coding-team/tarikihonganncalendar
cd tarikihonganncalendar
cp .env.example .env
vi .env                 # 3で埋める
docker compose up -d
```

モデルは自分で落とす。数GBあるので、起動時に黙って始めると固まったように見える。

```bash
docker compose exec ollama ollama pull gemma3:4b
```

進みを見る:

```bash
docker compose logs -f api
curl localhost:8080/healthz
```

## 3. `.env` に入れるもの

```bash
POSTGRES_PASSWORD=       # 適当に長いもの。使い回さない

SMTP_HOST=               # 契約しているメールの送信サーバー
SMTP_PORT=587            # 465 なら最初から TLS、587 なら STARTTLS
SMTP_USERNAME=           # たいていメールアドレスそのもの
SMTP_PASSWORD=
SMTP_FROM=               # 差出人。空なら SMTP_USERNAME を使う

APP_URL=https://...      # メール本文に入れるアプリのURL
FRONTEND_ORIGIN=https://...   # フロントを別オリジンに置く場合だけ
```

**`SMTP_HOST` が空だと、パスワード再設定のコードはサーバーのログに出るだけで
誰にも届かない。** 起動時のログにもその旨が出る。

送信は暗号化しないと通らない。STARTTLS を出さないサーバーには**送らずに失敗
する** — 再設定コードが平文で流れるのは、アカウントを盗聴者に渡すのと同じなので。

## 4. フロントを置く

[front-tarikihonganncalendar](https://github.com/gcs-coding-team/front-tarikihonganncalendar)
は `index.html` 一枚。同じサーバーに置いて `/v1` を API に回すのが一番簡単で、
CORS が関わらない。nginx なら:

```nginx
server {
    server_name calendar.example.jp;

    location / {
        root /var/www/tariki;
        try_files $uri /index.html;
    }

    location ~ ^/(v1|healthz|readyz) {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        # プリント画像は10MBまで通す
        client_max_body_size 12M;
    }
}
```

別オリジンに置くなら、API 側に `FRONTEND_ORIGIN`、フロント側に
`window.TARIKI_API_ORIGIN` を**対で**設定する。片方だけではブラウザが弾く。

## 5. 確かめる

```bash
# 登録できるか
curl -X POST https://calendar.example.jp/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.jp","password":"..."}'

# 開発用の抜け道が塞がっているか（401 と 404 が正しい）
curl -o /dev/null -w '%{http_code}\n' https://calendar.example.jp/v1/events \
  -H 'X-User-ID: intruder'
curl -o /dev/null -w '%{http_code}\n' -X POST \
  https://calendar.example.jp/v1/auth/sessions -d '{"userId":"intruder"}'
```

プリントは、アプリから1枚撮って試すのが早い。`docker compose logs -f api` に
解析の経過が出る。

## 気をつけること

- **`ALLOW_DEV_AUTH` は設定しない。** パスワードを迂回できる。
- **HTTPS で出す。** ログイントークンが平文で流れる。Let's Encrypt でよい。
- Postgres と Ollama はコンテナの外に出していない。外から触れる必要はない。
- バックアップは `docker volume` の `db` と `prints`。プリント画像は増え続けるので、
  ディスクの残りをたまに見ること。
