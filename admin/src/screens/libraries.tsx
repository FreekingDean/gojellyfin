import { useEffect, useState } from "preact/hooks";

import { api } from "../api";
import type { Schemas } from "../api";
import * as mock from "../mock";
import { Link, navigate } from "../router";
import { Card, Dot, Field, Page, Pill, since } from "../ui";

type Kind = "Radarr" | "Sonarr";
type Slug = "movies" | "shows";
type Folder = { remote: string; local: string };
type Requests = {
  profileId: number;
  profile?: string;
  folder: string;
  tag?: string;
};

type Downloader = {
  id: string;
  name: string;
  kind: Kind;
  version?: string;
  url: string;
  variable: string;
  contacted?: string;
  error?: string;
  paths: Folder[];
  synced?: string;
  files: number;
  requests?: Requests;
  webhook?: Schemas["SourceWebhook"];
  bindings: Schemas["SourceLibrary"][];
};

type Library = {
  id: string;
  slug: Slug;
  name: string;
  kind: Kind;
  counts: string;
  synced?: string;
};

type Loaded = {
  libraries: Library[];
  downloaders: Downloader[];
  variables: string[];
};

type Instance = {
  version?: string;
  error?: string;
  folders: string[];
  profiles: { id: number; name: string }[];
  tags: string[];
};

const kinds: Record<Slug, Kind> = { movies: "Radarr", shows: "Sonarr" };

function mocked(): Loaded {
  return {
    variables: mock.pickers.variables,
    libraries: mock.libraries.map((library) => ({
      ...library,
      slug: library.id,
    })),
    downloaders: mock.downloaders.map((downloader) => ({
      ...downloader,
      requests: downloader.requests && {
        ...downloader.requests,
        profileId:
          mock.pickers.profiles.indexOf(downloader.requests.profile) + 1,
      },
      bindings: [],
    })),
  };
}

function counted(value: number, one: string, many: string): string {
  return value.toLocaleString() + " " + (value === 1 ? one : many);
}

async function count(
  parentId: string,
  type: "Movie" | "Series" | "Episode",
): Promise<number> {
  const { data } = await api.GET("/Items", {
    params: {
      query: { parentId, recursive: true, includeItemTypes: [type], limit: 0 },
    },
  });

  return data?.TotalRecordCount ?? 0;
}

function newest(values: (string | null | undefined)[]): string | undefined {
  return values
    .filter((value): value is string => Boolean(value))
    .sort()
    .pop();
}

async function fetched(): Promise<Loaded | undefined> {
  const [folders, sources, variables] = await Promise.all([
    api.GET("/Library/VirtualFolders"),
    api.GET("/GoJellyfin/Sources"),
    api.GET("/GoJellyfin/SourceApiKeyVariables"),
  ]);
  if (!folders.data || !sources.data || !variables.data) {
    return undefined;
  }

  const downloaders: Downloader[] = sources.data.map((source) => ({
    id: source.Id ?? "",
    name: source.Name ?? "",
    kind: source.Kind === "Sonarr" ? "Sonarr" : "Radarr",
    version: source.Version ?? undefined,
    url: source.Url ?? "",
    variable: source.ApiKeyVariable ?? "",
    contacted: source.ContactedAt ?? undefined,
    error: source.LastError ?? undefined,
    paths: (source.Paths ?? []).map((path) => ({
      remote: path.RemotePath,
      local: path.LocalPath,
    })),
    synced: newest(
      (source.Libraries ?? []).map((binding) => binding.LastSyncedAt),
    ),
    files: source.Files ?? 0,
    requests: source.Requests
      ? {
          profileId: source.Requests.QualityProfileId,
          folder: source.Requests.RootFolder,
          tag: source.Requests.Tag ?? undefined,
        }
      : undefined,
    webhook: source.Webhook,
    bindings: source.Libraries ?? [],
  }));

  const libraries = await Promise.all(
    (["movies", "shows"] as Slug[]).map(
      async (slug): Promise<Library | undefined> => {
        const folder = folders.data.find(
          (each) =>
            each.CollectionType === (slug === "movies" ? "movies" : "tvshows"),
        );
        if (!folder?.ItemId) {
          return undefined;
        }
        const id = folder.ItemId;
        const counts =
          slug === "movies"
            ? counted(await count(id, "Movie"), "movie", "movies")
            : await Promise.all([
                count(id, "Series"),
                count(id, "Episode"),
              ]).then(
                ([shows, episodes]) =>
                  counted(shows, "show", "shows") +
                  ", " +
                  counted(episodes, "episode", "episodes"),
              );
        const synced = newest(
          downloaders.flatMap((downloader) =>
            downloader.bindings
              .filter((binding) => binding.Id === id)
              .map((binding) => binding.LastSyncedAt),
          ),
        );

        return {
          id,
          slug,
          name: folder.Name ?? "",
          kind: kinds[slug],
          counts,
          synced,
        };
      },
    ),
  );

  return {
    variables: variables.data,
    downloaders,
    libraries: libraries.filter((library): library is Library =>
      Boolean(library),
    ),
  };
}

