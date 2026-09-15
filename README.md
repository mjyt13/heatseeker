# Heatseeker

Платформа для учебной группы: единое хранилище материалов (с автоматической раскладкой файлов с
Google Диска по предметам), дедлайны и задачи, общее расписание, обсуждения по предметам,
предложения группы, уведомления и напоминания. Mobile-first (Android), веб — вторым этапом.

## Статус

Планирование завершено (2026-09-15), кодирование не начато. См.:

- [`docs/PLAN.md`](docs/PLAN.md) — план разработки и архитектура
- [`docs/DATA-MODEL.md`](docs/DATA-MODEL.md) — модель данных
- [`docs/DECISIONS.md`](docs/DECISIONS.md) — принятые решения
- [`docs/OPEN-QUESTIONS.md`](docs/OPEN-QUESTIONS.md) — открытые вопросы
- [`docs/adr/`](docs/adr/) — архитектурные решения (ADR)
- [`CLAUDE.md`](CLAUDE.md) — контекст и конвенции для работы с ИИ-агентом

## Стек

Go (chi + huma, sqlc, asynq) · PostgreSQL · Redis · MinIO · Expo + Tamagui · Vite + React ·
отдельный ИИ-сервис с локальной моделью.

## Структура

```
apps/      api (Go), mobile (Expo), web (Vite), ai (ИИ-сервис)
packages/  ui, shared, api-client, core, i18n, config
infra/     compose, scripts
docs/      план, модель данных, решения, ADR
```

## Запуск

Появится на этапе 0 (см. `CLAUDE.md`, раздел «Команды»).
