import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { Material } from '@heatseeker/api-client';
import {
  toSubjectInput,
  useBulkClassify,
  useClassifyMaterial,
  useMaterial,
  useMaterials,
  useUpdateSubject,
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

/** Сервер разбирает не больше 200 материалов за раз. */
const MAX_BULK = 200;

interface Learned {
  alias: string;
  subjectId: string;
}

/** «Входящие»: файлы, которым модератор назначает предмет и тип — по одному или пачкой. */
export default function InboxScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const ctx = useGroupContext();
  const inbox = useMaterials(ctx.groupId, { inbox: true }, 50);
  const [opened, setOpened] = useState<string | null>(null);
  const [selecting, setSelecting] = useState(false);
  const [picked, setPicked] = useState<ReadonlySet<string>>(new Set());
  const [learned, setLearned] = useState<Learned | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const items = inbox.data?.pages.flatMap((p) => p.items ?? []) ?? [];

  const toggle = (id: string) =>
    setPicked((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else if (next.size < MAX_BULK) next.add(id);
      return next;
    });

  const stopSelecting = () => {
    setSelecting(false);
    setPicked(new Set());
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
          <YStack flex={1}>
            <ScreenTitle title={t('inbox.title')} />
          </YStack>
          {items.length > 0 ? (
            <Button
              size="$3"
              chromeless
              onPress={() => (selecting ? stopSelecting() : setSelecting(true))}
            >
              {selecting ? t('inbox.select_done') : t('inbox.select')}
            </Button>
          ) : null}
        </XStack>

        {learned ? (
          <LearnedBanner learned={learned} onClose={() => setLearned(null)} onUndone={setNotice} />
        ) : null}
        {notice ? <Paragraph color="$color10">{notice}</Paragraph> : null}

        {selecting ? (
          <BulkPanel
            ids={[...picked]}
            onSelectAll={() => setPicked(new Set(items.slice(0, MAX_BULK).map((m) => m.id)))}
            onDone={(count) => {
              setNotice(t('inbox.bulk_done', { count }));
              stopSelecting();
            }}
          />
        ) : null}

        {inbox.isPending ? (
          <LoadingScreen />
        ) : items.length === 0 ? (
          <EmptyState title={t('inbox.empty')} />
        ) : (
          items.map((m) => (
            <YStack key={m.id} gap="$2">
              <XStack alignItems="center">
                {selecting ? (
                  <Ionicons
                    name={picked.has(m.id) ? 'checkbox' : 'square-outline'}
                    size={22}
                    onPress={() => toggle(m.id)}
                  />
                ) : null}
                <YStack flex={1}>
                  <MaterialRow
                    material={m}
                    subject={
                      m.classification.subject_id
                        ? ctx.subjectById.get(m.classification.subject_id)
                        : undefined
                    }
                    uploaderName={ctx.memberName(m.uploader_id)}
                    onPress={() =>
                      selecting ? toggle(m.id) : setOpened(opened === m.id ? null : m.id)
                    }
                  />
                </YStack>
              </XStack>
              {!selecting && opened === m.id ? (
                <ClassifyPanel
                  material={m}
                  onDone={(result) => {
                    setOpened(null);
                    setNotice(null);
                    setLearned(result);
                  }}
                />
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

/** «Папка стала синонимом» с отменой (синоним убирается из предмета). */
function LearnedBanner({
  learned,
  onClose,
  onUndone,
}: {
  learned: Learned;
  onClose: () => void;
  onUndone: (message: string) => void;
}) {
  const { t } = useTranslation();
  const ctx = useGroupContext();
  const update = useUpdateSubject(ctx.groupId ?? '');
  const subject = ctx.subjectById.get(learned.subjectId);

  const undo = () => {
    if (!subject) return;
    const aliases = (subject.aliases ?? []).filter((a) => a !== learned.alias);
    update.mutate(
      { subjectId: subject.id, ...toSubjectInput(subject, { aliases }) },
      {
        onSuccess: () => {
          onUndone(t('inbox.undone', { alias: learned.alias }));
          onClose();
        },
      },
    );
  };

  return (
    <YStack gap="$2" padding="$3" borderRadius="$4" backgroundColor="$color3">
      <Paragraph>
        {t('inbox.learned', {
          alias: learned.alias,
          subject: subject ? subjectLabel(subject) : '',
        })}
      </Paragraph>
      <ErrorText>{update.isError ? describeError(t, update.error) : null}</ErrorText>
      <XStack gap="$2">
        <Button size="$3" disabled={!subject || update.isPending} onPress={undo}>
          {t('inbox.undo')}
        </Button>
        <Button size="$3" chromeless onPress={onClose}>
          {t('common.close')}
        </Button>
      </XStack>
    </YStack>
  );
}

/** Панель массового разбора: предмет и (необязательно) тип для выбранных. */
function BulkPanel({
  ids,
  onSelectAll,
  onDone,
}: {
  ids: string[];
  onSelectAll: () => void;
  onDone: (count: number) => void;
}) {
  const { t } = useTranslation();
  const ctx = useGroupContext();
  const bulk = useBulkClassify(ctx.groupId ?? '');
  const [subjectId, setSubjectId] = useState<string | null>(null);
  const [kind, setKind] = useState<MaterialKind | null>(null);

  return (
    <YStack gap="$3" padding="$3" borderRadius="$4" borderWidth={1} borderColor="$borderColor">
      <XStack alignItems="center" justifyContent="space-between" gap="$2" flexWrap="wrap">
        <Paragraph fontWeight="600">{t('inbox.selected', { count: ids.length })}</Paragraph>
        <Button size="$2" chromeless onPress={onSelectAll}>
          {t('inbox.select_all')}
        </Button>
      </XStack>
      <SectionLabel>{t('inbox.choose_subject')}</SectionLabel>
      <SubjectPicker
        subjects={ctx.subjects}
        value={subjectId}
        onChange={setSubjectId}
        emptyLabel={t('materials.no_subject')}
      />
      <SectionLabel>{t('materials.kind')}</SectionLabel>
      <KindPicker value={kind} onChange={setKind} emptyLabel={t('inbox.bulk_kind_keep')} />
      <ErrorText>{bulk.isError ? describeError(t, bulk.error) : null}</ErrorText>
      <Button
        theme="accent"
        disabled={ids.length === 0 || bulk.isPending}
        onPress={() =>
          bulk.mutate(
            { material_ids: ids, subject_id: subjectId ?? undefined, kind: kind ?? undefined },
            { onSuccess: onDone },
          )
        }
      >
        {t('inbox.bulk_apply')}
      </Button>
    </YStack>
  );
}

function ClassifyPanel({
  material: m,
  onDone,
}: {
  material: Material;
  onDone: (learned: Learned | null) => void;
}) {
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
              {
                onSuccess: (res) =>
                  onDone(
                    res.learned_alias && subjectId ? { alias: res.learned_alias, subjectId } : null,
                  ),
              },
            )
          }
        >
          {t('inbox.confirm')}
        </Button>
      </XStack>
    </YStack>
  );
}
