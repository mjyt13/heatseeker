import { useMemo } from 'react';

import type { Membership } from '@heatseeker/api-client';
import { can, requiresSecured, type Action, type Role } from '@heatseeker/shared';

export interface Permissions {
  /** Может ли участник выполнить действие (по ролям и, для защищённых действий, по статусу аккаунта). */
  can: (action: Action) => boolean;
  /** Действие разрешено ролями, но требует защитить аккаунт. */
  needsSecuring: (action: Action) => boolean;
  roles: readonly Role[];
}

/** Права текущего участника: серверный ответ `permissions` — источник правды, матрица — запасной путь. */
export function usePermissions(membership: Membership | null | undefined, secured: boolean): Permissions {
  return useMemo(() => {
    const roles = (membership?.roles ?? []) as Role[];
    const granted = membership?.permissions ? new Set(membership.permissions) : null;
    const byRoles = (action: Action) => (granted ? granted.has(action) : can(roles, action));
    return {
      roles,
      can: (action) => byRoles(action) && (!requiresSecured(action) || secured),
      needsSecuring: (action) => byRoles(action) && requiresSecured(action) && !secured,
    };
  }, [membership, secured]);
}
