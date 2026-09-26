import Ionicons from '@expo/vector-icons/Ionicons';
import { useLocalSearchParams, useRouter } from 'expo-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SafeAreaView } from 'react-native-safe-area-context';

import {
  addDays,
  addMinutes,
  atTime,
  dayKey,
  formatTime,
  parseDayKey,
  parseTime,
  timeOf,
  toCalendarDay,
  useCreateScheduleEvent,
  useOccurrence,
  useResetOccurrence,
  useScheduleEvent,
  useSetOccurrence,
  useUpdateScheduleEvent,
  weekdayOf,
  type CalendarDay,
  type Occurrence,
  type ScheduleEvent,
  type ScheduleEventInput,
  type ScheduleKind,
  type TimeOfDay,
  type Weekday,
} from '@heatseeker/core';
import {
  Button,
  Chip,
  EmptyState,
  ErrorText,
  Field,
  LoadingScreen,
  Paragraph,
  Screen,
  ScreenTitle,
  TextArea,
  XStack,
  YStack,
} from '@heatseeker/ui';

import { CalendarMonth } from '@/components/calendar-month';
import { SectionLabel, SubjectPicker } from '@/components/materials';
import { formatDayKey, ScheduleKindPicker, TimeField, WeekdayPicker } from '@/components/schedule';
import { describeError } from '@/lib/errors';
import { useGroupContext } from '@/lib/group';
import { newId } from '@/lib/ids';
import { deviceTimeZone } from '@/lib/timezone';

type Params = {
  /** Серия для правки; нет — новое занятие. */
  eventId?: string;
  /** День: новое занятие — выбранный; FOLLOWING — первый меняемый; one — занятие. */
  date?: string;
  scope?: 'ALL' | 'FOLLOWING';
  /** one — только это занятие. */
  mode?: 'one';
  /** Метка открытия: новая форма при каждом переходе (вкладка не размонтируется). */
  n?: string;
};

/** Длительности для быстрого выбора, минуты. */
const DURATIONS = [40, 45, 90, 180];

/** Добавить занятие, изменить серию (всю или с даты) или одно занятие. */
export default function ClassEditScreen() {
  const params = useLocalSearchParams<Params>();
  // A hidden tab stays mounted: every opening gets a fresh form.
  return <ClassEdit key={JSON.stringify(params)} {...params} />;
}

function ClassEdit({ eventId, date, scope, mode }: Params) {
  const { t } = useTranslation();
  const router = useRouter();
  const event = useScheduleEvent(eventId);
  const occurrence = useOccurrence(mode === 'one' ? eventId : null, date);

  const header = (title: string) => (
    <XStack alignItems="center" gap="$2">
      <Button
        size="$3"
        chromeless
        icon={<Ionicons name="chevron-back" size={20} />}
        onPress={() => router.back()}
      />
      <ScreenTitle title={title} />
    </XStack>
  );

  if ((eventId && event.isPending) || (mode === 'one' && occurrence.isPending)) {
    return <LoadingScreen />;
  }
  const failed = eventId ? (event.error ?? occurrence.error) : null;
  if (eventId && (!event.data || (mode === 'one' && !occurrence.data))) {
    return (
      <SafeAreaView style={{ flex: 1 }} edges={['top']}>
        <Screen>
          <EmptyState title={failed ? describeError(t, failed) : t('errors.not_found')} />
          <Button onPress={() => router.back()}>{t('common.back')}</Button>
        </Screen>
      </SafeAreaView>
    );
  }

  const day = date ? formatDayKey(t, date) : '';
  const title =
    mode === 'one'
      ? t('schedule.form_edit_single', { date: day })
      : !eventId
        ? t('schedule.form_new')
        : scope === 'FOLLOWING'
          ? t('schedule.form_edit_following', { date: day })
          : t('schedule.form_edit');

  return (
    <SafeAreaView style={{ flex: 1 }} edges={['top']}>
      <Screen scroll>
        {header(title)}
        {mode === 'one' ? (
          <OccurrenceForm event={event.data!} occurrence={occurrence.data!} />
        ) : (
          <SeriesForm event={event.data ?? null} date={date} scope={scope ?? 'ALL'} />
        )}
      </Screen>
    </SafeAreaView>
  );
}

/** Время из поля ввода и ошибка, если введено что-то непонятное. */
function useTimeField(initial: TimeOfDay | null) {
  const [text, setText] = useState(initial ? formatTime(initial) : '');
  const time = parseTime(text);
  return { text, setText, time, invalid: text.trim() !== '' && !time };
}

