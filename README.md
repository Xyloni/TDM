# GrubDrops

[English](#english) · [Русский](#русский)

## English

A self-hosted Twitch Drops and Kick Drops miner with a web interface, SQLite storage, and support for multiple accounts.

> **About this repository:** This is a personal fork based on [aalejandrofer/GrubDrops](https://github.com/aalejandrofer/GrubDrops). It keeps the original project's foundation and adds changes for this fork. Thanks to the original author and the projects that informed it.

### Features

- Mine Twitch and Kick drops with multiple accounts in parallel.
- Discover active campaigns, queue rewards, and move to the next reward automatically.
- Import Twitch Android TV accounts from `account.txt`.
- Mine account groups from `account.txt` sequentially.
- Retry incomplete Twitch Inventory responses so a temporary API error does not make an account switch to the wrong reward.
- Send notifications through Discord and Telegram; optionally use OIDC sign-in.
- Run on Windows or deploy to a VPS with Docker Compose.

### Quick start on a VPS

The fork's Docker image is `ghcr.io/xyloni/grubdrops:latest`. GitHub Actions publishes it after the repository checks pass.

For a simple HTTP deployment, create `compose.yml` on the server:

```yaml
services:
  miner:
    image: ghcr.io/xyloni/grubdrops:latest
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
mkdir -p data
docker compose up -d
```

Open `http://SERVER_IP:8080` and create the administrator password. This simple setup uses unencrypted HTTP. For permanent public access, configure HTTPS or restrict the port and connect through an SSH tunnel.

#### HTTPS deployment

The [`vps/`](vps/) directory contains a Compose setup with Caddy, automatic HTTPS, and Watchtower:

1. Copy the `vps` directory to the server and enter it.
2. Copy `.env.example` to `.env`. Set `GRUB_IMAGE=ghcr.io/xyloni/grubdrops:latest`, your `GRUB_DOMAIN`, and your time zone.
3. Point the domain's DNS record to the VPS and allow inbound ports 80 and 443.
4. Start the services:

   ```sh
   mkdir -p data
   sudo chown -R 65532:65532 data
   docker compose -f compose.yaml -f compose.https.yaml up -d
   ```

Open `https://your-domain`. Watchtower checks for image updates every six hours. To apply an already-published update immediately, run `sh update.sh` from the `vps` directory. If Kick is enabled, run `GRUB_VPS_KICK=1 sh update.sh`.

If the GHCR package is private, make it public in GitHub Packages settings or configure Docker authentication on the VPS.

### Accounts and groups

Add accounts in **Settings → Accounts** or in the first-run wizard.

Twitch Android TV records in `account.txt` use this format:

```text
email:login:password:oauth_token:user_ID:device_ID
```

Put one full record or one OAuth token on each line. To assign groups, put a header such as `1:` on its own line; the accounts below it belong to that group until the next header. Groups mine a campaign one after another. The file contains account credentials: never publish it or add it to the repository.

Twitch Android and TV-code sign-in are also available. Kick accounts can be imported from cookies in Netscape format.

### Data and security

- The database is stored in `data/miner.db`; the session encryption key is stored in `data/master.key`. Keep them together when backing up or moving the installation.
- Do not delete `master.key`: saved account sessions cannot be decrypted without it.
- Git ignores local `data/` and `data-test/` directories, database files, keys, `.env`, and `account*.txt` files.
- The container runs as UID `65532`; that user needs write access to `data`.
- The Docker socket is only needed for Kick sidecars. Twitch-only deployments do not need to mount it.

### Updates and development

Changes pushed to `main` or `master` run through GitHub Actions. After CI succeeds, the workflow publishes `ghcr.io/xyloni/grubdrops:latest`. Watchtower updates the VPS deployment automatically. For the simple Compose setup, update manually:

```sh
docker compose pull
docker compose up -d
```

To build from source:

```sh
npm --prefix web ci
npm --prefix web run build
go build ./...
go test ./...
```

On Windows, run `run-miner.bat`; Go and Node.js LTS must be installed and available in `PATH`. The launcher installs web dependencies on the first run and builds both the web interface and miner before starting.

### License and upstream project

This fork is distributed under the [MIT License](LICENSE). The original project and its history are available at [aalejandrofer/GrubDrops](https://github.com/aalejandrofer/GrubDrops).

## Русский

Самостоятельно размещаемый майнер Twitch Drops и Kick Drops с веб-интерфейсом, хранением данных в SQLite и поддержкой нескольких аккаунтов.

> **Об этом репозитории:** Это персональный форк, основанный на [aalejandrofer/GrubDrops](https://github.com/aalejandrofer/GrubDrops). Здесь сохраняется основа оригинального проекта и добавляются изменения для этого форка. Спасибо автору оригинала и проектам, на которых он основан.

### Возможности

- Параллельный фарм Twitch и Kick на нескольких аккаунтах.
- Поиск активных кампаний, очередь наград и автоматический переход к следующей награде.
- Импорт аккаунтов Twitch Android TV из `account.txt`.
- Последовательный фарм групп аккаунтов из `account.txt`.
- Повторный запрос неполного Inventory Twitch, чтобы временная ошибка API не переключала аккаунт на неправильную награду.
- Уведомления через Discord и Telegram, а также вход через OIDC.
- Запуск на Windows и развёртывание на VPS через Docker Compose.

### Быстрый запуск на VPS

Docker-образ этого форка: `ghcr.io/xyloni/grubdrops:latest`. GitHub Actions публикует его после успешных проверок репозитория.

Для простого запуска по HTTP создай на сервере файл `compose.yml`:

```yaml
services:
  miner:
    image: ghcr.io/xyloni/grubdrops:latest
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
mkdir -p data
docker compose up -d
```

Открой `http://IP_СЕРВЕРА:8080` и задай пароль администратора. Этот простой вариант использует незашифрованный HTTP. Для постоянного публичного доступа настрой HTTPS или закрой порт от внешнего доступа и подключайся через SSH-туннель.

#### Развёртывание с HTTPS

В папке [`vps/`](vps/) находится Compose-набор с Caddy, автоматическим HTTPS и Watchtower:

1. Скопируй папку `vps` на сервер и перейди в неё.
2. Скопируй `.env.example` в `.env`. Укажи `GRUB_IMAGE=ghcr.io/xyloni/grubdrops:latest`, свой домен в `GRUB_DOMAIN` и часовой пояс.
3. Направь DNS домена на VPS и открой входящие порты 80 и 443.
4. Запусти службы:

   ```sh
   mkdir -p data
   sudo chown -R 65532:65532 data
   docker compose -f compose.yaml -f compose.https.yaml up -d
   ```

Открой `https://твой-домен`. Watchtower проверяет обновления образа каждые шесть часов. Чтобы сразу применить уже опубликованное обновление, из папки `vps` выполни `sh update.sh`. Если включён Kick, выполни `GRUB_VPS_KICK=1 sh update.sh`.

Если GHCR-пакет приватный, сделай его публичным в настройках GitHub Packages или настрой авторизацию Docker на VPS.

### Аккаунты и группы

Добавляй аккаунты в **Settings → Accounts** или через мастер первоначальной настройки.

Записи Twitch Android TV в `account.txt` используют такой формат:

```text
email:login:password:oauth_token:user_ID:device_ID
```

На каждой строке указывай полную запись или один OAuth-токен. Для разделения на группы добавь заголовок, например `1:`, на отдельной строке. Аккаунты ниже относятся к этой группе до следующего заголовка. Группы фармят одну кампанию по очереди. В файле содержатся данные аккаунтов: не публикуй его и не добавляй в репозиторий.

Также доступны вход через Twitch Android и TV-код. Аккаунты Kick можно импортировать из cookies в формате Netscape.

### Данные и безопасность

- База хранится в `data/miner.db`, ключ шифрования сессий — в `data/master.key`. При резервном копировании или переносе храни их вместе.
- Не удаляй `master.key`: без него сохранённые сессии аккаунтов нельзя расшифровать.
- Git игнорирует локальные папки `data/` и `data-test/`, файлы баз данных и ключей, `.env` и файлы `account*.txt`.
- Контейнер запускается с UID `65532`; этому пользователю нужна запись в папку `data`.
- Docker socket нужен только для Kick sidecar. Для фарма только Twitch монтировать его не требуется.

### Обновление и разработка

Изменения, отправленные в `main` или `master`, проходят GitHub Actions. После успешного CI workflow публикует `ghcr.io/xyloni/grubdrops:latest`. Watchtower автоматически обновляет VPS. Для простого Compose-запуска обнови образ вручную:

```sh
docker compose pull
docker compose up -d
```

Для сборки из исходников:

```sh
npm --prefix web ci
npm --prefix web run build
go build ./...
go test ./...
```

В Windows запусти `run-miner.bat`; должны быть установлены Go и Node.js LTS и доступны в `PATH`. При первом запуске батник установит зависимости веб-интерфейса, соберёт интерфейс и майнер, затем запустит приложение.

### Лицензия и оригинальный проект

Форк распространяется по [лицензии MIT](LICENSE). Оригинальный проект и его история находятся в [aalejandrofer/GrubDrops](https://github.com/aalejandrofer/GrubDrops).
