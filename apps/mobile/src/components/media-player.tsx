import Ionicons from '@expo/vector-icons/Ionicons';
import { setAudioModeAsync, useAudioPlayer, useAudioPlayerStatus } from 'expo-audio';
import { useVideoPlayer, VideoView } from 'expo-video';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { GestureResponderEvent, LayoutChangeEvent } from 'react-native';

import type { MaterialOpen } from '@heatseeker/api-client';
import { useOpenMaterial } from '@heatseeker/core';
import { Button, ErrorText, Paragraph, Spinner, XStack, YStack } from '@heatseeker/ui';

import { describeError } from '@/lib/errors';

/** Ссылка обновляется заранее: плеер дочитывает файл Range-запросами по той же ссылке. */
const REFRESH_BEFORE_MS = 60_000;
const MIN_REFRESH_MS = 10_000;
const SKIP_SECONDS = 15;

export type PlayableKind = 'audio' | 'video';

/** Аудио и видео играются во встроенном плеере, остальное открывается как раньше. */
export function playableKind(mime: string): PlayableKind | null {
  if (mime.startsWith('audio/')) return 'audio';
  if (mime.startsWith('video/')) return 'video';
  return null;
}

/**
 * Встроенный плеер материала: получает ссылку (`/open`), заранее обновляет её
 * до истечения срока и продолжает с того же места.
 */
export function MediaPlayer({
  materialId,
  versionId,
  kind,
  hidden = false,
}: {
  materialId: string;
  versionId?: string;
  kind: PlayableKind;
  /** Спрятан и на паузе, но загруженное не теряется: повторный показ продолжает с того же места. */
  hidden?: boolean;
}) {
  const { t } = useTranslation();
  const open = useOpenMaterial();
  const [links, setLinks] = useState<MaterialOpen | null>(null);
  const { mutate } = open;

  // The parent remounts the player (key) for another version.
  useEffect(() => {
    mutate({ materialId, versionId }, { onSuccess: setLinks });
  }, [materialId, versionId, mutate]);

  const expiresAt = links?.expires_at;
  useEffect(() => {
    if (!expiresAt) return;
    const delay = Math.max(
      new Date(expiresAt).getTime() - Date.now() - REFRESH_BEFORE_MS,
      MIN_REFRESH_MS,
    );
    const timer = setTimeout(
      () => mutate({ materialId, versionId }, { onSuccess: setLinks }),
      delay,
    );
    return () => clearTimeout(timer);
  }, [expiresAt, materialId, versionId, mutate]);

  const uri = links?.download_url ?? links?.stream_url;
  if (!uri) {
    return open.isError ? (
      <ErrorText>{describeError(t, open.error)}</ErrorText>
    ) : (
      <Spinner alignSelf="flex-start" />
    );
  }
  return (
    <YStack display={hidden ? 'none' : 'flex'}>
      {kind === 'video' ? (
        <VideoPlayerView uri={uri} hidden={hidden} />
      ) : (
        <AudioPlayerView uri={uri} hidden={hidden} />
      )}
    </YStack>
  );
}

/** Меняет источник, не теряя позицию и состояние воспроизведения. */
function useSourceSwap(uri: string, swap: (uri: string) => void) {
  const current = useRef(uri);
  useEffect(() => {
    if (current.current === uri) return;
    current.current = uri;
    swap(uri);
  }, [uri, swap]);
}

function VideoPlayerView({ uri, hidden }: { uri: string; hidden: boolean }) {
  const [initial] = useState(uri);
  const player = useVideoPlayer({ uri: initial });
  useEffect(() => {
    if (hidden) player.pause();
  }, [hidden, player]);
  const swap = useCallback(
    (next: string) => {
      const position = player.currentTime;
      const wasPlaying = player.playing;
      void player.replaceAsync({ uri: next }).then(() => {
        player.currentTime = position;
        if (wasPlaying) player.play();
      });
    },
    [player],
  );
  useSourceSwap(uri, swap);

  return (
    <VideoView
      player={player}
      nativeControls
      fullscreenOptions={{ enable: true }}
      contentFit="contain"
      style={{ width: '100%', aspectRatio: 16 / 9, backgroundColor: 'black', borderRadius: 8 }}
    />
  );
}

