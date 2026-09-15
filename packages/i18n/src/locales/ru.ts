// Словарь ru. Ключи — по модулям; плюрали — суффиксы _one/_few/_many/_other.
export default {
  "app": {
    "name": "Heatseeker"
  },
  "common": {
    "save": "Сохранить",
    "cancel": "Отмена",
    "delete": "Удалить",
    "edit": "Изменить",
    "back": "Назад",
    "next": "Далее",
    "done": "Готово",
    "retry": "Повторить",
    "loading": "Загрузка…",
    "empty": "Пока пусто",
    "search": "Поиск",
    "offline": "Нет сети — показаны сохранённые данные",
    "coming_soon": "Скоро появится"
  },
  "auth": {
    "welcome_title": "Как тебя зовут?",
    "welcome_hint": "Только имя — его увидят одногруппники. Пароль и почту можно добавить позже.",
    "name_placeholder": "Имя и фамилия",
    "continue": "Продолжить",
    "have_account": "Уже есть защищённый аккаунт? Войти",
    "login_title": "Вход",
    "email": "Электронная почта",
    "password": "Пароль",
    "login": "Войти",
    "secure_title": "Защитить аккаунт",
    "secure_hint": "Добавь пароль или почту, чтобы входить с других устройств и получать роли старосты, модератора или админа.",
    "secured": "Аккаунт защищён",
    "logout": "Выйти",
    "logout_all": "Выйти на всех устройствах",
    "password_hint": "Не короче {{min}} символов"
  },
  "groups": {
    "title": "Группы",
    "create": "Создать группу",
    "join": "Вступить по коду",
    "join_code": "Код группы или приглашения",
    "join_question": "Вступить в группу «{{name}}»?",
    "members_count_one": "{{count}} участник",
    "members_count_few": "{{count}} участника",
    "members_count_many": "{{count}} участников",
    "name": "Название группы",
    "kind": {
      "MASTERS": "Магистратура",
      "DPO": "ДПО",
      "OTHER": "Другое"
    },
    "members": "Участники",
    "invites": "Приглашения",
    "settings": "Настройки группы",
    "switch": "Сменить группу"
  },
  "roles": {
    "OWNER": "Владелец",
    "ADMIN": "Админ",
    "MODERATOR": "Модератор",
    "HEADMAN": "Староста",
    "STUDENT": "Студент",
    "GUEST": "Гость"
  },
  "quick_tags": {
    "mine": "Мои",
    "saved": "Сохранённое",
    "unread": "Непрочитанное"
  },
  "tabs": {
    "feed": "Лента",
    "schedule": "Расписание",
    "tasks": "Задачи",
    "threads": "Обсуждения",
    "more": "Ещё"
  },
  "subjects": {
    "title": "Предметы",
    "add": "Добавить предмет",
    "name": "Название",
    "short_name": "Короткое название",
    "teacher": "Преподаватель",
    "aliases": "Синонимы для авторазбора файлов",
    "archived": "В архиве"
  },
  "errors": {
    "network": "Не удалось связаться с сервером",
    "unauthorized": "Сессия истекла — войди снова",
    "forbidden": "Недостаточно прав",
    "not_found": "Не найдено",
    "conflict": "Такая запись уже есть",
    "gone": "Ссылка больше не действует",
    "validation": "Проверь введённые данные",
    "unknown": "Что-то пошло не так"
  },
  "feed": {
    "empty_hint": "Материалы и задачи группы появятся здесь. Пока — лента активности."
  },
  "stages": {
    "schedule": "Расписание группы — этап 3 плана разработки",
    "tasks": "Задачи и дедлайны — этап 2",
    "threads": "Обсуждения по предметам — этап 2"
  },
  "events": {
    "group_created": "Группа создана",
    "group_updated": "Настройки группы изменены",
    "member_joined": "Новый участник",
    "member_roles_changed": "Роли участника изменены",
    "member_status_changed": "Статус участника изменён",
    "subject_created": "Добавлен предмет",
    "subject_updated": "Предмет изменён",
    "subject_archived": "Предмет в архиве",
    "subject_restored": "Предмет восстановлен",
    "tag_created": "Добавлен тег",
    "tag_updated": "Тег изменён",
    "tag_deleted": "Тег удалён",
    "invite_created": "Создано приглашение",
    "invite_revoked": "Приглашение отозвано",
    "group_join_code_rotated": "Код группы обновлён"
  }
} as const;
