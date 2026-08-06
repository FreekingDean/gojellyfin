#!/usr/bin/env python3
"""Spec coverage: which operations have a handler.

    scripts/coverage.py            markdown table of every tag
    scripts/coverage.py --missing  operations with no handler, grouped by tag
    scripts/coverage.py --tag User operations with no handler for one tag

A handler counts as implemented when it exists, regardless of how much it does;
several return empty results because nothing backs them yet.
"""

import collections
import glob
import json
import re
import sys

SPEC = 'spec/jellyfin-openapi-10.10.0.json'
HANDLERS = 'internal/server/**/*.go'

# Recorded in CLAUDE.md: the web client does not need these to browse and play
# a library, and the generated Unimplemented base already answers 501.
NOT_PLANNED = {
    'LiveTv', 'SyncPlay', 'Trickplay', 'Lyrics', 'Tmdb', 'MediaSegments',
    'VideoAttachments',
}


def operations():
    doc = json.load(open(SPEC))
    tags = collections.defaultdict(dict)
    for path, item in doc['paths'].items():
        for method, op in item.items():
            if not isinstance(op, dict) or not op.get('operationId') or not op.get('tags'):
                continue
            tags[op['tags'][0]][op['operationId']] = '%s %s' % (method.upper(), path)
    return tags


def implemented():
    found = set()
    for path in glob.glob(HANDLERS, recursive=True):
        if '/api/' in path:
            continue
        source = open(path).read()
        for match in re.finditer(r'func \(s \*Server\) (\w+)\(ctx context\.Context, request api\.', source):
            found.add(match.group(1).lower())
    return found


def missing(tags, done):
    out = collections.OrderedDict()
    for tag in sorted(tags):
        gaps = {op: route for op, route in tags[tag].items()
                if op.replace('_', '').lower() not in done}
        if gaps:
            out[tag] = gaps
    return out


def table(tags, done):
    rows = sorted(
        ((tag, sum(1 for op in ops if op.replace('_', '').lower() in done), len(ops))
         for tag, ops in tags.items()),
        key=lambda row: (-row[1], -row[2], row[0]),
    )
    total_done = sum(row[1] for row in rows)
    total = sum(row[2] for row in rows)
    planned = sum(row[2] for row in rows if row[0] not in NOT_PLANNED)

    lines = [
        '`%d/%d` operations have a handler (`%d` in tags we intend to support).'
        % (total_done, total, planned),
        '',
        'Regenerate with `scripts/coverage.py`.',
        '',
        '| Tag | Done | Total | |',
        '| --- | ---: | ----: | --- |',
    ]
    for tag, have, count in rows:
        if tag in NOT_PLANNED:
            note = 'not planned'
        elif have == count:
            note = 'complete'
        elif have:
            note = 'partial'
        else:
            note = ''
        lines.append('| %s | %d | %d | %s |' % (tag, have, count, note))

    lines += ['', '**Not planned:** ' + ', '.join(sorted(NOT_PLANNED)) + '.']
    return '\n'.join(lines)


def main():
    tags, done = operations(), implemented()

    if '--tag' in sys.argv:
        tag = sys.argv[sys.argv.index('--tag') + 1]
        for op, route in sorted(missing(tags, done).get(tag, {}).items()):
            print('%-40s %s' % (op, route))
    elif '--missing' in sys.argv:
        for tag, gaps in missing(tags, done).items():
            print('%s (%d)' % (tag, len(gaps)))
            for op, route in sorted(gaps.items()):
                print('  %-38s %s' % (op, route))
    else:
        print(table(tags, done))


if __name__ == '__main__':
    main()
