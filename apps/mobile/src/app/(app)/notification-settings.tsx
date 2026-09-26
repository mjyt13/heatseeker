import Ionicons from '@expo/vector-icons/Ionicons';
import { Redirect, useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  useMutes,
  useNotificationPrefs,
  useNotificationSettings,
  useSaveNotificationSettings,
  useSetNotificationPrefs,
  useUnmute,
  type NotificationMute,
  type NotificationType,
} from '@heatseeker/core';
import {
  Button,
  ErrorText,
  H4,
  ListRow,
  LoadingScreen,
  Paragraph,
  Screen,
  ScreenTitle,
  Separator,
  SizableText,
  Switch,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { TimeField } from '@/components/schedule';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

/** Тихие часы по умолчанию, если участник их только что включил. */
const DEFAULT_QUIET_FROM = 23 * 60;
const DEFAULT_QUIET_TO = 8 * 60;

/** Настройки уведомлений: типы в группе, push, тихие часы, что приглушено. */
export default function NotificationSettingsScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId, subjectById } = useGroupContext();
  const prefs = useNotificationPrefs(groupId);
  const setPrefs = useSetNotificationPrefs(groupId ?? '');
  const settings = useNotificationSettings();
  const save = useSaveNotificationSettings();
  const mutes = useMutes(groupId);
  const unmute = useUnmute(groupId ?? '');

  if (!groupId) return <Redirect href="/(app)/groups" />;
  if (prefs.isPending || settings.isPending) return <LoadingScreen />;

  const current = settings.data ?? { push_enabled: true };
  const quietOn = current.quiet_from != null && current.quiet_to != null;
  const error = prefs.error ?? settings.error ?? setPrefs.error ?? save.error ?? unmute.error;

  const toggleType = (type: NotificationType, enabled: boolean) =>
    setPrefs.mutate([{ type, enabled }]);
  const saveQuiet = (from: number | null, to: number | null) =>
    save.mutate({
      push_enabled: current.push_enabled,
      quiet_from: from ?? undefined,
      quiet_to: to ?? undefined,
      urgent_in_quiet: current.urgent_in_quiet,
    });

  const muteLabel = (m: NotificationMute) => {
    switch (m.scope_type) {
      case 'GROUP':
        return t('notifications.muted_group');
      case 'SUBJECT':
        return subjectById.get(m.scope_id)?.name ?? t('notifications.muted_subject');
      case 'THREAD':
        return t('notifications.muted_thread');
      default:
        return t(`notification_types.${m.scope_id}` as 'notifications.muted_type', {
          defaultValue: t('notifications.muted_type'),
        });
    }
  };

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <Screen scroll>
        <XStack alignItems="center" gap="$2">
          <Button
            size="$3"
            chromeless
            icon={<Ionicons name="chevron-back" size={20} />}
            onPress={() => router.back()}
          />
          <ScreenTitle title={t('notifications.settings')} />
        </XStack>

        <YStack gap="$2">
          <H4>{t('notifications.types_title')}</H4>
          <Paragraph size="$2" color="$color10">
            {t('notifications.types_hint')}
          </Paragraph>
          {(prefs.data ?? []).map((p) => (
            <XStack key={p.type} alignItems="center" gap="$3" paddingVertical="$1.5">
              <SizableText flex={1} size="$4">
                {t(`notification_types.${p.type}`)}
              </SizableText>
              <Switch
                size="$2"
                checked={p.enabled}
                disabled={setPrefs.isPending}
                onCheckedChange={(v) => toggleType(p.type, v)}
              >
                <Switch.Thumb />
              </Switch>
            </XStack>
          ))}
        </YStack>

        <Separator />

        <YStack gap="$2">
          <H4>{t('notifications.delivery_title')}</H4>
          <XStack alignItems="center" gap="$3">
            <SizableText flex={1} size="$4">
              {t('notifications.push_enabled')}
            </SizableText>
            <Switch
              size="$2"
              checked={current.push_enabled}
              disabled={save.isPending}
              onCheckedChange={(v) =>
                save.mutate({
                  push_enabled: v,
                  quiet_from: current.quiet_from,
                  quiet_to: current.quiet_to,
                  urgent_in_quiet: current.urgent_in_quiet,
                })
              }
            >
              <Switch.Thumb />
            </Switch>
          </XStack>
          <Paragraph size="$2" color="$color10">
            {t('notifications.push_hint')}
          </Paragraph>

          <XStack alignItems="center" gap="$3">
            <SizableText flex={1} size="$4">
              {t('notifications.quiet_enabled')}
            </SizableText>
            <Switch
              size="$2"
              checked={quietOn}
              disabled={save.isPending}
              onCheckedChange={(v) =>
                saveQuiet(v ? DEFAULT_QUIET_FROM : null, v ? DEFAULT_QUIET_TO : null)
              }
            >
              <Switch.Thumb />
            </Switch>
          </XStack>
          {quietOn ? (
            <>
              <XStack gap="$3">
                <TimeField
                  label={t('notifications.quiet_from')}
                  value={minutesToText(current.quiet_from!)}
                  invalid={false}
                  onChange={(text) => {
                    const m = textToMinutes(text);
                    if (m != null) saveQuiet(m, current.quiet_to!);
                  }}
                />
                <TimeField
                  label={t('notifications.quiet_to')}
                  value={minutesToText(current.quiet_to!)}
                  invalid={false}
                  onChange={(text) => {
                    const m = textToMinutes(text);
                    if (m != null) saveQuiet(current.quiet_from!, m);
                  }}
                />
              </XStack>
              <Paragraph size="$2" color="$color10">
                {t('notifications.quiet_hint')}
              </Paragraph>
              <XStack alignItems="center" gap="$3">
                <YStack flex={1}>
                  <SizableText size="$4">{t('notifications.urgent_in_quiet')}</SizableText>
                  <SizableText size="$1" color="$color10">
                    {t('notifications.urgent_in_quiet_hint')}
                  </SizableText>
                </YStack>
                <Switch
                  size="$2"
                  checked={current.urgent_in_quiet ?? false}
                  disabled={save.isPending}
                  onCheckedChange={(v) =>
                    save.mutate({
                      push_enabled: current.push_enabled,
                      quiet_from: current.quiet_from,
                      quiet_to: current.quiet_to,
                      urgent_in_quiet: v,
                    })
                  }
                >
                  <Switch.Thumb />
                </Switch>
              </XStack>
            </>
          ) : null}
        </YStack>

        <Separator />

        <YStack gap="$2">
          <H4>{t('notifications.mutes_title')}</H4>
          {(mutes.data ?? []).length === 0 ? (
            <Paragraph size="$2" color="$color10">
              {t('notifications.mutes_empty')}
            </Paragraph>
          ) : (
            (mutes.data ?? []).map((m) => (
              <ListRow
                key={m.id}
                leading={<Ionicons name="notifications-off-outline" size={20} />}
                title={muteLabel(m)}
                subtitle={t('notifications.mute_until', {
                  when: new Date(m.until).toLocaleString(),
                })}
                trailing={
                  <Button
                    size="$2"
                    chromeless
                    disabled={unmute.isPending}
                    onPress={() => unmute.mutate({ scopeType: m.scope_type, scopeId: m.scope_id })}
                  >
                    {t('notifications.unmute')}
                  </Button>
                }
              />
            ))
          )}
        </YStack>

        <ErrorText>{error ? describeError(t, error) : null}</ErrorText>
      </Screen>
    </SafeAreaView>
  );
}

/** 1380 → «23:00». */
function minutesToText(minutes: number): string {
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`;
}

/** «23:00» → 1380; пока введено не всё время — null. */
function textToMinutes(text: string): number | null {
  const match = /^(\d{1,2}):(\d{2})$/.exec(text.trim());
  if (!match) return null;
  const h = Number(match[1]);
  const m = Number(match[2]);
  if (h > 23 || m > 59) return null;
  return h * 60 + m;
}
