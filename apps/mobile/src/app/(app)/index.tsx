import Ionicons from '@expo/vector-icons/Ionicons';
import { Redirect, useRouter } from 'expo-router';
import { useDeferredValue, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { FlatList, RefreshControl } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  filterFromQuickTag,
  useInboxCount,
  useMaterials,
  useQuickTags,
  type FileType,
} from '@heatseeker/core';
import {
  Button,
  Chip,
  EmptyState,
  ErrorText,
  H3,
  Input,
  ListRow,
  LoadingScreen,
  ScrollView,
  Spinner,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { FileTypePicker, MaterialRow } from '@/components/materials';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';

/** Главный экран: чипы быстрых тегов, поиск и лента материалов. */
export default function FeedScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId, group, permissions, subjectById, memberName } = useGroupContext();
  const quickTags = useQuickTags(groupId);
  const [selected, setSelected] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [fileType, setFileType] = useState<FileType | null>(null);
  const deferredQuery = useDeferredValue(query);

  const chipFilter = filterFromQuickTag(selected);
  const filter = { ...chipFilter, q: deferredQuery, file_type: fileType ?? undefined };
  const materials = useMaterials(chipFilter ? groupId : null, filter);
  const moderator = permissions.can('material.moderate');
  const inbox = useInboxCount(groupId, moderator);

  if (!groupId) return <Redirect href="/(app)/groups" />;
  if (group.isPending) return <LoadingScreen />;

  const items = materials.data?.pages.flatMap((p) => p.items ?? []) ?? [];
  const inboxCount = inbox.data ?? 0;

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <YStack flex={1} backgroundColor="$background">
        <XStack
          alignItems="center"
          justifyContent="space-between"
          paddingHorizontal="$4"
          paddingVertical="$2"
        >
          <H3 numberOfLines={1} flex={1}>
            {group.data?.group.name ?? '…'}
          </H3>
          <Button
            size="$3"
            chromeless
            icon={<Ionicons name="swap-horizontal-outline" size={18} />}
            onPress={() => router.push('/(app)/groups')}
          >
            {t('groups.switch')}
          </Button>
        </XStack>

        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          flexGrow={0}
          flexShrink={0}
          keyboardShouldPersistTaps="handled"
        >
          <XStack gap="$2" paddingHorizontal="$4" paddingVertical="$2">
            {(quickTags.data ?? []).map((tag) => (
              <Chip
                key={tag.key}
                label={tag.kind === 'SYSTEM' ? t(tag.label) : tag.label}
                color={tag.color}
                selected={selected === tag.key}
                onPress={() => setSelected(selected === tag.key ? null : tag.key)}
              />
            ))}
          </XStack>
        </ScrollView>

        <XStack paddingHorizontal="$4" paddingBottom="$2">
          <Input
            flex={1}
            size="$3"
            value={query}
            onChangeText={setQuery}
            placeholder={t('materials.search_placeholder')}
            returnKeyType="search"
            clearButtonMode="while-editing"
            autoCorrect={false}
          />
        </XStack>
        <XStack paddingHorizontal="$4" paddingBottom="$2">
          <FileTypePicker value={fileType} onChange={setFileType} />
        </XStack>

        {moderator && inboxCount > 0 ? (
          <YStack paddingHorizontal="$2">
            <ListRow
              leading={<Ionicons name="file-tray-full-outline" size={22} />}
              title={t('inbox.banner', { count: inboxCount })}
              trailing={<Ionicons name="chevron-forward" size={18} />}
              onPress={() => router.push('/(app)/inbox')}
            />
          </YStack>
        ) : null}

        {!chipFilter ? (
          <EmptyState title={t('common.coming_soon')} hint={t('materials.chip_unsupported')} />
        ) : materials.isPending ? (
          <LoadingScreen />
        ) : (
          <FlatList
            data={items}
            keyExtractor={(m) => m.id}
            keyboardShouldPersistTaps="handled"
            keyboardDismissMode="on-drag"
            contentContainerStyle={{ paddingHorizontal: 8, paddingBottom: 96, flexGrow: 1 }}
            renderItem={({ item }) => (
              <MaterialRow
                material={item}
                subject={item.subject_id ? subjectById.get(item.subject_id) : undefined}
                uploaderName={memberName(item.uploader_id)}
                showReview={moderator}
                onPress={() =>
                  router.push({ pathname: '/(app)/material/[id]', params: { id: item.id } })
                }
              />
            )}
            onEndReachedThreshold={0.4}
            onEndReached={() => {
              if (materials.hasNextPage && !materials.isFetchingNextPage)
                void materials.fetchNextPage();
            }}
            refreshControl={
              <RefreshControl
                refreshing={materials.isRefetching && !materials.isFetchingNextPage}
                onRefresh={() => void materials.refetch()}
              />
            }
            ListFooterComponent={materials.isFetchingNextPage ? <Spinner margin="$4" /> : null}
            ListEmptyComponent={
              materials.isError ? (
                <YStack padding="$4" gap="$2">
                  <ErrorText>{describeError(t, materials.error)}</ErrorText>
                  <Button onPress={() => void materials.refetch()}>{t('common.retry')}</Button>
                </YStack>
              ) : (
                <EmptyState
                  title={t('common.empty')}
                  hint={
                    deferredQuery || selected || fileType
                      ? t('materials.empty_filtered')
                      : t('materials.empty_hint')
                  }
                />
              )
            }
          />
        )}

        {permissions.can('material.upload') ? (
          <YStack position="absolute" right="$4" bottom="$4">
            <Button
              theme="accent"
              size="$5"
              borderRadius="$10"
              icon={<Ionicons name="cloud-upload-outline" size={20} />}
              onPress={() =>
                router.push({
                  pathname: '/(app)/upload',
                  params: chipFilter?.subject_id ? { subject: chipFilter.subject_id } : {},
                })
              }
            >
              {t('materials.upload')}
            </Button>
          </YStack>
        ) : null}
      </YStack>
    </SafeAreaView>
  );
}
