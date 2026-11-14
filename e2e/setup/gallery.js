const fs = require('node:fs');
const path = require('node:path');

// Turns a run's results into a directory GitHub Pages can serve and the
// comment the PR carries: every screenshot as this commit renders it, and for
// each one that changed, the baseline, what was rendered and the diff.
//
//   node setup/gallery.js <results.json> <out dir> <url the out dir is served at> <sha>

const MARKER = '<!-- e2e-screenshots -->';
const BASELINES = path.join(__dirname, '..', 'tests', '__screenshots__');
const SHOT = /^(.*)-(expected|actual|diff)\.png$/;

function main([resultsPath, out, url, sha]) {
  const results = JSON.parse(fs.readFileSync(resultsPath, 'utf8'));
  const tests = collect(results);
  const shots = screenshots(tests);

  fs.mkdirSync(out, { recursive: true });
  for (const shot of shots) {
    for (const [kind, from] of Object.entries(shot.files)) {
      copy(from, path.join(out, shot.dir, `${shot.name}-${kind}.png`));
    }
  }

  const summary = { stats: results.stats, tests, shots, url: url.replace(/\/$/, ''), sha };
  fs.writeFileSync(path.join(out, 'index.html'), page(summary));
  fs.writeFileSync(path.join(out, 'comment.md'), comment(summary));
}

function collect(results) {
  const tests = [];
  const walk = (suite, titles) => {
    for (const spec of suite.specs || []) {
      for (const test of spec.tests) {
        const last = test.results[test.results.length - 1] || { status: 'skipped', attachments: [] };
        tests.push({
          file: spec.file,
          title: [...titles, spec.title].join(' › '),
          status: test.status,
          attachments: last.attachments,
        });
      }
    }
    for (const child of suite.suites || []) {
      walk(child, [...titles, child.title]);
    }
  };
  for (const suite of results.suites) {
    walk(suite, []);
  }
  return tests;
}

function screenshots(tests) {
  const shots = new Map();
  const shot = (file, name) => {
    const key = `${file}/${name}`;
    if (!shots.has(key)) {
      shots.set(key, { key, dir: path.join('shots', file), name, title: '', files: {} });
    }
    return shots.get(key);
  };

  for (const file of listDir(BASELINES)) {
    for (const image of listDir(path.join(BASELINES, file))) {
      if (image.endsWith('.png')) {
        shot(file, image.slice(0, -4)).files.current = path.join(BASELINES, file, image);
      }
    }
  }

  for (const test of tests) {
    for (const attachment of test.attachments) {
      const match = attachment.path && SHOT.exec(attachment.name);
      if (!match || !fs.existsSync(attachment.path)) {
        continue;
      }
      const [, name, kind] = match;
      const entry = shot(test.file, name);
      entry.title = test.title;
      entry.files[kind] = attachment.path;
      if (kind === 'actual') {
        entry.files.current = attachment.path;
      }
    }
  }

  return [...shots.values()]
    .map((entry) => ({ ...entry, state: state(entry.files) }))
    .sort((a, b) => a.key.localeCompare(b.key));
}

function state(files) {
  if (files.diff) {
    return 'changed';
  }
  if (files.actual) {
    return 'new';
  }
  return 'unchanged';
}

