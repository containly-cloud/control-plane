import {
  Alert,
  AlertDialog,
  Button,
  Card,
  Checkbox,
  Dropdown,
  FieldError,
  Form,
  Input,
  Label,
  ListBox,
  Modal,
  Select,
  Table,
  TextField,
  Toast,
} from '@heroui/react';
import {
  type UseQueryResult,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { type FormEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { z } from 'zod';

import {
  type ActiveSessions,
  clearMonitoringMetrics,
  createBackup,
  createControlUser,
  createRootAccount,
  deleteBackup,
  deleteControlUser,
  getActiveSessions,
  getBackups,
  getControlUsers,
  getMonitoringSettings,
  getSession,
  getSetupStatus,
  getSystemOverview,
  type LoginInput,
  login,
  logout,
  type MonitoringSettings,
  type MonitoringSettingsUpdate,
  PublicAPIError,
  type RootAccountInput,
  resetControlUserPassword,
  type Session,
  type SystemOverview,
  updateControlUserPermissions,
  updateMonitoringSettings,
} from '../api/setup';
import { type Locale, selectLocale, toSupportedLocale } from '../i18n';

export const Route = createFileRoute('/')({ component: WelcomePage });

type FormErrors = Partial<
  Record<'username' | 'password' | 'confirmation', string>
>;

const pageTitleClassName = 'm-0 text-3xl';
const languageSelectTriggerClassName =
  'min-h-8 rounded-md border border-[oklch(45%_0.05_264)] bg-[oklch(18%_0.04_264)] px-2 text-[oklch(96%_0.01_255)] hover:!border-[oklch(60%_0.08_264)] hover:!bg-[oklch(23%_0.045_264)] data-[hovered=true]:!border-[oklch(60%_0.08_264)] data-[hovered=true]:!bg-[oklch(23%_0.045_264)] [&_.select__indicator]:!text-[oklch(89%_0.018_255)]';
const languageOptionClassName =
  '!text-[oklch(96%_0.01_255)] hover:!bg-[oklch(28%_0.05_264)] data-[hovered=true]:!bg-[oklch(28%_0.05_264)] data-[selected=true]:!bg-[oklch(35%_0.07_264)] aria-selected:!bg-[oklch(35%_0.07_264)] [&_[data-slot=list-box-item-indicator]]:!text-[oklch(96%_0.01_255)]';

function WelcomePage() {
  const { i18n, t } = useTranslation();
  const queryClient = useQueryClient();
  const locale = toSupportedLocale(i18n.resolvedLanguage);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [formErrors, setFormErrors] = useState<FormErrors>({});

  const setupQuery = useQuery({
    queryKey: ['setup-status', locale],
    queryFn: () => getSetupStatus(locale),
  });
  const sessionQuery = useQuery({
    queryKey: ['session', locale],
    queryFn: () => getSession(locale),
    enabled: setupQuery.data?.configured === true,
  });
  const createAccount = useMutation({
    mutationFn: (input: RootAccountInput) => createRootAccount(input, locale),
    onSuccess: () => {
      queryClient.setQueryData(['setup-status', locale], { configured: true });
      setPassword('');
      setConfirmation('');
      setFormErrors({});
    },
  });
  const loginMutation = useMutation({
    mutationFn: (input: LoginInput) => login(input, locale),
    onSuccess: (session) => {
      queryClient.setQueryData(['session', locale], session);
      setPassword('');
      setFormErrors({});
    },
  });
  const logoutMutation = useMutation({
    mutationFn: () => logout(locale),
    onSuccess: () => queryClient.setQueryData(['session', locale], null),
  });

  const validateCredentials = (withConfirmation: boolean) => {
    const credentials = z.object({
      username: z
        .string()
        .trim()
        .min(3, { error: t('onboarding.usernameMin', { min: 3 }) })
        .max(64, { error: t('onboarding.usernameMax', { max: 64 }) })
        .regex(/^[a-z0-9._-]+$/, { error: t('onboarding.usernameFormat') }),
      password: z
        .string()
        .min(12, { error: t('onboarding.passwordRequirement', { min: 12 }) })
        .max(128, { error: t('onboarding.passwordMax', { max: 128 }) }),
    });

    if (!withConfirmation) {
      return credentials.safeParse({ username, password });
    }
    return credentials
      .extend({ confirmation: z.string() })
      .refine((input) => input.password === input.confirmation, {
        error: t('onboarding.passwordConfirmationMismatch'),
        path: ['confirmation'],
      })
      .safeParse({ username, password, confirmation });
  };

  const applyValidationErrors = (issues: readonly z.ZodIssue[]) => {
    const errorFor = (field: 'username' | 'password' | 'confirmation') =>
      issues.find((issue) => issue.path[0] === field)?.message;
    setFormErrors({
      username: errorFor('username'),
      password: errorFor('password'),
      confirmation: errorFor('confirmation'),
    });
  };

  const submitSetup = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormErrors({});
    const result = validateCredentials(true);
    if (!result.success) {
      applyValidationErrors(result.error.issues);
      return;
    }
    createAccount.mutate({
      username: result.data.username,
      password: result.data.password,
    });
  };

  const submitLogin = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormErrors({});
    const result = validateCredentials(false);
    if (!result.success) {
      applyValidationErrors(result.error.issues);
      return;
    }
    loginMutation.mutate(result.data);
  };

  const changeLocale = (nextLocale: Locale) => {
    setFormErrors({});
    void selectLocale(nextLocale);
  };

  if (setupQuery.data?.configured && sessionQuery.data) {
    return (
      <Workspace
        locale={locale}
        onChangeLocale={changeLocale}
        onSignOut={() => logoutMutation.mutate()}
        session={sessionQuery.data}
      />
    );
  }

  const isLogin = setupQuery.data?.configured === true;
  const mutation = isLogin ? loginMutation : createAccount;
  const errorMessage = publicErrorMessage(
    mutation.error,
    t('onboarding.unableToCreateAccount'),
  );

  return (
    <main className="relative box-border grid min-h-screen grid-cols-[minmax(0,1fr)_minmax(20rem,29rem)] items-center gap-[clamp(3rem,10vw,10rem)] px-[clamp(2rem,8vw,8rem)] pt-[clamp(6.5rem,10vw,8rem)] pb-[clamp(2rem,8vw,8rem)] text-[var(--foreground)] max-md:grid-cols-1 max-md:gap-12 max-md:px-5 max-md:py-8">
      <AppHeader locale={locale} onChangeLocale={changeLocale} />
      <section
        aria-label="Containly Cloud"
        className="box-border flex min-h-[31rem] max-w-[37rem] flex-col justify-end overflow-hidden rounded-2xl bg-center bg-cover bg-no-repeat bg-[linear-gradient(180deg,oklch(13%_0.04_264_/_32%),oklch(12%_0.04_264_/_96%)),url('../public/icon.png')] p-[clamp(2rem,5vw,4rem)] text-white max-md:min-h-[22rem]"
      >
        <h1 className="m-0 max-w-[10ch] text-[clamp(2.25rem,5vw,4rem)] leading-[1.08] tracking-[-0.045em] max-md:max-w-[14ch]">
          {t('onboarding.welcomeTitle')}
        </h1>
        <p className="mt-6 max-w-lg text-[1.05rem] leading-[1.65] text-[oklch(89%_0.018_255)]">
          {t('onboarding.welcomeDescription')}
        </p>
      </section>

      <section aria-live="polite" className="w-full">
        <Card className="border border-[var(--border)] shadow-[0_20px_50px_oklch(22%_0.035_264_/_10%)]">
          <Card.Header>
            <p className="mb-2.5 text-[0.78rem] font-[650] tracking-[0.08em] text-[var(--muted)] uppercase">
              {isLogin
                ? t('onboarding.signInSection')
                : t('onboarding.setupSection')}
            </p>
            <Card.Title>
              {isLogin
                ? t('onboarding.signInTitle')
                : t('onboarding.setupTitle')}
            </Card.Title>
            <Card.Description>
              {isLogin
                ? t('onboarding.signInDescription')
                : t('onboarding.setupDescription')}
            </Card.Description>
          </Card.Header>
          <Card.Content>
            {setupQuery.isPending || (isLogin && sessionQuery.isPending) ? (
              <p className="m-0 text-sm leading-6 text-[var(--muted)]">
                {t('onboarding.loadingSetup')}
              </p>
            ) : null}

            {setupQuery.isError || sessionQuery.isError ? (
              <Alert status="danger">
                <Alert.Content>
                  <Alert.Title>
                    {t('onboarding.requestFailedTitle')}
                  </Alert.Title>
                  <Alert.Description>
                    {t('onboarding.unableToConnect')}
                  </Alert.Description>
                </Alert.Content>
              </Alert>
            ) : null}

            {!setupQuery.isPending &&
            !setupQuery.isError &&
            (!isLogin || !sessionQuery.isPending) ? (
              <Form
                className="grid gap-4"
                onSubmit={isLogin ? submitLogin : submitSetup}
                validationBehavior="aria"
              >
                <TextField
                  isInvalid={Boolean(formErrors.username)}
                  isRequired
                  fullWidth
                >
                  <Label>{t('onboarding.usernameLabel')}</Label>
                  <Input
                    autoComplete="username"
                    maxLength={64}
                    name="username"
                    onChange={(event) => {
                      setUsername(event.target.value.toLowerCase());
                      setFormErrors((current) => ({
                        ...current,
                        username: undefined,
                      }));
                    }}
                    placeholder={t('onboarding.usernamePlaceholder')}
                    required
                    value={username}
                  />
                  <FieldError>{formErrors.username}</FieldError>
                </TextField>
                <TextField
                  isInvalid={Boolean(formErrors.password)}
                  isRequired
                  fullWidth
                >
                  <Label>{t('onboarding.passwordLabel')}</Label>
                  <Input
                    autoComplete={isLogin ? 'current-password' : 'new-password'}
                    name="password"
                    onChange={(event) => {
                      setPassword(event.target.value);
                      setFormErrors((current) => ({
                        ...current,
                        password: undefined,
                      }));
                    }}
                    required
                    type="password"
                    value={password}
                  />
                  <FieldError>{formErrors.password}</FieldError>
                </TextField>

                {!isLogin ? (
                  <TextField
                    isInvalid={Boolean(formErrors.confirmation)}
                    isRequired
                    fullWidth
                  >
                    <Label>{t('onboarding.passwordConfirmationLabel')}</Label>
                    <Input
                      autoComplete="new-password"
                      name="confirmation"
                      onChange={(event) => {
                        setConfirmation(event.target.value);
                        setFormErrors((current) => ({
                          ...current,
                          confirmation: undefined,
                        }));
                      }}
                      required
                      type="password"
                      value={confirmation}
                    />
                    <FieldError>{formErrors.confirmation}</FieldError>
                  </TextField>
                ) : null}

                {mutation.isError ? (
                  <Alert status="danger">
                    <Alert.Content>
                      <Alert.Description>{errorMessage}</Alert.Description>
                    </Alert.Content>
                  </Alert>
                ) : null}

                <Button fullWidth isPending={mutation.isPending} type="submit">
                  {isLogin
                    ? t('onboarding.signIn')
                    : t('onboarding.createAccount')}
                </Button>
              </Form>
            ) : null}
          </Card.Content>
        </Card>
      </section>
    </main>
  );
}

