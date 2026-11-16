import { api } from '../api';
import type { Schemas } from '../api';

export type Folder = { id: string; name: string };
export type Day = NonNullable<Schemas['AccessSchedule']['DayOfWeek']>;
export type Schedule = { day: Day; start: number; end: number };
export type SyncPlay = Schemas['SyncPlayUserAccessType'];
export type Grants = {
  EnableLiveTvAccess: boolean;
  EnableLiveTvManagement: boolean;
  EnableContentDownloading: boolean;
  EnableLyricManagement: boolean;
  SyncPlayAccess: SyncPlay;
};
export type Device = { id: string; name: string; app: string; seen?: string };
export type Person = {
  id: string;
  name: string;
  admin: boolean;
  disabled: boolean;
  sso?: boolean;
  password: boolean;
  folders: string[] | 'all';
  canRequest: boolean;
  needsApproval: boolean;
  quota?: number;
  used: number;
  seen?: string;
  devices: Device[];
  schedules: Schedule[];
  grants: Grants;
  rating: Rating;
  policy?: Schemas['UserPolicy'];
};

export type Unrated = Schemas['UnratedItem'];
export type Rating = { max?: number; unrated: Unrated[] };
export type Level = { value: number; label: string };

export const unratedKinds: { kind: Unrated; label: string }[] = [
  { kind: 'Movie', label: 'Unrated movies' },
  { kind: 'Series', label: 'Unrated shows' },
  { kind: 'Music', label: 'Unrated music' },
];

export function ratingOf(policy?: Schemas['UserPolicy']): Rating {
  return { max: policy?.MaxParentalRating ?? undefined, unrated: policy?.BlockUnratedItems ?? [] };
}

export function levels(ratings: Schemas['ParentalRating'][]): Level[] {
  const grouped = new Map<number, string[]>();
  ratings.forEach((rating) => {
    if (rating.Value === undefined || rating.Value === null || !rating.Name) {
      return;
    }
    grouped.set(rating.Value, [...(grouped.get(rating.Value) ?? []), rating.Name]);
  });

  return [...grouped.entries()].sort(([a], [b]) => a - b).map(([value, names]) => ({ value, label: names.join(', ') }));
}

export function limit(rating: Rating, ratings: Level[]): string {
  if (rating.max === undefined) {
    return 'No rating limit';
  }

  return 'Up to ' + (ratings.find((level) => level.value === rating.max)?.label ?? rating.max);
}

export const days: { day: Day; label: string }[] = [
  { day: 'Everyday', label: 'Every day' },
  { day: 'Weekday', label: 'Weekdays' },
  { day: 'Weekend', label: 'Weekends' },
  { day: 'Monday', label: 'Monday' },
  { day: 'Tuesday', label: 'Tuesday' },
  { day: 'Wednesday', label: 'Wednesday' },
  { day: 'Thursday', label: 'Thursday' },
  { day: 'Friday', label: 'Friday' },
  { day: 'Saturday', label: 'Saturday' },
  { day: 'Sunday', label: 'Sunday' },
];

export const permissions: { key: Exclude<keyof Grants, 'SyncPlayAccess'>; label: string }[] = [
  { key: 'EnableLiveTvAccess', label: 'Watch Live TV' },
  { key: 'EnableLiveTvManagement', label: 'Manage Live TV channels' },
  { key: 'EnableContentDownloading', label: 'Download titles' },
  { key: 'EnableLyricManagement', label: 'Edit lyrics' },
];

export const syncPlay: { value: SyncPlay; label: string }[] = [
  { value: 'CreateAndJoinGroups', label: 'Start and join watch parties' },
  { value: 'JoinGroups', label: 'Join watch parties only' },
  { value: 'None', label: 'No watch parties' },
];

export function grantsOf(policy?: Schemas['UserPolicy']): Grants {
  return {
    EnableLiveTvAccess: policy?.EnableLiveTvAccess ?? true,
    EnableLiveTvManagement: policy?.EnableLiveTvManagement ?? false,
    EnableContentDownloading: policy?.EnableContentDownloading ?? true,
    EnableLyricManagement: policy?.EnableLyricManagement ?? false,
    SyncPlayAccess: policy?.SyncPlayAccess ?? 'CreateAndJoinGroups',
  };
}

