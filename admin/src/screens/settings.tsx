import * as mock from '../mock';
import { Card, Cell, Page, Pill, since } from '../ui';

export function Settings() {
  return (
    <Page title="Settings" lead="What this server is running with. These are set in the environment, so they are shown here and changed in your deployment.">
      <div class="grid two">
        {mock.settings.map((group) => (
          <Card key={group.title}>
            <header class="cardhead">
              <b>{group.title}</b>
              {group.state && <Pill tone={group.state === 'On' ? 'ok' : 'off'}>{group.state}</Pill>}
            </header>
            {group.why && <p class="muted">{group.why}</p>}
            {group.settings.length > 0 && (
              <table class="rows plain settings">
                <tbody>
                  {group.settings.map((setting) => (
                    <tr key={setting.label}>
                      <td class="muted">{setting.label}</td>
                      <td>
                        <div class="line">{setting.value}</div>
                        <code class="small">{setting.variable}</code>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Card>
        ))}

        <Card title="Branding">
          <div class="form one">
            <label>
              Sign-in message
              <textarea rows={2}>{mock.branding.disclaimer}</textarea>
            </label>
            <label>
              Custom CSS for the web client
              <textarea rows={3} class="mono">
                {mock.branding.css}
              </textarea>
            </label>
          </div>
          <div class="actions spaced">
            <button class="primary">Save branding</button>
          </div>
        </Card>

        <Card>
          <header class="cardhead">
            <b>API keys</b>
            <button>New key</button>
          </header>
          <table class="rows plain">
            <tbody>
              {mock.apiKeys.map((key) => (
                <tr key={key.name}>
                  <Cell main={key.name} sub={'Created ' + since(key.created)} />
                  <td class="muted">{key.used ? 'Used ' + since(key.used) : 'Never used'}</td>
                  <td class="end">
                    <button>Revoke</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      </div>
    </Page>
  );
}
