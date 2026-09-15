# infra/compose — контейнеры

## Разработка: `docker-compose.dev.yml`

| Сервис | Образ | Порт (loopback) |
|---|---|---|
| postgres | `mirror.gcr.io/library/postgres:17-alpine` | 5433 |
| redis | `mirror.gcr.io/library/redis:7-alpine` | 6379 |
| minio | `quay.io/minio/minio:latest` | 9100 (S3), 9101 (консоль) |
| asynqmon | `mirror.gcr.io/hibiken/asynqmon:latest` | 8082 |

Образы берутся с зеркал: Docker Hub из некоторых сетей недоступен. Порты смещены, чтобы не
конфликтовать с другими проектами на той же машине.

```bash
task dev:up      # из корня репо
task dev:down    # остановить, данные остаются в volume'ах
task dev:reset   # остановить и удалить данные
task dev:logs
```

Учётные данные по умолчанию: Postgres `heatseeker/heatseeker`, MinIO — из `S3_ACCESS_KEY_ID` /
`S3_SECRET_ACCESS_KEY` в окружении (иначе `heatseeker` / `heatseeker-dev-secret`).

## Прод (появится на этапе 6)

`docker-compose.prod.yml`: те же сервисы + `api`, `worker` (один образ `apps/api/Dockerfile`,
разные команды) + Caddy (TLS, reverse proxy на `127.0.0.1:8080`, `/storage/*` → MinIO).
Опциональные профили: `clamav`, `office` (LibreOffice для превью). Для сервера без
контейнеров — systemd-юниты для бинарника `heatseeker` и `STORAGE_DRIVER=local`; описание
появится, когда станут известны параметры сервера (`docs/OPEN-QUESTIONS.md`, №3).
