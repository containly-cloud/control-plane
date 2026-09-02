import { z } from 'zod';

import type { Locale } from '../i18n';

const setupStatusSchema = z.object({ configured: z.boolean() });
const accountCreatedSchema = z.object({ message: z.string() });
const sessionSchema = z.object({
  user: z.object({
    username: z.string(),
    permissions: z.array(z.string()),
  }),
});
const systemOverviewSchema = z.object({
  system: z.object({
    capturedAt: z.string(),
    machine: z.object({
      hostname: z.string(),
      distribution: z.string(),
      kernel: z.string(),
      architecture: z.string(),
    }),
    cpu: z.object({
      cores: z.number().int().nonnegative(),
      usagePercent: z.number().min(0).max(100),
    }),
    memory: z.object({
      totalBytes: z.number().nonnegative(),
      usedBytes: z.number().nonnegative(),
      availableBytes: z.number().nonnegative(),
    }),
    storage: z.object({
      totalBytes: z.number().nonnegative(),
      usedBytes: z.number().nonnegative(),
      availableBytes: z.number().nonnegative(),
      controlPlaneUsedBytes: z.number().nonnegative(),
    }),
    network: z.object({
      publicIp: z.string().optional(),
      interfaces: z.array(
        z.object({ name: z.string(), addresses: z.array(z.string()) }),
      ),
    }),
  }),
});
const monitoringSettingsSchema = z.object({
  enabled: z.boolean(),
  intervalSeconds: z.number().int().min(5).max(86400),
  retentionDays: z.number().int().min(1).max(3650),
  savedMetricsBytes: z.number().nonnegative(),
});
const activeSessionsSchema = z.object({
  sessions: z.array(
    z.object({
      ipAddress: z.string(),
      userAgent: z.string(),
      createdAt: z.string(),
      lastSeenAt: z.string(),
      expiresAt: z.string(),
    }),
  ),
});
const backupsSchema = z.object({
  backups: z.array(
    z.object({
      name: z.string(),
      sizeBytes: z.number(),
      createdAt: z.string(),
    }),
  ),
});
const controlUsersSchema = z.object({
  permissionScopes: z.array(
    z.object({
      scope: z.string(),
      read: z.array(z.string()),
      write: z.array(z.string()),
    }),
  ),
  users: z.array(
    z.object({
      id: z.number(),
      username: z.string(),
      role: z.string(),
      permissions: z.array(z.string()),
      active: z.boolean(),
      passwordTemporary: z.boolean(),
    }),
  ),
});
const createdControlUserSchema = z.object({
  user: z.unknown(),
  temporaryPassword: z.string(),
});
const resetControlUserPasswordSchema = z.object({
  temporaryPassword: z.string(),
});
const apiErrorSchema = z.object({
  error: z.string(),
  code: z.string().optional(),
});

export type RootAccountInput = {
  username: string;
  password: string;
};
export type LoginInput = RootAccountInput;
export type Session = z.infer<typeof sessionSchema>;
export type SystemOverview = z.infer<typeof systemOverviewSchema>;
export type MonitoringSettings = z.infer<typeof monitoringSettingsSchema>;
export type MonitoringSettingsUpdate = Pick<
  MonitoringSettings,
  'enabled' | 'intervalSeconds' | 'retentionDays'
> & { clearSavedMetrics?: boolean };
export type ActiveSessions = z.infer<typeof activeSessionsSchema>;
export type Backups = z.infer<typeof backupsSchema>;
export type ControlUsers = z.infer<typeof controlUsersSchema>;

export class PublicAPIError extends Error {
  readonly code?: string;

  constructor(message: string, code?: string) {
    super(message);
    this.name = 'PublicAPIError';
    this.code = code;
  }
}

async function parseError(response: Response): Promise<PublicAPIError> {
  const body = await response.json().catch(() => null);
  const parsed = apiErrorSchema.safeParse(body);
  if (parsed.success) {
    return new PublicAPIError(parsed.data.error, parsed.data.code);
  }
  return new PublicAPIError('');
}

export async function getSetupStatus(locale: Locale) {
  const response = await fetch('/api/v1/setup/status', {
    headers: { 'Accept-Language': locale },
  });
  if (!response.ok) {
    throw await parseError(response);
  }
  return setupStatusSchema.parse(await response.json());
}

