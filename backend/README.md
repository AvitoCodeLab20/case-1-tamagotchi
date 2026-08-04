# Backend

Базовый Go-сервис Avito Tamagotchi. На текущем этапе он поднимает HTTP-сервер, подключается к PostgreSQL и предоставляет probes для оркестратора. Доменное API добавляется следующими feature-ветками.

## Локальный запуск

Из корня репозитория:

```bash
cp .env.example .env
make up
make smoke
```

Сервисы по умолчанию:

- backend: `http://localhost:8080`;
- PostgreSQL: `localhost:5433`;
- `GET /healthz` — процесс запущен;
- `GET /readyz` — сервис готов и PostgreSQL доступна.

Профиль frontend выключен, пока frontend-команда не добавит Dockerfile. После этого весь проект можно будет поднять командой `docker compose --profile frontend up --build -d`.

## Конфигурация

| Переменная | Значение по умолчанию | Назначение |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | Адрес HTTP-сервера. |
| `LOG_LEVEL` | `info` | Уровень логирования: `debug`, `info`, `warn`, `error`. |
| `SHUTDOWN_TIMEOUT` | `10s` | Таймаут graceful shutdown. |
| `DATABASE_CONNECT_TIMEOUT` | `5s` | Таймаут первого подключения к PostgreSQL. |
| `DATABASE_URL` | — | Полный DSN; имеет приоритет над `POSTGRES_*`. |
| `POSTGRES_HOST` | `localhost` | Хост PostgreSQL. |
| `POSTGRES_PORT` | `5432` для backend, `5433` на хосте Compose | Порт PostgreSQL. |
| `POSTGRES_DB` | `tamagotchi` | Имя базы данных. |
| `POSTGRES_USER` | `postgres` | Пользователь PostgreSQL. |
| `POSTGRES_PASSWORD` | `postgres` | Пароль только для локальной разработки. |
| `POSTGRES_SSLMODE` | `disable` | Режим TLS подключения. В production нужен `require` или строже. |

Production-секреты не должны храниться в `.env` репозитория. Они передаются платформой деплоя через secret storage.

## Команды

```bash
make test              # unit-тесты и race detector
make build             # локальная сборка backend
make lint              # golangci-lint
make migrate           # применить новые миграции
make migration-status  # показать применённые версии
make logs              # посмотреть логи контейнеров
```

Описание модели данных и правил изменения схемы находится в [docs/database.md](docs/database.md).

## CI/CD

`backend-ci.yml` запускает тесты, линтер, сборку Docker-образа и проверяет миграции на чистой PostgreSQL при pull request и изменениях в `main`.

`backend-image.yml` после push в `main` или version-тега собирает backend и публикует образ `ghcr.io/<owner>/<repository>/backend` в GitHub Container Registry. Для публикации используется встроенный `GITHUB_TOKEN`; дополнительных секретов не требуется, если Actions разрешена запись в Packages.

## Линтер

Конфигурация находится в корневом `.golangci.yaml`. Помимо стандартного набора включены проверки HTTP response body, оборачивания ошибок, безопасности, контекста, опечаток и корректности `nolint`. Форматирование проверяется `gofmt` и `goimports`.
