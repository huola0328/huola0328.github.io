/* ===================================================
   Supabase 配置 — 只需填 2 个值，步骤如下：

   1. 打开 https://supabase.com，用 GitHub 账号登录
   2. 点 "New project"，填项目名，等 1 分钟初始化
   3. 左侧 Settings → API，复制：
      - Project URL  （形如 https://xxxx.supabase.co）
      - anon public  （很长的一串字母）
   4. 把两个值填到下面引号里

   ── 建表（一次性）──
   左侧点 "SQL Editor" → "New query"，粘贴以下内容执行：

   create table photos   (id bigserial primary key, url text, created_at timestamptz default now());
   create table glimpses (id bigserial primary key, url text, created_at timestamptz default now());
   create table sounds   (id bigserial primary key, url text, title text, created_at timestamptz default now());
   create table letters  (id bigserial primary key, name text, body text, uuid text, created_at timestamptz default now());
   alter table photos   disable row level security;
   alter table glimpses disable row level security;
   alter table sounds   disable row level security;
   alter table letters  disable row level security;

   ── 创建存储桶（一次性）──
   左侧 Storage → New bucket：
   分别建 photos / glimpses / sounds，勾选 "Public bucket"
   =================================================== */

window.SB_URL  = "https://eacqwwaqpemaorskepbu.supabase.co";
window.SB_KEY  = "sb_publishable_Jky7p9ypmI8qKoiBHD6WlA_IUNi4SYX";

/* 管理员密码 —— 改成你自己的 */
window.ADMIN_PASSWORD = "huola2024";

/* EmailJS 转发配置（收到书信时自动发邮件给你）
   注册步骤见下方说明，填好后信件会自动转发到你的邮箱 */
window.EJ_PUBLIC_KEY  = "cyFPsAGgbsrGNLQfS";
window.EJ_SERVICE_ID  = "service_jnpdwl9";
window.EJ_TEMPLATE_ID = "template_65wszeh";