async function probe(
  kind: Kind,
  url: string,
  variable: string,
): Promise<Instance> {
  if (mock.mocked) {
    return {
      version: "5.14",
      folders: mock.pickers.remoteFolders,
      profiles: mock.pickers.profiles.map((name, i) => ({ id: i + 1, name })),
      tags: mock.pickers.tags,
    };
  }
  const { data } = await api.POST("/GoJellyfin/Sources/Test", {
    body: { Kind: kind, Url: url, ApiKeyVariable: variable },
  });
  if (!data) {
    return {
      error: "The test could not be run.",
      folders: [],
      profiles: [],
      tags: [],
    };
  }
  if (!data.Reachable) {
    return {
      error: data.Error ?? "Did not answer.",
      folders: [],
      profiles: [],
      tags: [],
    };
  }

  return {
    version: data.Version ?? undefined,
    folders: data.RootFolders ?? [],
    profiles: (data.QualityProfiles ?? []).map((profile) => ({
      id: profile.Id,
      name: profile.Name,
    })),
    tags: data.Tags ?? [],
  };
}

export function Libraries({ id }: { id?: string }) {
  const [loaded, setLoaded] = useState<Loaded | undefined>(
    mock.mocked ? mocked() : undefined,
  );
  const [failure, setFailure] = useState("");

  async function load() {
    if (mock.mocked) {
      return;
    }
    const answer = await fetched();
    if (!answer) {
      setFailure("The libraries could not be loaded.");
      return;
    }
    setFailure("");
    setLoaded(answer);
  }

  useEffect(() => {
    void load();
  }, []);

  if (!loaded) {
    return (
      <Page
        title="Libraries"
        lead="Movies come from Radarr and Shows from Sonarr."
      >
        <p class={failure ? "failure" : "muted"}>{failure || "Loading…"}</p>
      </Page>
    );
  }

  const editing = loaded.downloaders.find((downloader) => downloader.id === id);
  if (editing) {
    const library = loaded.libraries.find((each) => each.kind === editing.kind);

    return (
      <Editor
        key={editing.id}
        kind={editing.kind}
        library={library}
        variables={loaded.variables}
        downloader={editing}
        saved={load}
      />
    );
  }
  const adding = loaded.libraries.find(
    (library) => "new-" + library.slug === id,
  );
  if (adding) {
    return (
      <Editor
        key={adding.slug}
        kind={adding.kind}
        library={adding}
        variables={loaded.variables}
        saved={load}
      />
    );
  }

  return <Overview loaded={loaded} />;
}