function AudioPlayerView({ uri, hidden }: { uri: string; hidden: boolean }) {
  const { t } = useTranslation();
  const [initial] = useState(uri);
  const player = useAudioPlayer({ uri: initial });
  useEffect(() => {
    if (hidden) player.pause();
  }, [hidden, player]);
  const status = useAudioPlayerStatus(player);
  const [trackWidth, setTrackWidth] = useState(0);
  const swap = useCallback(
    (next: string) => {
      const position = player.currentTime;
      const wasPlaying = player.playing;
      player.replace({ uri: next });
      if (position > 0) void player.seekTo(position);
      if (wasPlaying) player.play();
    },
    [player],
  );
  useSourceSwap(uri, swap);

  useEffect(() => {
    // iOS: play even when the silent switch is on.
    setAudioModeAsync({ playsInSilentMode: true }).catch(() => undefined);
  }, []);

  const duration = status.duration > 0 ? status.duration : 0;
  const position = Math.min(status.currentTime, duration || status.currentTime);
  const progress = duration ? position / duration : 0;
  const seek = (seconds: number) =>
    void player.seekTo(Math.max(0, duration ? Math.min(seconds, duration) : seconds));
  const toggle = () => {
    if (status.playing) {
      player.pause();
      return;
    }
    if (status.didJustFinish || (duration && position >= duration)) void player.seekTo(0);
    player.play();
  };
  const onTrackPress = (e: GestureResponderEvent) => {
    if (trackWidth > 0 && duration) seek((e.nativeEvent.locationX / trackWidth) * duration);
  };

  return (
    <YStack gap="$2" padding="$3" borderRadius="$4" borderWidth={1} borderColor="$borderColor">
      <YStack
        height={24}
        justifyContent="center"
        onLayout={(e: LayoutChangeEvent) => setTrackWidth(e.nativeEvent.layout.width)}
        onPress={onTrackPress}
        role="slider"
        aria-valuemin={0}
        aria-valuemax={Math.round(duration)}
        aria-valuenow={Math.round(position)}
      >
        <YStack height={6} borderRadius={3} backgroundColor="$color5" overflow="hidden">
          <YStack height={6} width={`${progress * 100}%`} backgroundColor="$color9" />
        </YStack>
      </YStack>
      <XStack justifyContent="space-between">
        <Paragraph size="$2" color="$color10">
          {formatTime(position)}
        </Paragraph>
        <Paragraph size="$2" color="$color10">
          {status.isLoaded ? formatTime(duration) : t('player.loading')}
        </Paragraph>
      </XStack>
      <XStack justifyContent="center" alignItems="center" gap="$3">
        <Button
          size="$4"
          circular
          chromeless
          aria-label={t('player.rewind', { seconds: SKIP_SECONDS })}
          icon={<Ionicons name="play-back" size={22} />}
          onPress={() => seek(position - SKIP_SECONDS)}
        />
        <Button
          size="$5"
          circular
          theme="accent"
          disabled={!status.isLoaded}
          aria-label={status.playing ? t('player.pause') : t('player.play')}
          icon={
            status.isBuffering && status.playing ? (
              <Spinner />
            ) : (
              <Ionicons name={status.playing ? 'pause' : 'play'} size={26} />
            )
          }
          onPress={toggle}
        />
        <Button
          size="$4"
          circular
          chromeless
          aria-label={t('player.forward', { seconds: SKIP_SECONDS })}
          icon={<Ionicons name="play-forward" size={22} />}
          onPress={() => seek(position + SKIP_SECONDS)}
        />
      </XStack>
    </YStack>
  );
}

function formatTime(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds));
  const h = Math.floor(total / 3600);
  const m = Math.floor((total % 3600) / 60);
  const s = String(total % 60).padStart(2, '0');
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`;
}