export async function createRootAccount(
  input: RootAccountInput,
  locale: Locale,
) {
  const response = await fetch('/api/v1/setup/root', {
    method: 'POST',
    headers: {
      'Accept-Language': locale,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw await parseError(response);
  }
  return accountCreatedSchema.parse(await response.json());
}

export async function getSession(locale: Locale): Promise<Session | null> {
  const response = await fetch('/api/v1/auth/session', {
    headers: { 'Accept-Language': locale },
  });
  if (response.status === 401) {
    return null;
  }
  if (!response.ok) {
    throw await parseError(response);
  }
  return sessionSchema.parse(await response.json());
}

export async function login(
  input: LoginInput,
  locale: Locale,
): Promise<Session> {
  const response = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: {
      'Accept-Language': locale,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });
  if (!response.ok) {
    throw await parseError(response);
  }
  return sessionSchema.parse(await response.json());
}

export async function logout(locale: Locale) {
  const response = await fetch('/api/v1/auth/logout', {
    method: 'POST',
    headers: { 'Accept-Language': locale },
  });
  if (!response.ok) {
    throw await parseError(response);
  }
}

export async function getSystemOverview(
  locale: Locale,
): Promise<SystemOverview> {
  const response = await fetch('/api/v1/system/overview', {
    headers: { 'Accept-Language': locale },
  });
  if (!response.ok) {
    throw await parseError(response);
  }
  return systemOverviewSchema.parse(await response.json());
}

export async function getMonitoringSettings(
  locale: Locale,
): Promise<MonitoringSettings> {
  const response = await fetch(
    '/api/v1/control-plane/settings/system-monitoring',
    {
      headers: { 'Accept-Language': locale },
    },
  );
  if (!response.ok) throw await parseError(response);
  return monitoringSettingsSchema.parse(await response.json());
}

export async function updateMonitoringSettings(
  settings: MonitoringSettingsUpdate,
  locale: Locale,
): Promise<MonitoringSettings> {
  const response = await fetch(
    '/api/v1/control-plane/settings/system-monitoring',
    {
      method: 'PUT',
      headers: {
        'Accept-Language': locale,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(settings),
    },
  );
  if (!response.ok) throw await parseError(response);
  return monitoringSettingsSchema.parse(await response.json());
}

export async function clearMonitoringMetrics(
  locale: Locale,
): Promise<MonitoringSettings> {
  const response = await fetch(
    '/api/v1/control-plane/settings/system-monitoring/metrics',
    {
      method: 'DELETE',
      headers: { 'Accept-Language': locale },
    },
  );
  if (!response.ok) throw await parseError(response);
  return monitoringSettingsSchema.parse(await response.json());
}

export async function getActiveSessions(
  locale: Locale,
): Promise<ActiveSessions> {
  const response = await fetch('/api/v1/account/sessions', {
    headers: { 'Accept-Language': locale },
  });
  if (!response.ok) {
    throw await parseError(response);
  }
  return activeSessionsSchema.parse(await response.json());
}

export async function getBackups(locale: Locale): Promise<Backups> {
  const response = await fetch('/api/v1/backups', {
    headers: { 'Accept-Language': locale },
  });
  if (!response.ok) throw await parseError(response);
  return backupsSchema.parse(await response.json());
}
export async function createBackup(locale: Locale) {
  const response = await fetch('/api/v1/backups', {
    method: 'POST',
    headers: { 'Accept-Language': locale },
  });
  if (!response.ok) throw await parseError(response);
}
export async function deleteBackup(name: string, locale: Locale) {
  const response = await fetch(
    `/api/v1/backups?name=${encodeURIComponent(name)}`,
    { method: 'DELETE', headers: { 'Accept-Language': locale } },
  );
  if (!response.ok) throw await parseError(response);
}
export async function updateControlUserPermissions(
  id: number,
  permissions: string[],
  locale: Locale,
) {
  const response = await fetch('/api/v1/control-plane/users/permissions', {
    method: 'PUT',
    headers: { 'Accept-Language': locale, 'Content-Type': 'application/json' },
    body: JSON.stringify({ id, permissions }),
  });
  if (!response.ok) throw await parseError(response);
}
export async function resetControlUserPassword(id: number, locale: Locale) {
  const response = await fetch('/api/v1/control-plane/users/password/reset', {
    method: 'POST',
    headers: { 'Accept-Language': locale, 'Content-Type': 'application/json' },
    body: JSON.stringify({ id }),
  });
  if (!response.ok) throw await parseError(response);
  return resetControlUserPasswordSchema.parse(await response.json());
}
export async function deleteControlUser(id: number, locale: Locale) {
  const response = await fetch(`/api/v1/control-plane/users/${id}`, {
    method: 'DELETE',
    headers: { 'Accept-Language': locale },
  });
  if (!response.ok) throw await parseError(response);
}
export async function getControlUsers(locale: Locale): Promise<ControlUsers> {
  const response = await fetch('/api/v1/control-plane/users', {
    headers: { 'Accept-Language': locale },
  });
  if (!response.ok) throw await parseError(response);
  return controlUsersSchema.parse(await response.json());
}
export async function createControlUser(
  input: { username: string; permissions: string[] },
  locale: Locale,
) {
  const response = await fetch('/api/v1/control-plane/users', {
    method: 'POST',
    headers: { 'Accept-Language': locale, 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
  if (!response.ok) throw await parseError(response);
  return createdControlUserSchema.parse(await response.json());
}
