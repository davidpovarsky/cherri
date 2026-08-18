import 'preview-shortcut/css';
import { ShortcutPreview } from 'preview-shortcut';

type CherriPreviewWindow = Window & {
  renderShortcutFromBase64?: (base64: string, name: string) => void;
  clearShortcutPreview?: () => void;
};

const hostWindow = window as CherriPreviewWindow;
const selector = '#shortcut-preview';
let preview: ShortcutPreview | null = null;

function decodeBase64UTF8(base64: string): string {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return new TextDecoder('utf-8').decode(bytes);
}

// preview-shortcut renders booleans as real Framework7 checkbox toggles. In
// WKWebView, taps on the styled label can occasionally leave the hidden checkbox
// unchanged even though the rest of the preview remains interactive. Preserve
// preview-shortcut's native behavior and only correct the state if the normal
// label activation did not settle on the expected value after the click.
const toggleStartState = new WeakMap<HTMLInputElement, boolean>();

function checkboxForEventTarget(target: EventTarget | null): HTMLInputElement | null {
  if (!(target instanceof Element)) {
    return null;
  }
  const label = target.closest('label.toggle');
  return label?.querySelector<HTMLInputElement>('input[type="checkbox"]') ?? null;
}

document.addEventListener('pointerdown', (event) => {
  const checkbox = checkboxForEventTarget(event.target);
  if (checkbox) {
    toggleStartState.set(checkbox, checkbox.checked);
  }
}, true);

document.addEventListener('click', (event) => {
  const checkbox = checkboxForEventTarget(event.target);
  if (!checkbox) {
    return;
  }

  const initial = toggleStartState.get(checkbox);
  if (initial === undefined) {
    return;
  }
  const expected = !initial;

  window.setTimeout(() => {
    if (checkbox.checked !== expected) {
      checkbox.checked = expected;
      checkbox.dispatchEvent(new Event('input', { bubbles: true }));
      checkbox.dispatchEvent(new Event('change', { bubbles: true }));
    }
    toggleStartState.delete(checkbox);
  }, 0);
}, true);

hostWindow.renderShortcutFromBase64 = (base64: string, name: string) => {
  const plist = decodeBase64UTF8(base64);

  if (preview === null) {
    preview = new ShortcutPreview({
      selector,
      name,
      data: {},
      header: true,
      meta: true,
      variables: true,
    } as never);
  }

  preview.name = name;
  preview.load(plist);
};

hostWindow.clearShortcutPreview = () => {
  const element = document.querySelector(selector);
  if (element) {
    element.innerHTML = '';
  }
};
