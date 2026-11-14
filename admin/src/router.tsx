import type { ComponentChildren } from 'preact';
import { useEffect, useState } from 'preact/hooks';

const BASE = '/admin';

function current(): string[] {
  return location.pathname.slice(BASE.length).split('/').filter(Boolean);
}

export function navigate(path: string): void {
  history.pushState(null, '', BASE + path + location.search);
  dispatchEvent(new PopStateEvent('popstate'));
}

export function useRoute(): string[] {
  const [route, setRoute] = useState(current());

  useEffect(() => {
    const moved = () => setRoute(current());
    addEventListener('popstate', moved);

    return () => removeEventListener('popstate', moved);
  }, []);

  return route;
}

export function Link({ to, class: className, children }: { to: string; class?: string; children: ComponentChildren }) {
  function follow(event: MouseEvent) {
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.button !== 0) {
      return;
    }
    event.preventDefault();
    navigate(to);
  }

  return (
    <a class={className} href={BASE + to + location.search} onClick={follow}>
      {children}
    </a>
  );
}