function Overview({ loaded }: { loaded: Loaded }) {
  const [notice, setNotice] = useState("");
  const [failure, setFailure] = useState("");
  const [tested, setTested] = useState<Record<string, Instance>>({});

  async function refresh() {
    if (mock.mocked) {
      return;
    }
    const { response } = await api.POST("/Library/Refresh");
    if (response.ok) {
      setFailure("");
      setNotice(
        "Refreshing. Every downloader is read again, and what changed shows up as it lands.",
      );
    } else {
      setNotice("");
      setFailure(
        "The refresh could not be started. Background work may be off on this server.",
      );
    }
  }

  async function test(downloader: Downloader) {
    setTested({
      ...tested,
      [downloader.id]: await probe(
        downloader.kind,
        downloader.url,
        downloader.variable,
      ),
    });
  }

  return (
    <Page
      title="Libraries"
      lead="Movies come from Radarr and Shows from Sonarr. Add a downloader and its folders, and the library fills itself."
    >
      {failure && (
        <p class="failure" role="alert">
          {failure}
        </p>
      )}
      {notice && <p class="muted">{notice}</p>}
      {loaded.libraries.map((library) => {
        const feeding = loaded.downloaders.filter(
          (downloader) => downloader.kind === library.kind,
        );

        return (
          <section class="library" key={library.id}>
            <header class="pagehead">
              <div>
                <h2>{library.name}</h2>
                <p class="muted">
                  {library.counts}
                  {library.synced && " · refreshed " + since(library.synced)}
                </p>
              </div>
              <span class="actions">
                <button disabled={feeding.length === 0} onClick={refresh}>
                  Refresh now
                </button>
                <Link
                  to={"/libraries/new-" + library.slug}
                  class="button primary"
                >
                  Add {library.kind}
                </Link>
              </span>
            </header>

            {feeding.length === 0 && (
              <div class="card hint">
                No {library.kind} yet. Add one and {library.name} appears for
                everyone who can see it.
              </div>
            )}

            <div class="grid two">
              {feeding.map((downloader) => {
                const answer = tested[downloader.id];
                const error = answer ? answer.error : downloader.error;

                return (
                  <Card key={downloader.id} class={error ? "troubled" : ""}>
                    <header class="cardhead">
                      <b>
                        <Dot tone={error ? "warn" : "ok"} />
                        {downloader.name}
                      </b>
                      <span class="actions">
                        {(answer?.version ?? downloader.version) && (
                          <Pill>
                            {downloader.kind}{" "}
                            {answer?.version ?? downloader.version}
                          </Pill>
                        )}
                        <button onClick={() => test(downloader)}>Test</button>
                        <Link to={"/libraries/" + downloader.id} class="button">
                          Edit
                        </Link>
                      </span>
                    </header>
                    <dl class="fields">
                      <Field label="Address">{downloader.url}</Field>
                      <Field label="API key">
                        <code>{downloader.variable}</code>{" "}
                        {loaded.variables.includes(downloader.variable) ? (
                          "set"
                        ) : (
                          <span class="bad">not set on this server</span>
                        )}
                      </Field>
                      <Field label="Last answered">
                        {answer ? (
                          answer.error ? (
                            <span class="warn">just now · {answer.error}</span>
                          ) : (
                            "just now"
                          )
                        ) : downloader.error ? (
                          <span class="warn">
                            {downloader.contacted
                              ? since(downloader.contacted)
                              : "never"}{" "}
                            · {downloader.error}
                          </span>
                        ) : downloader.contacted ? (
                          since(downloader.contacted)
                        ) : (
                          "not yet"
                        )}
                      </Field>
                      <Field label="Folders">
                        {downloader.paths.map((path) => (
                          <div key={path.remote}>
                            {path.remote}{" "}
                            <span class="muted">opened here as</span>{" "}
                            {path.local}
                          </div>
                        ))}
                      </Field>
                      <Field label="Files">
                        {downloader.files.toLocaleString()}{" "}
                        {downloader.synced && (
                          <span class="muted">
                            · synced {since(downloader.synced)}
                          </span>
                        )}
                      </Field>
                      <Field label="Requests">
                        {downloader.requests ? (
                          <>
                            <Pill tone="ok">On</Pill>{" "}
                            {downloader.requests.profile ??
                              "profile " + downloader.requests.profileId}{" "}
                            into {downloader.requests.folder}
                            {downloader.requests.tag && (
                              <>
                                , tag <code>{downloader.requests.tag}</code>
                              </>
                            )}
                          </>
                        ) : (
                          <>
                            <Pill>Off</Pill>{" "}
                            <span class="muted">
                              Pick a quality profile to take requests
                            </span>
                          </>
                        )}
                      </Field>
                      {downloader.webhook && (
                        <Field label="Webhook">
                          <Hook webhook={downloader.webhook} />
                        </Field>
                      )}
                    </dl>
                  </Card>
                );
              })}
            </div>
          </section>
        );
      })}
      <p class="muted small">
        Two downloaders holding the same title show it once, with a version from
        each. Keys are never stored here: a downloader names an environment
        variable starting with SOURCE_API_KEY_.
      </p>
    </Page>
  );
}

