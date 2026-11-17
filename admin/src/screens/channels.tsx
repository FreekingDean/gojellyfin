import { useEffect, useState } from 'preact/hooks';

import { api } from '../api';
import type { Schemas } from '../api';
import * as mock from '../mock';
import { Link, navigate } from '../router';
import { Card, Cell, Page, Pill } from '../ui';

type Channel = Schemas['LiveChannel'];
type Order = Schemas['LiveChannelOrder'];
type Title = { id: string; name: string; kind: string };
type Loaded = { channels: Channel[]; titles: Map<string, Title>; airing: Map<string, string> };

const sitcom = { id: 'a0000000-0000-4000-8000-000000000001', name: 'Example Sitcom', kind: 'Series' };
const cartoon = { id: 'a0000000-0000-4000-8000-000000000002', name: 'Example Cartoon', kind: 'Series' };
const feature = { id: 'a0000000-0000-4000-8000-000000000003', name: 'Example Feature', kind: 'Movie' };
const mockedTitles: Title[] = [sitcom, cartoon, feature];

function mocked(): Loaded {
  return {
    channels: [
      { Id: 'c0000000-0000-4000-8000-000000000001', Number: 2, Name: 'Sitcoms', Order: 'Shuffle', ItemIds: [sitcom.id, cartoon.id] },
      { Id: 'c0000000-0000-4000-8000-000000000002', Number: 5, Name: 'Movie Night', Order: 'Sequential', ItemIds: [feature.id] },
    ],
    titles: new Map(mockedTitles.map((title) => [title.id, title])),
    airing: new Map([['c0000000-0000-4000-8000-000000000001', 'Example Sitcom']]),
  };
}

function title(item: Schemas['BaseItemDto']): Title {
  return { id: item.Id ?? '', name: item.Name ?? '', kind: item.Type ?? '' };
}

async function titlesNamed(ids: string[]): Promise<Title[]> {
  if (ids.length === 0) {
    return [];
  }
  const { data } = await api.GET('/Items', { params: { query: { ids, recursive: true } } });

  return (data?.Items ?? []).map(title);
}

async function fetched(): Promise<Loaded | undefined> {
  const [listed, live] = await Promise.all([api.GET('/GoJellyfin/Channels'), api.GET('/LiveTv/Channels', { params: { query: { addCurrentProgram: true } } })]);
  if (!listed.data) {
    return undefined;
  }

  const named = await titlesNamed([...new Set(listed.data.flatMap((channel) => channel.ItemIds))]);
  const airing = new Map<string, string>();
  for (const channel of live.data?.Items ?? []) {
    const program = channel.CurrentProgram;
    if (channel.Id && program?.Name) {
      airing.set(channel.Id, program.EpisodeTitle ? program.Name + ': ' + program.EpisodeTitle : program.Name);
    }
  }

  return { channels: listed.data, titles: new Map(named.map((each) => [each.id, each])), airing };
}

function described(order: Order): string {
  if (order === 'Shuffle') {
    return 'Shuffled';
  }

  return order === 'Interleave' ? 'Taking turns' : 'In order';
}

