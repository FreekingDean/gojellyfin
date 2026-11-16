import { useState } from 'preact/hooks';

import * as mock from '../mock';
import { Card, Page, notify, refused } from '../ui';
import { Schedules, limit, permissions, saveAllowance, savePolicy, syncPlay, unratedKinds, when } from './policy';
import type { Answer, Folder, Grants, Level, Person, Schedule, SyncPlay, Unrated } from './policy';

type Turn = 'leave' | 'on' | 'off';
type Permission = Exclude<keyof Grants, 'SyncPlayAccess'>;

export type Change = {
  folders: Record<string, Turn>;
  schedules: 'leave' | 'any' | 'only';
  windows: Schedule[];
  grants: Partial<Record<Permission, Turn>>;
  syncPlay: 'leave' | SyncPlay;
  rating: string;
  unrated: Partial<Record<Unrated, Turn>>;
  disabled: Turn;
  canRequest: Turn;
  needsApproval: Turn;
  quota: 'leave' | 'none' | 'most';
  most: number;
};

const untouched: Change = {
  folders: {},
  schedules: 'leave',
  windows: [{ day: 'Everyday', start: 9, end: 22 }],
  grants: {},
  syncPlay: 'leave',
  rating: 'leave',
  unrated: {},
  disabled: 'leave',
  canRequest: 'leave',
  needsApproval: 'leave',
  quota: 'leave',
  most: 5,
};

function turned(value: boolean, turn: Turn | undefined): boolean {
  return turn === 'on' ? true : turn === 'off' ? false : value;
}

export function policyTouched(change: Change): boolean {
  return (
    Object.values(change.folders).some((turn) => turn !== 'leave') ||
    change.schedules !== 'leave' ||
    Object.values(change.grants).some((turn) => turn !== 'leave') ||
    change.syncPlay !== 'leave' ||
    change.rating !== 'leave' ||
    Object.values(change.unrated).some((turn) => turn !== 'leave') ||
    change.disabled !== 'leave'
  );
}

export function allowanceTouched(change: Change): boolean {
  return change.canRequest !== 'leave' || change.needsApproval !== 'leave' || change.quota !== 'leave';
}

export function changed(person: Person, change: Change, folders: Folder[]): Person {
  const seen = person.folders === 'all' ? folders.map((folder) => folder.id) : person.folders;
  const visible = seen.filter((each) => change.folders[each] !== 'off');
  folders.forEach((folder) => {
    if (change.folders[folder.id] === 'on' && !visible.includes(folder.id)) {
      visible.push(folder.id);
    }
  });
  const grants = { ...person.grants };
  permissions.forEach((permission) => {
    grants[permission.key] = turned(grants[permission.key], change.grants[permission.key]);
  });
  if (change.syncPlay !== 'leave') {
    grants.SyncPlayAccess = change.syncPlay;
  }

  const unrated = unratedKinds
    .map((each) => each.kind)
    .filter((kind) => turned(person.rating.unrated.includes(kind), change.unrated[kind]))
    .concat(person.rating.unrated.filter((kind) => !unratedKinds.some((each) => each.kind === kind)));
  const max = change.rating === 'leave' ? person.rating.max : change.rating === 'none' ? undefined : Number(change.rating);

  return {
    ...person,
    rating: { max, unrated },
    folders: folders.every((folder) => visible.includes(folder.id)) ? 'all' : visible,
    schedules: change.schedules === 'any' ? [] : change.schedules === 'only' ? change.windows : person.schedules,
    grants,
    disabled: turned(person.disabled, change.disabled),
    canRequest: turned(person.canRequest, change.canRequest),
    needsApproval: turned(person.needsApproval, change.needsApproval),
    quota: change.quota === 'none' ? undefined : change.quota === 'most' ? change.most : person.quota,
  };
}

function tally(people: Person[], holds: (person: Person) => boolean, state = 'now'): string {
  const count = people.filter(holds).length;
  if (count === people.length) {
    return 'All of them ' + state;
  }
  if (count === 0) {
    return 'None of them ' + state;
  }

  return count + ' of ' + people.length + ' ' + state;
}