function AppHeader({
  locale,
  onChangeLocale,
}: {
  locale: Locale;
  onChangeLocale: (locale: Locale) => void;
}) {
  const { t } = useTranslation();
  return (
    <header className="absolute inset-x-0 top-0 flex min-h-[4.5rem] items-center justify-between bg-[oklch(13%_0.04_264)] px-[clamp(1.25rem,3vw,2rem)]">
      <img
        alt="Containly Cloud"
        className="h-auto w-[min(10.5rem,45vw)]"
        src="/logo.png"
      />
      <div className="flex items-center gap-2 text-[0.8125rem] text-[oklch(89%_0.018_255)]">
        <span id="language-label">{t('onboarding.languageLabel')}</span>
        <Select
          aria-labelledby="language-label"
          className="w-36"
          onSelectionChange={(key) => onChangeLocale(key as Locale)}
          selectedKey={locale}
        >
          <Select.Trigger className={languageSelectTriggerClassName}>
            <Select.Value />
            <Select.Indicator />
          </Select.Trigger>
          <Select.Popover className="border border-[oklch(45%_0.05_264)] bg-[oklch(18%_0.04_264)] text-[oklch(96%_0.01_255)]">
            <ListBox>
              <ListBox.Item className={languageOptionClassName} id="pt-BR">
                {t('onboarding.languagePortuguese')}
                <ListBox.ItemIndicator />
              </ListBox.Item>
              <ListBox.Item className={languageOptionClassName} id="en-US">
                {t('onboarding.languageEnglish')}
                <ListBox.ItemIndicator />
              </ListBox.Item>
            </ListBox>
          </Select.Popover>
        </Select>
      </div>
    </header>
  );
}