/** Начало и конец с быстрыми длительностями. */
function TimeRange({
  start,
  end,
  tried,
}: {
  start: ReturnType<typeof useTimeField>;
  end: ReturnType<typeof useTimeField>;
  /** Уже пытались сохранить: пустые и неверные поля подсвечиваются. */
  tried: boolean;
}) {
  const { t } = useTranslation();
  const minutes = (x: TimeOfDay) => x.hour * 60 + x.minute;
  const endBeforeStart = !!start.time && !!end.time && minutes(end.time) <= minutes(start.time);
  const startMissing = tried && !start.time;
  const endMissing = tried && (!end.time || endBeforeStart);
  return (
    <YStack gap="$2">
      <XStack gap="$3">
        <TimeField
          label={t('schedule.form_start')}
          value={start.text}
          invalid={start.invalid || startMissing}
          onChange={(text) => {
            // Keep the length of the class when its start moves.
            const next = parseTime(text);
            if (next && start.time && end.time) {
              const length =
                end.time.hour * 60 + end.time.minute - (start.time.hour * 60 + start.time.minute);
              if (length > 0) end.setText(formatTime(addMinutes(next, length)));
            }
            start.setText(text);
          }}
        />
        <TimeField
          label={t('schedule.form_end')}
          value={end.text}
          invalid={end.invalid || endMissing}
          onChange={end.setText}
        />
      </XStack>
      {start.time ? (
        <XStack gap="$2" flexWrap="wrap">
          {DURATIONS.map((minutes) => (
            <Chip
              key={minutes}
              label={t('schedule.duration', { minutes })}
              selected={
                !!end.time && formatTime(addMinutes(start.time!, minutes)) === formatTime(end.time)
              }
              onPress={() => end.setText(formatTime(addMinutes(start.time!, minutes)))}
            />
          ))}
        </XStack>
      ) : null}
      {startMissing || endMissing ? (
        <Paragraph size="$2" color="$red10">
          {t('schedule.form_time_required')}
        </Paragraph>
      ) : null}
    </YStack>
  );
}

/** Проверенные начало и конец в пределах одного дня, ISO; null — ошибка. */
function timesOn(day: CalendarDay, start: TimeOfDay | null, end: TimeOfDay | null) {
  if (!start || !end) return null;
  const from = atTime(day, start);
  const to = atTime(day, end);
  return to > from ? { starts_at: from, ends_at: to } : null;
}

