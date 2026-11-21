# TMR Discord Bot

Bot tự động thông báo chap/truyện mới của team **The Mavericks** lên Discord, lấy từ
DocLn (light novel) và MangaDex / Mòe Truyện / Cứu Truyện (manga).

Viết bằng Go, **chỉ dùng stdlib + yaml** — không cần websocket, không cần quyền đặc biệt.

## Bot làm gì

Mỗi 15 phút (tuỳ chỉnh) bot quét:

| Nguồn | Cách lấy | Thông báo vào |
|---|---|---|
| DocLn | trang nhóm + trang truyện (chỉ fetch khi có cập nhật) | `#chap-novel-mới`, `#new-novel` |
| MangaDex | API chính thức | `#chap-manga-mới`, `#new-manga` |
| Mòe Truyện | panel "Hoạt động gần đây" trang nhóm | `#chap-manga-mới`, `#new-manga` |
| Cứu Truyện | trang cập nhật (fallback qua r.jina.ai nếu trang chặn bot) | `#chap-manga-mới`, `#new-manga` |

Đặc biệt:

- **Ping role** `@PING TRUYỆN MỚI VÀ CHAP MỚI` trong mỗi thông báo.
- **Không spam mục cũ**: lần chạy đầu bot chỉ "học" dữ liệu nền, từ lần sau mới báo mục mới (mới hơn 72h).
- **Gộp link nhiều nền tảng**: cùng một truyện + cùng số chap từ nguồn khác lên sau → bot **edit tin nhắn cũ**,
  thêm dòng "🔗 Cũng có tại: [MangaDex](...) • [Cứu Truyện](...)" (giống style team tự viết
  "cuu - mgd - moe - fb"). Bot so khớp theo tên truyện đã chuẩn hoá — MD thường có tên VI trong
  alt titles nên khớp được; nếu MD đặt tên khác hẳn (không có alt title VI) thì sẽ thành tin riêng.
  Tắt bằng `merge_links: false` (khi đó trùng trong 6h sẽ bị bỏ qua hoàn toàn).
- **Kênh status**: bot báo online + lỗi nguồn (tối đa 1 lần/giờ/nguồn) vào `#novel-status`.

## Cài đặt trên server

1. Tạo bot tại <https://discord.com/developers/applications> → tab *Bot* → copy token.
   Mời bot vào server với quyền **Send Messages + Embed Links** (không cần intent đặc biệt).
2. Tạo file `.env` cạnh `run.sh`:

   ```env
   DISCORD_TOKEN=token_cua_ban
   GUILD_ID=1447576235550052455
   ```

3. Channel ID và role ID đã cấu hình sẵn trong `config/config.yml` — sửa lại nếu đổi kênh.
4. Chạy:

   ```bash
   bash run.sh
   ```

   Script tự clone code, cài Go (nếu thiếu), build `tmr-bot` và chạy.

## Lệnh chạy thủ công

```bash
go build -o tmr-bot . && ./tmr-bot            # chạy thường
./tmr-bot --once --dry                        # poll 1 lần, in thông báo ra console (không gửi)
./tmr-bot --once                              # poll 1 lần (gửi thật)
./tmr-bot --seed                              # nạp lại dữ liệu nền (bỏ qua mục đang có)
```

## Cấu hình (`config/config.yml`)

- `poll_interval_min` — chu kỳ poll (phút).
- `max_age_hours` — chỉ thông báo mục đăng trong N giờ gần nhất (mặc định 72).
- `dedup_hours` — trùng truyện+chap giữa các nguồn trong N giờ thì bỏ qua (mặc định 6).
- `cuutruyen_base` — đổi sang mirror (`hetcuutruyen.net`, `nettrom.com`...) nếu bị chặn.
- `cuutruyen_jina: true` — luôn lấy Cứu Truyện qua r.jinai.ai (khi direct bị Cloudflare chặn).
- `enable_*` — bật/tắt từng nguồn.

## Nếu bị Cloudflare chặn (403)

Hosting dùng IP datacenter hay bị Cloudflare của các trang chặn (403). Bot xử lý theo lớp:

1. Header giả browser chuẩn (UA Chrome + Accept-Language vi) cho trang HTML, header API thẳng
   cho API (MangaDex **chống** request giả browser).
2. Vẫn 403 → tự lấy qua **r.jina.ai** (throttle 3.5s, retry khi gặp challenge).
3. **WARP proxy (khuyến nghị)** — xem mục dưới, giải quyết triệt để: IP đi ra là IP Cloudflare,
   đã verify 200 sạch với cả 4 nguồn (kể cả Mòe Truyện).
4. Nguồn nào vẫn chết: sau 3 lần seed lỗi bot chấp nhận chạy thiếu nó, và tự "hồi phục" ngầm
   khi nguồn mở lại (nạp nền trước, không spam chap cũ). Lỗi được báo vào `#novel-status`
   (tối đa 1 lần/6 giờ).

Lần chạy đầu nếu nguồn nào lỗi, bot **chưa coi là seed xong** — tự thử lại ở lần poll sau
cho tới khi đủ nguồn (hoặc bỏ cuộc sau 3 lần với nguồn đã chết hẳn).

## Chạy qua WARP (wireproxy)

Tại sao: WARP của Cloudflare cho ra IP Cloudflare, các trang dùng CF thường thông cảm với
IP "nhà mình". Không cần TUN/root nhờ `wireproxy` (userspace WireGuard → SOCKS5 local).

