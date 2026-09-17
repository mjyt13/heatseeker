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
    "coming_soon": "Coming soon",
    "open": "Open",
    "download": "Download",
    "archive": "Archive",
    "restore": "Restore",
    "confirm_delete": "Delete permanently from the app?",
    "all": "All",
    "none": "None",
    "load_more": "Load more",
    "refresh": "Refresh"
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
    "password_hint": "At least {{min}} characters",
    "code_optional": "Group code (optional)",
    "code_hint": "If you have a code or an invite link. Without one you can create or join a group after signing in."
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
    "archived": "Archived",
    "manage": "Group subjects",
    "saved": "Subject saved",
    "aliases_hint": "Comma-separated: how the subject is called in folders and files"
  },
  "errors": {
    "network": "Could not reach the server",
    "unauthorized": "Session expired — sign in again",
    "forbidden": "Not enough permissions",
    "not_found": "Not found",
    "conflict": "Already exists",
    "gone": "This link is no longer valid",
    "validation": "Check the entered data",
    "unknown": "Something went wrong",
    "too_large": "File is too large",
    "unavailable": "This feature is unavailable on the server",
    "range": "Could not read part of the file"
  },
  "feed": {
    "empty_hint": "Group materials will appear here."
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
    "group_join_code_rotated": "Group code rotated",
    "material_added": "New material",
    "material_updated": "Material changed",
    "material_classified": "Material sorted",
    "material_archived": "Material archived",
    "material_restored": "Material restored",
    "material_deleted": "Material deleted",
    "drive_connected": "Google Drive folder connected",
    "drive_disconnected": "Google Drive disconnected",
    "drive_synced": "Google Drive synced"
  },
  "units": {
    "b": "B",
    "kb": "KB",
    "mb": "MB",
    "gb": "GB"
  },
  "kinds": {
    "LECTURE": "Lecture",
    "NOTES": "Notes",
    "REPORT": "Report",
    "CALC": "Calculation",
    "ASSIGNMENT": "Assignment",
    "OTHER": "Other"
  },
  "materials": {
    "title": "Materials",
    "search_placeholder": "Search by title",
    "empty_hint": "Files from the group's Google Drive folder and uploads will appear here.",
    "empty_filtered": "Nothing found — try another filter.",
    "chip_unsupported": "This filter arrives together with discussions (stage 2).",
    "upload": "Upload",
    "source_drive": "Google Drive",
    "source_upload": "Uploaded by {{name}}",
    "no_subject": "No subject",
    "subject": "Subject",
    "kind": "Type",
    "tags": "Tags",
    "description": "Description",
    "versions": "Versions",
    "version_n": "Version {{n}}",
    "drive_path": "Drive folder",
    "opened_times_one": "Opened {{count}} time",
    "opened_times_other": "Opened {{count}} times",
    "open_in_drive": "Open in Google Drive",
    "open_in_app": "View",
    "archived_badge": "Archived",
    "deleted_badge": "Deleted",
    "needs_review": "Needs review",
    "review_reason": {
      "LOW_CONFIDENCE": "Subject could not be detected",
      "REMOVED_FROM_DRIVE": "File disappeared from Google Drive"
    },
    "suggested": "Looks like: {{subject}}",
    "drive_upload": {
      "PENDING": "Publishing to Google Drive…",
      "DONE": "Published to Google Drive",
      "FAILED": "Publishing to Drive failed"
    },
    "cannot_delete_opened": "Someone already opened this file — it can only be archived.",
    "archive_list": "Archive",
    "edit_title": "Edit material",
    "name": "Title"
  },
  "upload": {
    "title": "Upload a file",
    "pick": "Choose file",
    "picked": "{{name}} · {{size}}",
    "to_drive": "Publish to the group's Google Drive folder",
    "to_drive_needs_secure": "Secure your account under “More” to publish to Drive.",
    "subject_auto": "Detect from file name",
    "kind_auto": "Detect from file name",
    "send": "Upload",
    "progress": "Uploaded {{percent}}%",
    "done": "File uploaded",
    "too_large": "File is larger than {{size}}",
    "bad_type": "This file type is not allowed. Allowed: {{list}}",
    "empty": "File is empty",
    "failed": "Could not send the file"
  },
  "inbox": {
    "title": "Inbox",
    "banner_one": "{{count}} material needs review",
    "banner_other": "{{count}} materials need review",
    "empty": "All sorted",
    "confirm": "Confirm",
    "learn_alias": "Remember folder “{{folder}}” for this subject",
    "choose_subject": "Choose a subject"
  },
  "drive": {
    "title": "Google Drive",
    "not_configured": "The server has no Google service account configured — ask the administrator.",
    "how_to_title": "How to connect a folder",
    "how_to_1": "Open the group folder in Google Drive and tap “Share”.",
    "how_to_2": "Add {{email}} as “Editor” (“Viewer” is enough for reading).",
    "how_to_3": "Copy the folder link and paste it below.",
    "folder_link": "Folder link",
    "connect": "Connect",
    "reconnect": "Change folder",
    "disconnect": "Disconnect",
    "disconnect_confirm": "Disconnect Drive? Materials stay in the app.",
    "sync_now": "Sync now",
    "full_rescan": "Full rescan",
    "folder": "Folder",
    "status": {
      "PENDING": "Waiting for the first sync",
      "SYNCING": "Syncing…",
      "OK": "Synced",
      "ERROR": "Sync failed"
    },
    "last_sync": "Last sync: {{when}}",
    "never": "never",
    "read_only": "The service account can only read the folder — publishing from the app is unavailable.",
    "writable": "Publishing from the app is available",
    "shared_drive": "Shared drive",
    "stats": "Files: {{files}} · folders: {{folders}} · skipped: {{skipped}} · in inbox: {{inbox}}",
    "needs_manage": "Headmen and admins with a secured account can connect Drive.",
    "items": "Indexed files"
  },
  "activity": {
    "title": "Group activity"
  },
  "more": {
    "section_group": "Group",
    "section_account": "Account"
  }
} as const;
