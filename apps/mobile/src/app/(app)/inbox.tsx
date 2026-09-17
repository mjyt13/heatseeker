import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { Material } from '@heatseeker/api-client';
import {
  useClassifyMaterial,
  useMaterial,
  useMaterials,
  type MaterialKind,
} from '@heatseeker/core';
import {
  Button,
  EmptyState,
  ErrorText,
  LoadingScreen,
  Paragraph,
  Screen,
  ScreenTitle,
  Switch,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { KindPicker, MaterialRow, SectionLabel, SubjectPicker } from '@/components/materials';
import { describeError } from '@/lib/errors';
import { subjectLabel, useGroupContext } from '@/lib/group';

/** «Входящие»: файлы, которым модератор назначает предмет и тип. */
export default function InboxScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const ctx = useGroupContext();
  const inbox = useMaterials(ctx.groupId, { inbox: true }, 50);
  const [selected, setSelected] = useState<string | null>(null);

  const items = inbox.data?.pages.flatMap((p) => p.items ?? []) ?? [];

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
          <ScreenTitle title={t('inbox.title')} />
        </XStack>
        {inbox.isPending ? (
          <LoadingScreen />
        ) : items.length === 0 ? (
          <EmptyState title={t('inbox.empty')} />
        ) : (
          items.map((m) => (
            <YStack key={m.id} gap="$2">
              <MaterialRow
                material={m}
                subject={
                  m.classification.subject_id
                    ? ctx.subjectById.get(m.classification.subject_id)
                    : undefined
                }
                uploaderName={ctx.memberName(m.uploader_id)}
                onPress={() => setSelected(selected === m.id ? null : m.id)}
              />
              {selected === m.id ? (
                <ClassifyPanel material={m} onDone={() => setSelected(null)} />
              ) : null}
            </YStack>
          ))
        )}
        {inbox.hasNextPage ? (
          <Button disabled={inbox.isFetchingNextPage} onPress={() => void inbox.fetchNextPage()}>
            {t('common.load_more')}
          </Button>
        ) : null}
        <ErrorText>{inbox.isError ? describeError(t, inbox.error) : null}</ErrorText>
      </Screen>
    </SafeAreaView>
  );
}

function ClassifyPanel({ material: m, onDone }: { material: Material; onDone: () => void }) {
  const { t } = useTranslation();
  const router = useRouter();
  const ctx = useGroupContext();
  const details = useMaterial(m.id);
  const classify = useClassifyMaterial(ctx.groupId ?? '');
  const [subjectId, setSubjectId] = useState<string | null>(
    m.classification.subject_id ?? m.subject_id ?? null,
  );
  const [kind, setKind] = useState<MaterialKind>(m.kind);
  const [learn, setLearn] = useState(true);

  const folder = details.data?.drive_path?.at(-1);
  const suggested = m.classification.subject_id
    ? ctx.subjectById.get(m.classification.subject_id)
    : undefined;

  return (
    <YStack gap="$3" padding="$3" borderRadius="$4" borderWidth={1} borderColor="$borderColor">
      {m.review_reason ? (
        <Paragraph>{t(`materials.review_reason.${m.review_reason}`)}</Paragraph>
      ) : null}
      {suggested ? (
        <Paragraph color="$color10">
          {t('materials.suggested', { subject: subjectLabel(suggested) })}
        </Paragraph>
      ) : null}
      {details.data?.drive_path?.length ? (
        <Paragraph color="$color10">
          {t('materials.drive_path')}: {details.data.drive_path.join(' / ')}
        </Paragraph>
      ) : null}
      <SectionLabel>{t('inbox.choose_subject')}</SectionLabel>
      <SubjectPicker
        subjects={ctx.subjects}
        value={subjectId}
        onChange={setSubjectId}
        emptyLabel={t('materials.no_subject')}
      />
      <SectionLabel>{t('materials.kind')}</SectionLabel>
      <KindPicker value={kind} onChange={(k) => k && setKind(k)} />
      {folder && subjectId ? (
        <XStack alignItems="center" gap="$3">
          <Switch checked={learn} onCheckedChange={setLearn} size="$3">
            <Switch.Thumb />
          </Switch>
          <Paragraph flex={1}>{t('inbox.learn_alias', { folder })}</Paragraph>
        </XStack>
      ) : null}
      <ErrorText>{classify.isError ? describeError(t, classify.error) : null}</ErrorText>
      <XStack gap="$2">
        <Button
          flex={1}
          onPress={() => router.push({ pathname: '/(app)/material/[id]', params: { id: m.id } })}
        >
          {t('common.open')}
        </Button>
        <Button
          flex={1}
          theme="accent"
          disabled={classify.isPending}
          onPress={() =>
            classify.mutate(
              {
                materialId: m.id,
                subject_id: subjectId ?? undefined,
                kind,
                learn_alias: !!folder && learn,
              },
              { onSuccess: onDone },
            )
          }
        >
          {t('inbox.confirm')}
        </Button>
      </XStack>
    </YStack>
  );
}
