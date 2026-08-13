# nginx + Let's Encrypt در جلوی بدرود

بدرود (Bedrud) را به صورت HTTP معمولی روی `localhost` اجرا کرده و مدیریت و خاتمه TLS را با استفاده از Certbot در nginx انجام دهید. دستور `bedrud install` را برای صدور یا مدیریت گواهی‌ها پیکربندی **نکنید**.

مثال‌های این راهنما از دامنه رزروشده `meet.example.com` و IP `203.0.113.10` استفاده می‌کنند. آن‌ها را با مقادیر مربوط به سرور خود جایگزین کنید.

---

## علت استفاده از این ساختار

| بخش | مسئول |
|-----------|-------|
| HTTPS وب‌سایت | nginx + Let's Encrypt (`/etc/letsencrypt/live/meet.example.com/`) |
| API و رابط کاربری بدرود | فقط HTTP روی `127.0.0.1:8090` (`enableTLS: false`) |
| سیگنالینگ LiveKit (وب‌سوکت) | منبع یکسان (Same origin): `https://meet.example.com/livekit` ← بدرود ← `127.0.0.1:7880` |
| داده‌های رسانه‌ای WebRTC (پروتکل UDP) | مستقیم به IP سرور (`node_ip`)، بدون عبور از nginx |

به بدرود دستور ندهید گواهی ایجاد کند (`--tls` ،`--self-signed` یا `--domain` + `--email` ACME). مدیریت Let's Encrypt باید کاملاً بر عهده پروکسی باشد.

اجرای مجدد `bedrud install` بدون فلگ `--fresh` فایل‌های `/etc/bedrud/config.yaml` و `livekit.yaml` را بدون تغییر باقی می‌گذارد و فایل‌های Let's Encrypt را جایگزین نمی‌کند. اجرای `install --tls` روی نصب موجود ممکن است یک جفت گواهی خودامضا شده (self-signed) بدون استفاده را در `/etc/bedrud/cert.pem` بنویسد؛ تا زمانی که `enableTLS` برابر `false` باشد، این جفت گواهی نادیده گرفته می‌شود.

---

## ۱. تنظیمات DNS

```text
meet.example.com.  A  203.0.113.10
```

پورت ۸۰ باید به این سرور دسترسی داشته باشد تا Certbot بتواند چالش‌های HTTP-01 را تکمیل کند.

---

## ۲. نصب بدرود بدون TLS

```bash
sudo bedrud install \
  --no-tls \
  --behind-proxy \
  --domain meet.example.com \
  --ip 203.0.113.10 \
  --port 8090
```

فلگ‌های `--tls` ،`--self-signed` ،`--cert` ،`--key` یا `--email` را پاس ندهید.

فایل `/etc/bedrud/config.yaml` را ویرایش کنید:

```yaml
server:
  port: "8090"
  host: "127.0.0.1"          # به IP عمومی متصل نشوید؛ پورت 8090 را خارج از اینترنت نگه دارید
  enableTLS: false
  useACME: false
  domain: meet.example.com
  behindProxy: true
  trustedProxies:
    - "127.0.0.1"
    - "::1"
  proxyHeader: X-Forwarded-For

livekit:
  host: "https://meet.example.com/livekit"   # باید از https روی پورت 443 استفاده کند، نه :8090
  internalHost: "http://127.0.0.1:7880"
  skipTLSVerify: true
  external: false

auth:
  frontendURL: "https://meet.example.com"

cors:
  allowedOrigins: "https://meet.example.com"
  allowCredentials: true
```

فایل `/etc/bedrud/livekit.yaml` را مطابق با این ساختار بسازید (دستور `bedrud install --no-tls` نیز همین ساختار را تولید می‌کند). کلیدهای `keys` باید با `livekit.apiKey` / `livekit.apiSecret` در فایل `config.yaml` مطابقت داشته باشند. مقدار `rtc.node_ip` آدرس ICE **عمومی** است (در اینجا `203.0.113.10`). مقادیر `cert_file` / `key_file` بخش TURN را به فایل‌های Let's Encrypt اشاره ندهید؛ مدیریت TLS روی nginx باقی می‌ماند.

```yaml
# /etc/bedrud/livekit.yaml
port: 7880
bind_addresses:
  - 127.0.0.1
keys:
  CHANGE_ME_LIVEKIT_API_KEY: CHANGE_ME_LIVEKIT_API_SECRET
rtc:
  tcp_port: 7881
  port_range_start: 50000
  port_range_end: 60000
  use_external_ip: false
  node_ip: 203.0.113.10
turn:
  enabled: true
  domain: meet.example.com
  udp_port: 3478
  # بدون tls_port / cert_file / key_file: اتصالات HTTPS در nginx خاتمه می‌یابند.
webhook:
  urls: []
  api_key: ""
logging:
  json: true
  level: info
```

پس از ویرایش، سرویس‌ها را ری‌استارت کنید:

```bash
sudo systemctl restart bedrud livekit
```

---

## ۳. پیکربندی nginx + Certbot

یک بلوک سایت (site block) موقت HTTP تنظیم کنید تا Certbot بتواند گواهی را صادر کند، سپس به HTTPS سوئیچ کنید.

