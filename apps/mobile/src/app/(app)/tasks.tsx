import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import { EmptyState, Screen, ScreenTitle } from '@heatseeker/ui';

export default function TasksScreen() {
  const { t } = useTranslation();
  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <Screen>
        <ScreenTitle title={t('tabs.tasks')} />
        <EmptyState title={t('common.coming_soon')} hint={t('stages.tasks')} />
      </Screen>
    </SafeAreaView>
  );
}