1. Sinh profile WARP (một lần, chạy ở đâu cũng được):

   ```bash
   wgcf register --accept-tos && wgcf generate   # -> wgcf-profile.conf
   ```

2. Tạo `wireproxy.conf` cạnh `run.sh` (lấy PrivateKey/Address từ wgcf-profile.conf,
   chỉ giữ IPv4, giữ nguyên PublicKey/Endpoint của Peer):

   ```ini
   [Interface]
   PrivateKey = <PRIVATE_KEY_CỦA_BẠN>
   Address = 172.16.0.2/32
   DNS = 1.1.1.1

   [Peer]
   PublicKey = bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=
   Endpoint = engage.cloudflareclient.com:2408
   AllowedIPs = 0.0.0.0/0

   [Socks5]
   BindAddress = 127.0.0.1:40000
   ```

3. `run.sh` thấy `wireproxy.conf` là tự tải wireproxy + chạy nền SOCKS5 tại
   `127.0.0.1:40000`.
4. Bật trong `config/config.yml`:

   ```yaml
   proxy: "socks5://127.0.0.1:40000"
   ```

Lưu ý: WARP đi UDP ra ngoài (mặc định port 2408) — vài host Pterodactyl chặn UDP;
nếu `wireproxy.log` không thấy "Interface state ... Up" thì host không cho, xoá
`proxy:` khỏi config là bot về chế độ direct + jina như cũ. `PrivateKey` là bí mật —
đừng commit, đừng gửi ai.

State (danh sách mục đã thấy) lưu ở `data/state.json` — xoá file này = bot quét lại từ đầu
(seed, không spam).

## Chạy trên Termux (Android)

Binary cho Termux là `tmr-bot-armv7` (linux/arm, CGO_ENABLED=0):

    CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -o tmr-bot-armv7 .

Android không có `/etc/resolv.conf` và store CA kiểu Linux, nên bot tự fallback:
DNS hỏi thẳng 1.1.1.1/8.8.8.8, TLS ưu tiên bundle của Termux rồi tới store
`/system`. Cài một lần cho TLS chắc ăn (store `/system` trên máy cũ thường thiếu
root hiện đại):

    pkg install ca-certificates

Bot tự tìm `$PREFIX/etc/tls/cert.pem`, không cần set biến môi trường gì thêm.

## Thông báo Components V2 (từ v1.4.0)

Thông báo chap/truyện mới là card **Components V2** (container viền màu theo nguồn):
role ping + tên truyện + chương, ảnh bìa (Media Gallery), nút link "Đọc tại ...",
chân card có thời gian. Khi merge đa nguồn, bot PATCH tin gốc và chèn dòng
"🔗 Cũng có tại: ..." vào card (giữ nguyên cờ `IS_COMPONENTS_V2`).

- Tin V2 không được chứa `content`/`embeds` — toàn bộ nội dung nằm trong `components`.
- Tin nhắn cũ kiểu embed (trước v1.4) vẫn được merge theo kiểu cũ khi PATCH lại.
- Tool kiểm tra payload với API thật (gửi rồi tự xoá): `go run ./cmd/v2test`.

## Kiểm tra cấu trúc nguồn (auto-task bảo trì)

    go run . --check

Lệnh này tải đúng trang feed mà bot cào rồi đếm số mục parser trích được — **không đọc/ghi state, không gửi Discord**. Exit 0 nếu cả 4 nguồn parse được, exit 1 nếu có nguồn lỗi. Dùng để phát hiện site đổi markup trước khi bot âm thầm không cào được nữa.

- `HTTP 200 nhưng 0 mục` → cấu trúc đổi (class/regex/JSON shape), cần sửa parser trong `sources/`.
- `403/429/DNS/TLS` → lỗi hạ tầng/bị chặn, **đừng sửa parser**; cân nhắc mirror hoặc proxy.

Có scheduled task định kỳ chạy lệnh này và tự sửa parser khi phát hiện markup đổi.

## Build matrix CI (GitHub Actions)

Mỗi push/PR lên `main` (hoặc bấm Run workflow) build 9 target, mỗi target một job riêng, artifact giữ 14 ngày kèm file `.sha256`:

| Nhóm | Target | GOOS/GOARCH |
|---|---|---|
| Termux / Android ABI | `tmr-bot-termux-armeabi-v7a` | linux/arm GOARM=7 |
| | `tmr-bot-termux-arm64-v8a` | linux/arm64 |
| | `tmr-bot-termux-x86` | linux/386 |
| | `tmr-bot-termux-x86_64` | linux/amd64 |
| Linux | `tmr-bot-linux-amd64` / `-arm64` | linux/amd64 · linux/arm64 |
| Windows | `tmr-bot-windows-amd64.exe` | windows/amd64 |
| macOS | `tmr-bot-macos-arm64` / `-amd64` | darwin/arm64 · darwin/amd64 |

Tại sao build Termux với `GOOS=linux`: Termux là userspace Linux chạy trên kernel Android; `GOOS=android` cần CGO + Android NDK và không tương thích binary tĩnh `CGO_ENABLED=0`. Xem thêm phần "Chạy trên Termux" ở trên.

Tải artifact: tab Actions → chọn run → phần Artifacts. Đổi tên theo nhu cầu sau khi tải (ví dụ `mv tmr-bot-termux-armeabi-v7a tmr-bot-armv7`).

## Cập nhật

`run.sh` tự `git fetch` + build lại mỗi lần khởi động, giữ nguyên `.env` và `config/config.yml`.
