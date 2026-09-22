import Ionicons from '@expo/vector-icons/Ionicons';
import { Image } from 'expo-image';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { MaterialOpen } from '@heatseeker/api-client';
import { useOpenMaterial } from '@heatseeker/core';
import { Button, ErrorText, Paragraph, Spinner, XStack, YStack } from '@heatseeker/ui';

import { describeError } from '@/lib/errors';
import { showMaterial } from '@/lib/open';

/** Файл, у которого может быть уменьшенная копия. */
export interface PictureFile {
  /** id версии: по нему картинки лежат в кеше, ссылки при этом временные. */
  id: string;
  thumbnail_url?: string | null;
}

const thumbKey = (versionId: string) => `thumb:${versionId}`;
const fullKey = (versionId: string) => `picture:${versionId}`;

/**
 * Уменьшенная копия картинки: загружается сама и остаётся в кеше телефона,
 * поэтому видна сразу и без сети. Нажатие — полный размер.
 */
export function Thumbnail({
  file,
  height = 180,
  onPress,
}: {
  file: PictureFile;
  height?: number;
  onPress?: () => void;
}) {
  const [failed, setFailed] = useState(false);
  if (!file.thumbnail_url || failed) return null;
  return (
    <YStack
      borderRadius="$4"
      overflow="hidden"
      backgroundColor="$color3"
      role={onPress ? 'button' : undefined}
      onPress={onPress}
      pressStyle={{ opacity: 0.85 }}
    >
      <Image
        source={{ uri: file.thumbnail_url, cacheKey: thumbKey(file.id) }}
        cachePolicy="disk"
        contentFit="cover"
        transition={150}
        style={{ width: '100%', height }}
        onError={() => setFailed(true)}
      />
    </YStack>
  );
}

/**
 * Картинка во весь экран внутри приложения. Уже открытая показывается из
 * кеша (и без сети); иначе — по ссылке сервера, а пока грузится — уменьшенная
 * копия. «Открыть в браузере» — запасной путь.
 */
export function PictureViewer({
  materialId,
  file,
  onClose,
}: {
  materialId: string;
  file: PictureFile;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const open = useOpenMaterial();
  const [uri, setUri] = useState<string | null>(null);
  const [links, setLinks] = useState<MaterialOpen | null>(null);
  const [failed, setFailed] = useState(false);
  const { mutate } = open;

  useEffect(() => {
    let alive = true;
    void Image.getCachePathAsync(fullKey(file.id))
      .catch(() => null)
      .then((cached) => {
        if (!alive) return;
        if (cached) {
          setUri(cached.startsWith('file://') ? cached : `file://${cached}`);
          return;
        }
        mutate(
          { materialId, versionId: file.id },
          {
            onSuccess: (res) => {
              if (!alive) return;
              setLinks(res);
              setUri(res.download_url ?? res.stream_url ?? null);
            },
          },
        );
      });
    return () => {
      alive = false;
    };
  }, [materialId, file.id, mutate]);

  const toBrowser = () => {
    if (links) {
      void showMaterial(links);
      return;
    }
    open.mutate({ materialId, versionId: file.id }, { onSuccess: (res) => void showMaterial(res) });
  };

  return (
    <Modal visible animationType="fade" onRequestClose={onClose} statusBarTranslucent>
      <SafeAreaView style={{ flex: 1, backgroundColor: 'black' }}>
        <XStack justifyContent="space-between" alignItems="center" padding="$2">
          <Button
            size="$3"
            chromeless
            aria-label={t('viewer.close')}
            icon={<Ionicons name="close" size={24} color="white" />}
            onPress={onClose}
          />
          <Button
            size="$3"
            chromeless
            color="white"
            icon={<Ionicons name="open-outline" size={18} color="white" />}
            onPress={toBrowser}
          >
            {t('viewer.open_browser')}
          </Button>
        </XStack>
        <YStack flex={1} alignItems="center" justifyContent="center">
          {uri && !failed ? (
            <Image
              source={{ uri, cacheKey: fullKey(file.id) }}
              placeholder={
                file.thumbnail_url
                  ? { uri: file.thumbnail_url, cacheKey: thumbKey(file.id) }
                  : undefined
              }
              placeholderContentFit="contain"
              cachePolicy="disk"
              contentFit="contain"
              transition={200}
              style={{ width: '100%', height: '100%' }}
              onError={() => setFailed(true)}
            />
          ) : failed ? (
            <Paragraph color="white">{t('viewer.failed')}</Paragraph>
          ) : (
            <Spinner color="white" />
          )}
        </YStack>
        <ErrorText>{open.isError ? describeError(t, open.error) : null}</ErrorText>
      </SafeAreaView>
    </Modal>
  );
}
