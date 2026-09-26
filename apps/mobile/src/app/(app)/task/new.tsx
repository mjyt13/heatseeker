import Ionicons from '@expo/vector-icons/Ionicons';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useCreateTask } from '@heatseeker/core';
import { Button, Screen, ScreenTitle, XStack } from '@heatseeker/ui';

import { TaskForm } from '@/components/task-form';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';
import { newId } from '@/lib/ids';

/** Новая задача. client_id делает повтор отправки безопасным. */
export default function NewTaskScreen() {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId } = useGroupContext();
  const create = useCreateTask(groupId ?? '');
  // A hidden tab stays mounted: every new task needs a fresh client_id and
  // an empty form, or the next one would be taken for a repeat of this one.
  const [clientId, setClientId] = useState(newId);
  const reset = () => {
    setClientId(newId());
    create.reset();
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
          <ScreenTitle title={t('tasks.add')} />
        </XStack>
        <TaskForm
          key={clientId}
          task={null}
          submitLabel={t('common.save')}
          pending={create.isPending}
          error={create.isError ? describeError(t, create.error) : null}
          onCancel={() => {
            reset();
            router.back();
          }}
          onSubmit={(body) =>
            create.mutate(
              { ...body, client_id: clientId },
              {
                // Open the new task: files are attached there.
                onSuccess: (task) => {
                  reset();
                  router.replace({ pathname: '/(app)/task/[id]', params: { id: task.id } });
                },
              },
            )
          }
        />
      </Screen>
    </SafeAreaView>
  );
}
