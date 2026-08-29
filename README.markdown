# MOSAIC — Universal Data Transformer

<p align="center"><i>A visual data engineering studio — Photoshop for data.</i></p>
<p align="center">
  <b>Go</b> core engine · <b>Tauri 2 + React + TypeScript</b> desktop UI · Node-based Pipeline Canvas
</p>

<p align="center">
  <a href="#-english">English</a> ·
  <a href="#-فارسی">فارسی</a> ·
  <a href="#-中文">中文</a>
</p>

---

## 🇬🇧 English

### What is MOSAIC?

MOSAIC is a desktop **Visual Data Engineering Studio**. Import structured, semi‑structured or binary data, profile it automatically, build a transformation pipeline on a node‑based canvas, watch each step's output live, and export to whatever format you need — CSV, JSON, XML, YAML, SQL, Markdown and more.

Every byte of actual data processing — parsing, streaming, transforming, validating, joining, aggregating, caching, exporting — runs in a **pure Go engine**. The **React + TypeScript** desktop shell (built on **Tauri 2**) is responsible for UI, interaction and visualization only; it talks to the Go engine over a local HTTP API and never touches your data directly.

### Architecture at a glance

```
frontend/                     Tauri 2 + React + TypeScript desktop shell (UI only)
  src-tauri/                  Rust shell: launches the Go engine as a sidecar process
  src/
    components/               Canvas, Inspector, Console, Scheduler, Command Palette…
    i18n/                     English / Persian (RTL) / Chinese translations
    services/                 The single api.ts that talks to the Go engine
    state/                    Zustand store (canvas graph, theme, language, undo/redo)
    styles/                   Windows 11 / Light / Dark / AMOLED / Red / Blue themes

backend/                      The Go Data Engine — all heavy lifting happens here
  cmd/mosaic/                 Entry point; starts the local HTTP bridge
  internal/
    schema/                   Universal type system + Data Profiler
    parser/                   Plugin-based format parsers (CSV, JSON, NDJSON, …)
    expression/                Transformation Expression Language (lexer/parser/evaluator)
    transform/                 20+ Pipeline Canvas nodes (filter, join, pivot, group-by…)
    pipeline/                  DAG builder, parallel executor, Job Engine
    runtime/                   Goroutine worker pool + streaming/chunked reader
    cache/                     Content-hash based node output cache
    quality/                   Data Quality Score (completeness/validity/uniqueness/…)
    storage/                   Project persistence, autosave & crash recovery
    connector/                 REST/GraphQL + Database (Postgres/MySQL/SQLite/SQL Server)
    export/                    JSON/CSV/XML/YAML/Markdown/SQL export writers
    security/                  Secrets Vault (AES-GCM) + Script Node sandbox policy
    scheduler/                 User-defined operating hours, live open/closed + next-run ETA
    ai/                        AI Data Assistant: tool-based, pluggable LLM provider
    bridge/                    The local HTTP API the frontend calls
  tests/                       Unit tests for the expression engine, parsers, pipeline DAG
```

### Requirements

