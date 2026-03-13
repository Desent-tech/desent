# desent

Decentralized streaming platform where streamers self-host their own servers.

<details>
<summary>Русская версия</summary>

## Русская версия

Децентрализованная стриминговая платформа, где стримеры сами хостят свои серверы.

Каждый стример запускает один Go-бинарник, который принимает RTMP-поток, транскодирует в HLS, обеспечивает WebSocket-чат и управление пользователями через SQLite.

### Развертывание на сервере

Нужен VPS с SSH-доступом (Ubuntu/Debian). Docker установится автоматически.

```bash
git clone https://github.com/yourname/desent.git && cd desent
./deploy.sh
```

Скрипт спросит SSH-адрес сервера (например `root@1.2.3.4`), после чего:
- Установит Docker на сервере (если не установлен)
- Соберёт образы на вашей машине
- Загрузит их на сервер
- Запустит всё через Traefik (reverse proxy)

После завершения — стрим через OBS на `rtmp://ваш-ip:1935/live/live`, смотреть на `http://ваш-ip`.

### Локальный запуск

```bash
git clone https://github.com/yourname/desent.git && cd desent
export JWT_SECRET="ваш-секретный-ключ"
docker compose up --build
```

Сервер: `http://localhost:8080`, фронтенд: `http://localhost:3000`.
OBS → `rtmp://localhost:1935/live`, ключ потока `live`.

### Сборка вручную

Требуется Go 1.23+ и FFmpeg.

```bash
go build -o desent-server ./cmd/server
JWT_SECRET="ваш-секретный-ключ" ./desent-server
```

### Переменные окружения

| Переменная | По умолчанию | Описание |
|---|---|---|
| `JWT_SECRET` | *(обязательно)* | Секретный ключ для подписи JWT |
| `HTTP_ADDR` | `:8080` | Адрес HTTP-сервера |
| `RTMP_ADDR` | `0.0.0.0:1935` | Адрес RTMP-приема |
| `STREAM_KEY` | `live` | Ключ RTMP-потока |
| `DB_PATH` | `./desent.db` | Путь к файлу БД SQLite |
| `HLS_DIR` | `/tmp/hls` | Директория для HLS-сегментов |
| `FFMPEG_PATH` | `ffmpeg` | Путь к FFmpeg |
| `LOG_LEVEL` | `info` | Уровень логирования: `debug`, `info`, `warn`, `error` |
| `BCRYPT_COST` | `12` | Стоимость хеширования bcrypt |
| `CHAT_MAX_MSG_LEN` | `500` | Максимальная длина сообщения в чате |
| `CHAT_RATE_LIMIT_MS` | `500` | Лимит частоты сообщений на клиента (мс) |
| `SERVER_BANDWIDTH_MBPS` | `100` | Пропускная способность сервера для расчёта качества |
| `HLS_CACHE_MB` | `128` | Размер кеша HLS-сегментов в памяти |

### Разработка

```bash
go run ./cmd/server              # запустить сервер
go test ./...                    # запустить тесты
go test -race ./...              # тесты с детектором гонок
LOG_LEVEL=debug go run ./cmd/server  # отладочное логирование
```

### Архитектура

```
OBS (RTMP :1935) --> Go-сервер --> FFmpeg транскодер --> HLS-сегменты
                        |
                        +--> HTTP :8080 (раздача HLS, REST API)
                        +--> WebSocket (чат)
                        +--> SQLite (пользователи, сессии, баны)
```

Сервер транскодирует входящий RTMP-поток в несколько уровней качества HLS (1080p, 720p, 480p, 360p) и раздаёт их по HTTP. WebSocket-хаб чата рассылает сообщения подключённым зрителям.

### API-эндпоинты

| Метод | Путь | Авторизация | Описание |
|---|---|---|---|
| POST | `/api/auth/register` | Нет | Регистрация, возвращает JWT |
| POST | `/api/auth/login` | Нет | Вход, возвращает JWT |
| GET | `/live/{quality}/playlist.m3u8` | Нет | HLS-плейлист |
| GET | `/live/{quality}/{segment}.ts` | Нет | HLS-сегмент |
| GET | `/api/stream/status` | Нет | Информация о стриме |
| WS | `/ws/chat?token=JWT` | Да | WebSocket-чат |
| GET | `/api/chat/history/{sessionId}` | Нет | История чата |
| GET | `/api/chat/sessions` | Нет | Список сессий |
| GET | `/api/admin/settings` | Админ | Получить настройки |
| PUT | `/api/admin/settings` | Админ | Обновить настройки |
| GET | `/api/admin/users` | Админ | Список пользователей |
| POST | `/api/admin/ban` | Админ | Забанить пользователя |
| DELETE | `/api/admin/ban/{userId}` | Админ | Разбанить пользователя |
| GET | `/health` | Нет | Проверка здоровья |

### Структура проекта

