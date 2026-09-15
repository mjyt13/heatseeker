# infra/compose — контейнеры

Планируется:

- `docker-compose.dev.yml` — Postgres, Redis, MinIO, asynqmon (+ опционально ClamAV, Mailpit).
  Порты публикуются только на loopback.
- `docker-compose.prod.yml` — то же + `api`, `worker` (один образ, разные команды), Caddy
  (TLS, reverse proxy на `127.0.0.1:8080`, `/storage/*` → MinIO). Опциональные профили:
  `clamav`, `office` (LibreOffice для превью).
- `Caddyfile`.

Вариант без docker (только ssh на сервере): systemd-юниты для бинарника `heatseeker` и
`STORAGE_DRIVER=local` — описывается здесь же, когда станут известны параметры сервера
(`docs/OPEN-QUESTIONS.md`, №3).

Запуск локально — через `podman compose` или `docker compose`.

Статус: создаётся на этапе 0.
