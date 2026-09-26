// Package gen emits artifacts derived from Go code: the OpenAPI document for
// packages/api-client and the permission matrix / enums for packages/shared.
package gen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"heatseeker/api/internal/authz"
	"heatseeker/api/internal/domain"
	httptransport "heatseeker/api/internal/transport/http"
)

// OpenAPI writes the API description to path.
func OpenAPI(path string) error {
	srv := httptransport.NewServer(httptransport.Deps{})
	doc, err := json.MarshalIndent(srv.API.OpenAPI(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal openapi: %w", err)
	}
	return writeFile(path, append(doc, '\n'))
}

type enumDef struct {
	Name   string
	Values []string
}

var enums = []enumDef{
	{"Role", strs(domain.AllRoles)},
	{"GlobalRole", []string{string(domain.GlobalRoleUser), string(domain.GlobalRoleSuperadmin)}},
	{"GroupKind", []string{string(domain.GroupKindMasters), string(domain.GroupKindDPO), string(domain.GroupKindOther)}},
	{"JoinPolicy", []string{string(domain.JoinOpen), string(domain.JoinInvite), string(domain.JoinApproval)}},
	{"MediaMode", []string{string(domain.MediaLink), string(domain.MediaCache), string(domain.MediaImport)}},
	{"MembershipStatus", []string{string(domain.MembershipActive), string(domain.MembershipPending), string(domain.MembershipBanned)}},
	{"JoinedVia", []string{string(domain.JoinedViaLink), string(domain.JoinedViaInvite), string(domain.JoinedViaApproval), string(domain.JoinedViaCreator)}},
	{"TagKind", []string{string(domain.TagKindSubject), string(domain.TagKindTopic), string(domain.TagKindType), string(domain.TagKindSystem), string(domain.TagKindCustom)}},
	{"DevicePlatform", []string{string(domain.PlatformIOS), string(domain.PlatformAndroid), string(domain.PlatformWeb)}},
	{"PushProvider", []string{string(domain.PushExpo), string(domain.PushWebPush)}},
	{"IdentityProvider", []string{string(domain.ProviderGoogle), string(domain.ProviderApple)}},
	{"EventKind", strsOf(domain.AllEventKinds)},
	{"MaterialKind", strsOf(domain.AllMaterialKinds)},
	{"FileType", strsOf(domain.AllFileTypes)},
	{"ErrorCode", domain.AllErrorCodes},
	{"TaskKind", strsOf(domain.AllTaskKinds)},
	{"TaskStatus", strsOf(domain.AllTaskStatuses)},
	{"TaskPriority", strsOf(domain.AllTaskPriorities)},
	{"TaskAssignMode", strsOf(domain.AllTaskAssignModes)},
	{"TaskVisibility", strsOf(domain.AllTaskVisibilities)},
	{"ThreadTarget", strsOf(domain.AllThreadTargets)},
	{"NotificationType", strsOf(domain.AllNotificationTypes)},
	{"MuteScope", strsOf(domain.AllMuteScopes)},
	{"ReminderRepeat", strsOf(domain.AllReminderRepeats)},
	{"ReminderStatus", strsOf(domain.AllReminderStatuses)},
	{"ReminderTarget", strsOf(domain.AllReminderTargets)},
	{"MaterialSource", strsOf([]domain.MaterialSource{domain.SourceUpload, domain.SourceDrive})},
	{"MaterialStatus", strsOf([]domain.MaterialStatus{domain.MaterialActive, domain.MaterialArchived, domain.MaterialDeleted})},
	{"ReviewReason", strsOf([]domain.ReviewReason{domain.ReviewLowConfidence, domain.ReviewRemovedFromDrive})},
	{"StorageKind", strsOf([]domain.StorageKind{domain.StorageDrive, domain.StorageS3, domain.StorageLocal})},
	{"ScanStatus", strsOf([]domain.ScanStatus{domain.ScanPending, domain.ScanClean, domain.ScanInfected, domain.ScanSkipped})},
	{"DriveUploadStatus", strsOf([]domain.DriveUploadStatus{domain.DriveUploadPending, domain.DriveUploadDone, domain.DriveUploadFailed})},
	{"DriveConnectionStatus", strsOf([]domain.DriveConnectionStatus{domain.DrivePending, domain.DriveSyncing, domain.DriveOK, domain.DriveError})},
	{"DriveItemState", strsOf([]domain.DriveItemState{
		domain.DriveItemNew, domain.DriveItemLinked, domain.DriveItemImported,
		domain.DriveItemSkipped, domain.DriveItemError, domain.DriveItemDeleted,
	})},
}

// Shared writes permissions.json, permissions.ts and enums.ts into dir.
func Shared(dir string) error {
	actions := authz.Actions()
	matrix := map[string][]string{}
	for _, a := range actions {
		roles := authz.Matrix[a]
		matrix[string(a)] = strs(roles)
	}
	secured := make([]string, 0, len(authz.SecuredActions))
	for a := range authz.SecuredActions {
		secured = append(secured, string(a))
	}
	sort.Strings(secured)

	doc := map[string]any{
		"roles":   strs(domain.AllRoles),
		"actions": strsA(actions),
		"matrix":  matrix,
		"secured": secured,
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "permissions.json"), append(raw, '\n')); err != nil {
		return err
	}

	var ts strings.Builder
	ts.WriteString(header)
	ts.WriteString("import type { Role } from './enums';\n\n")
	ts.WriteString("export const ACTIONS = [\n")
	for _, a := range actions {
		fmt.Fprintf(&ts, "  %q,\n", a)
	}
	ts.WriteString("] as const;\n\nexport type Action = (typeof ACTIONS)[number];\n\n")
	ts.WriteString("export const PERMISSIONS: Record<Action, readonly Role[]> = {\n")
	for _, a := range actions {
		fmt.Fprintf(&ts, "  %q: [%s],\n", a, quoted(matrix[string(a)]))
	}
	ts.WriteString("};\n\n")
	fmt.Fprintf(&ts, "export const SECURED_ACTIONS: readonly Action[] = [%s];\n\n", quoted(secured))
	ts.WriteString(`/** Returns true when any of the roles grants the action (union semantics, as on the server). */
export function can(roles: readonly Role[], action: Action): boolean {
  const allowed = PERMISSIONS[action];
  return roles.some((r) => allowed.includes(r));
}

/** Returns true when the action additionally requires a secured (L2) account. */
export function requiresSecured(action: Action): boolean {
  return SECURED_ACTIONS.includes(action);
}
`)
	if err := writeFile(filepath.Join(dir, "permissions.ts"), []byte(ts.String())); err != nil {
		return err
	}

	var en strings.Builder
	en.WriteString(header)
	for _, e := range enums {
		fmt.Fprintf(&en, "export const %s = [%s] as const;\nexport type %s = (typeof %s)[number];\n\n",
			constName(e.Name), quoted(e.Values), e.Name, constName(e.Name))
	}
	fmt.Fprintf(&en, "/** RFC 7807 problem type prefix of coded API errors: `${ERROR_TYPE_PREFIX}<ErrorCode>`. */\nexport const ERROR_TYPE_PREFIX = %q;\n", domain.ErrorPrefix)
	if err := writeFile(filepath.Join(dir, "enums.ts"), []byte(en.String())); err != nil {
		return err
	}
	index := header + "export * from './enums';\nexport * from './permissions';\n"
	return writeFile(filepath.Join(dir, "index.ts"), []byte(index))
}

const header = "// Generated by `heatseeker gen shared` from apps/api. DO NOT EDIT.\n\n"

func constName(typeName string) string {
	var b strings.Builder
	for i, r := range typeName {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToUpper(b.String()) + "S"
}

func quoted(values []string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = fmt.Sprintf("%q", v)
	}
	return strings.Join(parts, ", ")
}

func strs(roles []domain.Role) []string { return domain.RolesToStrings(roles) }

func strsOf[T ~string](values []T) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = string(v)
	}
	return out
}

func strsA(actions []authz.Action) []string {
	out := make([]string, len(actions))
	for i, a := range actions {
		out[i] = string(a)
	}
	return out
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