function Choice<T extends string>({
  label,
  now,
  value,
  options,
  choose,
}: {
  label: string;
  now?: string;
  value: T;
  options: { value: T; label: string }[];
  choose: (value: T) => void;
}) {
  return (
    <label class="choice">
      <span>
        {label}
        {now && <span class="muted small"> · {now}</span>}
      </span>
      <select aria-label={label} class={value === 'leave' ? '' : 'touched'} value={value} onChange={(event) => choose(event.currentTarget.value as T)}>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </label>
  );
}

function Turns({ label, now, value, on, off, choose }: { label: string; now: string; value: Turn; on: string; off: string; choose: (turn: Turn) => void }) {
  return (
    <Choice
      label={label}
      now={now}
      value={value}
      choose={choose}
      options={[
        { value: 'leave', label: 'Leave as is' },
        { value: 'on', label: on },
        { value: 'off', label: off },
      ]}
    />
  );
}

export function Bulk({
  people,
  folders,
  ratings,
  cancel,
  finish,
}: {
  people: Person[];
  folders: Folder[];
  ratings: Level[];
  cancel: () => void;
  finish: (failed: string[], drafts: Person[]) => Promise<void>;
}) {
  const [change, setChange] = useState<Change>(untouched);
  const [saving, setSaving] = useState(false);
  const touched = policyTouched(change) || allowanceTouched(change);
  const admins = people.filter((person) => person.admin).length;

  function edit(patch: Partial<Change>) {
    setChange({ ...change, ...patch });
  }

  async function saveOne(person: Person, draft: Person): Promise<Answer | undefined> {
    if (policyTouched(change)) {
      const policy = await savePolicy(person, draft);
      if (policy && !policy.response.ok) {
        return policy;
      }
    }
    if (allowanceTouched(change)) {
      const allowance = await saveAllowance(draft);
      if (!allowance.response.ok) {
        return allowance;
      }
    }

    return undefined;
  }

  async function apply() {
    setSaving(true);
    const drafts = people.map((person) => changed(person, change, folders));
    const failed: string[] = [];
    for (const [index, person] of people.entries()) {
      const draft = drafts[index];
      if (mock.mocked || !draft) {
        continue;
      }
      try {
        const refusal = await saveOne(person, draft);
        if (refusal) {
          failed.push(person.id);
          refused(refusal.error, draft.disabled && person.admin ? person.name + ' is an administrator and cannot be disabled.' : person.name + ' could not be changed.');
        }
      } catch {
        failed.push(person.id);
        refused(undefined, person.name + ' could not be changed.');
      }
    }
    const saved = people.length - failed.length;
    if (saved > 0) {
      notify(failed.length === 0 ? (saved === 1 ? '1 user changed.' : saved + ' users changed.') : saved + ' of ' + people.length + ' users changed.');
    }
    setSaving(false);
    await finish(failed, drafts);
  }

  return (
    <Page
      title={people.length === 1 ? 'Edit 1 user' : 'Edit ' + people.length + ' users'}
      lead="Only what you change here is saved. Everything left as is stays the way each person has it."
      action={
        <span class="actions">
          <button onClick={cancel}>Cancel</button>
          <button class="primary" disabled={!touched || saving} onClick={apply}>
            {saving ? 'Saving…' : 'Save to ' + (people.length === 1 ? '1 user' : people.length + ' users')}
          </button>
        </span>
      }
    >
      <p class="names">
        {people.map((person) => (
          <span key={person.id} class="pill off">
            {person.name}
          </span>
        ))}
      </p>
      <div class="grid two">
        <Card title="Can see">
          {folders.length === 0 && <p class="muted">There are no libraries yet.</p>}
          {folders.map((folder) => (
            <Turns
              key={folder.id}
              label={folder.name}
              now={tally(people, (person) => person.folders === 'all' || person.folders.includes(folder.id))}
              value={change.folders[folder.id] ?? 'leave'}
              on="Can see"
              off="Cannot see"
              choose={(turn) => edit({ folders: { ...change.folders, [folder.id]: turn } })}
            />
          ))}
          {admins > 0 && <p class="muted small">Administrators see every library whatever is chosen here.</p>}
        </Card>

        <Card title="When they can sign in">
          <Choice
            label="Sign-in hours"
            now={new Set(people.map((person) => when(person.schedules))).size === 1 ? when(people[0]?.schedules ?? []) : 'Differs'}
            value={change.schedules}
            choose={(schedules) => edit({ schedules })}
            options={[
              { value: 'leave', label: 'Leave as is' },
              { value: 'any', label: 'Any time' },
              { value: 'only', label: 'Only at these times' },
            ]}
          />
          {change.schedules === 'only' && <Schedules schedules={change.windows} change={(windows) => edit({ windows })} />}
        </Card>

        <Card title="Permissions">
          {permissions.map((permission) => (
            <Turns
              key={permission.key}
              label={permission.label}
              now={tally(people, (person) => person.grants[permission.key])}
              value={change.grants[permission.key] ?? 'leave'}
              on="Allow"
              off="Do not allow"
              choose={(turn) => edit({ grants: { ...change.grants, [permission.key]: turn } })}
            />
          ))}
          <Choice
            label="Watch parties"
            value={change.syncPlay}
            choose={(value) => edit({ syncPlay: value })}
            options={[{ value: 'leave' as const, label: 'Leave as is' }, ...syncPlay]}
          />
        </Card>

        <Card title="Parental controls">
          <Choice
            label="Highest rating they can see"
            now={new Set(people.map((person) => limit(person.rating, ratings))).size === 1 ? limit(people[0]?.rating ?? { unrated: [] }, ratings) : 'Differs'}
            value={change.rating}
            choose={(rating) => edit({ rating })}
            options={[
              { value: 'leave', label: 'Leave as is' },
              { value: 'none', label: 'No limit' },
              ...ratings.map((level) => ({ value: String(level.value), label: 'Up to ' + level.label })),
            ]}
          />
          {unratedKinds.map((each) => (
            <Turns
              key={each.kind}
              label={each.label}
              now={tally(people, (person) => person.rating.unrated.includes(each.kind), 'hidden now')}
              value={change.unrated[each.kind] ?? 'leave'}
              on="Hide"
              off="Show"
              choose={(turn) => edit({ unrated: { ...change.unrated, [each.kind]: turn } })}
            />
          ))}
          {admins > 0 && <p class="muted small">Administrators see everything whatever is chosen here.</p>}
        </Card>

        <Card title="Requests and account">
          <Turns
            label="Can ask for titles the library lacks"
            now={tally(people, (person) => person.canRequest)}
            value={change.canRequest}
            on="Can ask"
            off="Cannot ask"
            choose={(canRequest) => edit({ canRequest })}
          />
          <Turns
            label="An administrator approves each request"
            now={tally(people, (person) => person.needsApproval)}
            value={change.needsApproval}
            on="Needs approval"
            off="Straight to the downloader"
            choose={(needsApproval) => edit({ needsApproval })}
          />
          <Choice
            label="Most titles in seven days"
            value={change.quota}
            choose={(quota) => edit({ quota })}
            options={[
              { value: 'leave', label: 'Leave as is' },
              { value: 'none', label: 'No limit' },
              { value: 'most', label: 'At most…' },
            ]}
          />
          {change.quota === 'most' && (
            <div class="form one">
              <label>
                Titles
                <input name="most" type="number" min="0" value={change.most} onInput={(event) => edit({ most: Number(event.currentTarget.value) })} />
              </label>
            </div>
          )}
          <Turns
            label="Disabled"
            now={tally(people, (person) => person.disabled)}
            value={change.disabled}
            on="Disable"
            off="Enable"
            choose={(disabled) => edit({ disabled })}
          />
        </Card>
      </div>
    </Page>
  );
}