```nginx
# /etc/nginx/sites-available/meet.example.com
server {
    listen 80;
    listen [::]:80;
    server_name meet.example.com;

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

سایت را فعال کرده و گواهی را دریافت کنید:

```bash
sudo ln -sfn /etc/nginx/sites-available/meet.example.com /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo apt-get install -y certbot python3-certbot-nginx
sudo certbot --nginx -d meet.example.com --agree-tos --redirect
```

سپس فایل `/etc/nginx/sites-available/meet.example.com` را به‌روزرسانی کنید تا از این پیکربندی HTTPS استفاده کند (دستورات `ssl_certificate` اضافه شده توسط Certbot را نگه دارید):

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name meet.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    listen [::]:443 ssl;
    server_name meet.example.com;

    ssl_certificate     /etc/letsencrypt/live/meet.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/meet.example.com/privkey.pem;
    include /etc/letsencrypt/options-ssl-nginx.conf;
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem;

    # سیگنالینگ: ارتقا به WebSocket. بدرود پیشوند /livekit را حذف کرده و درخواست را به پورت :7880 پروکسی می‌کند.
    location /livekit {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }

    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

صحت پیکربندی nginx را تست کرده و آن را مجدداً بارگذاری کنید:

```bash
sudo nginx -t && sudo systemctl reload nginx
```

nginx مستقیماً از فایل‌های Let's Encrypt استفاده می‌کند. نیازی نیست مقادیر `certFile` یا `keyFile` در بدرود به این مسیرها اشاره کنند.

---

## ۴. پیکربندی فایروال

ترافیک سیگنالینگ از طریق پورت ۴۴۳ پروکسی می‌شود. ترافیک رسانه‌ای (Media) WebRTC از nginx عبور **نمی‌کند**.

| پورت | پروتکل | کاربرد |
|------|----------|---------|
| 80, 443 | TCP | چالش HTTP و وب‌سایت HTTPS |
| 7881 | TCP | حالت پشتیبان TCP برای WebRTC |
| 50000–60000 | UDP | ترافیک رسانه‌ای WebRTC (یا بازه تنظیم‌شده در `livekit.yaml`) |
| 3478 | UDP | سرویس TURN |

پورت‌های `8090` یا `7880` را در اینترفیس عمومی (Public) منتشر **نکنید**.

---

## ۵. عیب‌یابی: `wss://meet.example.com:8090` و `ERR_SSL_PROTOCOL_ERROR`

اگر صفحه جلسه تلاش کند به آدرس زیر متصل شود:

```text
wss://meet.example.com:8090/livekit/rtc/v1?...
```

مرورگر در حال ارسال ترافیک TLS به پورت **HTTP معمولی** بدرود است. این مشکل زمانی رخ می‌دهد که `livekit.host` روی یک URL صریح HTTP با پورت ۸۰۹۰ تنظیم شده باشد:

```yaml
livekit:
  host: "http://meet.example.com:8090/livekit"
```

از آنجا که صفحه وب روی HTTPS بارگذاری شده است، کلاینت طرحواره (scheme) وب‌سوکت را به `wss` ارتقا می‌دهد اما پورت ۸۰۹۰ را حفظ می‌کند. از آنجا که هیچ گواهی در پورت ۸۰۹۰ ارائه نمی‌شود، مرورگر خطای `ERR_SSL_PROTOCOL_ERROR` را داده و اتصال به اتاق ناموفق می‌شود.

**راه حل:** مقدار `livekit.host` را روی `https://meet.example.com/livekit` قرار دهید (بدون ذکر پورت یا با مشخص کردن `:443`). در DevTools مرورگر بررسی کنید که URL سوکت به صورت `wss://meet.example.com/livekit/...` باشد. پس از تغییر پیکربندی، مرورگر را Hard Refresh کنید تا از URLهای ورود کش‌شده استفاده نشود.

*نکته:* بازگرداندن کد HTTP **400** برای درخواست `POST /api/auth/refresh` کاربر مهمان (بدون کوکی ریفرش)، رفتاری طبیعی است و ربطی به خطاهای اتصال LiveKit ندارد.

---

## ۶. اعتبارسنجی و بررسی صحت کارکرد

سلامت API را از طریق HTTPS بررسی کنید:

```bash
curl -fsS https://meet.example.com/api/health
# {"status":"healthy",...}
```

*نکته:* درخواست `GET /livekit/rtc/v1` بدون هدر ارتقای WebSocket معمولاً کد `404` برمی‌گرداند که طبیعی است. اطمینان حاصل کنید که TLS روی پورت ۴۴۳ گواهی Let's Encrypt را ارائه می‌دهد، نه گواهی خودامضا شده بدرود.

شنوندگان (Listeners) فعال را بررسی کنید:

```bash
sudo ss -lntp | grep -E '8090|7880|443'
# 443 → nginx
# 127.0.0.1:8090 → bedrud
# 127.0.0.1:7880 → livekit
```

هش فایل‌های `/etc/letsencrypt/live/meet.example.com/*.pem` را قبل و بعد از اجرای `bedrud install` (بدون فلگ `--fresh`) مقایسه کنید تا مطمئن شوید بدون تغییر باقی مانده‌اند.

---

## مستندات مرتبط

- [راهنمای نصب CLI](../../server/cli/install.md) — مقایسه `--fresh` در برابر حفظ پیکربندی موجود
- [راهنمای پیکربندی پشت پروکسی](https://bedrud.org/en/docs/guides/behind-proxy)