| Tool | Minimum version | Used for |
|---|---|---|
| [Go](https://go.dev/dl/) | 1.22+ | Building the data engine |
| [Node.js](https://nodejs.org/) | 18+ | Building the React UI |
| [Rust](https://www.rust-lang.org/tools/install) | stable | Building the Tauri desktop shell |
| [Tauri CLI prerequisites](https://tauri.app/start/prerequisites/) | — | Platform WebView/build tools (WebView2 on Windows, webkit2gtk on Linux, Xcode CLT on macOS) |

### Install & run (development)

```bash
# 1. Build & smoke-test the Go engine
cd backend
go mod tidy
go build ./...
go test ./...

# 2. Install frontend dependencies
cd ../frontend
npm install

# 3. Install the Tauri CLI (once, globally or as a dev dependency)
npm install --save-dev @tauri-apps/cli

# 4. Run the desktop app in development mode
#    (this starts Vite, then launches the Tauri window; the Go engine is
#    spawned automatically as a sidecar — see src-tauri/src/main.rs)
npm run tauri:dev
```

### Building a production installer

```bash
# 1. Build the Go engine binary and place it where Tauri expects its sidecar
cd backend
go build -o ../frontend/src-tauri/binaries/mosaic-engine-x86_64-unknown-linux-gnu ./cmd/mosaic
# (name the binary per Tauri's target-triple sidecar convention for your OS,
#  e.g. *-pc-windows-msvc.exe on Windows, *-apple-darwin on macOS)

# 2. Build the installer (.msi/.nsis on Windows, .dmg on macOS, .deb/.AppImage on Linux)
cd ../frontend
npm run tauri:build
```

### Optional: enabling a database driver

The Database Connector Layer is driver-agnostic by design (zero mandatory third-party Go dependencies). To connect to a real database, add the driver you need and blank-import it once in `backend/cmd/mosaic/main.go`:

```bash
cd backend
go get github.com/lib/pq                 # PostgreSQL
go get github.com/go-sql-driver/mysql    # MySQL
go get modernc.org/sqlite                # SQLite (pure Go, no CGO)
go get github.com/microsoft/go-mssqldb   # SQL Server
```

### Using the Scheduler

Open **Scheduler** from the top navigation, tick the days you want the pipeline to run automatically, and set a start/end time per day plus your timezone. MOSAIC works out — live — whether the pipeline is inside its allowed window right now, exactly how long until the next window opens, and (once the pipeline has run at least once) about how long the next run is expected to take, based on its own execution history. Every value shown there is something *you* entered — nothing is hardcoded.

### Themes & languages

Six themes are built in: **Windows 11** (default), **Light**, **Dark Premium**, **AMOLED**, **Red Accent** and **Blue Accent** — switch from the title bar. Three languages ship out of the box: **English** and **中文** (left‑to‑right), and **فارسی** (right‑to‑left, with a real mirrored layout, not just flipped text).

### Project status & scope

This repository is a genuine, compiling, tested foundation for MOSAIC — not a mockup. The Go engine builds with `go build ./...`, its unit tests pass, and the frontend passes a full TypeScript check and production `vite build`. The transform-node library, parser registry and export-format registry are all plugin/registry based specifically so the remaining item on a very long feature wishlist (more parsers, more node types, a full SQL Studio UI, Binary Inspector, AI Assistant provider wiring, Custom Node SDK packaging, etc.) can be added incrementally without touching the core. Contributions welcome.

### License

MIT — see `LICENSE`.

---

## 🇮🇷 فارسی

<div dir="rtl">

### موزاییک چیست؟

موزاییک یک **استودیوی مهندسی داده تصویری** برای دسکتاپ است. داده‌های ساختاریافته، نیمه‌ساختاریافته یا باینری را وارد کنید، به‌صورت خودکار پروفایل بگیرید، روی یک بومِ مبتنی بر گره پایپ‌لاین بسازید، خروجی هر مرحله را به‌صورت زنده ببینید و در نهایت به هر فرمتی که نیاز دارید خروجی بگیرید — CSV، JSON، XML، YAML، SQL، Markdown و موارد دیگر.

تمام پردازش واقعی داده — پارس کردن، استریم، تبدیل، اعتبارسنجی، جوین، تجمیع، کش کردن، خروجی گرفتن — در یک **موتور خالص Go** اجرا می‌شود. پوستهٔ دسکتاپ با **React + TypeScript** (روی **Tauri 2**) فقط مسئول رابط کاربری، تعامل و نمایش بصری است؛ این پوسته از طریق یک API محلی HTTP با موتور Go صحبت می‌کند و هرگز مستقیماً با داده‌های شما کار نمی‌کند.

### معماری در یک نگاه

```
frontend/                     پوستهٔ دسکتاپ Tauri 2 + React + TypeScript (فقط رابط کاربری)
  src-tauri/                  پوستهٔ Rust: موتور Go را به‌عنوان یک sidecar اجرا می‌کند
  src/
    components/               بوم پایپ‌لاین، بازرس، کنسول، زمان‌بند، پالت فرمان…
    i18n/                     ترجمه‌های انگلیسی / فارسی (راست‌چین) / چینی
    services/                 فایل api.ts که با موتور Go صحبت می‌کند
    state/                    استور Zustand (گراف بوم، پوسته، زبان، واگرد/ازنو)
    styles/                   پوسته‌های ویندوز 11 / روشن / تیره / امولد / قرمز / آبی

backend/                      موتور داده Go — تمام پردازش سنگین اینجا انجام می‌شود
  cmd/mosaic/                 نقطهٔ ورود؛ سرور محلی HTTP را اجرا می‌کند
  internal/
    schema/                   سیستم نوع جامع + پروفایلر داده
    parser/                   پارسرهای فرمت به‌صورت پلاگین (CSV، JSON، NDJSON و…)
    expression/                زبان بیان تبدیل (Lexer/Parser/Evaluator)
    transform/                 بیش از ۲۰ گرهٔ بوم پایپ‌لاین (فیلتر، جوین، پیوت، گروه‌بندی…)
    pipeline/                  سازندهٔ DAG، اجراکنندهٔ موازی، موتور Job
    runtime/                   استخر کارگر Goroutine + خوانندهٔ استریم/تکه‌ای
    cache/                     کش خروجی گره بر اساس Hash محتوا
    quality/                   امتیاز کیفیت داده (کامل بودن/اعتبار/یکتایی/…)
    storage/                   ماندگاری پروژه، ذخیرهٔ خودکار و بازیابی از کرش
    connector/                 REST/GraphQL و پایگاه‌داده (Postgres/MySQL/SQLite/SQL Server)
    export/                    خروجی JSON/CSV/XML/YAML/Markdown/SQL
    security/                  Vault رمزنگاری‌شده (AES-GCM) + سیاست Sandbox گره اسکریپت
    scheduler/                 ساعات کاری تعریف‌شده توسط کاربر، وضعیت زنده باز/بسته + زمان اجرای بعدی
    ai/                        دستیار هوش مصنوعی داده: مبتنی بر Tool، با Provider قابل تعویض
    bridge/                    API محلی HTTP که رابط کاربری با آن صحبت می‌کند
  tests/                       تست‌های واحد برای موتور Expression، پارسرها، DAG پایپ‌لاین
```

### پیش‌نیازها

| ابزار | حداقل نسخه | کاربرد |
|---|---|---|
| [Go](https://go.dev/dl/) | 1.22 به بالا | ساخت موتور داده |
| [Node.js](https://nodejs.org/) | 18 به بالا | ساخت رابط کاربری React |
| [Rust](https://www.rust-lang.org/tools/install) | پایدار (stable) | ساخت پوستهٔ دسکتاپ Tauri |
| [پیش‌نیازهای Tauri CLI](https://tauri.app/start/prerequisites/) | — | ابزارهای WebView/Build سیستم‌عامل (WebView2 در ویندوز، webkit2gtk در لینوکس، Xcode CLT در مک) |

### نصب و اجرا (محیط توسعه)

```bash
# ۱. ساخت و تست موتور Go
cd backend
go mod tidy
go build ./...
go test ./...

# ۲. نصب وابستگی‌های رابط کاربری
cd ../frontend
npm install

# ۳. نصب Tauri CLI (یک‌بار، به‌صورت سراسری یا به‌عنوان وابستگی توسعه)
npm install --save-dev @tauri-apps/cli

# ۴. اجرای برنامهٔ دسکتاپ در حالت توسعه
#    (ابتدا Vite اجرا می‌شود، سپس پنجرهٔ Tauri باز می‌شود؛ موتور Go به‌صورت
#     خودکار به‌عنوان sidecar اجرا می‌شود — به src-tauri/src/main.rs نگاه کنید)
npm run tauri:dev
```

### ساخت نصب‌کنندهٔ نهایی (Production)

```bash
# ۱. ساخت فایل باینری موتور Go و قرار دادن آن در مسیر مورد انتظار Tauri
cd backend
go build -o ../frontend/src-tauri/binaries/mosaic-engine-x86_64-unknown-linux-gnu ./cmd/mosaic
# (نام باینری را طبق قرارداد target-triple سیستم‌عامل خودتان تنظیم کنید،
#  مثلاً در ویندوز *-pc-windows-msvc.exe و در مک *-apple-darwin)

# ۲. ساخت نصب‌کننده (.msi/.nsis در ویندوز، .dmg در مک، .deb/.AppImage در لینوکس)
cd ../frontend
npm run tauri:build
```

### اختیاری: فعال‌سازی درایور پایگاه‌داده

لایهٔ اتصال پایگاه‌داده به‌صورت عمدی مستقل از درایور خاصی طراحی شده (بدون هیچ وابستگی اجباری شخص‌ثالث در Go). برای اتصال به یک پایگاه‌دادهٔ واقعی، درایور موردنیاز را اضافه کرده و یک‌بار آن را در `backend/cmd/mosaic/main.go` به‌صورت blank import وارد کنید:

```bash
cd backend
go get github.com/lib/pq                 # PostgreSQL
go get github.com/go-sql-driver/mysql    # MySQL
go get modernc.org/sqlite                # SQLite (Go خالص، بدون CGO)
go get github.com/microsoft/go-mssqldb   # SQL Server
```

### استفاده از زمان‌بند (Scheduler)

از نوار بالای برنامه وارد بخش **زمان‌بند** شوید، روزهایی که می‌خواهید پایپ‌لاین به‌صورت خودکار اجرا شود را علامت بزنید و برای هر روز ساعت شروع/پایان و همچنین منطقهٔ زمانی خود را وارد کنید. موزاییک به‌صورت زنده تشخیص می‌دهد که آیا پایپ‌لاین هم‌اکنون در بازهٔ مجاز خود قرار دارد یا نه، دقیقاً چقدر تا باز شدن بازهٔ بعدی زمان باقی مانده، و (پس از حداقل یک اجرا) بر اساس تاریخچهٔ واقعی اجراها، اجرای بعدی حدوداً چقدر طول می‌کشد. تمام مقادیر نمایش‌داده‌شده چیزی است که *شما* وارد کرده‌اید — هیچ‌چیز از پیش در کد ثابت نشده است.

### پوسته‌ها و زبان‌ها

شش پوسته به‌صورت پیش‌فرض موجود است: **ویندوز ۱۱** (پیش‌فرض)، **روشن**، **تیرهٔ ویژه**، **امولد**، **قرمز** و **آبی** — از نوار عنوان قابل تغییرند. سه زبان به‌صورت پیش‌فرض پشتیبانی می‌شود: **انگلیسی** و **中文** (چپ‌به‌راست)، و **فارسی** (راست‌به‌چپ، با چیدمان واقعی آینه‌ای، نه فقط متن معکوس‌شده).

### وضعیت و محدودهٔ پروژه

این مخزن یک پایهٔ واقعی، قابل‌ساخت و تست‌شده برای موزاییک است — نه یک نمونهٔ ظاهری. موتور Go با دستور `go build ./...` ساخته می‌شود، تست‌های واحد آن با موفقیت اجرا می‌شوند، و رابط کاربری از بررسی کامل TypeScript و ساخت نهایی `vite build` بدون خطا عبور می‌کند. کتابخانهٔ گره‌های تبدیل، رجیستری پارسرها و رجیستری فرمت‌های خروجی همگی به‌صورت پلاگین/رجیستری‌محور طراحی شده‌اند تا موارد باقی‌ماندهٔ فهرست بلند ویژگی‌ها (پارسرهای بیشتر، گره‌های بیشتر، رابط کامل SQL Studio، بازرس باینری، اتصال Provider واقعی به دستیار هوش مصنوعی، بسته‌بندی کامل Custom Node SDK و…) بتوانند به‌مرور و بدون دست‌زدن به هستهٔ اصلی اضافه شوند. مشارکت شما خوش‌آمد است.

### مجوز

MIT — به فایل `LICENSE` مراجعه کنید.

</div>

---

## 🇨🇳 中文

### MOSAIC 是什么？

MOSAIC 是一款桌面端的**可视化数据工程工作室**。导入结构化、半结构化甚至二进制数据，自动进行数据画像分析，在基于节点的画布上搭建转换管道，实时查看每一步的输出结果，最后导出为你需要的任意格式——CSV、JSON、XML、YAML、SQL、Markdown 等等。

所有真正的数据处理工作——解析、流式读取、转换、校验、连接、聚合、缓存、导出——都运行在一个**纯 Go 引擎**中。基于 **Tauri 2** 构建的 **React + TypeScript** 桌面外壳只负责界面、交互与可视化；它通过本地 HTTP API 与 Go 引擎通信，绝不直接接触你的数据。

### 架构概览

```
frontend/                     Tauri 2 + React + TypeScript 桌面外壳（仅负责界面）
  src-tauri/                  Rust 外壳：以 sidecar 进程方式启动 Go 引擎
  src/
    components/               管道画布、检查器、控制台、调度器、命令面板……
    i18n/                     英文 / 波斯语（RTL）/ 中文 翻译
    services/                 唯一与 Go 引擎通信的 api.ts
    state/                    Zustand 状态管理（画布图、主题、语言、撤销/重做）
    styles/                   Windows 11 / 浅色 / 深色 / AMOLED / 红色 / 蓝色 主题

backend/                      Go 数据引擎 —— 所有繁重的工作都在这里完成
  cmd/mosaic/                 入口文件；启动本地 HTTP 桥接服务
  internal/
    schema/                   通用类型系统 + 数据画像器
    parser/                   基于插件的格式解析器（CSV、JSON、NDJSON 等）
    expression/                转换表达式语言（词法/语法分析器与求值器）
    transform/                 20 余种管道画布节点（过滤、连接、透视、分组……）
    pipeline/                  DAG 构建器、并行执行器、任务引擎
    runtime/                   Goroutine 工作池 + 流式/分块读取器
    cache/                     基于内容哈希的节点输出缓存
    quality/                   数据质量评分（完整性/有效性/唯一性/……）
    storage/                   项目持久化、自动保存与崩溃恢复
    connector/                 REST/GraphQL 及数据库连接（Postgres/MySQL/SQLite/SQL Server）
    export/                    JSON/CSV/XML/YAML/Markdown/SQL 导出器
    security/                  加密密钥库（AES-GCM）+ 脚本节点沙箱策略
    scheduler/                 用户自定义运行时段、实时开放/关闭状态 + 下次运行预估
    ai/                        AI 数据助手：基于工具调用，可插拔的 LLM 提供方
    bridge/                    前端调用的本地 HTTP API
  tests/                       表达式引擎、解析器、管道 DAG 的单元测试
```

### 环境要求

| 工具 | 最低版本 | 用途 |
|---|---|---|
| [Go](https://go.dev/dl/) | 1.22+ | 构建数据引擎 |
| [Node.js](https://nodejs.org/) | 18+ | 构建 React 界面 |
| [Rust](https://www.rust-lang.org/tools/install) | 稳定版 | 构建 Tauri 桌面外壳 |
| [Tauri CLI 前置依赖](https://tauri.app/start/prerequisites/) | — | 系统级 WebView/构建工具（Windows 上的 WebView2、Linux 上的 webkit2gtk、macOS 上的 Xcode 命令行工具） |

### 安装与运行（开发环境）

```bash
# 1. 构建并测试 Go 引擎
cd backend
go mod tidy
go build ./...
go test ./...

# 2. 安装前端依赖
cd ../frontend
npm install

# 3. 安装 Tauri CLI（全局安装一次，或作为开发依赖）
npm install --save-dev @tauri-apps/cli

# 4. 以开发模式运行桌面应用
#    （会先启动 Vite，再打开 Tauri 窗口；Go 引擎会作为 sidecar 自动启动，
#     详见 src-tauri/src/main.rs）
npm run tauri:dev
```

### 构建生产环境安装包

```bash
# 1. 构建 Go 引擎二进制文件，并放到 Tauri 期望的 sidecar 路径下
cd backend
go build -o ../frontend/src-tauri/binaries/mosaic-engine-x86_64-unknown-linux-gnu ./cmd/mosaic
# （请按你所用操作系统的 target-triple sidecar 命名约定命名该文件，
#   例如 Windows 下为 *-pc-windows-msvc.exe，macOS 下为 *-apple-darwin）

# 2. 构建安装包（Windows 下为 .msi/.nsis，macOS 下为 .dmg，Linux 下为 .deb/.AppImage）
cd ../frontend
npm run tauri:build
```

### 可选：启用数据库驱动

数据库连接层在设计上刻意与具体驱动解耦（Go 侧没有任何强制的第三方依赖）。如需连接真实数据库，请添加所需驱动，并在 `backend/cmd/mosaic/main.go` 中做一次空白导入（blank import）：

```bash
cd backend
go get github.com/lib/pq                 # PostgreSQL
go get github.com/go-sql-driver/mysql    # MySQL
go get modernc.org/sqlite                # SQLite（纯 Go 实现，无需 CGO）
go get github.com/microsoft/go-mssqldb   # SQL Server
```

### 使用调度器（Scheduler）

从顶部导航栏打开**调度器**，勾选你希望自动运行该管道的星期，并为每一天设置开始/结束时间以及所在时区。MOSAIC 会实时判断该管道当前是否处于允许运行的时间段内、距离下一个时间段开启还有多久，并且（在该管道至少运行过一次之后）根据其真实的历史执行记录，估算下一次运行大概需要多长时间。这里展示的每一个数值都是*你自己*输入的——没有任何硬编码内容。

### 主题与语言

内置六种主题：**Windows 11**（默认）、**浅色**、**深色高级版**、**AMOLED 纯黑**、**红色主题**和**蓝色主题**——可在标题栏切换。开箱即支持三种语言：**英文**与**中文**（从左到右），以及**波斯语**（从右到左，采用真正镜像的布局，而非仅仅翻转文字方向）。

### 项目状态与范围说明

本仓库是 MOSAIC 一个真实、可编译、经过测试的基础版本——而非概念演示。Go 引擎可通过 `go build ./...` 成功构建，其单元测试全部通过；前端也通过了完整的 TypeScript 类型检查以及无错误的 `vite build` 生产构建。转换节点库、解析器注册表和导出格式注册表均采用插件/注册表模式设计，目的正是让那份很长的功能愿望清单中尚未实现的部分（更多解析器、更多节点类型、完整的 SQL Studio 界面、二进制检查器、AI 助手实际提供方接入、Custom Node SDK 打包等）可以在不触及核心代码的前提下逐步添加。欢迎贡献代码。

### 许可证

MIT —— 详见 `LICENSE` 文件。