function Hook({ webhook }: { webhook: Schemas["SourceWebhook"] }) {
  if (webhook.RegisteredAt) {
    return (
      <>
        <Pill tone="ok">On</Pill>{" "}
        <span class="muted">
          new files appear as soon as they are imported · registered{" "}
          {since(webhook.RegisteredAt)}
        </span>
      </>
    );
  }

  return (
    <>
      <span class="warn">
        not registered{webhook.Error && " · " + webhook.Error}
      </span>
      <details>
        <summary class="muted">
          Add it by hand under Settings, Connect, Webhook
        </summary>
        <div>
          URL{" "}
          <code>
            {webhook.Url ?? "<published address>/webhooks/<source id>"}
          </code>
        </div>
        <div>
          Method <code>POST</code>, username <code>{webhook.Username}</code>,
          password <code>{webhook.Password}</code>
        </div>
        <div class="muted">
          On Import, On Upgrade, On Rename and the deletes; the next save or
          refresh registers it again.
        </div>
      </details>
    </>
  );
}

function Options({
  options,
  value,
  empty,
}: {
  options: string[];
  value?: string;
  empty?: string;
}) {
  const listed =
    value && !options.includes(value) ? [value, ...options] : options;

  return (
    <>
      {empty !== undefined && (
        <option value="" selected={!value}>
          {empty}
        </option>
      )}
      {listed.map((option) => (
        <option key={option} value={option} selected={option === value}>
          {option}
        </option>
      ))}
    </>
  );
}

function Browser({
  path,
  choose,
  close,
}: {
  path: string;
  choose: (path: string) => void;
  close: () => void;
}) {
  const [entries, setEntries] = useState<
    { name: string; path: string }[] | undefined
  >(undefined);
  const [failure, setFailure] = useState("");

  useEffect(() => {
    void (async () => {
      setEntries(undefined);
      const { data } = path
        ? await api.GET("/Environment/DirectoryContents", {
            params: {
              query: { path, includeDirectories: true, includeFiles: false },
            },
          })
        : await api.GET("/Environment/Drives");
      if (!data) {
        setFailure(path + " cannot be listed here.");
        setEntries([]);
        return;
      }
      setFailure("");
      setEntries(
        data.map((entry) => ({
          name: entry.Name ?? entry.Path ?? "",
          path: entry.Path ?? "",
        })),
      );
    })();
  }, [path]);

  const parent = path.replace(/\/[^/]*\/?$/, "");

  return (
    <div class="picker">
      <div class="actions">
        <b>{path || "Media folders"}</b>
        {path && <button onClick={() => choose(parent)}>Up</button>}
        <button onClick={close}>Done</button>
      </div>
      {failure && <p class="failure">{failure}</p>}
      {entries === undefined ? (
        <p class="muted">Loading…</p>
      ) : (
        <ul>
          {entries.map((entry) => (
            <li key={entry.path}>
              <button class="link" onClick={() => choose(entry.path)}>
                {entry.name}
              </button>
            </li>
          ))}
          {entries.length === 0 && !failure && (
            <li class="muted">No folders inside.</li>
          )}
        </ul>
      )}
    </div>
  );
}

