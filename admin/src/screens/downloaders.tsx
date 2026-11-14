import * as mock from '../mock';
import { Link } from '../router';
import { Card, Dot, Field, Page, Pill, since } from '../ui';

export function Downloaders({ id }: { id?: string }) {
  const editing = mock.downloaders.find((downloader) => downloader.id === id);
  if (editing || id === 'new') {
    return <Editor downloader={editing} />;
  }

  return (
    <Page
      title="Downloaders"
      lead="Sonarr and Radarr instances that fill your libraries and take requests."
      action={
        <Link to="/downloaders/new" class="button primary">
          Add downloader
        </Link>
      }
    >
      <div class="grid two">
        {mock.downloaders.map((downloader) => (
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
                <Link to={'/downloaders/' + downloader.id} class="button">
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
                    {path.remote} → {path.local}
                  </div>
                ))}
              </Field>
              <Field label="Feeds">
                {downloader.feeds.map((feed) => (
                  <div key={feed.library}>
                    {feed.library}{' '}
                    <span class="muted">
                      {feed.tag ? 'tag ' + feed.tag : 'all titles'} · synced {since(feed.synced)} · {feed.files.toLocaleString()} files
                    </span>
                  </div>
                ))}
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
        <div class="card hint">Keys are never stored here. A downloader names an environment variable starting with SOURCE_API_KEY_, and the server reads the key from it.</div>
      </div>
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

function Editor({ downloader }: { downloader?: mock.Downloader }) {
  return (
    <Page
      title={downloader ? downloader.name : 'Add downloader'}
      lead="Test the connection first. The folder, profile and tag pickers fill in from what the instance answers."
      action={
        <span class="actions">
          <Link to="/downloaders" class="button">
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
              <input value={downloader?.name ?? ''} />
            </label>
            <label>
              Kind
              <Select options={['Radarr', 'Sonarr']} value={downloader?.kind} />
            </label>
            <label>
              Address
              <input value={downloader?.url ?? ''} placeholder="http://radarr:7878" />
            </label>
            <label>
              API key variable
              <Select options={mock.pickers.variables} value={downloader?.variable} />
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

        <Card title="Requests">
          <label class="check">
            <input type="checkbox" checked={Boolean(downloader?.requests)} /> Send titles people ask for to this downloader
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

        <Card title="Folders">
          <p class="muted small">Where the downloader says a file is, and where this server opens it.</p>
          <table class="rows plain">
            <thead>
              <tr>
                <th>On the downloader</th>
                <th>On this server</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {(downloader?.paths ?? [{ remote: '', local: '' }]).map((path) => (
                <tr key={path.remote}>
                  <td>
                    <Select options={mock.pickers.remoteFolders} value={path.remote} />
                  </td>
                  <td>
                    <Select options={mock.pickers.localFolders} value={path.local} />
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

        <Card title="Libraries it feeds">
          <p class="muted small">A tag takes only the titles carrying it. No tag takes everything the downloader manages.</p>
          <table class="rows plain">
            <thead>
              <tr>
                <th>Library</th>
                <th>Only titles tagged</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {(downloader?.feeds ?? [{ library: '', tag: undefined }]).map((feed) => (
                <tr key={feed.library}>
                  <td>
                    <Select options={mock.libraries.map((library) => library.name)} value={feed.library} />
                  </td>
                  <td>
                    <Select options={mock.pickers.tags} value={feed.tag} empty="Everything" />
                  </td>
                  <td class="end">
                    <button>Remove</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <button class="spaced">Add library</button>
        </Card>
      </div>
      {downloader && (
        <p class="danger">
          <button class="bad">Delete downloader</button>
          <span class="muted small">Deleting removes every file record it reported. Titles nothing else holds leave the libraries.</span>
        </p>
      )}
    </Page>
  );
}
