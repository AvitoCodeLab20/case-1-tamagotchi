# Backend

Go-сервис Avito Tamagotchi. На текущем этапе он поднимает HTTP-сервер, подключается к PostgreSQL, предоставляет probes для оркестратора и реализует аутентификацию по email и паролю. Игровое API добавляется следующими feature-ветками.

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

Без `JWT_SECRET` backend не стартует, поэтому `cp .env.example .env` перед `make up` обязателен.

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
| `JWT_SECRET` | — | Ключ подписи access-токенов, минимум 32 символа. Обязателен, без него сервис не стартует. |
| `JWT_ISSUER` | `avito-tamagotchi` | Значение claim `iss`. |
| `ACCESS_TOKEN_TTL` | `15m` | Время жизни access-токена. |
| `REFRESH_TOKEN_TTL` | `720h` | Время жизни refresh-токена. |
| `BCRYPT_COST` | `12` | Стоимость bcrypt, допустимо 10–15. |

Production-секреты не должны храниться в `.env` репозитория. Они передаются платформой деплоя через secret storage.

## API

| Метод | Путь | Авторизация | Назначение |
| --- | --- | --- | --- |
| `POST` | `/api/v1/auth/register` | — | Создать аккаунт и сразу войти. |
| `POST` | `/api/v1/auth/login` | — | Войти по email и паролю. |
| `POST` | `/api/v1/auth/refresh` | — | Обменять refresh-токен на новую пару. |
| `POST` | `/api/v1/auth/logout` | — | Отозвать предъявленный refresh-токен. |
| `POST` | `/api/v1/auth/logout-all` | Bearer | Отозвать все сессии пользователя. |
| `GET` | `/api/v1/auth/me` | Bearer | Профиль текущего пользователя. |

Быстрая проверка на поднятом стенде:

```bash
curl -sS -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"player@avito.ru","display_name":"Игрок","password":"correct-horse-battery"}'
```

Схема токенов, коды ошибок и правила валидации описаны в [docs/authentication.md](docs/authentication.md).

## Команды

```bash
make test              # unit-тесты и race detector
make test-integration  # плюс тесты репозиториев на поднятой БД
make build             # локальная сборка backend
make lint              # golangci-lint
make migrate           # применить новые миграции
make migration-status  # показать применённые версии
make logs              # посмотреть логи контейнеров
```

Описание модели данных и правил изменения схемы находится в [docs/database.md](docs/database.md), устройство аутентификации — в [docs/authentication.md](docs/authentication.md).

## CI/CD

`backend-ci.yml` запускает тесты, линтер, сборку Docker-образа и проверяет миграции на чистой PostgreSQL при pull request и изменениях в `main`.

`backend-image.yml` после push в `main` или version-тега собирает backend и публикует образ `ghcr.io/<owner>/<repository>/backend` в GitHub Container Registry. Для публикации используется встроенный `GITHUB_TOKEN`; дополнительных секретов не требуется, если Actions разрешена запись в Packages.

## Линтер

Конфигурация находится в корневом `.golangci.yaml`. Помимо стандартного набора включены проверки HTTP response body, оборачивания ошибок, безопасности, контекста, опечаток и корректности `nolint`. Форматирование проверяется `gofmt` и `goimports`.
