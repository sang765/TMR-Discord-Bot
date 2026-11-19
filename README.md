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

State (danh sách mục đã thấy) lưu ở `data/state.json` — xoá file này = bot quét lại từ đầu
(seed, không spam).

## Cập nhật

`run.sh` tự `git fetch` + build lại mỗi lần khởi động, giữ nguyên `.env` và `config/config.yml`.
