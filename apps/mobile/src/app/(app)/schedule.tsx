import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import { EmptyState, Screen, ScreenTitle } from '@heatseeker/ui';

export default function ScheduleScreen() {
  const { t } = useTranslation();
  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <Screen>
        <ScreenTitle title={t('tabs.schedule')} />
        <EmptyState title={t('common.coming_soon')} hint={t('stages.schedule')} />
      </Screen>
    </SafeAreaView>
  );
}
