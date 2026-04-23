# ebifly デプロイ手順（初回）

Terraform で環境を準備してから、VM 上で docker compose を立ち上げる流れです。

## 前提

- Cloudflare アカウントあり
- Cloudflare API トークン（Tunnel Edit / Zone Edit / DNS Edit）を 1Password に登録済み
- `op signin` して `source set_secrets.sh` できる状態

## 手順

### 1. Cloudflare ゾーン + トンネルを作成

```sh
cd terraform/cloudflare
source ./set_secrets.sh   # CLOUDFLARE_API_TOKEN, TF_VAR_cloudflare_account_id を export
terraform init
terraform apply
```

Apply 後、以下の 2 つの output を控える:

```sh
terraform output zone_nameservers    # → NS 4 件。次ステップで使う
terraform output -raw tunnel_token   # → 1Password に "ebifly tunnel token" として保存
```

### 2. GCP DNS に NS 委任を追記

`terraform/dns/locals.tf` の `cf_NS` エントリに、前ステップで取った NS 値を貼り付け：

```hcl
"cf_NS" = {
  subdomain = "cf",
  type      = "NS",
  value = [
    "alice.ns.cloudflare.com.",
    "bob.ns.cloudflare.com.",
    # ... 4 件分 ...
  ],
},
```

```sh
cd terraform/dns
terraform apply
```

委任が効くまで数分〜十数分。`dig NS cf.kameneko.dev` で Cloudflare の NS が返ってくれば OK。

### 3. IP 予約と VM 作成

```sh
cd terraform/netbox && terraform apply
cd ../proxmox03-instance && source ./set_secrets.sh && terraform apply
```

`ebifly.g3.lab-dev.net` が `192.168.20.131` で内部 DNS に登録され、VM が起動します。

### 4. VM に Docker をインストール

```sh
ssh kameneko@ebifly.g3.lab-dev.net
curl -fsSL https://raw.githubusercontent.com/<your-repo>/main/deploy/bootstrap.sh | bash
# リポジトリに push してない場合は scp で bootstrap.sh を転送して実行
```

### 5. ebifly ソースを VM に転送

ローカルから:

```sh
rsync -av --exclude .git --exclude frontend/node_modules --exclude '*.db' \
  ./ kameneko@ebifly.g3.lab-dev.net:/opt/ebifly/
```

### 6. 環境変数ファイル作成

VM 上で:

```sh
cd /opt/ebifly
cat > .env <<EOF
CLOUDFLARE_TUNNEL_TOKEN=<1Password に保存したトークン>
ALLOWED_ORIGINS=ebifly.cf.kameneko.dev
EOF
chmod 600 .env
```

### 7. 起動

```sh
docker compose -f docker-compose.tunnel.yml up -d --build
docker compose -f docker-compose.tunnel.yml logs -f
```

`https://ebifly.cf.kameneko.dev` にアクセスして動けば完了 🦐

## 更新（2 回目以降）

```sh
# ローカルで rsync
rsync -av --exclude .git --exclude frontend/node_modules --exclude '*.db' \
  ./ kameneko@ebifly.g3.lab-dev.net:/opt/ebifly/

# VM 上で rebuild
ssh kameneko@ebifly.g3.lab-dev.net '
  cd /opt/ebifly && \
  docker compose -f docker-compose.tunnel.yml up -d --build
'
```

将来的にはイメージを `registry.kameneko.dev` に push して VM は pull だけにする予定。

## トラブルシュート

- `dig NS cf.kameneko.dev` が空 → GCP の NS 委任が反映されていない。数分待つか、`terraform/dns` の apply が完了しているか確認。
- `https://ebifly.cf.kameneko.dev` で 502 → cloudflared は動いているが app に届いていない。`docker compose ... logs app` を確認。
- WebSocket が切断 → Cloudflare 側の WebSocket はデフォルト有効だが、ダッシュボードで確認してもいい。
