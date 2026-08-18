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
