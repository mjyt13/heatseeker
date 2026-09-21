# Heatseeker — модель данных

Редакция от 2026-09-15. Реализуется миграциями goose в `apps/api/db/migrations/` и запросами
sqlc в `apps/api/db/queries/`. Общие правила:

- `id` — UUIDv7 (сортируемые по времени); `created_at`, `updated_at` — везде.
- Сущности группы несут `group_id`; все запросы фильтруются по нему.
- Soft-delete (`deleted_at` / `archived_at`) там, где есть модерация или восстановление.
- Текстовые тела — markdown.
- Enum'ы — Postgres `ENUM` или `TEXT` + CHECK; источник правды — Go (`internal/domain`),
  генерируется в `packages/shared`.

## Пользователи и доступ

| Таблица | Поля |
|---|---|
| `users` | `name` (обязательно), `email?` (uniq nullable), `password_hash?`, `locale`, `timezone`, `avatar_key?`, `global_role` `USER \| SUPERADMIN`, `secured_at?` (когда добавлены credentials — уровень L2), `register_client_id?` (uniq nullable: UUID экрана регистрации — повтор `POST /auth/register` в течение 15 минут возвращает тот же незащищённый аккаунт), `deleted_at` |
| `auth_identities` | `user_id`, `provider` `GOOGLE \| APPLE`, `provider_user_id` (uniq per provider), `email`, `google_refresh_token_enc?` (только если дан scope для загрузки на Диск; AES-GCM) |
| `refresh_tokens` | `user_id`, `device_id`, `token_hash`, `expires_at`, `revoked_at` |
| `devices` | `user_id`, `platform` `IOS \| ANDROID \| WEB`, `push_provider` `EXPO \| WEBPUSH`, `push_token` / `subscription` json, `last_seen_at`, `enabled` |

## Группы и членство

| Таблица | Поля |
|---|---|
| `groups` | `name`, `slug`, `kind` `MASTERS \| DPO \| OTHER`, `join_policy` `OPEN \| INVITE \| APPROVAL`, `public_read` bool (L0, по умолчанию false), `media_mode` `LINK \| CACHE \| IMPORT`, `settings` json (timezone, дефолты уведомлений — для DPO off, квоты), `created_by`, `archived_at` |
| `memberships` | `user_id` + `group_id` (uniq), **`roles` role[]** (`OWNER \| ADMIN \| MODERATOR \| HEADMAN \| STUDENT \| GUEST`), `status` `ACTIVE \| PENDING \| BANNED`, `joined_via` `LINK \| INVITE \| APPROVAL`, `joined_at` |
| `invites` | `group_id`, `code` (uniq), `roles[]`, `expires_at`, `max_uses`, `uses`, `created_by` |

## Предметы, теги, материалы

| Таблица | Поля |
|---|---|
| `subjects` | `group_id`, `name`, `short_name`, `teacher`, `teacher_contact`, `color`, `semester`, `aliases` text[] (для классификации; пополняются правками), `archived_at` |
| `tags` | `group_id`, `name`, `slug` (uniq per group), `color`, `kind` `SUBJECT \| TOPIC \| TYPE \| SYSTEM \| CUSTOM`, `subject_id?` (тег-двойник предмета создаётся автоматически) |
| `materials` | `group_id`, `subject_id?`, `uploader_id?`, `title`, `description` (md), `kind` `LECTURE \| NOTES \| REPORT \| CALC \| ASSIGNMENT \| OTHER`, `source` `UPLOAD \| GDRIVE`, `status` `ACTIVE \| ARCHIVED \| DELETED`, `current_version_id`, `classification` json `{subject_id, kind, confidence, method}`, `needs_review`, `review_reason` `LOW_CONFIDENCE \| REMOVED_FROM_DRIVE`, `download_count` (открытия не автором), `sort_at` (время загрузки / создания файла на Диске — порядок ленты), `search` tsvector (generated: `russian` + `simple` по названию), `archived_by/at`, `deleted_by/at`, `task_id?` (файл оставлен только в этой задаче — не в ленте, D43) |
| `material_versions` | `material_id`, `version_no`, **`storage`** `DRIVE \| S3 \| LOCAL`, `storage_key?`, `cache_expires_at?` (режим CACHE), `drive_file_id?`, `drive_web_view_link?`, `drive_md5?`, `drive_modified_time?`, `drive_revision_id?` (ревизия Диска, из которой проиндексирована версия; старые версии отдаются из неё), `drive_upload_status?` `PENDING \| DONE \| FAILED` + `drive_upload_error?` (публикация загрузки на Диск), `original_name`, `mime`, `size_bytes`, `sha256?`, `scan_status` `PENDING \| CLEAN \| INFECTED \| SKIPPED`, `text_key?`, `preview_key?` + `preview_status?` `PENDING \| READY \| FAILED \| SKIPPED` + `preview_error?` (PDF-превью офисного файла, Gotenberg; NULL — не запрошено), `uploaded_by?` |
| `material_tags`, `task_tags`, `proposal_tags` | (`entity_id`, `tag_id`) |
| `uploads` | `group_id`, `user_id`, `storage` `S3 \| LOCAL`, `storage_key` (`tmp/uploads/…`), `file_name`, `mime`, `size_bytes`, `meta` json (форма материала), `status` `PENDING \| COMPLETED \| EXPIRED`, `material_id?`, `expires_at` — прямые загрузки до `complete` |
| `media_cache` (проекция/индекс) | `version_id`, `storage_key`, `size`, `last_access_at`, `expires_at` — для вытеснения по TTL/объёму (LRU) |