function Workspace({
  locale,
  onChangeLocale,
  onSignOut,
  session,
}: {
  locale: Locale;
  onChangeLocale: (locale: Locale) => void;
  onSignOut: () => void;
  session: Session;
}) {
  const { i18n, t } = useTranslation();
  const queryClient = useQueryClient();
  const [activeView, setActiveView] = useState<
    'system' | 'sessions' | 'backups' | 'users' | 'settings'
  >('system');
  const systemQuery = useQuery({
    queryKey: ['system-overview', locale],
    queryFn: () => getSystemOverview(locale),
    enabled: activeView === 'system',
    refetchInterval: 5_000,
    refetchIntervalInBackground: false,
  });
  const accountSessionsQuery = useQuery({
    queryKey: ['account-sessions', locale],
    queryFn: () => getActiveSessions(locale),
    enabled: activeView === 'sessions',
    refetchInterval: 15_000,
    refetchIntervalInBackground: false,
  });
  const backupsQuery = useQuery({
    queryKey: ['backups', locale],
    queryFn: () => getBackups(locale),
    enabled: activeView === 'backups',
  });
  const usersQuery = useQuery({
    queryKey: ['control-users', locale],
    queryFn: () => getControlUsers(locale),
    enabled: activeView === 'users',
  });
  const monitoringSettingsQuery = useQuery({
    queryKey: ['monitoring-settings', locale],
    queryFn: () => getMonitoringSettings(locale),
    enabled: activeView === 'settings',
  });
  const backupMutation = useMutation({
    mutationFn: () => createBackup(locale),
    onSuccess: () =>
      void queryClient.invalidateQueries({ queryKey: ['backups', locale] }),
  });
  const canReadBackups =
    session.user.permissions.includes('backups:read') ||
    session.user.permissions.includes('backups:manage');
  const canManageBackups = session.user.permissions.includes('backups:manage');
  const canManageUsers = session.user.permissions.includes('users:manage');
  const canManageSettings =
    session.user.permissions.includes('settings:manage');
  const canReadUserPermissions =
    canManageUsers ||
    session.user.permissions.includes('permissions:read') ||
    session.user.permissions.includes('permissions:manage');

  return (
    <main className="relative grid h-screen grid-cols-[15rem_minmax(0,1fr)] overflow-hidden pt-[4.5rem] text-[var(--foreground)] max-md:block max-md:h-auto max-md:overflow-visible">
      <AppHeader locale={locale} onChangeLocale={onChangeLocale} />
      <aside className="flex min-h-0 flex-col overflow-y-auto border-r border-[var(--border)] bg-[var(--surface)] p-4 max-md:block max-md:border-r-0 max-md:border-b max-md:px-5 max-md:py-3">
        <nav aria-label={t('workspace.navigationLabel')} className="grid gap-1">
          <Button
            aria-current={activeView === 'system' ? 'page' : undefined}
            className={`w-full justify-start px-3 py-2.5 text-left text-sm text-[var(--foreground)] ${activeView === 'system' ? 'bg-[var(--surface-secondary)]' : ''}`}
            onPress={() => setActiveView('system')}
            variant="ghost"
          >
            {t('system.title')}
          </Button>
          {canReadBackups ? (
            <Button
              className={`w-full justify-start px-3 py-2.5 text-left text-sm text-[var(--foreground)] ${activeView === 'backups' ? 'bg-[var(--surface-secondary)]' : ''}`}
              onPress={() => setActiveView('backups')}
              variant="ghost"
            >
              {t('admin.backups')}
            </Button>
          ) : null}
          {canReadUserPermissions ? (
            <Button
              className={`w-full justify-start px-3 py-2.5 text-left text-sm text-[var(--foreground)] ${activeView === 'users' ? 'bg-[var(--surface-secondary)]' : ''}`}
              onPress={() => setActiveView('users')}
              variant="ghost"
            >
              {t('admin.users')}
            </Button>
          ) : null}
          {canManageSettings ? (
            <Button
              className={`w-full justify-start px-3 py-2.5 text-left text-sm text-[var(--foreground)] ${activeView === 'settings' ? 'bg-[var(--surface-secondary)]' : ''}`}
              onPress={() => setActiveView('settings')}
              variant="ghost"
            >
              {t('admin.permissionScopes.settings')}
            </Button>
          ) : null}
        </nav>
        <AccountMenu
          activeView={activeView}
          onSelectSessions={() => setActiveView('sessions')}
          onSignOut={onSignOut}
          session={session}
        />
      </aside>
      <section className="min-w-0 overflow-y-auto p-[clamp(1.75rem,4vw,3.5rem)]">
        <p className="mb-2 text-xs font-semibold tracking-[0.08em] text-[var(--muted)] uppercase">
          Containly Cloud
        </p>
        {activeView === 'system' ? (
          <>
            <h1 className={pageTitleClassName}>{t('system.title')}</h1>
            <p className="mt-4 max-w-2xl text-[var(--muted)] leading-7">
              {t('system.description')}
            </p>
            {systemQuery.isPending ? (
              <p className="m-0 text-sm leading-6 text-[var(--muted)]">
                {t('system.loading')}
              </p>
            ) : null}
            {systemQuery.isError ? (
              <Alert status="danger">
                <Alert.Content>
                  <Alert.Title>{t('system.unavailableTitle')}</Alert.Title>
                  <Alert.Description>
                    {publicErrorMessage(
                      systemQuery.error,
                      t('system.unavailableDescription'),
                    )}
                  </Alert.Description>
                </Alert.Content>
              </Alert>
            ) : null}
            {systemQuery.data ? (
              <SystemOverviewPanel
                overview={systemQuery.data}
                locale={i18n.language}
              />
            ) : null}
          </>
        ) : activeView === 'sessions' ? (
          <AccountSessionsPanel
            locale={i18n.language}
            query={accountSessionsQuery}
          />
        ) : activeView === 'backups' ? (
          <BackupsPanel
            canManage={canManageBackups}
            locale={locale}
            onCreate={() => backupMutation.mutate()}
            query={backupsQuery}
          />
        ) : activeView === 'users' ? (
          <UsersPanel
            canManageUsers={canManageUsers}
            canManagePermissions={session.user.permissions.includes(
              'permissions:manage',
            )}
            locale={locale}
            query={usersQuery}
          />
        ) : (
          <SettingsPanel locale={locale} query={monitoringSettingsQuery} />
        )}
      </section>
    </main>
  );
}

