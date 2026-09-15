// Словарь en. Ключи — по модулям; плюрали — суффиксы _one/_few/_many/_other.
export default {
  "app": {
    "name": "Heatseeker"
  },
  "common": {
    "save": "Save",
    "cancel": "Cancel",
    "delete": "Delete",
    "edit": "Edit",
    "back": "Back",
    "next": "Next",
    "done": "Done",
    "retry": "Retry",
    "loading": "Loading…",
    "empty": "Nothing here yet",
    "search": "Search",
    "offline": "Offline — showing cached data",
    "coming_soon": "Coming soon"
  },
  "auth": {
    "welcome_title": "What's your name?",
    "welcome_hint": "Just a name — your groupmates will see it. Password and email can be added later.",
    "name_placeholder": "First and last name",
    "continue": "Continue",
    "have_account": "Already have a secured account? Sign in",
    "login_title": "Sign in",
    "email": "Email",
    "password": "Password",
    "login": "Sign in",
    "secure_title": "Secure your account",
    "secure_hint": "Add a password or email to sign in on other devices and to hold headman, moderator or admin roles.",
    "secured": "Account secured",
    "logout": "Sign out",
    "logout_all": "Sign out everywhere",
    "password_hint": "At least {{min}} characters"
  },
  "groups": {
    "title": "Groups",
    "create": "Create group",
    "join": "Join by code",
    "join_code": "Group or invite code",
    "join_question": "Join “{{name}}”?",
    "members_count_one": "{{count}} member",
    "members_count_other": "{{count}} members",
    "name": "Group name",
    "kind": {
      "MASTERS": "Master's",
      "DPO": "Continuing education",
      "OTHER": "Other"
    },
    "members": "Members",
    "invites": "Invites",
    "settings": "Group settings",
    "switch": "Switch group"
  },
  "roles": {
    "OWNER": "Owner",
    "ADMIN": "Admin",
    "MODERATOR": "Moderator",
    "HEADMAN": "Headman",
    "STUDENT": "Student",
    "GUEST": "Guest"
  },
  "quick_tags": {
    "mine": "Mine",
    "saved": "Saved",
    "unread": "Unread"
  },
  "tabs": {
    "feed": "Feed",
    "schedule": "Schedule",
    "tasks": "Tasks",
    "threads": "Discussions",
    "more": "More"
  },
  "subjects": {
    "title": "Subjects",
    "add": "Add subject",
    "name": "Name",
    "short_name": "Short name",
    "teacher": "Teacher",
    "aliases": "Aliases for automatic file sorting",
    "archived": "Archived"
  },
  "errors": {
    "network": "Could not reach the server",
    "unauthorized": "Session expired — sign in again",
    "forbidden": "Not enough permissions",
    "not_found": "Not found",
    "conflict": "Already exists",
    "gone": "This link is no longer valid",
    "validation": "Check the entered data",
    "unknown": "Something went wrong"
  },
  "feed": {
    "empty_hint": "Group materials and tasks will appear here. For now — the activity feed."
  },
  "stages": {
    "schedule": "Group schedule — stage 3 of the roadmap",
    "tasks": "Tasks and deadlines — stage 2",
    "threads": "Subject discussions — stage 2"
  },
  "events": {
    "group_created": "Group created",
    "group_updated": "Group settings changed",
    "member_joined": "New member",
    "member_roles_changed": "Member roles changed",
    "member_status_changed": "Member status changed",
    "subject_created": "Subject added",
    "subject_updated": "Subject changed",
    "subject_archived": "Subject archived",
    "subject_restored": "Subject restored",
    "tag_created": "Tag added",
    "tag_updated": "Tag changed",
    "tag_deleted": "Tag deleted",
    "invite_created": "Invite created",
    "invite_revoked": "Invite revoked",
    "group_join_code_rotated": "Group code rotated"
  }
} as const;