/** Новое занятие или правка серии (вся / с даты). */
function SeriesForm({
  event,
  date,
  scope,
}: {
  event: ScheduleEvent | null;
  date?: string;
  scope: 'ALL' | 'FOLLOWING';
}) {
  const { t } = useTranslation();
  const router = useRouter();
  const ctx = useGroupContext();
  const create = useCreateScheduleEvent(ctx.groupId ?? '');
  const update = useUpdateScheduleEvent(ctx.groupId ?? '');
  const [clientId] = useState(newId);
  const following = !!event && scope === 'FOLLOWING';

  const firstDay = event ? toCalendarDay(new Date(event.starts_at)) : null;
  const [day, setDay] = useState<CalendarDay>(
    () =>
      (following && date ? parseDayKey(date) : null) ??
      firstDay ??
      (date ? parseDayKey(date) : null) ??
      toCalendarDay(new Date()),
  );
  const [subjectId, setSubjectId] = useState<string | null>(event?.subject_id ?? null);
  const [title, setTitle] = useState(event?.title ?? '');
  const [kind, setKind] = useState<ScheduleKind>(event?.kind ?? 'LECTURE');
  const start = useTimeField(event ? timeOf(event.starts_at) : null);
  const end = useTimeField(event ? timeOf(event.ends_at) : null);
  const [repeatWeeks, setRepeatWeeks] = useState<0 | 1 | 2>(
    (event?.repeat?.interval_weeks as 1 | 2 | undefined) ?? 0,
  );
  const [weekdays, setWeekdays] = useState<Weekday[]>(event?.repeat?.weekdays ?? []);
  const [until, setUntil] = useState<CalendarDay | null>(
    event?.until ? parseDayKey(event.until) : null,
  );
  const [pickUntil, setPickUntil] = useState(false);
  const [location, setLocation] = useState(event?.location ?? '');
  const [teacher, setTeacher] = useState(event?.teacher ?? '');
  const [note, setNote] = useState(event?.note ?? '');
  const [problem, setProblem] = useState<string | null>(null);
  const [tried, setTried] = useState(false);

  const mutation = event ? update : create;
  // The first class's day is always in the series: the chips show it with the
  // chosen days, and dropping it moves the first class to the next chosen day.
  const firstWeekday = weekdayOf(day);
  const repeatDays: Weekday[] = weekdays.includes(firstWeekday)
    ? weekdays
    : [...weekdays, firstWeekday];
  const toggleWeekday = (d: Weekday) => {
    if (!repeatDays.includes(d)) {
      setWeekdays([...repeatDays, d]);
      return;
    }
    const rest = repeatDays.filter((x) => x !== d);
    if (rest.length === 0) return;
    setWeekdays(rest);
    if (d === firstWeekday) {
      for (let i = 1; i <= 7; i++) {
        const next = addDays(day, i);
        if (rest.includes(weekdayOf(next))) {
          setDay(next);
          break;
        }
      }
    }
  };
  const titleMissing = tried && !title.trim() && !subjectId;

  const submit = () => {
    setProblem(null);
    setTried(true);
    if (!title.trim() && !subjectId) {
      setProblem(t('schedule.form_title_required'));
      return;
    }
    const times = timesOn(day, start.time, end.time);
    if (!times) {
      setProblem(t('schedule.form_time_required'));
      return;
    }
    const body: ScheduleEventInput = {
      ...times,
      title: title.trim() || undefined,
      subject_id: subjectId ?? undefined,
      kind,
      // A series keeps the zone it was planned in.
      timezone: event?.timezone ?? deviceTimeZone(),
      location: location.trim() || undefined,
      teacher: teacher.trim() || undefined,
      note: note.trim() || undefined,
      repeat: repeatWeeks ? { interval_weeks: repeatWeeks, weekdays: repeatDays } : undefined,
      until: repeatWeeks && until ? dayKey(until) : undefined,
    };
    const done = () => router.navigate('/(app)/schedule');
    if (!event) {
      create.mutate({ ...body, client_id: clientId }, { onSuccess: done });
      return;
    }
    update.mutate(
      {
        ...body,
        eventId: event.id,
        version: event.version,
        scope,
        ...(following && date ? { from: date } : {}),
      },
      { onSuccess: done },
    );
  };

  return (
    <YStack gap="$3">
      {following ? (
        <Paragraph color="$color10">{t('schedule.form_following_hint')}</Paragraph>
      ) : null}
      <YStack gap="$1.5">
        <SectionLabel>{t('schedule.form_subject')}</SectionLabel>
        <SubjectPicker
          subjects={ctx.subjects}
          value={subjectId}
          onChange={setSubjectId}
          emptyLabel={t('schedule.no_subject')}
        />
      </YStack>
      <Field
        id="class-title"
        label={t('schedule.form_title')}
        error={titleMissing ? t('schedule.form_title_required') : undefined}
        value={title}
        onChangeText={setTitle}
        placeholder={t('schedule.form_title_placeholder')}
        maxLength={200}
      />
      <YStack gap="$1.5">
        <SectionLabel>{t('schedule.form_kind')}</SectionLabel>
        <ScheduleKindPicker value={kind} onChange={setKind} />
      </YStack>

      <YStack gap="$1.5">
        <SectionLabel>
          {repeatWeeks ? t('schedule.form_first_date') : t('schedule.form_date')}
        </SectionLabel>
        {following ? (
          <Paragraph fontWeight="600">{formatDayKey(t, dayKey(day))}</Paragraph>
        ) : (
          <CalendarMonth selected={day} onPick={setDay} />
        )}
      </YStack>
      <TimeRange start={start} end={end} tried={tried} />

      <YStack gap="$1.5">
        <SectionLabel>{t('schedule.form_repeat')}</SectionLabel>
        <XStack gap="$2" flexWrap="wrap">
          <Chip
            label={t('schedule.form_repeat_none')}
            selected={repeatWeeks === 0}
            onPress={() => setRepeatWeeks(0)}
          />
          <Chip
            label={t('schedule.repeat_weekly')}
            selected={repeatWeeks === 1}
            onPress={() => setRepeatWeeks(1)}
          />
          <Chip
            label={t('schedule.repeat_biweekly')}
            selected={repeatWeeks === 2}
            onPress={() => setRepeatWeeks(2)}
          />
        </XStack>
      </YStack>
      {repeatWeeks ? (
        <>
          <YStack gap="$1.5">
            <SectionLabel>{t('schedule.form_weekdays')}</SectionLabel>
            <WeekdayPicker value={repeatDays} onToggle={toggleWeekday} />
            <Paragraph size="$2" color="$color10">
              {t('schedule.form_weekdays_hint')}
            </Paragraph>
          </YStack>
          <YStack gap="$1.5">
            <SectionLabel>{t('schedule.form_until')}</SectionLabel>
            <XStack gap="$2" flexWrap="wrap">
              <Chip
                label={t('schedule.form_no_end')}
                selected={!until}
                onPress={() => {
                  setUntil(null);
                  setPickUntil(false);
                }}
              />
              <Chip
                label={until ? formatDayKey(t, dayKey(until)) : t('schedule.form_pick_until')}
                selected={!!until}
                onPress={() => setPickUntil((v) => !v)}
              />
            </XStack>
            {pickUntil ? (
              <CalendarMonth
                selected={until}
                onPick={(d) => {
                  setUntil(d);
                  setPickUntil(false);
                }}
              />
            ) : null}
          </YStack>
        </>
      ) : null}

      <Field
        id="class-location"
        label={t('schedule.location')}
        value={location}
        onChangeText={setLocation}
        maxLength={200}
      />
      <Field
        id="class-teacher"
        label={t('schedule.teacher')}
        value={teacher}
        onChangeText={setTeacher}
        maxLength={200}
      />
      <YStack gap="$1.5">
        <SectionLabel>{t('schedule.note')}</SectionLabel>
        <TextArea value={note} onChangeText={setNote} numberOfLines={3} maxLength={2000} />
      </YStack>

      <ErrorText>
        {problem ?? (mutation.isError ? describeError(t, mutation.error) : null)}
      </ErrorText>
      <XStack gap="$2">
        <Button flex={1} onPress={() => router.back()}>
          {t('common.cancel')}
        </Button>
        <Button flex={1} theme="accent" disabled={mutation.isPending} onPress={submit}>
          {t('common.save')}
        </Button>
      </XStack>
    </YStack>
  );
}