export function schedulesOf(policy?: Schemas['UserPolicy']): Schedule[] {
  return (policy?.AccessSchedules ?? []).map((each) => ({ day: each.DayOfWeek ?? 'Everyday', start: each.StartHour ?? 0, end: each.EndHour ?? 24 }));
}

export function hour(value: number): string {
  const whole = Math.floor(value);
  const minutes = Math.round((value - whole) * 60);

  return String(whole).padStart(2, '0') + ':' + String(minutes).padStart(2, '0');
}

export function when(schedules: Schedule[]): string {
  if (schedules.length === 0) {
    return 'Any time';
  }

  return schedules.map((each) => (days.find((day) => day.day === each.day)?.label ?? each.day) + ' ' + hour(each.start) + '–' + hour(each.end)).join(', ');
}

export function policyBody(person: Person, draft: Person): Schemas['UserPolicy'] | undefined {
  if (!person.policy) {
    return undefined;
  }

  return {
    ...person.policy,
    ...draft.grants,
    IsAdministrator: draft.admin,
    IsDisabled: draft.disabled,
    MaxParentalRating: draft.rating.max ?? null,
    BlockUnratedItems: draft.rating.unrated,
    EnableAllFolders: draft.folders === 'all',
    EnabledFolders: draft.folders === 'all' ? [] : draft.folders,
    AccessSchedules: draft.schedules.map((each) => ({ DayOfWeek: each.day, StartHour: each.start, EndHour: each.end })),
  };
}

export type Answer = { response: Response; error?: unknown };

export async function savePolicy(person: Person, draft: Person): Promise<Answer | undefined> {
  const body = policyBody(person, draft);
  if (!body) {
    return undefined;
  }

  return api.POST('/Users/{userId}/Policy', { params: { path: { userId: person.id } }, body });
}

export async function saveAllowance(draft: Person): Promise<Answer> {
  return api.PUT('/GoJellyfin/RequestUsers/{userId}', {
    params: { path: { userId: draft.id } },
    body: { CanRequest: draft.canRequest, NeedsApproval: draft.needsApproval, Quota: draft.quota ?? null },
  });
}
export function Schedules({ schedules, change }: { schedules: Schedule[]; change: (schedules: Schedule[]) => void }) {
  function edit(index: number, patch: Partial<Schedule>) {
    change(schedules.map((each, at) => (at === index ? { ...each, ...patch } : each)));
  }

  return (
    <>
      {schedules.length === 0 ? (
        <p class="muted">Any time. Add a window to allow signing in only then.</p>
      ) : (
        <table class="rows plain schedules">
          <tbody>
            {schedules.map((each, index) => (
              <tr key={index}>
                <td>
                  <select aria-label="Days" value={each.day} onChange={(event) => edit(index, { day: event.currentTarget.value as Day })}>
                    {days.map((day) => (
                      <option key={day.day} value={day.day}>
                        {day.label}
                      </option>
                    ))}
                  </select>
                </td>
                <td>
                  <input aria-label="From" type="time" step="1800" value={hour(each.start)} onChange={(event) => edit(index, { start: hours(event.currentTarget.value) })} />
                </td>
                <td>
                  <input aria-label="Until" type="time" step="1800" value={hour(each.end === 24 ? 0 : each.end)} onChange={(event) => edit(index, { end: hours(event.currentTarget.value) || 24 })} />
                </td>
                <td class="end">
                  <button class="link" aria-label="Remove window" onClick={() => change(schedules.filter((_, at) => at !== index))}>
                    ×
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <div class="actions spaced">
        <button type="button" onClick={() => change([...schedules, { day: 'Everyday', start: 9, end: 22 }])}>
          Add a window
        </button>
      </div>
      <p class="muted small">Times are the server's. A window that ends before it starts runs past midnight. Administrators can always sign in.</p>
    </>
  );
}

function hours(value: string): number {
  const [whole, minutes] = value.split(':').map(Number);

  return (whole ?? 0) + (minutes ?? 0) / 60;
}