## Задачи

| Таблица | Поля |
|---|---|
| `tasks` | `group_id`, `subject_id?`, `created_by` (любой участник), **`client_id?`** (идемпотентность создания, uniq в группе), `title`, `description` (md), `kind` `TEACHER \| GROUP \| PERSONAL`, `due_at`, `status` `TODO \| IN_PROGRESS \| IN_REVIEW \| DONE \| CANCELLED`, `priority` `LOW \| NORMAL \| HIGH`, `assign_mode` `ALL \| SELECTED \| SELF`, `visibility` `GROUP \| PRIVATE`, **`notified_offsets` int[]** (уже объявленные напоминания о сроке, в минутах до него — D38), **`pinned_by?`, `pinned_at?`** (староста/модератор/админ), `completed_at?`, `deleted_at?` (мягкое удаление) |
| `task_assignments` | `task_id` + `user_id`, `status` (персональный прогресс), `completed_at` |
| `task_attachments` | `task_id`, `material_id` |

Миграция `00008_tasks.sql`. Личные задачи (`visibility=PRIVATE`) видит только автор — фильтр
в запросах списка и карточки. Строка `task_assignments` появляется, когда участника выбрали
(`SELECTED`/`SELF`) или когда он впервые отметил свой прогресс.

## Расписание

| Таблица | Поля |
|---|---|
| `schedule_events` | `group_id`, `subject_id?`, `title`, `kind` `LECTURE \| SEMINAR \| LAB \| EXAM \| CONSULTATION \| OTHER`, `starts_at`, `ends_at`, `timezone`, `location`, `teacher`, `rrule?`, `rrule_until?`, `created_by`, `updated_by`, `version` |
| `schedule_exceptions` | `event_id`, `original_date`, `kind` `CANCELLED \| MOVED`, `overrides` json |
| `schedule_change_requests` (этап 3) | `group_id`, `event_id?`, `proposed_by`, `op` `CREATE \| UPDATE \| DELETE`, `payload` json, `status` `PENDING \| APPROVED \| REJECTED`, `reviewed_by/at`, `comment` |

## Обсуждения

| Таблица | Поля |
|---|---|
| `threads` | `group_id`, `target_type` `SUBJECT \| LESSON \| MATERIAL \| TASK \| PROPOSAL \| GENERAL`, `target_id` (id предмета/материала/задачи; для GENERAL — id группы; для LESSON этап 3 добавит `occurrence_date`), uniq `(group_id, target_type, target_id)`, `subject_id?` (предмет материала/задачи — для навигации, денормализация), `title?` (название материала/задачи на момент первого сообщения), `created_by`, `message_count` (без удалённых), `last_message_at`, `last_seq` (seq последнего сообщения — для непрочитанного). Создаётся первым сообщением (D40) |
| `messages` | `group_id`, `thread_id`, `author_id`, **`client_id`** (uniq per thread — идемпотентность офлайн-отправки), **`seq`** bigint (seq события `message.created` в `group_events`), `body` (md, до 4000 символов), `reply_to_id?`, `edited_at`, `deleted_at` (удалённое остаётся пометкой), **`hidden_for_all_by?` / `hidden_for_all_at?`** (модерация) |
| `message_hides` | `message_id` + `user_id`, `created_at` (личное скрытие; «Скрытые мной» — по `created_at`) |
| `thread_reads` | `thread_id` + `user_id`, `last_read_seq` (только растёт) |
| `bookmarks` | `user_id`, `target_type`, `target_id` («Сохранённое») |

