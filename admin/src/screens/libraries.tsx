import * as mock from '../mock';
import { Cell, Page, Pill, since } from '../ui';

export function Libraries() {
  return (
    <Page title="Libraries" lead="What people browse. Each is filled by the downloaders bound to it." action={<button class="primary">Add library</button>}>
      <div class="card flush">
        <table class="rows">
          <colgroup>
            <col style="width:18%" />
            <col style="width:24%" />
            <col style="width:22%" />
            <col style="width:10%" />
            <col style="width:26%" />
          </colgroup>
          <thead>
            <tr>
              <th>Library</th>
              <th>Folders</th>
              <th>Filled by</th>
              <th>Titles</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {mock.libraries.map((library) => (
              <tr key={library.id}>
                <Cell main={library.name} sub={library.type} />
                <Cell main={library.folders[0]} sub={library.folders.length > 1 ? 'and ' + library.folders.slice(1).join(', ') : undefined} />
                {library.fedBy.length === 0 ? (
                  <Cell main={<Pill tone="warn">No downloader</Pill>} sub="Bind one in Downloaders to fill it" />
                ) : (
                  <Cell
                    main={library.fedBy.join(', ')}
                    sub={library.error ?? (library.synced ? 'synced ' + since(library.synced) : undefined)}
                    tone={library.error ? 'warn' : undefined}
                  />
                )}
                <td>{library.titles.toLocaleString()}</td>
                <td>
                  <div class="actions right">
                    <button disabled={library.fedBy.length === 0}>Refresh now</button>
                    <button>Rename</button>
                    <button>Delete</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p class="muted small">Deleting a library removes the titles no other library holds. Watch state for those titles goes with them.</p>
    </Page>
  );
}
