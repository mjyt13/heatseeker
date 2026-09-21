import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useTranslation } from 'react-i18next';

import { useDiscussions, type DiscussionTarget } from '@heatseeker/core';
import { Button } from '@heatseeker/ui';

import { discussionHref } from '@/lib/discussion';
import { useGroupContext } from '@/lib/group';

/** Кнопка «Обсуждение (N)» на карточке материала или задачи. */
export function DiscussionButton({ target, title }: { target: DiscussionTarget; title: string }) {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId } = useGroupContext();
  const discussions = useDiscussions(groupId);
  const found = discussions.data?.others?.find(
    (d) => d.target_type === target.type && d.target_id === target.id,
  );
  const count = found?.message_count ?? 0;
  const unread = found?.unread ?? 0;
  return (
    <Button
      icon={<Ionicons name={unread ? 'chatbubbles' : 'chatbubbles-outline'} size={18} />}
      onPress={() => router.push(discussionHref(target, title))}
    >
      {count ? t('discussions.open_count', { count }) : t('discussions.open')}
    </Button>
  );
}