export function Channels({ id }: { id?: string }) {
  const [loaded, setLoaded] = useState<Loaded | undefined>();
  const [failed, setFailed] = useState(false);

  async function load() {
    if (mock.mocked) {
      setLoaded(mocked());
      return;
    }
    const result = await fetched();
    setFailed(!result);
    setLoaded(result);
  }

  useEffect(() => {
    void load();
  }, []);

  if (failed) {
    return <p class="failure">The channels could not be loaded.</p>;
  }
  if (!loaded) {
    return <p class="muted">Loading…</p>;
  }

  if (id === 'new') {
    const used = new Set(loaded.channels.map((channel) => channel.Number));
    let number = 1;
    while (used.has(number)) {
      number++;
    }

    return <Edit channel={{ Id: '', Number: number, Name: '', Order: 'Shuffle', ItemIds: [] }} titles={loaded.titles} reload={load} />;
  }
  const channel = loaded.channels.find((each) => each.Id === id);
  if (channel) {
    return <Edit key={channel.Id} channel={channel} titles={loaded.titles} reload={load} />;
  }

  return (
    <Page
      title="Channels"
      lead="Live channels that play your own shows and films around the clock, in the Live TV section of every Jellyfin app."
      action={
        <Link to="/channels/new" class="button primary">
          Add channel
        </Link>
      }
    >
      <div class="card flush">
        {loaded.channels.length === 0 ? (
          <p class="muted">There are no channels yet.</p>
        ) : (
          <table class="rows">
            <colgroup>
              <col style="width:10%" />
              <col style="width:30%" />
              <col style="width:46%" />
              <col style="width:14%" />
            </colgroup>
            <thead>
              <tr>
                <th>Number</th>
                <th>Channel</th>
                <th>On now</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {loaded.channels.map((each) => (
                <tr key={each.Id}>
                  <Cell main={each.Number} />
                  <Cell
                    main={each.Name}
                    sub={described(each.Order) + ', ' + each.ItemIds.length + (each.ItemIds.length === 1 ? ' title' : ' titles')}
                  />
                  <Cell main={loaded.airing.get(each.Id) ?? <Pill>Nothing to play</Pill>} />
                  <td>
                    <div class="actions right">
                      <Link to={'/channels/' + each.Id} class="button">
                        Edit
                      </Link>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </Page>
  );
}

function Edit({ channel, titles, reload }: { channel: Channel; titles: Map<string, Title>; reload: () => Promise<void> }) {
  const [draft, setDraft] = useState(channel);
  const [known, setKnown] = useState(titles);
  const [search, setSearch] = useState('');
  const [found, setFound] = useState<Title[]>([]);
  const [failure, setFailure] = useState('');
  const creating = channel.Id === '';

  useEffect(() => {
    const term = search.trim();
    if (term.length < 2) {
      setFound([]);
      return;
    }
    if (mock.mocked) {
      setFound(mockedTitles.filter((each) => each.name.toLowerCase().includes(term.toLowerCase())));
      return;
    }
    const timer = window.setTimeout(async () => {
      const { data } = await api.GET('/Items', {
        params: { query: { searchTerm: term, recursive: true, includeItemTypes: ['Series', 'Movie'], limit: 10 } },
      });
      setFound((data?.Items ?? []).map(title));
    }, 250);

    return () => window.clearTimeout(timer);
  }, [search]);

  function change(patch: Partial<Channel>) {
    setDraft({ ...draft, ...patch });
  }

  function add(each: Title) {
    if (!draft.ItemIds.includes(each.id)) {
      setKnown(new Map(known).set(each.id, each));
      change({ ItemIds: [...draft.ItemIds, each.id] });
    }
    setSearch('');
  }

  function move(index: number, by: number) {
    const ids = [...draft.ItemIds];
    ids.splice(index + by, 0, ...ids.splice(index, 1));
    change({ ItemIds: ids });
  }

  async function save() {
    if (mock.mocked) {
      navigate('/channels');
      return;
    }
    const body = { Number: draft.Number, Name: draft.Name, Order: draft.Order, ItemIds: draft.ItemIds };
    const { data, error } = creating
      ? await api.POST('/GoJellyfin/Channels', { body })
      : await api.PUT('/GoJellyfin/Channels/{channelId}', { params: { path: { channelId: channel.Id } }, body });
    if (!data) {
      setFailure(error && 'Message' in error ? String(error.Message) : 'The channel could not be saved.');
      return;
    }
    await reload();
    navigate('/channels');
  }

  async function remove() {
    if (!window.confirm('Delete channel ' + channel.Number + ', ' + channel.Name + '?')) {
      return;
    }
    const { response } = await api.DELETE('/GoJellyfin/Channels/{channelId}', { params: { path: { channelId: channel.Id } } });
    if (!response.ok) {
      setFailure('The channel could not be deleted.');
      return;
    }
    await reload();
    navigate('/channels');
  }

  return (
    <Page
      title={creating ? 'Add channel' : channel.Number + ' ' + channel.Name}
      lead="A channel loops through its titles end to end. Shuffled channels play a new order every time round."
      action={
        <span class="actions">
          <Link to="/channels" class="button">
            Cancel
          </Link>
          <button class="primary" onClick={save}>
            Save
          </button>
        </span>
      }
    >
      {failure && (
        <p class="failure" role="alert">
          {failure}
        </p>
      )}
      <div class="grid two">
        <Card title="Channel">
          <div class="form">
            <label>
              Number
              <input
                name="number"
                type="number"
                min="1"
                max="65535"
                value={draft.Number}
                onInput={(event) => change({ Number: Number(event.currentTarget.value) })}
              />
            </label>
            <label>
              Name
              <input name="name" autocomplete="off" value={draft.Name} onInput={(event) => change({ Name: event.currentTarget.value })} />
            </label>
            <label>
              Order
              <select value={draft.Order} onChange={(event) => change({ Order: event.currentTarget.value as Order })}>
                <option value="Shuffle">Shuffled</option>
                <option value="Interleave">Taking turns, each series by season and episode</option>
                <option value="Sequential">In order, each series by season and episode</option>
              </select>
            </label>
          </div>
        </Card>

        <Card title="Plays">
          <div class="form">
            <label>
              Add a show or film
              <input name="search" autocomplete="off" placeholder="Search the library" value={search} onInput={(event) => setSearch(event.currentTarget.value)} />
            </label>
          </div>
          {found.length > 0 && (
            <table class="rows plain">
              <tbody>
                {found.map((each) => (
                  <tr key={each.id}>
                    <Cell main={each.name} sub={each.kind} />
                    <td class="end">
                      <button disabled={draft.ItemIds.includes(each.id)} onClick={() => add(each)}>
                        Add
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          <p class="muted small">On this channel</p>
          {draft.ItemIds.length === 0 ? (
            <p class="muted">Nothing yet. Search for a show or film to add it.</p>
          ) : (
            <table class="rows plain">
              <tbody>
                {draft.ItemIds.map((itemId, index) => (
                  <tr key={itemId}>
                    <Cell main={known.get(itemId)?.name ?? 'A title no longer in the library'} sub={known.get(itemId)?.kind} />
                    <td class="end">
                      <span class="actions right">
                        <button disabled={index === 0} onClick={() => move(index, -1)} aria-label="Move up">
                          ↑
                        </button>
                        <button disabled={index === draft.ItemIds.length - 1} onClick={() => move(index, 1)} aria-label="Move down">
                          ↓
                        </button>
                        <button onClick={() => change({ ItemIds: draft.ItemIds.filter((each) => each !== itemId) })}>Remove</button>
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </Card>
      </div>
      {!creating && (
        <p class="danger">
          <button class="bad" disabled={mock.mocked} onClick={remove}>
            Delete channel
          </button>
          <span class="muted small">The titles stay in the library.</span>
        </p>
      )}
    </Page>
  );
}
