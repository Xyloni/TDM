<p align="center">
  <img src="assets/img/logo.png" alt="Pzhomba Pump logo" width="150" />
</p>

<h1 align="center">Pzhomba Pump</h1>

<p align="center"><strong>Your drops, accounts and farming progress in one place.</strong></p>
<p align="center">Self-hosted Twitch &amp; Kick drops miner · Web dashboard · Account groups · Telegram</p>

<p align="center">
  <a href="https://github.com/Xyloni/TDM/actions/workflows/ci.yml"><img src="https://github.com/Xyloni/TDM/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI" /></a>
  <a href="https://github.com/Xyloni/TDM/releases"><img src="https://img.shields.io/github/v/release/Xyloni/TDM?color=8957e5" alt="Latest release" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-3fb950" alt="MIT license" /></a>
</p>

<p align="center"><a href="#english">English</a> · <a href="#русский">Русский</a> · <a href="https://github.com/Xyloni/TDM/releases">Releases</a> · <a href="docs/FARMING.md">Farming guide / Контроль фарма</a></p>

---

## English

**Pzhomba Pump** is a self-hosted Twitch Drops and Kick Drops miner with a live web dashboard, encrypted account sessions and SQLite storage. Choose your games, add accounts and follow their progress from a browser on your PC or VPS.

