import type { ComponentChildren } from 'preact';
import { useEffect, useState } from 'preact/hooks';

export type Tone = 'ok' | 'warn' | 'bad' | 'off';

export function Page({ title, lead, action, children }: { title: string; lead: string; action?: ComponentChildren; children: ComponentChildren }) {
  return (
    <section>
      <header class="pagehead">
        <div>
          <h1>{title}</h1>
          <p class="muted lead">{lead}</p>
        </div>
        {action}
      </header>
      {children}
    </section>
  );
}

export function Pill({ tone, children }: { tone?: Tone; children: ComponentChildren }) {
  return <span class={'pill ' + (tone ?? 'off')}>{children}</span>;
}

export function Dot({ tone }: { tone: Tone }) {
  return <span class={'dot ' + tone} />;
}

export function Cell({ main, sub, tone }: { main: ComponentChildren; sub?: ComponentChildren; tone?: Tone }) {
  return (
    <td>
      <div class="line">{main}</div>
      <div class={'line small ' + (tone ?? 'muted')}>{sub ?? ' '}</div>
    </td>
  );
}

export function Card({ title, children, class: className }: { title?: string; children: ComponentChildren; class?: string }) {
  return (
    <div class={'card ' + (className ?? '')}>
      {title && <h3>{title}</h3>}
      {children}
    </div>
  );
}

export function Field({ label, children }: { label: string; children: ComponentChildren }) {
  return (
    <>
      <dt>{label}</dt>
      <dd>{children}</dd>
    </>
  );
}

export function Tabs<T extends string>({ tabs, selected, select }: { tabs: { id: T; label: string; count?: number }[]; selected: T; select: (id: T) => void }) {
  return (
    <div class="tabs" role="tablist">
      {tabs.map((tab) => (
        <button key={tab.id} role="tab" aria-selected={tab.id === selected} onClick={() => select(tab.id)}>
          {tab.label}
          {Boolean(tab.count) && <span class="count">{tab.count}</span>}
        </button>
      ))}
    </div>
  );
}

export function since(value: string): string {
  const minutes = Math.round((Date.now() - new Date(value).getTime()) / 60000);
  if (minutes < 1) {
    return 'just now';
  }
  if (minutes < 60) {
    return minutes + ' min ago';
  }
  if (minutes < 60 * 24) {
    return Math.round(minutes / 60) + ' h ago';
  }
  const days = Math.round(minutes / (60 * 24));

  return days === 1 ? 'yesterday' : days + ' days ago';
}

export type Toast = { id: number; tone: 'ok' | 'bad'; text: string };

let toasts: Toast[] = [];
let counter = 0;
const listeners = new Set<(shown: Toast[]) => void>();

function publish(next: Toast[]) {
  toasts = next;
  listeners.forEach((listener) => listener(toasts));
}

export function dismiss(id: number): void {
  publish(toasts.filter((toast) => toast.id !== id));
}

export function notify(text: string, tone: Toast['tone'] = 'ok'): void {
  counter += 1;
  const toast = { id: counter, tone, text };
  publish([...toasts.filter((each) => each.text !== text), toast]);
  if (tone === 'ok') {
    setTimeout(() => dismiss(toast.id), 5000);
  }
}

export function reason(error: unknown): string {
  if (typeof error === 'string' && error.trim()) {
    return error.trim();
  }
  if (error && typeof error === 'object') {
    for (const key of ['Message', 'message', 'detail', 'title']) {
      const value = (error as Record<string, unknown>)[key];
      if (typeof value === 'string' && value.trim()) {
        return value.trim();
      }
    }
  }

  return '';
}

export function refused(error: unknown, fallback: string): void {
  const said = reason(error);
  notify(said ? fallback + ' The server said: ' + said : fallback, 'bad');
}

export function Toasts() {
  const [shown, setShown] = useState(toasts);

  useEffect(() => {
    listeners.add(setShown);

    return () => {
      listeners.delete(setShown);
    };
  }, []);

  return (
    <div class="toasts">
      {shown.map((toast) => (
        <div key={toast.id} class={'toast ' + toast.tone} role={toast.tone === 'ok' ? 'status' : 'alert'}>
          <Dot tone={toast.tone} />
          <span>{toast.text}</span>
          <button class="link" aria-label="Dismiss" onClick={() => dismiss(toast.id)}>
            ×
          </button>
        </div>
      ))}
    </div>
  );
}
