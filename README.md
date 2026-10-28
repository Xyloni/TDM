# GrubDrops

Самостоятельно размещаемый майнер Twitch Drops и Kick Drops с веб-интерфейсом, SQLite и параллельной поддержкой нескольких аккаунтов.

> **О проекте.** Это персональный форк [оригинального GrubDrops от aalejandrofer](https://github.com/aalejandrofer/GrubDrops). Здесь сохраняется основа оригинального проекта и добавляются изменения для этого форка. Благодарность автору оригинала и участникам проектов, на которых он основан.

## Возможности

- Одновременный фарм Twitch и Kick на нескольких аккаунтах.
- Поиск активных кампаний, очередь наград и автоматический переход к следующей награде.
- Импорт Twitch Android TV аккаунтов из `account.txt`.
- Последовательный фарм групп аккаунтов из `account.txt`.
- Повторный запрос Inventory при неполных ответах Twitch, чтобы временная ошибка не переключала аккаунт на неправильную награду.
- Уведомления через Discord и Telegram, а также вход через OIDC.
- Запуск на Windows или развёртывание через Docker Compose на VPS.

## Быстрый запуск на VPS

Для персонального образа используется `ghcr.io/xyloni/grubdrops:latest`. GitHub Actions публикует его после успешных проверок репозитория.

Если нужен простой запуск по HTTP, создай `compose.yml` в отдельной папке на VPS:

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

Открой `http://IP_СЕРВЕРА:8080` и создай пароль администратора. Этот упрощённый вариант использует незашифрованный HTTP. Для постоянного публичного доступа настрой HTTPS или ограничь порт и подключайся через SSH-туннель.

### HTTPS-развёртывание

В папке [`vps/`](vps/) находится Compose-набор с Caddy, автоматическим HTTPS и Watchtower:

1. Скопируй папку `vps` на сервер и перейди в неё.
2. Скопируй `.env.example` в `.env`; укажи `GRUB_IMAGE=ghcr.io/xyloni/grubdrops:latest`, домен в `GRUB_DOMAIN` и часовой пояс.
3. Убедись, что DNS домена указывает на VPS и входящие порты 80 и 443 открыты.
4. Запусти:

   ```sh
   mkdir -p data
   sudo chown -R 65532:65532 data
   docker compose -f compose.yaml -f compose.https.yaml up -d
   ```

После этого открой `https://твой-домен`. Watchtower проверяет наличие обновлений образа каждые шесть часов. Чтобы применить уже опубликованное обновление сразу, из папки `vps` выполни `sh update.sh`. Если включён Kick, выполни `GRUB_VPS_KICK=1 sh update.sh`.

Если GHCR-пакет создан как приватный, сделай его публичным в настройках GitHub Packages либо настрой авторизацию Docker на VPS.

## Аккаунты и группы

Добавляй аккаунты в **Settings → Accounts** или через мастер первоначальной настройки.

Для Twitch Android TV поддерживаются записи из `account.txt` в формате:

```text
email:login:password:oauth_token:user_ID:device_ID
```

Также поддерживается один OAuth-токен на строку. Группу можно задать заголовком вида `1:` на отдельной строке; следующие аккаунты относятся к этой группе до следующего заголовка. Группы фармят кампанию последовательно. Файл содержит учётные данные — не публикуй его и не отправляй в репозиторий.

Для Twitch также доступны вход через Android и TV-код. Для Kick можно загрузить cookies в Netscape-формате.

## Данные и безопасность

- База хранится в `data/miner.db`, ключ шифрования сессий — в `data/master.key`. Храни и переноси их вместе.
- Не удаляй `master.key`: без него сохранённые сессии аккаунтов нельзя расшифровать.
- Папки `data/`, `data-test/`, файлы базы, ключи, `.env` и `account*.txt` исключены из Git.
- Docker-образ запускается от пользователя с UID `65532`; папка `data` должна быть ему доступна на запись.
- Монтирование Docker socket нужно только для Kick sidecar. Для Twitch его можно не подключать.

## Обновления и сборка

Изменения, отправленные в ветку `main` или `master`, проходят GitHub Actions. После успешного CI workflow публикует новый образ `ghcr.io/xyloni/grubdrops:latest`. На VPS с папкой `vps` Watchtower установит его автоматически; при простом Compose-запуске обнови вручную:

```sh
docker compose pull
docker compose up -d
```

Для сборки приложения из исходников:

```sh
npm --prefix web ci
npm --prefix web run build
go build ./...
go test ./...
```

На Windows можно запускать `run-miner.bat`; нужен установленный Go.

## Лицензия и оригинальный проект

Этот форк распространяется по лицензии [MIT](LICENSE). Исходный проект и его история доступны в [репозитории aalejandrofer/GrubDrops](https://github.com/aalejandrofer/GrubDrops).