function Editor({
  kind,
  library,
  variables,
  downloader,
  saved,
}: {
  kind: Kind;
  library?: Library;
  variables: string[];
  downloader?: Downloader;
  saved: () => Promise<void>;
}) {
  const [name, setName] = useState(downloader?.name ?? "");
  const [variable, setVariable] = useState(downloader?.variable ?? "");
  const [url, setUrl] = useState(downloader?.url ?? "");
  const [paths, setPaths] = useState<Folder[]>(
    downloader?.paths.length ? downloader.paths : [{ remote: "", local: "" }],
  );
  const [requests, setRequests] = useState<Requests | undefined>(
    downloader?.requests,
  );
  const [instance, setInstance] = useState<Instance | undefined>(undefined);
  const [browsing, setBrowsing] = useState<number | undefined>(undefined);
  const [failure, setFailure] = useState("");
  const [busy, setBusy] = useState(false);
  const what = kind === "Radarr" ? "movies" : "shows";

  async function test() {
    setInstance(await probe(kind, url.trim(), variable));
  }

  useEffect(() => {
    if (downloader) {
      void test();
    }
  }, []);

  function setPath(index: number, change: Partial<Folder>) {
    setPaths(
      paths.map((path, i) => (i === index ? { ...path, ...change } : path)),
    );
  }

  async function save() {
    if (mock.mocked) {
      return;
    }
    setBusy(true);
    const bindings = library
      ? [
          {
            Id: library.id,
            TagFilter:
              downloader?.bindings.find((binding) => binding.Id === library.id)
                ?.TagFilter ?? "",
          },
        ]
      : [];
    const body: Schemas["Source"] = {
      Name: name.trim(),
      Kind: kind,
      Url: url.trim(),
      ApiKeyVariable: variable,
      Paths: paths
        .filter((path) => path.remote || path.local)
        .map((path) => ({
          RemotePath: path.remote,
          LocalPath: path.local.trim(),
        })),
      Libraries: bindings,
      Requests: requests
        ? {
            QualityProfileId: requests.profileId,
            RootFolder: requests.folder,
            Tag: requests.tag || null,
          }
        : null,
    };
    const answer = downloader
      ? await api.PUT("/GoJellyfin/Sources/{sourceId}", {
          params: { path: { sourceId: downloader.id } },
          body,
        })
      : await api.POST("/GoJellyfin/Sources", { body });
    setBusy(false);
    if (!answer.response.ok) {
      setFailure(answer.error?.Message ?? "The downloader could not be saved.");
      return;
    }
    await saved();
    navigate("/libraries");
  }

  async function remove() {
    if (mock.mocked || !downloader) {
      return;
    }
    if (
      !window.confirm(
        "Delete " +
          downloader.name +
          "? Titles no other downloader holds leave " +
          (library?.name ?? "the library") +
          ".",
      )
    ) {
      return;
    }
    const { response } = await api.DELETE("/GoJellyfin/Sources/{sourceId}", {
      params: { path: { sourceId: downloader.id } },
    });
    if (!response.ok) {
      setFailure("The downloader could not be deleted.");
      return;
    }
    await saved();
    navigate("/libraries");
  }

  const folders = instance?.folders ?? [];
  const profileName = (id: number) =>
    instance?.profiles.find((profile) => profile.id === id)?.name ??
    "Profile " + id;

  return (
    <Page
      title={downloader ? downloader.name : "Add " + kind}
      lead={
        "Fills " +
        (library?.name ?? what) +
        ". Test the connection first, and the folder, profile and tag pickers fill in from what it answers."
      }
      action={
        <span class="actions">
          <Link to="/libraries" class="button">
            Cancel
          </Link>
          <button class="primary" disabled={busy} onClick={save}>
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
        <Card title="Connection">
          <div class="form">
            <label>
              Name
              <input
                value={name}
                placeholder={kind}
                onInput={(event) => setName(event.currentTarget.value)}
              />
            </label>
            <label>
              API key variable
              <select
                onChange={(event) => setVariable(event.currentTarget.value)}
              >
                <Options
                  options={variables}
                  value={variable}
                  empty={variable ? undefined : "Pick a variable"}
                />
              </select>
            </label>
            <label class="span">
              Address
              <input
                value={url}
                placeholder={
                  kind === "Radarr"
                    ? "http://radarr:7878"
                    : "http://sonarr:8989"
                }
                onInput={(event) => setUrl(event.currentTarget.value)}
              />
            </label>
          </div>
          {variables.length === 0 && (
            <p class="muted small">
              No SOURCE_API_KEY_ variable is set on this server. Set one to the
              instance's API key and restart it.
            </p>
          )}
          <div class="actions spaced">
            <button disabled={!url || !variable} onClick={test}>
              Test connection
            </button>
            {instance &&
              (instance.error ? (
                <span class="bad">
                  <Dot tone="bad" />
                  {instance.error}
                </span>
              ) : (
                <span class="ok">
                  <Dot tone="ok" />
                  {kind} {instance.version} answered
                </span>
              ))}
          </div>
        </Card>

        <Card title="Folders">
          <p class="muted small">
            Each root folder {kind} manages, and where this server finds the
            same files. Leave them equal when both see the same disk at the same
            path.
          </p>
          <table class="rows plain folders">
            <thead>
              <tr>
                <th style="width:32%">Root folder in {kind}</th>
                <th>Path on this server</th>
                <th style="width:100px" />
              </tr>
            </thead>
            <tbody>
              {paths.map((path, index) => (
                <tr key={index}>
                  <td>
                    <select
                      aria-label={"Root folder in " + kind}
                      onChange={(event) =>
                        setPath(index, { remote: event.currentTarget.value })
                      }
                    >
                      <Options
                        options={folders}
                        value={path.remote}
                        empty={
                          path.remote
                            ? undefined
                            : folders.length
                              ? "Pick a folder"
                              : "Test the connection to list folders"
                        }
                      />
                    </select>
                  </td>
                  <td>
                    <span class="browse">
                      <input
                        aria-label="Path on this server"
                        value={path.local}
                        placeholder="/media/…"
                        onInput={(event) =>
                          setPath(index, { local: event.currentTarget.value })
                        }
                      />
                      <button
                        disabled={mock.mocked}
                        onClick={() =>
                          setBrowsing(browsing === index ? undefined : index)
                        }
                      >
                        Browse
                      </button>
                    </span>
                    {browsing === index && (
                      <Browser
                        path={path.local}
                        choose={(local) => setPath(index, { local })}
                        close={() => setBrowsing(undefined)}
                      />
                    )}
                  </td>
                  <td class="end">
                    <button
                      disabled={paths.length === 1}
                      onClick={() =>
                        setPaths(paths.filter((_, i) => i !== index))
                      }
                    >
                      Remove
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <button
            class="spaced"
            onClick={() => setPaths([...paths, { remote: "", local: "" }])}
          >
            Add folder
          </button>
        </Card>

        <Card title="Requests">
          <label class="check">
            <input
              type="checkbox"
              checked={Boolean(requests)}
              onChange={(event) =>
                setRequests(
                  event.currentTarget.checked
                    ? {
                        profileId: instance?.profiles[0]?.id ?? 0,
                        folder: paths[0]?.remote ?? "",
                      }
                    : undefined,
                )
              }
            />{" "}
            Send {what} people ask for to this downloader
          </label>
          {requests && (
            <div class="form">
              <label>
                Quality profile
                <select
                  onChange={(event) =>
                    setRequests({
                      ...requests,
                      profileId: Number(event.currentTarget.value),
                    })
                  }
                >
                  {!instance?.profiles.some(
                    (profile) => profile.id === requests.profileId,
                  ) && (
                    <option value={requests.profileId} selected>
                      {requests.profileId
                        ? (requests.profile ?? profileName(requests.profileId))
                        : "Pick a profile"}
                    </option>
                  )}
                  {instance?.profiles.map((profile) => (
                    <option
                      key={profile.id}
                      value={profile.id}
                      selected={profile.id === requests.profileId}
                    >
                      {profile.name}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Add under
                <select
                  onChange={(event) =>
                    setRequests({
                      ...requests,
                      folder: event.currentTarget.value,
                    })
                  }
                >
                  <Options
                    options={paths.map((path) => path.remote).filter(Boolean)}
                    value={requests.folder}
                    empty={requests.folder ? undefined : "Pick a folder"}
                  />
                </select>
              </label>
              <label>
                Tag requested titles
                <select
                  onChange={(event) =>
                    setRequests({
                      ...requests,
                      tag: event.currentTarget.value || undefined,
                    })
                  }
                >
                  <Options
                    options={instance?.tags ?? []}
                    value={requests.tag}
                    empty="No tag"
                  />
                </select>
              </label>
            </div>
          )}
        </Card>

        {downloader && (
          <Card title="Remove">
            <p class="muted">
              Deleting removes every file record this downloader reported.
              Titles no other downloader holds leave{" "}
              {library?.name ?? "the library"}, along with their watch history.
            </p>
            <button class="bad" onClick={remove}>
              Delete downloader
            </button>
          </Card>
        )}
      </div>
    </Page>
  );
}