function BackupsPanel({
  canManage,
  locale,
  onCreate,
  query,
}: {
  canManage: boolean;
  locale: Locale;
  onCreate: () => void;
  query: UseQueryResult<import('../api/setup').Backups>;
}) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [selected, setSelected] = useState<string[]>([]);
  const [deleteAcknowledgement, setDeleteAcknowledgement] = useState('');
  const [isDeleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const deleteAcknowledgementPhrase = t('admin.deleteAcknowledgementPhrase');
  const remove = useMutation({
    mutationFn: () =>
      Promise.all(selected.map((name) => deleteBackup(name, locale))),
    onSuccess: () => {
      setSelected([]);
      setDeleteAcknowledgement('');
      setDeleteDialogOpen(false);
      Toast.toast.success(t('admin.backupsDeleted'));
      void client.invalidateQueries({ queryKey: ['backups', locale] });
    },
    onError: () => Toast.toast.danger(t('admin.backupsDeletionFailed')),
  });
  return (
    <>
      <div className="flex items-center justify-between gap-4">
        <h1 className={pageTitleClassName}>{t('admin.backups')}</h1>
        {canManage ? (
          <div className="flex gap-2">
            <Button onPress={onCreate}>{t('admin.createBackup')}</Button>
            <AlertDialog
              isOpen={isDeleteDialogOpen}
              onOpenChange={(isOpen) => {
                setDeleteDialogOpen(isOpen);
                if (!isOpen) setDeleteAcknowledgement('');
              }}
            >
              <Button
                isDisabled={!selected.length}
                onPress={() => {
                  setDeleteAcknowledgement('');
                  setDeleteDialogOpen(true);
                }}
                variant="outline"
              >
                {t('admin.actions')}
              </Button>
              <AlertDialog.Backdrop>
                <AlertDialog.Container>
                  <AlertDialog.Dialog>
                    <AlertDialog.Header>
                      <AlertDialog.Heading>
                        {t('admin.deleteBackups')}
                      </AlertDialog.Heading>
                    </AlertDialog.Header>
                    <AlertDialog.Body>
                      <div className="grid gap-4">
                        <p className="m-0 font-medium text-danger">
                          {t('admin.deleteIrreversible')}
                        </p>
                        <p className="m-0">{t('admin.deleteConfirmation')}</p>
                        <TextField fullWidth>
                          <Label>
                            {t('admin.deleteAcknowledgement', {
                              phrase: deleteAcknowledgementPhrase,
                            })}
                          </Label>
                          <Input
                            onChange={(event) =>
                              setDeleteAcknowledgement(event.target.value)
                            }
                            placeholder={deleteAcknowledgementPhrase}
                            value={deleteAcknowledgement}
                          />
                        </TextField>
                      </div>
                    </AlertDialog.Body>
                    <AlertDialog.Footer className="flex-wrap">
                      <Button
                        className="min-w-28 whitespace-nowrap"
                        onPress={() => {
                          setDeleteAcknowledgement('');
                          setDeleteDialogOpen(false);
                        }}
                        slot="close"
                        variant="outline"
                      >
                        {t('admin.cancel')}
                      </Button>
                      <Button
                        className="min-w-36 whitespace-nowrap"
                        isDisabled={
                          deleteAcknowledgement.trim() !==
                          deleteAcknowledgementPhrase
                        }
                        isPending={remove.isPending}
                        onPress={() => remove.mutate()}
                        variant="danger"
                      >
                        {t('admin.confirmDelete')}
                      </Button>
                    </AlertDialog.Footer>
                  </AlertDialog.Dialog>
                </AlertDialog.Container>
              </AlertDialog.Backdrop>
            </AlertDialog>
          </div>
        ) : null}
      </div>
      <p className="mt-4 text-[var(--muted)]">
        {t('admin.backupsDescription')}
      </p>
      {query.data?.backups.length ? (
        <Table className="mt-5">
          <Table.ScrollContainer>
            <Table.Content aria-label={t('admin.backups')}>
              <Table.Header>
                <Table.Column isRowHeader>{t('admin.file')}</Table.Column>
                <Table.Column>{t('admin.size')}</Table.Column>
              </Table.Header>
              <Table.Body>
                {query.data.backups.map((backup) => (
                  <Table.Row key={backup.name}>
                    <Table.Cell>
                      <Checkbox
                        isSelected={selected.includes(backup.name)}
                        onChange={(isSelected) =>
                          setSelected((current) =>
                            isSelected
                              ? [...current, backup.name]
                              : current.filter((name) => name !== backup.name),
                          )
                        }
                      >
                        <Checkbox.Content>
                          <Checkbox.Control>
                            <Checkbox.Indicator />
                          </Checkbox.Control>
                          <Label>{backup.name}</Label>
                        </Checkbox.Content>
                      </Checkbox>
                    </Table.Cell>
                    <Table.Cell>
                      {formatBytes(backup.sizeBytes, locale)}
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table>
      ) : query.data ? (
        <div className="mt-5 flex min-h-48 items-center justify-center rounded-lg border border-[var(--border)] bg-[var(--surface)] px-6 py-10 text-center">
          <p className="m-0 text-sm text-[var(--muted)]">
            {t('admin.emptyBackups')}
          </p>
        </div>
      ) : (
        <p>{t('admin.loading')}</p>
      )}
    </>
  );
}

function SettingsPanel({
  locale,
  query,
}: {
  locale: Locale;
  query: UseQueryResult<MonitoringSettings>;
}) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const update = useMutation({
    mutationFn: (settings: MonitoringSettingsUpdate) =>
      updateMonitoringSettings(settings, locale),
    onSuccess: () => {
      Toast.toast.success(t('settings.saved'));
      void client.invalidateQueries({
        queryKey: ['monitoring-settings', locale],
      });
    },
    onError: () => Toast.toast.danger(t('settings.saveFailed')),
  });
  const clear = useMutation({
    mutationFn: () => clearMonitoringMetrics(locale),
    onSuccess: () => {
      Toast.toast.success(t('settings.metricsCleared'));
      void client.invalidateQueries({
        queryKey: ['monitoring-settings', locale],
      });
    },
    onError: () => Toast.toast.danger(t('settings.metricsClearFailed')),
  });

  return (
    <>
      <h1 className={pageTitleClassName}>{t('settings.title')}</h1>
      <p className="mt-4 max-w-2xl text-[var(--muted)] leading-7">
        {t('settings.description')}
      </p>
      {query.isPending ? <p>{t('admin.loading')}</p> : null}
      {query.isError ? (
        <Alert className="mt-5" status="danger">
          <Alert.Content>
            <Alert.Title>{t('settings.unavailableTitle')}</Alert.Title>
            <Alert.Description>
              {publicErrorMessage(
                query.error,
                t('settings.unavailableDescription'),
              )}
            </Alert.Description>
          </Alert.Content>
        </Alert>
      ) : null}
      {query.data ? (
        <MonitoringSettingsForm
          isPending={update.isPending}
          isClearing={clear.isPending}
          key={`${query.data.enabled}-${query.data.intervalSeconds}-${query.data.retentionDays}-${query.data.savedMetricsBytes}`}
          locale={locale}
          onSave={(settings) => update.mutate(settings)}
          onClear={() => clear.mutateAsync()}
          settings={query.data}
        />
      ) : null}
    </>
  );
}

function MonitoringSettingsForm({
  isClearing,
  isPending,
  locale,
  onClear,
  onSave,
  settings,
}: {
  isClearing: boolean;
  isPending: boolean;
  locale: Locale;
  onClear: () => Promise<unknown>;
  onSave: (settings: MonitoringSettingsUpdate) => void;
  settings: MonitoringSettings;
}) {
  const { t } = useTranslation();
  const [enabled, setEnabled] = useState(settings.enabled);
  const [intervalSeconds, setIntervalSeconds] = useState(
    String(settings.intervalSeconds),
  );
  const [retentionDays, setRetentionDays] = useState(
    String(settings.retentionDays),
  );
  const [isClearDialogOpen, setClearDialogOpen] = useState(false);
  const [clearIntent, setClearIntent] = useState<'disable' | 'manual' | null>(
    null,
  );
  const [clearConfirmation, setClearConfirmation] = useState('');
  const interval = Number(intervalSeconds);
  const retention = Number(retentionDays);
  const isValidInterval =
    Number.isInteger(interval) && interval >= 5 && interval <= 86400;
  const isValidRetention =
    Number.isInteger(retention) && retention >= 1 && retention <= 3650;
  const nextSettings = (): MonitoringSettingsUpdate => ({
    enabled,
    intervalSeconds: interval,
    retentionDays: retention,
  });
  const clearPhrase = t('settings.clearMetricsAcknowledgementPhrase');
  const canClear = clearConfirmation === clearPhrase;
  const openClearConfirmation = (intent: 'disable' | 'manual') => {
    setClearConfirmation('');
    setClearIntent(intent);
    setClearDialogOpen(true);
  };

  return (
    <Card className="mt-5 max-w-2xl">
      <Card.Header>
        <Card.Title>{t('settings.monitoringTitle')}</Card.Title>
        <Card.Description>
          {t('settings.monitoringDescription')}
        </Card.Description>
      </Card.Header>
      <Card.Content>
        <Form
          className="grid gap-5"
          onSubmit={(event) => {
            event.preventDefault();
            if (!isValidInterval || !isValidRetention) return;
            if (settings.enabled && !enabled) {
              openClearConfirmation('disable');
              return;
            }
            onSave(nextSettings());
          }}
        >
          <Checkbox isSelected={enabled} onChange={setEnabled}>
            <Checkbox.Content>
              <Checkbox.Control>
                <Checkbox.Indicator />
              </Checkbox.Control>
              <Label>{t('settings.monitoringEnabled')}</Label>
            </Checkbox.Content>
          </Checkbox>
          <TextField
            fullWidth
            isDisabled={!enabled}
            isInvalid={!isValidInterval}
          >
            <Label>{t('settings.intervalLabel')}</Label>
            <Input
              max={86400}
              min={5}
              onChange={(event) => setIntervalSeconds(event.target.value)}
              type="number"
              value={intervalSeconds}
            />
            <FieldError>{t('settings.intervalValidation')}</FieldError>
          </TextField>
          <TextField
            fullWidth
            isDisabled={!enabled}
            isInvalid={!isValidRetention}
          >
            <Label>{t('settings.retentionLabel')}</Label>
            <Input
              max={3650}
              min={1}
              onChange={(event) => setRetentionDays(event.target.value)}
              type="number"
              value={retentionDays}
            />
            <FieldError>{t('settings.retentionValidation')}</FieldError>
          </TextField>
          <p className="m-0 text-sm leading-6 text-[var(--muted)]">
            {t('settings.storageNotice')}
          </p>
          <Button
            isDisabled={!isValidInterval || !isValidRetention}
            isPending={isPending}
            type="submit"
          >
            {t('settings.save')}
          </Button>
          <Button
            isDisabled={settings.savedMetricsBytes === 0}
            isPending={isClearing}
            onPress={() => openClearConfirmation('manual')}
            variant="danger"
          >
            {t('settings.clearMetrics')}
          </Button>
        </Form>
      </Card.Content>
      <AlertDialog
        isOpen={isClearDialogOpen}
        onOpenChange={(isOpen) => {
          setClearDialogOpen(isOpen);
          if (!isOpen) {
            setClearIntent(null);
            setClearConfirmation('');
          }
        }}
      >
        <Button aria-hidden className="sr-only" />
        <AlertDialog.Backdrop>
          <AlertDialog.Container>
            <AlertDialog.Dialog>
              <AlertDialog.Header>
                <AlertDialog.Heading>
                  {t('settings.disableCollectionTitle')}
                </AlertDialog.Heading>
              </AlertDialog.Header>
              <AlertDialog.Body>
                <div className="grid gap-3">
                  {clearIntent === 'disable' ? (
                    <p className="m-0">
                      {t('settings.disableCollectionDescription')}
                    </p>
                  ) : null}
                  <p className="m-0 text-danger">
                    {t('settings.clearMetricsDescription')}
                  </p>
                  <p className="m-0 text-sm text-[var(--muted)]">
                    {t('settings.savedMetricsSize', {
                      size: formatBytes(settings.savedMetricsBytes, locale),
                    })}
                  </p>
                  <TextField fullWidth>
                    <Label>
                      {t('settings.clearMetricsAcknowledgement', {
                        phrase: clearPhrase,
                      })}
                    </Label>
                    <Input
                      onChange={(event) =>
                        setClearConfirmation(event.target.value)
                      }
                      placeholder={clearPhrase}
                      value={clearConfirmation}
                    />
                  </TextField>
                </div>
              </AlertDialog.Body>
              <AlertDialog.Footer className="flex-wrap">
                <Button slot="close" variant="outline">
                  {t('admin.cancel')}
                </Button>
                {clearIntent === 'disable' ? (
                  <Button
                    onPress={() => {
                      setClearDialogOpen(false);
                      onSave(nextSettings());
                    }}
                    variant="outline"
                  >
                    {t('settings.keepMetrics')}
                  </Button>
                ) : null}
                <Button
                  isDisabled={!canClear}
                  isPending={isClearing}
                  onPress={() => {
                    if (clearIntent === 'disable') {
                      setClearDialogOpen(false);
                      onSave({ ...nextSettings(), clearSavedMetrics: true });
                      return;
                    }
                    void onClear().then(
                      () => setClearDialogOpen(false),
                      () => undefined,
                    );
                  }}
                  variant="danger"
                >
                  {t('settings.clearMetrics')}
                </Button>
              </AlertDialog.Footer>
            </AlertDialog.Dialog>
          </AlertDialog.Container>
        </AlertDialog.Backdrop>
      </AlertDialog>
    </Card>
  );
}

function UsersPanel({
  canManageUsers,
  canManagePermissions,
  locale,
  query,
}: {
  canManageUsers: boolean;
  canManagePermissions: boolean;
  locale: Locale;
  query: UseQueryResult<import('../api/setup').ControlUsers>;
}) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [username, setUsername] = useState('');
  const [temporary, setTemporary] = useState('');
  const create = useMutation({
    mutationFn: () =>
      createControlUser({ username, permissions: ['workspace:read'] }, locale),
    onSuccess: (result) => {
      setTemporary(result.temporaryPassword);
      setUsername('');
      void client.invalidateQueries({ queryKey: ['control-users', locale] });
    },
  });
  const update = useMutation({
    mutationFn: (input: { id: number; permissions: string[] }) =>
      updateControlUserPermissions(input.id, input.permissions, locale),
    onSuccess: () => {
      Toast.toast.success(t('admin.permissionsSaved'));
      void client.invalidateQueries({ queryKey: ['control-users', locale] });
    },
    onError: () => Toast.toast.danger(t('admin.permissionsSaveFailed')),
  });
  const resetPassword = useMutation({
    mutationFn: (id: number) => resetControlUserPassword(id, locale),
    onSuccess: (result) => {
      setTemporary(result.temporaryPassword);
      Toast.toast.success(t('admin.passwordReset'));
      void client.invalidateQueries({ queryKey: ['control-users', locale] });
    },
    onError: () => Toast.toast.danger(t('admin.passwordResetFailed')),
  });
  const deleteUser = useMutation({
    mutationFn: (id: number) => deleteControlUser(id, locale),
    onSuccess: () => {
      Toast.toast.success(t('admin.userDeleted'));
      void client.invalidateQueries({ queryKey: ['control-users', locale] });
    },
    onError: () => Toast.toast.danger(t('admin.userDeletionFailed')),
  });
  return (
    <>
      <h1 className={pageTitleClassName}>{t('admin.users')}</h1>
      <p className="mt-4 text-[var(--muted)]">{t('admin.usersDescription')}</p>
      {canManageUsers ? (
        <Modal>
          <Button className="mt-5" onPress={() => undefined}>
            {t('admin.invite')}
          </Button>
          <Modal.Backdrop>
            <Modal.Container>
              <Modal.Dialog>
                <Modal.CloseTrigger />
                <Modal.Header>
                  <Modal.Heading>{t('admin.invite')}</Modal.Heading>
                </Modal.Header>
                <Modal.Body>
                  <TextField fullWidth>
                    <Label>{t('onboarding.usernameLabel')}</Label>
                    <Input
                      value={username}
                      onChange={(e) => setUsername(e.target.value)}
                      placeholder={t('onboarding.usernamePlaceholder')}
                    />
                  </TextField>
                </Modal.Body>
                <Modal.Footer>
                  <Button
                    isPending={create.isPending}
                    onPress={() => create.mutate()}
                  >
                    {t('admin.createUser')}
                  </Button>
                </Modal.Footer>
              </Modal.Dialog>
            </Modal.Container>
          </Modal.Backdrop>
        </Modal>
      ) : null}
      {temporary ? (
        <Alert className="mt-4" status="warning">
          <Alert.Content>
            <Alert.Title>{t('admin.temporaryPassword')}</Alert.Title>
            <Alert.Description>{temporary}</Alert.Description>
          </Alert.Content>
        </Alert>
      ) : null}
      {query.isError ? (
        <Alert className="mt-5" status="danger">
          <Alert.Content>
            <Alert.Title>{t('admin.usersUnavailableTitle')}</Alert.Title>
            <Alert.Description>
              {publicErrorMessage(
                query.error,
                t('admin.usersUnavailableDescription'),
              )}
            </Alert.Description>
          </Alert.Content>
        </Alert>
      ) : null}
      {query.data ? (
        <Table className="mt-5">
          <Table.ScrollContainer>
            <Table.Content aria-label={t('admin.users')}>
              <Table.Header>
                <Table.Column isRowHeader>
                  {t('onboarding.usernameLabel')}
                </Table.Column>
                <Table.Column>{t('admin.actions')}</Table.Column>
              </Table.Header>
              <Table.Body>
                {query.data.users.map((user) => (
                  <Table.Row key={user.id}>
                    <Table.Cell>{user.username}</Table.Cell>
                    <Table.Cell>
                      <UserActionsMenu
                        canManageUsers={canManageUsers}
                        canResetPassword={canManageUsers}
                        canManagePermissions={canManagePermissions}
                        isPending={update.isPending}
                        key={user.id}
                        onSave={(permissions) =>
                          update.mutateAsync({ id: user.id, permissions })
                        }
                        onResetPassword={() =>
                          resetPassword.mutateAsync(user.id)
                        }
                        onDeleteUser={() => deleteUser.mutateAsync(user.id)}
                        permissionScopes={query.data.permissionScopes}
                        user={user}
                      />
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table>
      ) : query.isPending ? (
        <p>{t('admin.loading')}</p>
      ) : null}
    </>
  );
}

function UserActionsMenu({
  canManageUsers,
  canResetPassword,
  canManagePermissions,
  isPending,
  onSave,
  onResetPassword,
  onDeleteUser,
  permissionScopes,
  user,
}: {
  canManageUsers: boolean;
  canResetPassword: boolean;
  canManagePermissions: boolean;
  isPending: boolean;
  onSave: (permissions: string[]) => Promise<void>;
  onResetPassword: () => Promise<unknown>;
  onDeleteUser: () => Promise<unknown>;
  permissionScopes: import('../api/setup').ControlUsers['permissionScopes'];
  user: import('../api/setup').ControlUsers['users'][number];
}) {
  const { t } = useTranslation();
  const [permissions, setPermissions] = useState(user.permissions);
  const [isPermissionsOpen, setPermissionsOpen] = useState(false);
  const [isDeleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [isDeletingUser, setIsDeletingUser] = useState(false);
  const isRoot = user.role === 'root';
  const canEdit = canManagePermissions && !isRoot;
  const hasChanges =
    permissions.length !== user.permissions.length ||
    permissions.some((permission) => !user.permissions.includes(permission));
  const scopeLabels: Record<string, string> = {
    backups: t('admin.permissionScopes.backups'),
    commands: t('admin.permissionScopes.commands'),
    permissions: t('admin.permissionScopes.permissions'),
    settings: t('admin.permissionScopes.settings'),
    users: t('admin.permissionScopes.users'),
    workspace: t('admin.permissionScopes.workspace'),
  };

  const setScopePermissions = (
    scope: (typeof permissionScopes)[number],
    nextPermissions: string[],
  ) => {
    const scopedPermissions = [...scope.read, ...scope.write];
    setPermissions((current) => [
      ...current.filter(
        (permission) => !scopedPermissions.includes(permission),
      ),
      ...nextPermissions,
    ]);
  };

  return (
    <>
      <Dropdown>
        <Dropdown.Trigger
          aria-label={t('admin.actions')}
          className="min-w-0 px-2 text-lg leading-none"
        >
          ⋮
        </Dropdown.Trigger>
        <Dropdown.Popover placement="bottom end">
          <Dropdown.Menu
            aria-label={t('admin.actions')}
            onAction={(key) => {
              if (key === 'permissions') {
                setPermissions(user.permissions);
                setPermissionsOpen(true);
              }
              if (key === 'reset-password') void onResetPassword();
              if (key === 'delete-user') setDeleteDialogOpen(true);
            }}
          >
            <Dropdown.Item id="permissions">
              {t('admin.permissions')}
            </Dropdown.Item>
            {canResetPassword && !isRoot ? (
              <Dropdown.Item id="reset-password">
                {t('admin.resetPassword')}
              </Dropdown.Item>
            ) : null}
            {canManageUsers && !isRoot ? (
              <Dropdown.Item id="delete-user" variant="danger">
                {t('admin.deleteUser')}
              </Dropdown.Item>
            ) : null}
          </Dropdown.Menu>
        </Dropdown.Popover>
      </Dropdown>
      <Modal
        isOpen={isPermissionsOpen}
        onOpenChange={(isOpen) => {
          setPermissionsOpen(isOpen);
          if (!isOpen) setPermissions(user.permissions);
        }}
      >
        <Button aria-hidden className="sr-only" />
        <Modal.Backdrop>
          <Modal.Container size="lg">
            <Modal.Dialog>
              <Modal.Header>
                <Modal.Heading>{user.username}</Modal.Heading>
              </Modal.Header>
              <Modal.Body>
                <div className="grid gap-4">
                  {isRoot ? (
                    <p className="m-0 text-sm text-[var(--muted)]">
                      {t('admin.protectedUser')}
                    </p>
                  ) : null}
                  {permissionScopes.map((scope) => {
                    const scopedPermissions = [...scope.read, ...scope.write];
                    const isReadOnly =
                      scope.read.length > 0 &&
                      scope.read.every((permission) =>
                        permissions.includes(permission),
                      ) &&
                      scope.write.every(
                        (permission) => !permissions.includes(permission),
                      );
                    const hasFullAccess = scopedPermissions.every(
                      (permission) => permissions.includes(permission),
                    );

                    return (
                      <section
                        className="grid gap-3 rounded-lg border border-[var(--border)] bg-[var(--surface-secondary)] p-4"
                        key={scope.scope}
                      >
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <h3 className="m-0 text-sm font-semibold">
                            {scopeLabels[scope.scope] ?? scope.scope}
                          </h3>
                          {canEdit ? (
                            <div className="flex flex-wrap gap-2">
                              {scope.read.length ? (
                                <Button
                                  onPress={() =>
                                    setScopePermissions(scope, scope.read)
                                  }
                                  size="sm"
                                  variant={isReadOnly ? 'primary' : 'outline'}
                                >
                                  {t('admin.readOnly')}
                                </Button>
                              ) : null}
                              {scope.write.length ? (
                                <Button
                                  onPress={() =>
                                    setScopePermissions(
                                      scope,
                                      scopedPermissions,
                                    )
                                  }
                                  size="sm"
                                  variant={
                                    hasFullAccess ? 'primary' : 'outline'
                                  }
                                >
                                  {scope.read.length
                                    ? t('admin.readAndWrite')
                                    : t('admin.write')}
                                </Button>
                              ) : null}
                            </div>
                          ) : null}
                        </div>
                        <div className="grid gap-2">
                          {scopedPermissions.map((permission) => (
                            <Checkbox
                              isDisabled={!canEdit}
                              isSelected={permissions.includes(permission)}
                              key={permission}
                              onChange={(selected) =>
                                setPermissions((current) =>
                                  selected
                                    ? [...current, permission]
                                    : current.filter(
                                        (value) => value !== permission,
                                      ),
                                )
                              }
                            >
                              <Checkbox.Content>
                                <Checkbox.Control>
                                  <Checkbox.Indicator />
                                </Checkbox.Control>
                                <Label>{permission}</Label>
                              </Checkbox.Content>
                            </Checkbox>
                          ))}
                        </div>
                      </section>
                    );
                  })}
                </div>
              </Modal.Body>
              <Modal.Footer className="flex-wrap">
                <Button slot="close" variant="outline">
                  {t('admin.cancel')}
                </Button>
                {canEdit ? (
                  <Button
                    isDisabled={!hasChanges}
                    isPending={isPending}
                    onPress={() => {
                      void onSave(permissions).then(
                        () => setPermissionsOpen(false),
                        () => undefined,
                      );
                    }}
                  >
                    {t('admin.savePermissions')}
                  </Button>
                ) : null}
              </Modal.Footer>
            </Modal.Dialog>
          </Modal.Container>
        </Modal.Backdrop>
      </Modal>
      <AlertDialog
        isOpen={isDeleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
      >
        <Button aria-hidden className="sr-only" />
        <AlertDialog.Backdrop>
          <AlertDialog.Container>
            <AlertDialog.Dialog>
              <AlertDialog.Header>
                <AlertDialog.Heading>
                  {t('admin.deleteUser')}
                </AlertDialog.Heading>
              </AlertDialog.Header>
              <AlertDialog.Body>
                {t('admin.deleteUserConfirmation')}
              </AlertDialog.Body>
              <AlertDialog.Footer className="flex-wrap">
                <Button slot="close" variant="outline">
                  {t('admin.cancel')}
                </Button>
                <Button
                  isPending={isDeletingUser}
                  onPress={() => {
                    setIsDeletingUser(true);
                    void onDeleteUser().then(
                      () => {
                        setIsDeletingUser(false);
                        setDeleteDialogOpen(false);
                      },
                      () => setIsDeletingUser(false),
                    );
                  }}
                  variant="danger"
                >
                  {t('admin.deleteUser')}
                </Button>
              </AlertDialog.Footer>
            </AlertDialog.Dialog>
          </AlertDialog.Container>
        </AlertDialog.Backdrop>
      </AlertDialog>
    </>
  );
}

function AccountMenu({
  activeView,
  onSelectSessions,
  onSignOut,
  session,
}: {
  activeView: 'system' | 'sessions' | 'backups' | 'users' | 'settings';
  onSelectSessions: () => void;
  onSignOut: () => void;
  session: Session;
}) {
  const { t } = useTranslation();

  return (
    <Dropdown>
      <Dropdown.Trigger className="mt-auto flex w-full items-center justify-between gap-2 px-3 py-2.5 text-left text-sm text-[var(--foreground)] max-md:mt-2">
        {session.user.username}
      </Dropdown.Trigger>
      <Dropdown.Popover placement="top end">
        <Dropdown.Menu
          aria-label={t('account.menuLabel')}
          onAction={(key) => {
            if (key === 'sessions') onSelectSessions();
            if (key === 'sign-out') onSignOut();
          }}
        >
          <Dropdown.Item
            aria-current={activeView === 'sessions' ? 'page' : undefined}
            id="sessions"
          >
            {t('account.connectedDevices')}
          </Dropdown.Item>
          <Dropdown.Item id="sign-out">{t('onboarding.signOut')}</Dropdown.Item>
        </Dropdown.Menu>
      </Dropdown.Popover>
    </Dropdown>
  );
}

function AccountSessionsPanel({
  locale,
  query,
}: {
  locale: string;
  query: UseQueryResult<ActiveSessions>;
}) {
  const { t } = useTranslation();
  const date = (value: string) => formatDate(value, locale);
  const sessions = query.data?.sessions ?? [];
  return (
    <>
      <h1 className={pageTitleClassName}>{t('account.connectedDevices')}</h1>
      <p className="mt-4 max-w-2xl text-[var(--muted)] leading-7">
        {t('account.connectedDevicesDescription')}
      </p>
      {query.isPending ? (
        <p className="m-0 text-sm leading-6 text-[var(--muted)]">
          {t('account.loadingSessions')}
        </p>
      ) : null}
      {query.isError ? (
        <Alert status="danger">
          <Alert.Content>
            <Alert.Title>{t('account.sessionsUnavailableTitle')}</Alert.Title>
            <Alert.Description>
              {publicErrorMessage(
                query.error,
                t('account.sessionsUnavailableDescription'),
              )}
            </Alert.Description>
          </Alert.Content>
        </Alert>
      ) : null}
      {query.data ? (
        <Card className="mt-5">
          <Card.Content className="grid gap-0">
            {sessions.length === 0 ? (
              <p className="m-0 text-sm leading-6 text-[var(--muted)]">
                {t('account.noSessions')}
              </p>
            ) : null}
            {sessions.map((activeSession) => (
              <div
                className="flex items-center justify-between gap-4 border-t border-[var(--border)] py-3 first:border-t-0 first:pt-0 max-sm:flex-col max-sm:items-start max-sm:gap-1.5"
                key={`${activeSession.createdAt}-${activeSession.ipAddress}`}
              >
                <div className="grid min-w-0 gap-1">
                  <strong>
                    {activeSession.userAgent || t('account.unknownDevice')}
                  </strong>
                  <span className="[overflow-wrap:anywhere] text-sm text-[var(--muted)]">
                    {activeSession.ipAddress || t('system.notAvailable')}
                  </span>
                </div>
                <span className="[overflow-wrap:anywhere] text-sm text-[var(--muted)]">
                  {t('account.lastActive', {
                    date: date(activeSession.lastSeenAt),
                  })}
                </span>
              </div>
            ))}
          </Card.Content>
        </Card>
      ) : null}
    </>
  );
}

function SystemOverviewPanel({
  locale,
  overview,
}: {
  locale: string;
  overview: SystemOverview;
}) {
  const { t } = useTranslation();
  const { system } = overview;
  const usage = (value: number) => `${value.toFixed(1)}%`;

  return (
    <div className="mt-5 grid gap-3" aria-live="polite">
      <div className="grid grid-cols-2 gap-3 max-md:grid-cols-1">
        <MetricCard
          label={t('system.cpu')}
          value={usage(system.cpu.usagePercent)}
          detail={t('system.cpuDetail', { count: system.cpu.cores })}
        />
        <MetricCard
          label={t('system.memory')}
          value={formatBytes(system.memory.usedBytes, locale)}
          detail={t('system.memoryDetail', {
            total: formatBytes(system.memory.totalBytes, locale),
          })}
        />
        <MetricCard
          label={t('system.storage')}
          value={formatBytes(system.storage.usedBytes, locale)}
          detail={t('system.storageDetail', {
            total: formatBytes(system.storage.totalBytes, locale),
          })}
        />
        <MetricCard
          label={t('system.controlPlaneStorage')}
          value={formatBytes(system.storage.controlPlaneUsedBytes, locale)}
          detail={t('system.controlPlaneStorageDetail')}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 max-md:grid-cols-1">
        <Card>
          <Card.Header className="gap-1 pb-2">
            <Card.Title>{t('system.machine')}</Card.Title>
            <Card.Description>
              {t('system.machineDescription')}
            </Card.Description>
          </Card.Header>
          <Card.Content className="pt-2">
            <DefinitionList
              items={[
                [t('system.hostname'), system.machine.hostname],
                [t('system.distribution'), system.machine.distribution],
                [
                  t('system.kernel'),
                  system.machine.kernel || t('system.notAvailable'),
                ],
                [t('system.architecture'), system.machine.architecture],
              ]}
            />
          </Card.Content>
        </Card>
        <Card>
          <Card.Header className="gap-1 pb-2">
            <Card.Title>{t('system.network')}</Card.Title>
            <Card.Description>
              {t('system.networkDescription')}
            </Card.Description>
          </Card.Header>
          <Card.Content className="pt-2">
            <DefinitionList
              items={[
                [
                  t('system.publicIp'),
                  system.network.publicIp || t('system.notAvailable'),
                ],
                [
                  t('system.networkInterfaces'),
                  system.network.interfaces.length > 0
                    ? system.network.interfaces
                        .map(
                          (network) =>
                            `${network.name}: ${network.addresses.join(', ') || t('system.notAvailable')}`,
                        )
                        .join('\n')
                    : t('system.notAvailable'),
                ],
              ]}
            />
          </Card.Content>
        </Card>
      </div>

      <p className="m-0 text-sm text-[var(--muted)]">
        {t('system.updatedAt', { date: formatDate(system.capturedAt, locale) })}
      </p>
    </div>
  );
}

function formatDate(value: string, locale: string) {
  return new Intl.DateTimeFormat(locale, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(value));
}

function MetricCard({
  label,
  value,
  detail,
}: {
  label: string;
  value: string;
  detail: string;
}) {
  return (
    <Card>
      <Card.Header className="gap-1">
        <Card.Description>{label}</Card.Description>
        <Card.Title className="text-[1.65rem] tracking-[-0.04em]">
          {value}
        </Card.Title>
      </Card.Header>
      <Card.Content className="grid gap-0">
        <p className="m-0 text-sm text-[var(--muted)]">{detail}</p>
      </Card.Content>
    </Card>
  );
}

function DefinitionList({ items }: { items: [string, string][] }) {
  return (
    <dl className="grid gap-2.5">
      {items.map(([label, value]) => (
        <div className="grid gap-0.5" key={label}>
          <dt className="text-xs text-[var(--muted)]">{label}</dt>
          <dd className="m-0 [overflow-wrap:anywhere] whitespace-pre-line leading-5">
            {value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function formatBytes(value: number, locale: string) {
  if (value === 0) return '0 B';
  const unit = Math.min(Math.floor(Math.log(value) / Math.log(1024)), 4);
  const labels = ['B', 'KB', 'MB', 'GB', 'TB'];
  return `${new Intl.NumberFormat(locale, { maximumFractionDigits: 1 }).format(value / 1024 ** unit)} ${labels[unit]}`;
}

function publicErrorMessage(error: unknown, fallback: string) {
  return error instanceof PublicAPIError && error.message
    ? error.message
    : fallback;
}
