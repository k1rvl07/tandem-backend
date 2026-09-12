# Tandem Backend

Go-приложение коллаборативного таск-трекера Tandem. Модуль
`github.com/tandem/tandem`, язык Go 1.27.

## Стек

- Gin — HTTP-фреймворк;
- GORM + драйвер `gorm.io/driver/postgres` (pgx v5) — ORM и миграции (AutoMigrate);
- Redis (`github.com/redis/go-redis/v9`) — кеш, Pub/Sub, присутствие, rate limit;
- MinIO (`minio-go/v7`) — файловое хранилище (бакет `tandem-files`);
- JWT (`golang-jwt/v5`), bcrypt — аутентификация;
- gorilla/websocket — реальное время;
- zap — структурированное логирование;
- swag — OpenAPI 3.1 документация.

## Требования

- Go 1.27;
- Docker Compose (Postgres/Redis/MinIO) — `make infra-up` из корня репозитория.

## Быстрый старт

```sh
make swag                 # сгенерировать docs/ (обязательно на свежем клоне)
make backend              # запуск через Air, hot-reload, порт 8080
```

`docs/` — сгенерированный пакет OpenAPI, не отслеживается git, но требуется
для сборки (blank-import в `internal/app/app.go`). `make build` сам генерирует
docs при их отсутствии (`make ensure-docs`); принудительная перегенерация после
изменения контрактов — `make swag` (цель корневого Makefile).

Конфигурация читается из файла, переданного при старте (`.env.dev` для dev,
`.env.prod` для prod), затем из переменных окружения.

## Переменные окружения