## Предложения, объявления, напоминания

| Таблица | Поля |
|---|---|
| `proposals` | `group_id`, `author_id`, `title`, `body` (md), `status` `NEW \| DISCUSSION \| ACCEPTED \| REJECTED \| DONE`; обсуждение — тред `PROPOSAL` |
| `proposal_votes` | `proposal_id` + `user_id`, `value` ±1 |
| `announcements` | `group_id`, `author_id`, `title`, `body` (md), `pinned`, `urgent`, `sent_at` |
| `reminders` | `user_id`, `group_id?`, `text`, `remind_at`, `rrule?`, `target_type?` / `target_id?`, `status` `SCHEDULED \| SENT \| CANCELLED`, `last_fired_at` |

## Уведомления

| Таблица | Поля |
|---|---|
| `notifications` | `user_id`, `group_id?`, `type`, `title`, `body`, `data` json (deep link), `read_at`, `dedupe_key` (uniq) |
| `notification_deliveries` | `notification_id`, `channel` `PUSH \| WEBPUSH \| EMAIL \| TELEGRAM`, `device_id?`, `status` `QUEUED \| SENT \| FAILED`, `error`, `sent_at` |
| `notification_preferences` | `user_id`, `group_id?`, `type`, `channel`, `enabled`; на уровне пользователя — `quiet_hours`, `deadline_offsets` int[] |
| `notification_mutes` | `user_id`, `scope_type` `GROUP \| SUBJECT \| THREAD \| TYPE`, `scope_id`, `until` (обязательно) |

## Журнал событий и активность

| Таблица | Поля |
|---|---|
| `group_events` | `group_id`, **`seq`** bigint (монотонный per group), `kind` (`message.created`, `material.added`, `task.updated`, `schedule.changed`, `drive.synced`, …), `actor_id`, `entity_type`, `entity_id`, `payload` json, `audit` bool (показывать в ленте активности), `created_at` — **append-only; единый источник для realtime, `sync?since=` и ленты активности**; ретеншен `EVENT_LOG_RETENTION_DAYS` |

## Google Drive

| Таблица | Поля |
|---|---|
| `drive_publishers` | `group_id` PK — аккаунт Google, от имени которого загрузки публикуются на Диск (D34): `google_email`, `refresh_token_enc` bytea (AES-GCM под `APP_ENCRYPTION_KEY`, связан с `group_id`), `scopes`, `last_error?` (Google отозвал доступ), `connected_by?` |
| `drive_connections` | `group_id` (uniq — одна папка на группу), `mode` `SERVICE_ACCOUNT`, `root_folder_id`, `root_folder_name`, `drive_id?` (Shared Drive), `status` `PENDING \| SYNCING \| OK \| ERROR`, `last_error`, `last_error_code?` (код ошибки для перевода), `sync_started_at`, `last_sync_at`, `last_full_scan_at`, `changes_page_token`, `sync_interval_sec`, `writable` bool |
| `drive_items` | `connection_id`, `drive_file_id` (uniq в пределах подключения), `parent_id`, `path_cache` (папки от корня через `/`), `name`, `mime`, `is_folder`, `md5`, `size`, `modified_time`, `web_view_link`, `material_id?`, `state` `NEW \| LINKED \| IMPORTED \| SKIPPED \| ERROR \| DELETED` (SKIPPED — неподдерживаемый тип или удалён в приложении: синк его не восстанавливает), `classification` json, `last_error`, `seen_at` (метка прохода полного скана) |

## ИИ-сервис (схема `ai.*`, этап W2-B)

`ai_jobs` (kind, status, input ref, provider, model, tokens, cost), `ai_artifacts` (target,
kind `SUMMARY | STRUCTURE | QA | CLASSIFICATION`, body md), `material_chunks`
(version_id, idx, text, embedding vector).

## Индексы

- `group_events (group_id, seq)`, `messages (thread_id, seq)`, `messages (group_id, seq)`.
- Уникальные: `messages (thread_id, client_id)`, `tags (group_id, slug)`,
  `memberships (user_id, group_id)`, `notifications (dedupe_key)`, `drive_items (connection_id, drive_file_id)`,
  `material_versions (material_id, version_no)`, `drive_connections (group_id)`.
- GIN: `materials.search`, `subjects.aliases`.
- `notifications (user_id, read_at)`, `reminders (remind_at, status)`, `tasks (group_id, due_at)`,
  `material_versions (cache_expires_at)`, `media_cache (last_access_at)`,
  `materials (group_id, status, sort_at, id)`, `threads (group_id, subject_id, last_message_at)`.