function comment({ stats, shots, url, sha }) {
  const changed = shots.filter((shot) => shot.state !== 'unchanged');
  const failed = stats.unexpected;
  const headline = failed
    ? `❌ ${failed} failed, ${stats.expected} passed${stats.flaky ? `, ${stats.flaky} flaky` : ''}`
    : `✅ ${stats.expected} passed${stats.flaky ? `, ${stats.flaky} flaky` : ''}`;

  const lines = [
    MARKER,
    `### E2E screenshots: ${headline}`,
    '',
    `Rendered at ${sha.slice(0, 7)}. [Gallery](${url}/) · [Playwright report](${url}/report/)`,
    '',
  ];

  if (changed.length) {
    lines.push(`#### ${changed.length} screenshot${changed.length === 1 ? ' differs' : 's differ'} from the baseline`, '');
    for (const shot of changed) {
      const src = (kind) => `${url}/${shot.dir}/${shot.name}-${kind}.png`;
      lines.push(`**${shot.key}.png** (${shot.state})`, '');
      if (shot.state === 'new') {
        lines.push(`<img src="${src('actual')}" width="420">`, '');
        continue;
      }
      lines.push(
        '| Baseline | This commit | Diff |',
        '| --- | --- | --- |',
        `| <img src="${src('expected')}" width="280"> | <img src="${src('actual')}" width="280"> | <img src="${src('diff')}" width="280"> |`,
        '',
      );
    }
    lines.push(
      'If the change is intended, run `make e2e-update` and commit the new baselines.',
      '',
    );
  }

  lines.push(
    `<details><summary>All ${shots.length} screenshots</summary>`,
    '',
    ...shots.map(
      (shot) => `**${shot.key}.png**<br><img src="${url}/${shot.dir}/${shot.name}-current.png" width="420">\n`,
    ),
    '</details>',
    '',
  );

  return lines.join('\n');
}

function page({ stats, tests, shots, sha }) {
  const escape = (text) => text.replace(/[&<>"]/g, (c) => `&#${c.charCodeAt(0)};`);
  const figure = (shot) => {
    const src = (kind) => `${shot.dir}/${shot.name}-${kind}.png`;
    const images =
      shot.state === 'changed'
        ? ['expected', 'actual', 'diff'].map((kind) => `<a href="${src(kind)}"><img src="${src(kind)}" alt="${kind}"></a>`)
        : [`<a href="${src('current')}"><img src="${src('current')}" alt=""></a>`];
    return `<section class="${shot.state}"><h2>${escape(shot.key)}.png <span>${shot.state}</span></h2><div class="row">${images.join('')}</div></section>`;
  };
  const failing = tests.filter((test) => test.status === 'unexpected');

  return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>E2E screenshots ${sha.slice(0, 7)}</title>
<style>
  :root { color-scheme: light dark; --bg: #fff; --fg: #1f2328; --muted: #59636e; --line: #d1d9e0; --bad: #cf222e; --new: #9a6700; }
  @media (prefers-color-scheme: dark) { :root { --bg: #0d1117; --fg: #e6edf3; --muted: #9198a1; --line: #3d444d; --bad: #f85149; --new: #d29922; } }
  body { margin: 0 auto; max-width: 1400px; padding: 24px 16px; background: var(--bg); color: var(--fg); font: 14px/1.5 system-ui, sans-serif; }
  h1 { font-size: 20px; margin: 0 0 4px; }
  p { color: var(--muted); margin: 0 0 24px; }
  section { border-top: 1px solid var(--line); padding: 16px 0; }
  h2 { font-size: 15px; margin: 0 0 12px; font-family: ui-monospace, monospace; }
  h2 span { font-family: system-ui, sans-serif; font-size: 12px; color: var(--muted); margin-left: 8px; }
  .changed h2 span { color: var(--bad); }
  .new h2 span { color: var(--new); }
  .row { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 12px; }
  img { width: 100%; height: auto; border: 1px solid var(--line); display: block; }
  li { font-family: ui-monospace, monospace; }
</style>
</head>
<body>
<h1>E2E screenshots at ${sha.slice(0, 7)}</h1>
<p>${stats.expected} passed, ${stats.unexpected} failed, ${stats.flaky} flaky. <a href="report/">Playwright report</a></p>
${failing.length ? `<ul>${failing.map((test) => `<li>${escape(test.file)} › ${escape(test.title)}</li>`).join('')}</ul>` : ''}
${shots.filter((shot) => shot.state !== 'unchanged').map(figure).join('\n')}
${shots.filter((shot) => shot.state === 'unchanged').map(figure).join('\n')}
</body>
</html>
`;
}

function listDir(dir) {
  return fs.existsSync(dir) ? fs.readdirSync(dir).sort() : [];
}

function copy(from, to) {
  fs.mkdirSync(path.dirname(to), { recursive: true });
  fs.copyFileSync(from, to);
}

module.exports = { MARKER };

if (require.main === module) {
  main(process.argv.slice(2));
}