| Переменная | По умолчанию | Описание |
|------------|-------------|----------|
| `APP_ENV` | `dev` | Режим: `dev` / `production` |
| `APP_PORT` | `8080` | Порт HTTP-сервера |
| `POSTGRES_HOST` | `localhost` | Хост PostgreSQL |
| `POSTGRES_PORT` | `5432` | Порт PostgreSQL |
| `POSTGRES_USER` | `tandem` | Пользователь БД |
| `POSTGRES_PASSWORD` | `change_me` | Пароль БД |
| `POSTGRES_DB` | `tandem` | Имя БД |
| `REDIS_ADDR` | `localhost:6379` | Адрес Redis |
| `REDIS_PASSWORD` | — | Пароль Redis |
| `MINIO_ENDPOINT` | `localhost:9000` | Адрес MinIO |
| `MINIO_ROOT_USER` | `tandem` | Access key |
| `MINIO_ROOT_PASSWORD` | `change_me_minio` | Secret key |
| `MINIO_USE_SSL` | `false` | TLS для MinIO |
| `MINIO_PUBLIC_ENDPOINT` | — | Публичный endpoint для presigned URL (в прод — через nginx) |
| `MINIO_PUBLIC_USE_SSL` | `false` | TLS для публичного endpoint |
| `MINIO_BUCKET` | `tandem-files` | Бакет для вложений |
| `JWT_SECRET` | — | Секрет JWT, обязателен |
| `JWT_TTL` | `24h` | Время жизни access-токена |
| `JWT_REFRESH_TTL` | `168h` | Время жизни refresh-токена |
| `ADMIN_LOGIN` | — | Логин администратора (сидер при старте) |
| `ADMIN_PASSWORD` | — | Пароль администратора |
| `BCRYPT_COST` | `12` | Стоимость bcrypt (10–15) |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5173` | Разрешённые origin (через запятую) |
| `SWAGGER_ENABLED` | `true` (dev) / `false` (prod) | Служить Swagger UI на `/swagger` |

`.env.dev` и `.env.prod` не отслеживаются git — создаются локально.

## Архитектура

Clean Architecture с полным разделением слоёв:

```
http → usecase → domain/<порты + модели>
repository, infrastructure → domain/ports
```

- `domain/models` — доменные сущности без ORM-тегов;
- `domain/ports` — контракты (repository, service, cache, ws);
- `usecase/*` — прикладной слой, зависит только от портов и `pkg/`;
- `repository/` — GORM-адаптеры; ORM-теги живут только в `repository/entity`;
- `infrastructure/` — адаптеры Redis, MinIO, bcrypt, JWT, WebSocket hub;
- `http/` — хендлеры, DTO, мидлвари, роутер, Swagger UI;
- `app` — единственное место DI и graceful shutdown.

Принципы:

- кеш-инвалидация версионная (`usecase/cacheutil`): ключи `t:wsver:`,
  `t:uver:`, `t:usersver`; при изменении сущности инкремент версии;
- rate limit (sliding window в Redis, префикс `rl:sw:`): `/auth/login` — по IP
  (15/5 мин), read/write/upload/ws — по пользователю (300/120/20/30 в минуту);
- файлы: загрузка через `POST /files/images`, отдача — только presigned URL
  (`GET /files/sign`, TTL 1 ч); прямого стрима `/files/{key}` нет;
- WS-аутентификация — subprotocol `tandem, <JWT>`;
- миграции: GORM `AutoMigrate` при старте + точечные data-миграции
  (в `internal/app/app.go`).

## API

База: `/api/v1`, документация — http://localhost:8080/swagger.

| Группа | Методы |
|--------|--------|
| `/auth` | `POST /login`, `POST /logout` |
| `/me` | `GET`, `PATCH`, `POST /avatar`, `POST /password` |
| `/admin/users` | `GET`, `POST`, `PATCH /:id/role`, `DELETE /:id` (staff) |
| `/workspaces` | `GET`, `POST`, `GET|PATCH|DELETE /:id`, `PUT /:id/theme` |
| `/workspaces/:id/members` | `POST`, `PATCH /:userId`, `DELETE /:userId` |
| `/workspaces/:id/owner` | `POST` (передача владения) |
| `/workspaces/:id/invite` | `GET`, `DELETE`; `POST /invite/:token/join` |
| `/workspaces/:id/boards` | `GET`, `POST`, `PUT /reorder`, `GET|PATCH|DELETE /:boardId`, `PUT /:boardId/main` |
| `/workspaces/:id/tasks` | `GET`, `GET /:taskId` |
| `/workspaces/:id/boards/:boardId/tasks` | `POST`, `PATCH|DELETE /:taskId` |
| `/workspaces/:id/tasks/:taskId/attachments` | `POST`, `GET`, `GET /:attachmentId`, `DELETE /:attachmentId` |
| `/favorites/workspaces|boards/:id` | `PUT`, `DELETE` |
| `/tasks/tree` | `GET` (дерево воркспейсов) |
| `/files/images` | `POST` (auth), `GET /files/sign` (presigned URL) |
| `/ws` | WebSocket (присутствие и события) |

Служебное: `GET /healthz` — проверка работоспособности.

## Роли

- `owner` — полный доступ, передача владения;
- `editor` — управление досками и задачами;
- `member` — работа с задачами, без управления досками и воркспейсом.

## Проверки

```sh
gofmt -l .        # форматирование (пусто = чисто)
goimports -l .    # организация импортов
go vet ./...
go build ./...
go test ./...
make swag         # перегенерировать OpenAPI после изменений хендлеров
```

Тестами покрыт прикладной слой (`usecase/*`, фейки репозиториев/кеша — в
`internal/usecase/testutil`), хендлеры и инфраструктурные адаптеры.
`make test` (из корня репозитория) — то же, что `go test ./...`.

## Индексация кода (Repowise)

Репозиторий проиндексирован [Repowise](https://repowise.dev); авто-артефакты
(`.repowise/`, `.claude/CLAUDE.md`, `.mcp.json`, `.vscode/`) в git не
отслеживаются. Обновлять индекс после крупных изменений: `repowise update`
(инкрементально) или `repowise init` (полная переиндексация).