```
desent/
├── cmd/server/main.go        # Точка входа
├── internal/
│   ├── config/                # Конфигурация через переменные окружения
│   ├── auth/                  # JWT-авторизация, регистрация/вход, middleware
│   ├── chat/                  # WebSocket-хаб, рассылка сообщений
│   ├── ingest/                # Управление подпроцессом FFmpeg
│   ├── hls/                   # Раздача и кеширование HLS-сегментов
│   ├── admin/                 # Админ REST API (настройки, баны)
│   └── db/                    # Настройка SQLite, миграции
├── web/desent/            # Next.js-фронтенд (Turborepo)
├── deploy.sh                  # Скрипт развёртывания на сервере
├── Dockerfile                 # Многоэтапная сборка Go + FFmpeg
└── docker-compose.yml         # Сервер + веб-фронтенд
```

### Лицензия

MIT

</details>

---

Each streamer runs a single Go binary that handles RTMP ingest, HLS transcoding, WebSocket chat, and user management with SQLite.

## Deploy to a Server

All you need is a VPS with SSH access (Ubuntu/Debian). Docker will be installed automatically.

```bash
git clone https://github.com/yourname/desent.git && cd desent
./deploy.sh
```

The script will ask for your server's SSH address (e.g. `root@1.2.3.4`), then:
- Install Docker on the server (if not already installed)
- Build images on your local machine
- Transfer them to the server
- Start everything behind Traefik (reverse proxy)

When done — stream via OBS to `rtmp://your-ip:1935/live/live`, watch at `http://your-ip`.

## Run Locally

```bash
git clone https://github.com/yourname/desent.git && cd desent
export JWT_SECRET="your-secret-key"
docker compose up --build
```

Server: `http://localhost:8080`, web frontend: `http://localhost:3000`.
OBS → `rtmp://localhost:1935/live` with stream key `live`.

### Manual Build

Requires Go 1.23+ and FFmpeg.

```bash
go build -o desent-server ./cmd/server
JWT_SECRET="your-secret-key" ./desent-server
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `JWT_SECRET` | *(required)* | Secret key for JWT token signing |
| `HTTP_ADDR` | `:8080` | HTTP server listen address |
| `RTMP_ADDR` | `0.0.0.0:1935` | RTMP ingest listen address |
| `STREAM_KEY` | `live` | RTMP stream key |
| `DB_PATH` | `./desent.db` | SQLite database file path |
| `HLS_DIR` | `/tmp/hls` | Directory for HLS segments |
| `FFMPEG_PATH` | `ffmpeg` | Path to FFmpeg binary |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `BCRYPT_COST` | `12` | Bcrypt hashing cost |
| `CHAT_MAX_MSG_LEN` | `500` | Max chat message length (chars) |
| `CHAT_RATE_LIMIT_MS` | `500` | Chat rate limit per client (ms) |
| `SERVER_BANDWIDTH_MBPS` | `100` | Server bandwidth for quality calculation |
| `HLS_CACHE_MB` | `128` | In-memory HLS segment cache size |

## Development

```bash
go run ./cmd/server              # run server
go test ./...                    # run tests
go test -race ./...              # tests with race detector
LOG_LEVEL=debug go run ./cmd/server  # debug logging
```

## Architecture

```
OBS (RTMP :1935) --> Go Server --> FFmpeg transcoder --> HLS segments
                        |
                        +--> HTTP :8080 (HLS delivery, REST API)
                        +--> WebSocket (chat)
                        +--> SQLite (users, sessions, bans)
```

The server transcodes the incoming RTMP stream into multiple HLS quality levels (1080p, 720p, 480p, 360p) and serves them over HTTP. A WebSocket chat hub broadcasts messages to connected viewers.

## API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | `/api/auth/register` | No | Register user, returns JWT |
| POST | `/api/auth/login` | No | Login, returns JWT |
| GET | `/live/{quality}/playlist.m3u8` | No | HLS playlist |
| GET | `/live/{quality}/{segment}.ts` | No | HLS segment |
| GET | `/api/stream/status` | No | Current stream info |
| WS | `/ws/chat?token=JWT` | Yes | WebSocket chat |
| GET | `/api/chat/history/{sessionId}` | No | Chat history |
| GET | `/api/chat/sessions` | No | List stream sessions |
| GET | `/api/admin/settings` | Admin | Get settings |
| PUT | `/api/admin/settings` | Admin | Update settings |
| GET | `/api/admin/users` | Admin | List users |
| POST | `/api/admin/ban` | Admin | Ban user |
| DELETE | `/api/admin/ban/{userId}` | Admin | Unban user |
| GET | `/health` | No | Health check |

## Project Structure

```
desent/
├── cmd/server/main.go        # Entry point
├── internal/
│   ├── config/                # Environment-based configuration
│   ├── auth/                  # JWT auth, register/login, middleware
│   ├── chat/                  # WebSocket hub, message broadcasting
│   ├── ingest/                # FFmpeg subprocess management
│   ├── hls/                   # HLS segment serving and caching
│   ├── admin/                 # Admin REST API (settings, bans)
│   └── db/                    # SQLite setup, migrations
├── web/desent/            # Next.js frontend (Turborepo)
├── deploy.sh                  # One-command server deployment
├── Dockerfile                 # Multi-stage Go build + FFmpeg
└── docker-compose.yml         # Server + web frontend
```

## License

MIT
