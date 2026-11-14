import * as mock from '../mock';
import { Link } from '../router';
import { Card, Dot, Field, Page, Pill, since } from '../ui';

export function Libraries({ id }: { id?: string }) {
  const editing = mock.downloaders.find((downloader) => downloader.id === id);
  if (editing) {
    return <Editor kind={editing.kind} downloader={editing} />;
  }
  const adding = mock.libraries.find((library) => 'new-' + library.id === id);
  if (adding) {
    return <Editor kind={adding.kind} />;
  }

  return (
    <Page title="Libraries" lead="Movies come from Radarr and Shows from Sonarr. Add a downloader and its folders, and the library fills itself.">
      {mock.libraries.map((library) => {
        const feeding = mock.downloaders.filter((downloader) => downloader.kind === library.kind);

        return (
          <section class="library" key={library.id}>
            <header class="pagehead">
              <div>
                <h2>{library.name}</h2>
                <p class="muted">
                  {library.counts} · refreshed {since(library.synced)}
                </p>
              </div>
              <span class="actions">
                <button disabled={feeding.length === 0}>Refresh now</button>
                <Link to={'/libraries/new-' + library.id} class="button primary">
                  Add {library.kind}
                </Link>
              </span>
            </header>

            {feeding.length === 0 && (
              <div class="card hint">
                No {library.kind} yet. Add one and {library.name} appears for everyone who can see it.
              </div>
            )}

            <div class="grid two">
              {feeding.map((downloader) => (
                <Card key={downloader.id} class={downloader.error ? 'troubled' : ''}>
                  <header class="cardhead">
                    <b>
                      <Dot tone={downloader.error ? 'warn' : 'ok'} />
                      {downloader.name}
                    </b>
                    <span class="actions">
                      <Pill>
                        {downloader.kind} {downloader.version}
                      </Pill>
                      <button>Test</button>
                      <Link to={'/libraries/' + downloader.id} class="button">
                        Edit
                      </Link>
                    </span>
                  </header>
                  <dl class="fields">
                    <Field label="Address">{downloader.url}</Field>
                    <Field label="API key">
                      <code>{downloader.variable}</code> set
                    </Field>
                    <Field label="Last answered">
                      {downloader.error ? (
                        <span class="warn">
                          {since(downloader.contacted)} · {downloader.error}
                        </span>
                      ) : (
                        since(downloader.contacted)
                      )}
                    </Field>
                    <Field label="Folders">
                      {downloader.paths.map((path) => (
                        <div key={path.remote}>
                          {path.remote} <span class="muted">opened here as</span> {path.local}
                        </div>
                      ))}
                    </Field>
                    <Field label="Files">
                      {downloader.files.toLocaleString()} <span class="muted">· synced {since(downloader.synced)}</span>
                    </Field>
                    <Field label="Requests">
                      {downloader.requests ? (
                        <>
                          <Pill tone="ok">On</Pill> {downloader.requests.profile} into {downloader.requests.folder}
                          {downloader.requests.tag && (
                            <>
                              , tag <code>{downloader.requests.tag}</code>
                            </>
                          )}
                        </>
                      ) : (
                        <>
                          <Pill>Off</Pill> <span class="muted">Pick a quality profile to take requests</span>
                        </>
                      )}
                    </Field>
                  </dl>
                </Card>
              ))}
            </div>
          </section>
        );
      })}
      <p class="muted small">
        Two downloaders holding the same title show it once, with a version from each. Keys are never stored here: a downloader names an environment
        variable starting with SOURCE_API_KEY_.
      </p>
    </Page>
  );
}

function Select({ options, value, empty }: { options: string[]; value?: string; empty?: string }) {
  return (
    <select>
      {empty && <option selected={!value}>{empty}</option>}
      {options.map((option) => (
        <option key={option} selected={option === value}>
          {option}
        </option>
      ))}
    </select>
  );
}

function Editor({ kind, downloader }: { kind: mock.Downloader['kind']; downloader?: mock.Downloader }) {
  const library = mock.libraries.find((each) => each.kind === kind);

  return (
    <Page
      title={downloader ? downloader.name : 'Add ' + kind}
      lead={'Fills ' + (library?.name ?? '') + '. Test the connection first, and the folder, profile and tag pickers fill in from what it answers.'}
      action={
        <span class="actions">
          <Link to="/libraries" class="button">
            Cancel
          </Link>
          <button class="primary">Save</button>
        </span>
      }
    >
      <div class="grid two">
        <Card title="Connection">
          <div class="form">
            <label>
              Name
              <input value={downloader?.name ?? ''} placeholder={kind} />
            </label>
            <label>
              API key variable
              <Select options={mock.pickers.variables} value={downloader?.variable} />
            </label>
            <label class="span">
              Address
              <input value={downloader?.url ?? ''} placeholder={kind === 'Radarr' ? 'http://radarr:7878' : 'http://sonarr:8989'} />
            </label>
          </div>
          <div class="actions spaced">
            <button>Test connection</button>
            {downloader &&
              (downloader.error ? (
                <span class="bad">
                  <Dot tone="bad" />
                  {downloader.error}
                </span>
              ) : (
                <span class="ok">
                  <Dot tone="ok" />
                  {downloader.kind} {downloader.version} answered
                </span>
              ))}
          </div>
        </Card>

        <Card title="Folders">
          <p class="muted small">
            Each root folder {kind} manages, and where this server finds the same files. Leave them equal when both see the same disk at the same path.
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
              {(downloader?.paths ?? [{ remote: '', local: '' }]).map((path) => (
                <tr key={path.remote}>
                  <td>
                    <Select options={mock.pickers.remoteFolders} value={path.remote} />
                  </td>
                  <td>
                    <span class="browse">
                      <input value={path.local} placeholder="/media/…" />
                      <button>Browse</button>
                    </span>
                  </td>
                  <td class="end">
                    <button>Remove</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <button class="spaced">Add folder</button>
        </Card>

        <Card title="Requests">
          <label class="check">
            <input type="checkbox" checked={Boolean(downloader?.requests)} /> Send {kind === 'Radarr' ? 'movies' : 'shows'} people ask for to this downloader
          </label>
          <div class="form">
            <label>
              Quality profile
              <Select options={mock.pickers.profiles} value={downloader?.requests?.profile} />
            </label>
            <label>
              Add under
              <Select options={mock.pickers.remoteFolders} value={downloader?.requests?.folder} />
            </label>
            <label>
              Tag requested titles
              <Select options={mock.pickers.tags} value={downloader?.requests?.tag} empty="No tag" />
            </label>
          </div>
        </Card>

        {downloader && (
          <Card title="Remove">
            <p class="muted">Deleting removes every file record this downloader reported. Titles no other downloader holds leave {library?.name}, along with their watch history.</p>
            <button class="bad">Delete downloader</button>
          </Card>
        )}
      </div>
    </Page>
  );
}