> 🌱 **Built on GrubDrops.** This personal fork is based on [aalejandrofer/GrubDrops](https://github.com/aalejandrofer/GrubDrops). It keeps the original foundation and adds tools for running account groups, diagnosing slow accrual and managing a larger account roster. Thanks to the original author and the projects behind it.

### ✨ Features

| Area | What you can do |
| --- | --- |
| 👥 Accounts | Run multiple accounts in parallel, pause individual accounts, rename and reorder them, and set shared or per-account game lists. |
| ⚡ Import and cleanup | Import Twitch Android TV records or OAuth tokens with up to 16 concurrent validation workers. Remove all accounts in one operation. Miners stop for import, sign-in and removal, and resume after maintenance. |
| 🔄 Account groups | Import numbered groups, mine a campaign group by group, choose the startup wave size, and advance to the next campaign after a round completes. |
| 🎮 Campaigns and planning | Discover campaigns, inspect rewards and account progress, prioritize games or campaigns ending soon, reorder the session queue and use **Mine now**. |
| 🎁 Reward controls | Exclude or restore a reward for every current and future account. Mark rewards collected locally, remove manual marks and recheck linked-account requirements. Platform prerequisites still apply. |
| 📊 Farming uniformity | See confirmed gains and speed over 15/30-minute windows, current channels, waiting reasons and recent recovery switches. Search, filter and sort slow accounts first. |
| ⏳ Group forecast | See finished/total members, the estimated remaining time and accounts holding up the group. Missing or blocked data is shown explicitly. |
| 🛠️ Twitch recovery | Refresh HLS independently every 20 seconds while keeping minute beacons at 60 seconds. Keep watching through temporary inventory-read failures and switch channels after sustained slow accrual is confirmed. |
| 📺 Force watch | Choose channels to watch when no eligible reward is queued. Reward mining takes priority when a target becomes available. |
| 📚 Inventory and history | Browse campaigns, per-account reward states, claim history, and all-time/today viewing totals. |
| 🤖 Telegram | Receive claim, progress, auth and error events, pinned group handoffs, an editable problem digest and daily summaries. Use `/help`, `/status`, `/groups` and `/lagging`. |
| 🔔 Discord | Send reward and health notifications through a configurable webhook, with event filters and a test-send action. |
| 🩺 Diagnostics | View health events, run configured canaries, check updates, and download the full current-session debug log. |
| 🔐 Settings and access | Password-protected administration, optional OIDC, age-encrypted sessions, an outbound proxy, timezone and clock settings, and light/dark themes. |
| 🌍 Languages and deployment | Russian, English, Spanish, Turkish and Simplified Chinese. Windows launcher and Docker images for Linux amd64/arm64, with optional HTTPS and automatic updates. |

**How to read the numbers:** 100% speed means one platform-credited minute per observed minute. Statistics need time to warm up and restart when a worker is rebuilt. A forecast assumes streams remain available; it is an estimate, not a promised finish time. The [farming guide](docs/FARMING.md) explains observation windows and recovery rules.

### 🚀 Quick start on a VPS

The miner image is **`ghcr.io/xyloni/grubdrops`**. Use `:v1.1.0` for this release or `:latest` for the latest successful development build. GitHub Actions publishes images after checks pass.

For a simple HTTP deployment, create a working directory and `compose.yml` on the server:

```sh
mkdir -p /opt/grubdrops/data
cd /opt/grubdrops
```

```yaml
services:
  miner:
    image: ghcr.io/xyloni/grubdrops:v1.1.0
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      GRUB_SECURE_COOKIES: "0"
      TZ: Europe/Moscow
    volumes:
      - ./data:/data
```

Then start it:

```sh
sudo chown -R 65532:65532 data
docker compose up -d
```

Open **`http://SERVER_IP:8080`** and create the administrator password. This setup uses unencrypted HTTP. For permanent public access, configure HTTPS or restrict the port and connect through an SSH tunnel.

Twitch uses direct HTTP and does not require a browser sidecar. Kick's normal playback mode needs Chrome sidecars; configure them using the [VPS guide](vps/README.md). The minimal Compose example above is sufficient for Twitch.

#### 🔒 HTTPS deployment

The [`vps/`](vps/) directory contains a Compose setup with Caddy, automatic HTTPS and Watchtower:

1. Copy the `vps` directory to the server and enter it.
2. Copy `.env.example` to `.env`. Set `GRUB_IMAGE=ghcr.io/xyloni/grubdrops:v1.1.0`, your `GRUB_DOMAIN`, and your timezone. Choose `:latest` if you want updates from the development branch.
3. Point the domain's DNS record to the VPS and allow inbound ports 80 and 443.
4. Start the services:

   ```sh
   mkdir -p data
   sudo chown -R 65532:65532 data
   docker compose -f compose.yaml -f compose.https.yaml up -d
   ```

Open **`https://your-domain`**. Watchtower checks the selected image tag every six hours. A pinned version stays on that release; change the tag to adopt another version. To apply an already-published image immediately, run `sh update.sh` from the `vps` directory. With Kick enabled, use `GRUB_VPS_KICK=1 sh update.sh`.

If the GHCR package is private, make it public in GitHub Packages settings or configure Docker authentication on the VPS.

### 👥 Accounts and groups

Add accounts in **Settings → Accounts** or through the first-run wizard.

Twitch Android TV records in `account.txt` use this format:

```text
email:login:password:oauth_token:user_ID:device_ID
```

Put one full record or one OAuth token on each line. For groups, place a header such as `1:` on its own line:

```text
1:
<first group's account records>

2:
<second group's account records>
```

Accounts below a header belong to that group until the next header. Groups process the same campaign in order, then start the next round. Startup waves control how many accounts begin together. They do not limit the total number of active accounts in the group.

Twitch Android and TV-code sign-in are also available. Kick accounts can be imported from Netscape-format cookies. Account files contain credentials: keep them private and out of Git.

To leave a reward out of the plan, open its campaign and confirm **Exclude for all accounts**. The choice survives restarts, applies to future accounts and can be reversed. Exclusion does not forge a platform claim or satisfy a prerequisite.

#### 🤖 Telegram at a glance

Set the bot token and destination chat in **Settings → Notifications**, then send `/help` in that chat.

| Command | Response |
| --- | --- |
| `/start`, `/help` | Available commands. |
| `/status` | Farming health and the current group forecast. |
| `/groups` | Group completion, remaining time and delaying accounts. |
| `/lagging` | Accounts with current problems. |

Group-handoff messages are pinned when the bot has permission. Pin failures are logged and farming continues. Other pins remain in place.

The **Errors** event toggle controls the problem digest. Its first check is 15 minutes after startup, then every five minutes. An incident creates one message; later changes and recovery edit it, including after app restarts. Healthy operation does not create an initial digest.

Enable **Handle Telegram commands** on one app instance per bot. Other instances can still send notifications. Commands are accepted only in the configured chat; an existing webhook or another polling consumer must be resolved before command reception can work.

### 🗃️ Data and security

- **Keep the database and key together.** Docker stores them in `data/miner.db` and `data/master.key`. Back up both before moving or replacing an installation. A configured `GRUB_MASTER_KEY` overrides the generated file.
- **Keep credentials private.** Local databases, keys, account files, `.env`, logs and build output are excluded by `.gitignore`.
- **Keep the data directory writable.** The container runs as UID `65532`.
- **Use the Docker socket where needed.** Kick's managed sidecars require it; a Twitch-only miner does not. The optional Watchtower service also uses the socket to replace containers.
- **Account removal preserves history.** Removing saved accounts clears their sessions and working state; claim and watch-minute history remains.

### 🔧 Updates and development

A push to `main` or `master` runs CI and then publishes the development image tagged `:latest`. Release tags publish versioned miner and browser-sidecar images. The interface checks GitHub releases; checking for an update does not install it.

For the simple Compose setup, run the following **from the directory containing `compose.yml`**. For the example above, that is `/opt/grubdrops`:

```sh
cd /opt/grubdrops
docker compose pull && docker compose up -d
```

With a pinned tag, update the image version in `compose.yml` first. With `:latest`, the command pulls the newest published development image. For the HTTPS bundle, use its `update.sh` helper. Keep a known working version tag when you need a predictable rollback.

To build from source:

```sh
npm --prefix web ci
npm --prefix web run check
npm --prefix web test
npm --prefix web run build
go build ./...
go test ./...
```

🪟 On Windows, run **`run-miner.bat`** with Go and Node.js LTS installed and available in `PATH`. It installs web dependencies on the first run, builds the UI and miner, then starts the app. Its local data directory is `data-test/`.

📄 The complete current-session log is **`miner.log` next to the database**. It includes debug-level watch events while console verbosity is controlled separately by `GRUB_LOG_LEVEL`. Each app start replaces the old log. **Settings → Health** downloads the full session; save a copy before restarting if you need to keep it.

More details: [Farming and Telegram](docs/FARMING.md) · [VPS deployment](vps/README.md) · [Changelog](docs/CHANGELOG.md).

### 📜 License and upstream project

Distributed under the [MIT License](LICENSE). The original project and its history are available at [aalejandrofer/GrubDrops](https://github.com/aalejandrofer/GrubDrops).

---

## Русский

**Pzhomba Pump** — самостоятельно размещаемый майнер Twitch Drops и Kick Drops с веб-панелью, зашифрованными сессиями аккаунтов и хранением данных в SQLite. Выбирай игры, добавляй аккаунты и следи за прогрессом из браузера на ПК или VPS.

> 🌱 **На основе GrubDrops.** Этот персональный форк основан на [aalejandrofer/GrubDrops](https://github.com/aalejandrofer/GrubDrops). Он сохраняет основу оригинала и добавляет инструменты для очереди групп, диагностики медленного начисления и управления большим списком аккаунтов. Спасибо автору оригинала и проектам, на которых он основан.

### ✨ Возможности

| Раздел | Что доступно |
| --- | --- |
| 👥 Аккаунты | Параллельный запуск нескольких аккаунтов, отдельная пауза, переименование и сортировка, общие и индивидуальные списки игр. |
| ⚡ Импорт и очистка | Импорт записей Twitch Android TV и OAuth-токенов с 16 параллельными проверками. Удаление всех аккаунтов одной операцией. На время добавления, входа и удаления аккаунтов майнеры останавливаются и затем возобновляют работу. |
| 🔄 Группы аккаунтов | Импорт пронумерованных групп, последовательный фарм кампании, размер волны запуска и переход к следующей кампании после завершения круга. |
| 🎮 Кампании и планирование | Поиск кампаний, просмотр наград и прогресса аккаунтов, приоритет игр или ближайшего окончания, временная сортировка очереди и команда **«Фармить сейчас»**. |
| 🎁 Управление наградами | Исключение и возврат предмета для всех текущих и будущих аккаунтов, локальная отметка получения, удаление ручной отметки и повторная проверка привязки. Условия платформы сохраняются. |
| 📊 Равномерность фарма | Фактический прирост и скорость за 15/30 минут, текущие каналы, причины ожидания и недавние переключения из-за отставания. Поиск, фильтры и сортировка отстающих. |
| ⏳ Прогноз группы | Завершившие аккаунты, примерное оставшееся время и участники, задерживающие переход. Отсутствующие данные и блокировки показываются явно. |
| 🛠️ Восстановление Twitch | Независимый HLS-цикл раз в 20 секунд и минутный маяк раз в 60 секунд. Просмотр продолжается при временных ошибках инвентаря; подтверждённое медленное начисление запускает смену канала. |
| 📺 Просмотр выбранных каналов | Просмотр указанных каналов, когда в очереди нет доступной награды. При появлении цели фарм наград получает приоритет. |
| 📚 Инвентарь и история | Кампании, состояние наград по аккаунтам, история получений, минуты просмотра за сегодня и за всё время. |
| 🤖 Telegram | Получения, прогресс, авторизация и ошибки, закреплённые переходы групп, редактируемая сводка проблем и ежедневные итоги. Команды `/help`, `/status`, `/groups`, `/lagging`. |
| 🔔 Discord | Уведомления о наградах и состоянии через webhook, фильтры событий и проверка отправки. |
| 🩺 Диагностика | События здоровья, запуск настроенных контрольных проверок, поиск обновлений и скачивание полного DEBUG-журнала текущего сеанса. |
| 🔐 Настройки и доступ | Пароль администратора, необязательный OIDC, шифрование сессий age, исходящий прокси, часовой пояс, формат времени и светлая/тёмная тема. |
| 🌍 Языки и запуск | Русский, английский, испанский, турецкий и упрощённый китайский. Запуск Windows и Docker-образы для Linux amd64/arm64, необязательные HTTPS и автообновление. |

**Как читать показатели:** скорость 100% означает одну зачётную минуту за минуту наблюдения. Статистике нужно время на накопление; при пересоздании майнера она начинается заново. Прогноз предполагает доступность стримов и не обещает точное время окончания. Правила расчёта и восстановления описаны в [руководстве по фарму](docs/FARMING.md).

### 🚀 Быстрый запуск на VPS

Образ майнера: **`ghcr.io/xyloni/grubdrops`**. Тег `:v1.1.0` закрепляет этот релиз, `:latest` получает последнюю успешно собранную версию ветки разработки. GitHub Actions публикует образы после проверок.

Для простого запуска по HTTP создай рабочую папку и файл `compose.yml` на сервере:

```sh
mkdir -p /opt/grubdrops/data
cd /opt/grubdrops
```

```yaml
services:
  miner:
    image: ghcr.io/xyloni/grubdrops:v1.1.0
    restart: unless-stopped
    ports:
      - "8080:8080"
    environment:
      GRUB_SECURE_COOKIES: "0"
      TZ: Europe/Moscow
    volumes:
      - ./data:/data
```

Затем запусти:

```sh
sudo chown -R 65532:65532 data
docker compose up -d
```

Открой **`http://IP_СЕРВЕРА:8080`** и задай пароль администратора. Этот вариант использует незашифрованный HTTP. Для постоянного публичного доступа настрой HTTPS или закрой порт от внешнего доступа и подключайся через SSH-туннель.

Twitch работает через прямые HTTP-запросы и не требует браузерного sidecar. Обычный режим просмотра Kick требует Chrome-sidecar; его настройка описана в [руководстве VPS](vps/README.md). Минимального Compose выше достаточно для Twitch.

#### 🔒 Развёртывание с HTTPS

В папке [`vps/`](vps/) находится Compose-набор с Caddy, автоматическим HTTPS и Watchtower:

1. Скопируй папку `vps` на сервер и перейди в неё.
2. Скопируй `.env.example` в `.env`. Укажи `GRUB_IMAGE=ghcr.io/xyloni/grubdrops:v1.1.0`, домен в `GRUB_DOMAIN` и часовой пояс. Для обновлений из ветки разработки выбери `:latest`.
3. Направь DNS домена на VPS и открой входящие порты 80 и 443.
4. Запусти службы:

   ```sh
   mkdir -p data
   sudo chown -R 65532:65532 data
   docker compose -f compose.yaml -f compose.https.yaml up -d
   ```

Открой **`https://твой-домен`**. Watchtower проверяет выбранный тег каждые шесть часов. Закреплённый тег остаётся на своём релизе; для другого выпуска измени его. Чтобы сразу применить опубликованный образ, из папки `vps` выполни `sh update.sh`. Если включён Kick, используй `GRUB_VPS_KICK=1 sh update.sh`.

Если GHCR-пакет приватный, сделай его публичным в настройках GitHub Packages или настрой авторизацию Docker на VPS.

### 👥 Аккаунты и группы

Добавляй аккаунты в **«Настройки → Аккаунты»** или через мастер первоначальной настройки.

Записи Twitch Android TV в `account.txt` используют формат:

```text
email:login:password:oauth_token:user_ID:device_ID
```

На каждой строке указывай полную запись или один OAuth-токен. Для разделения на группы добавь заголовок, например `1:`, на отдельной строке:

```text
1:
<записи аккаунтов первой группы>

2:
<записи аккаунтов второй группы>
```

Аккаунты ниже заголовка относятся к этой группе до следующего заголовка. Группы обрабатывают одну кампанию по очереди, затем начинают новый круг. Волны управляют количеством одновременно стартующих аккаунтов, а не общим размером активной группы.

Также доступны вход через Twitch Android и TV-код. Аккаунты Kick можно импортировать из cookies в формате Netscape. Файлы аккаунтов содержат учётные данные: храни их приватно и не добавляй в Git.

Чтобы убрать предмет из плана, открой его кампанию и подтверди **«Исключить для всех аккаунтов»**. Выбор сохраняется после перезапуска, применяется к будущим аккаунтам и отменяется возвратом награды в фарм. Исключение не подменяет настоящее получение и не выполняет предварительные условия платформы.

#### 🤖 Telegram: короткая справка

Укажи токен бота и чат в **«Настройки → Уведомления»**, затем отправь `/help` в этом чате.

| Команда | Ответ |
| --- | --- |
| `/start`, `/help` | Доступные команды. |
| `/status` | Состояние фарма и прогноз текущей группы. |
| `/groups` | Завершение группы, остаток времени и задерживающие аккаунты. |
| `/lagging` | Аккаунты с текущими проблемами. |

Сообщения перехода групп закрепляются, если у бота достаточно прав. Ошибка закрепления записывается в журнал; фарм продолжается. Остальные закрепы сохраняются.

Переключатель событий **«Ошибки»** управляет сводкой проблем. Первая проверка выполняется через 15 минут после старта, далее каждые пять минут. Проблема создаёт одно сообщение; последующие изменения и восстановление редактируют его, в том числе после перезапуска приложения. При исправной работе первое сообщение не создаётся.

Включай **«Обрабатывать команды Telegram»** на одном экземпляре приложения для каждого бота. Остальные экземпляры могут отправлять уведомления. Команды принимаются только в настроенном чате; действующий webhook или другой потребитель polling мешает их приёму.

### 🗃️ Данные и безопасность

- **Храни базу и ключ вместе.** В Docker это `data/miner.db` и `data/master.key`. Перед переносом или заменой установки копируй оба файла. Настроенный `GRUB_MASTER_KEY` имеет приоритет над созданным файлом.
- **Не публикуй учётные данные.** Локальные базы, ключи, файлы аккаунтов, `.env`, журналы и результаты сборки исключены через `.gitignore`.
- **Сохраняй доступ к папке данных.** Контейнер работает от UID `65532`, которому нужна запись в `data`.
- **Подключай Docker socket по назначению.** Он требуется для управляемых Kick-sidecar; майнеру только Twitch он не нужен. Необязательный Watchtower тоже использует socket для замены контейнеров.
- **Удаление аккаунтов сохраняет историю.** Очищаются сессии и рабочие данные, а история наград и минут остаётся.

### 🔧 Обновление и разработка

Push в `main` или `master` запускает CI, затем публикацию образа разработки `:latest`. Релизные теги публикуют версионированные образы майнера и браузерного sidecar. Проверка обновлений в интерфейсе ищет GitHub-релизы, но не устанавливает их.

Для простого Compose-запуска выполняй команды **из папки с `compose.yml`**. В примере выше это `/opt/grubdrops`:

```sh
cd /opt/grubdrops
docker compose pull && docker compose up -d
```

При закреплённом теге сначала измени версию образа в `compose.yml`. При `:latest` команда подтянет последнюю опубликованную сборку разработки. Для HTTPS-набора используй его `update.sh`. Сохраняй тег рабочей версии, если нужен предсказуемый откат.

Для сборки из исходников:

```sh
npm --prefix web ci
npm --prefix web run check
npm --prefix web test
npm --prefix web run build
go build ./...
go test ./...
```

🪟 В Windows запусти **`run-miner.bat`**. Go и Node.js LTS должны быть установлены и доступны в `PATH`. При первом запуске батник установит зависимости, соберёт интерфейс и майнер, затем запустит приложение. Локальные данные находятся в `data-test/`.

📄 Полный журнал текущего сеанса сохраняется в **`miner.log` рядом с базой**. Он включает DEBUG-события просмотра; уровень вывода консоли задаётся отдельно через `GRUB_LOG_LEVEL`. Каждый запуск заменяет старый журнал. В **«Настройки → Здоровье»** скачивается весь сеанс. Если журнал нужен для сравнения, сохрани копию до перезапуска.

Подробнее: [Фарм и Telegram](docs/FARMING.md) · [Развёртывание VPS](vps/README.md) · [История изменений](docs/CHANGELOG.md).

### 📜 Лицензия и оригинальный проект

Форк распространяется по [лицензии MIT](LICENSE). Оригинальный проект и его история находятся в [aalejandrofer/GrubDrops](https://github.com/aalejandrofer/GrubDrops).
