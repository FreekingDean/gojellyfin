const fs = require('node:fs');
const path = require('node:path');

// Turns each suite's results into a directory GitHub Pages can serve and the
// comment the PR carries: every screenshot as this commit renders it, and for
// each one that changed, the baseline, what was rendered and the diff.
//
// A suite runs in its own job, so the work is split where the jobs are. Each
// collects its own screenshots beside a summary, and one render reads every
// summary it finds and writes the page and the comment over all of them.
//
//   node setup/gallery.js collect <slug> <title> <results.json> <out dir> [baselines dir]
//   node setup/gallery.js render <dir> <url the dir is served at> <sha>

const MARKER = '<!-- e2e-screenshots -->';
const SHOT = /^(.*)-(expected|actual|diff|current)\.png$/;

function collect([slug, title, resultsPath, out, baselines]) {
  const results = JSON.parse(fs.readFileSync(resultsPath, 'utf8'));
  const tests = walk(results);
  const shots = screenshots(slug, tests, baselines);

  fs.mkdirSync(out, { recursive: true });
  for (const shot of shots) {
    for (const [kind, from] of Object.entries(shot.files)) {
      copy(from, path.join(out, shot.dir, `${shot.name}-${kind}.png`));
    }
  }

  const summary = {
    slug,
    title,
    stats: results.stats,
    failing: tests.filter((test) => test.status === 'unexpected').map((test) => `${test.file} › ${test.title}`),
    shots: shots.map(({ files, ...shot }) => shot),
  };
  fs.writeFileSync(path.join(out, `${slug}.json`), JSON.stringify(summary, null, 2));
}

function render([dir, url, sha]) {
  const suites = fs
    .readdirSync(dir)
    .filter((name) => name.endsWith('.json'))
    .sort()
    .map((name) => JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8')))
    .sort((a, b) => Number(b.slug === 'web') - Number(a.slug === 'web'));
  if (!suites.length) {
    throw new Error(`${dir} holds no suite summary to render`);
  }

  const summary = {
    suites,
    url: url.replace(/\/$/, ''),
    sha,
    report: fs.existsSync(path.join(dir, 'report', 'index.html')),
  };
  fs.writeFileSync(path.join(dir, 'index.html'), page(summary));
  fs.writeFileSync(path.join(dir, 'comment.md'), comment(summary));
}

function walk(results) {
  const tests = [];
  const visit = (suite, titles) => {
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
      visit(child, [...titles, child.title]);
    }
  };
  for (const suite of results.suites) {
    visit(suite, []);
  }
  return tests;
}

function screenshots(slug, tests, baselines) {
  const shots = new Map();
  const shot = (file, name) => {
    const key = `${slug}/${file}/${name}`;
    if (!shots.has(key)) {
      shots.set(key, { key, dir: path.join('shots', slug, file), name, title: '', files: {} });
    }
    return shots.get(key);
  };

  for (const file of listDir(baselines)) {
    for (const image of listDir(path.join(baselines, file))) {
      if (image.endsWith('.png')) {
        shot(file, image.slice(0, -4)).files.current = path.join(baselines, file, image);
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

function headline({ title, stats }) {
  const flaky = stats.flaky ? `, ${stats.flaky} flaky` : '';

  return stats.unexpected
    ? `❌ ${title}: ${stats.unexpected} failed, ${stats.expected} passed${flaky}`
    : `✅ ${title}: ${stats.expected} passed${flaky}`;
}

function comment({ suites, url, sha, report }) {
  const changed = suites.flatMap((suite) => suite.shots).filter((shot) => shot.state !== 'unchanged');

  const lines = [
    MARKER,
    `### E2E screenshots: ${suites.map(headline).join(' · ')}`,
    '',
    `Rendered at ${sha.slice(0, 7)}. [Gallery](${url}/)${report ? ` · [Playwright report](${url}/report/)` : ''}`,
    '',
  ];

  for (const suite of suites) {
    if (suite.failing.length) {
      lines.push(`**${suite.title} failed:**`, '', ...suite.failing.map((test) => `- ${test}`), '');
    }
  }

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

  for (const suite of suites) {
    lines.push(
      `<details><summary>${suite.title}: all ${suite.shots.length} screenshots</summary>`,
      '',
      ...suite.shots.map(
        (shot) => `**${shot.key}.png**<br><img src="${url}/${shot.dir}/${shot.name}-current.png" width="420">\n`,
      ),
      '</details>',
      '',
    );
  }

  return lines.join('\n');
}

function page({ suites, sha, report }) {
  const escape = (text) => text.replace(/[&<>"]/g, (c) => `&#${c.charCodeAt(0)};`);
  const figure = (shot) => {
    const src = (kind) => `${shot.dir}/${shot.name}-${kind}.png`;
    const images =
      shot.state === 'changed'
        ? ['expected', 'actual', 'diff'].map((kind) => `<a href="${src(kind)}"><img src="${src(kind)}" alt="${kind}"></a>`)
        : [`<a href="${src('current')}"><img src="${src('current')}" alt=""></a>`];
    return `<section class="${shot.state}"><h2>${escape(shot.key)}.png <span>${shot.state}</span></h2><div class="row">${images.join('')}</div></section>`;
  };
  const section = (suite) => `<h1>${escape(suite.title)}</h1>
<p>${suite.sha ? `At ${suite.sha.slice(0, 7)}, ${escape(suite.at)}. ` : ''}${suite.stats.expected} passed, ${suite.stats.unexpected} failed, ${suite.stats.flaky} flaky.${suite.slug === 'web' && report ? ' <a href="report/">Playwright report</a>' : ''}</p>
${suite.failing.length ? `<ul>${suite.failing.map((test) => `<li>${escape(test)}</li>`).join('')}</ul>` : ''}
${suite.shots.filter((shot) => shot.state !== 'unchanged').map(figure).join('\n')}
${suite.shots.filter((shot) => shot.state === 'unchanged').map(figure).join('\n')}`;

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
  h1 { font-size: 20px; margin: 24px 0 4px; }
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
<p>E2E screenshots at ${sha.slice(0, 7)}</p>
${suites.map(section).join('\n')}
</body>
</html>
`;
}

function listDir(dir) {
  return dir && fs.existsSync(dir) ? fs.readdirSync(dir).sort() : [];
}

function copy(from, to) {
  fs.mkdirSync(path.dirname(to), { recursive: true });
  fs.copyFileSync(from, to);
}

module.exports = { MARKER };

if (require.main === module) {
  const [command, ...args] = process.argv.slice(2);
  const commands = { collect, render };
  if (!commands[command]) {
    console.error('usage: gallery.js collect <slug> <title> <results.json> <out> [baselines] | render <dir> <url> <sha>');
    process.exit(2);
  }
  commands[command](args);
}