/**
 * Одно занятие: другой день и время, аудитория, преподаватель, причина.
 * Изменение хранится относительно плана серии: совпавшее с планом не
 * отправляется, пустая правка возвращает занятие как по плану.
 */
function OccurrenceForm({
  event,
  occurrence: o,
}: {
  event: ScheduleEvent;
  occurrence: Occurrence;
}) {
  const { t } = useTranslation();
  const router = useRouter();
  const { groupId } = useGroupContext();
  const change = useSetOccurrence(groupId ?? '');
  const reset = useResetOccurrence(groupId ?? '');
  const [day, setDay] = useState<CalendarDay>(() => toCalendarDay(new Date(o.starts_at)));
  const start = useTimeField(timeOf(o.starts_at));
  const end = useTimeField(timeOf(o.ends_at));
  const [location, setLocation] = useState(o.location);
  const [teacher, setTeacher] = useState(o.teacher);
  // A cancellation's reason is not a reason to change the class.
  const [note, setNote] = useState(o.status === 'CHANGED' ? (o.change_note ?? '') : '');
  const [problem, setProblem] = useState<string | null>(null);
  const [tried, setTried] = useState(false);

  const plannedStart = o.planned_starts_at ?? o.starts_at;
  const length = new Date(event.ends_at).getTime() - new Date(event.starts_at).getTime();
  const plannedEnd = new Date(new Date(plannedStart).getTime() + length).toISOString();
  const pending = change.isPending || reset.isPending;

  const submit = () => {
    setProblem(null);
    setTried(true);
    const times = timesOn(day, start.time, end.time);
    if (!times) {
      setProblem(t('schedule.form_time_required'));
      return;
    }
    const same = (a: string, b: string) => new Date(a).getTime() === new Date(b).getTime();
    const moved = !same(times.starts_at, plannedStart) || !same(times.ends_at, plannedEnd);
    const body = {
      ...(moved ? times : {}),
      ...(location.trim() !== event.location ? { location: location.trim() } : {}),
      ...(teacher.trim() !== event.teacher ? { teacher: teacher.trim() } : {}),
      ...(note.trim() ? { note: note.trim() } : {}),
    };
    const back = () => router.back();
    if (Object.keys(body).length > 0) {
      change.mutate({ eventId: event.id, date: o.date, ...body }, { onSuccess: back });
    } else if (o.status === 'CHANGED') {
      reset.mutate({ eventId: event.id, date: o.date }, { onSuccess: back });
    } else {
      back();
    }
  };

  const error = change.error ?? reset.error;
  return (
    <YStack gap="$3">
      <Paragraph color="$color10">{t('schedule.form_single_hint')}</Paragraph>
      <YStack gap="$1.5">
        <SectionLabel>{t('schedule.form_date')}</SectionLabel>
        <CalendarMonth selected={day} onPick={setDay} />
      </YStack>
      <TimeRange start={start} end={end} tried={tried} />
      <Field
        id="occurrence-location"
        label={t('schedule.location')}
        value={location}
        onChangeText={setLocation}
        maxLength={200}
      />
      <Field
        id="occurrence-teacher"
        label={t('schedule.teacher')}
        value={teacher}
        onChangeText={setTeacher}
        maxLength={200}
      />
      <Field
        id="occurrence-note"
        label={t('schedule.change_note')}
        value={note}
        onChangeText={setNote}
        maxLength={500}
      />
      <ErrorText>{problem ?? (error ? describeError(t, error) : null)}</ErrorText>
      <XStack gap="$2">
        <Button flex={1} onPress={() => router.back()}>
          {t('common.cancel')}
        </Button>
        <Button flex={1} theme="accent" disabled={pending} onPress={submit}>
          {t('common.save')}
        </Button>
      </XStack>
    </YStack>
  );
}
