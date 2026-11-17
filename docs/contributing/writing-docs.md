# Writing these docs

This site is built with [MkDocs](https://www.mkdocs.org/) and the
[Material theme](https://squidfunk.github.io/mkdocs-material/) from the
Markdown files in `docs/`. Changing the site means changing those files and
nothing else.

## Adding a page

1. Create a `.md` file anywhere under `docs/`. A folder becomes a section of
   the navigation, so `docs/setup/install.md` appears under **Setup**.
2. Start the file with a single `#` heading. That heading is the page's title
   in the navigation and in search.
3. Open a pull request. There is no navigation list to edit: every file under
   `docs/` is in the site automatically.

Pages in a section are listed alphabetically by file name. A folder's
`index.md` is that section's own page and is listed first.

## Linking

Link to another page by its path relative to the current file, including the
`.md` extension: `[Installing](../setup/install.md)`, or
`[a heading](install.md#upgrading)` for a section of it. MkDocs turns those
into the right URLs, and the build fails if the file or the heading does not
exist.

Link to anything outside the docs, including files elsewhere in the
repository, with a full `https://` URL.

Images go beside the page that uses them and are linked the same way:
`![The home screen](home.png)`.

## What not to put here

Nothing about a real library or server: no titles, paths, hostnames,
addresses, ids, screenshots or log excerpts from a deployed install. Use
visibly invented examples such as `example.com`, `192.0.2.10` and
`Example Movie (2000)`.

## Checking a change

Every pull request builds the site with `mkdocs build --strict`, which fails
on a broken link, a link to a heading that does not exist, or a page Markdown
could not read. To see the site locally before pushing:

```sh
python3 -m venv .venv
.venv/bin/pip install -r requirements-docs.txt
.venv/bin/mkdocs serve
```

and open <http://127.0.0.1:8000/>.

## Publishing

Every merge to `main` that touches `docs/`, `mkdocs.yml` or
`requirements-docs.txt` publishes the site to
<https://freekingdean.github.io/gojellyfin/>. It shares that address with the
end-to-end screenshot gallery, which keeps `/main/` and `/pr-<number>/`, so a
page may not be called `main.md` or `pr-<anything>.md` at the top of `docs/`.

## Formatting extras

Besides ordinary Markdown, these work:

```markdown
!!! note
    An admonition. `warning`, `tip` and `danger` work too.

??? note "Collapsed by default"
    Click to open.

=== "Docker"
    One tab.

=== "Helm"
    Another tab.
```
