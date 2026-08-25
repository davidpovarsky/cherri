import 'preview-shortcut/css';
import { ShortcutPreview } from 'preview-shortcut';

type CherriPreviewWindow = Window & {
  renderShortcutFromBase64?: (base64: string, name: string) => void;
  clearShortcutPreview?: () => void;
  webkit?: {
    messageHandlers?: {
      cherriPreviewEdit?: {
        postMessage: (message: object) => void;
      };
    };
  };
};

type WorkflowToggleBinding = {
  kind: 'workflowType' | 'quickActionSurface';
  value: string;
};

const hostWindow = window as CherriPreviewWindow;
const selector = '#shortcut-preview';
let preview: ShortcutPreview | null = null;

const workflowToggleBindings: WorkflowToggleBinding[] = [
  { kind: 'workflowType', value: 'ActionExtension' },
  { kind: 'workflowType', value: 'WFWorkflowTypeShowInSearch' },
  { kind: 'workflowType', value: 'MenuBar' },
  { kind: 'workflowType', value: 'ReceivesOnScreenContent' },
  { kind: 'workflowType', value: 'QuickActions' },
  { kind: 'quickActionSurface', value: 'Finder' },
  { kind: 'quickActionSurface', value: 'Services' },
  { kind: 'workflowType', value: 'WFWorkflowTypeReceivesInputFromSearch' },
  { kind: 'workflowType', value: 'Watch' },
];

function decodeBase64UTF8(base64: string): string {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index);
  }
  return new TextDecoder('utf-8').decode(bytes);
}

function postPreviewEdit(message: object) {
  hostWindow.webkit?.messageHandlers?.cherriPreviewEdit?.postMessage(message);
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

function bindWorkflowToggles() {
  const toggles = Array.from(
    document.querySelectorAll<HTMLInputElement>(`${selector} .sp-modal input[type="checkbox"]`),
  );

  workflowToggleBindings.forEach((binding, index) => {
    const checkbox = toggles[index];
    if (!checkbox || checkbox.dataset.cherriEditBound === 'true') {
      return;
    }
    checkbox.dataset.cherriEditBound = 'true';
    checkbox.addEventListener('change', () => {
      postPreviewEdit({ kind: binding.kind, value: binding.value, enabled: checkbox.checked });
    });
  });
}

function visibleActionCards(): HTMLElement[] {
  return Array.from(document.querySelectorAll<HTMLElement>(`${selector} .sp-container .card`))
    .filter((card) => card.id !== 'shortcut-input');
}

function actionParameters(action: any): Record<string, any> {
  return (action?.WFWorkflowActionParameters ?? {}) as Record<string, any>;
}

function editableParameterEntries(action: any): Array<[string, any]> {
  const ignored = new Set(['UUID', 'CustomOutputName', 'WFControlFlowMode', 'GroupingIdentifier']);
  return Object.entries(actionParameters(action)).filter(([key]) => !ignored.has(key));
}

function bindActionBooleanParameters(card: HTMLElement, action: any, actionIndex: number) {
  const booleanEntries = editableParameterEntries(action)
    .filter(([, value]) => typeof value === 'boolean');
  if (!booleanEntries.length) {
    return;
  }

  const checkboxes = Array.from(card.querySelectorAll<HTMLInputElement>('input[type="checkbox"]'));
  // Custom renderers can contain derived switches which cannot safely be mapped
  // back to a raw parameter. Only bind when the rendered and raw boolean counts
  // match exactly, making the positional mapping unambiguous.
  if (checkboxes.length !== booleanEntries.length) {
    return;
  }

  checkboxes.forEach((checkbox, index) => {
    if (checkbox.dataset.cherriEditBound === 'true') {
      return;
    }
    const [key] = booleanEntries[index];
    checkbox.dataset.cherriEditBound = 'true';
    checkbox.addEventListener('change', () => {
      postPreviewEdit({
        kind: 'actionParameter',
        actionIndex,
        key,
        valueType: 'bool',
        value: checkbox.checked,
      });
    });
  });
}

function bindPrimitiveActionParameters(card: HTMLElement, action: any, actionIndex: number) {
  const candidates = Array.from(card.querySelectorAll<HTMLElement>('.sp-value'))
    .filter((element) => !element.querySelector('.sp-variable-value'));
  const used = new Set<HTMLElement>();

  for (const [key, value] of editableParameterEntries(action)) {
    if (typeof value !== 'string' && typeof value !== 'number') {
      continue;
    }
    if (typeof value === 'string' && (value.includes('<') || value.includes('>') || value.length === 0)) {
      continue;
    }

    const rendered = String(value);
    const matches = candidates.filter((element) => !used.has(element) && element.textContent === rendered);
    if (matches.length !== 1) {
      continue;
    }

    const element = matches[0];
    used.add(element);
    element.classList.add('cherri-editable-value');
    element.contentEditable = 'true';
    element.setAttribute('role', 'textbox');
    element.setAttribute('aria-label', key);
    element.spellcheck = false;

    element.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') {
        element.textContent = rendered;
        element.blur();
      }
      if (typeof value === 'number' && event.key === 'Enter') {
        event.preventDefault();
        element.blur();
      }
    });

    element.addEventListener('blur', () => {
      const edited = element.textContent ?? '';
      if (edited === rendered) {
        return;
      }

      if (typeof value === 'number') {
        const number = Number(edited);
        if (!Number.isFinite(number) || (Number.isInteger(value) && !Number.isInteger(number))) {
          element.textContent = rendered;
          return;
        }
        postPreviewEdit({
          kind: 'actionParameter',
          actionIndex,
          key,
          valueType: Number.isInteger(value) ? 'integer' : 'real',
          value: number,
        });
        return;
      }

      postPreviewEdit({
        kind: 'actionParameter',
        actionIndex,
        key,
        valueType: 'string',
        value: edited,
      });
    });
  }
}

function wireGraphicalEditing() {
  if (!preview) {
    return;
  }

  bindWorkflowToggles();

  const actions = preview.data.WFWorkflowActions ?? [];
  const cards = visibleActionCards();
  actions.forEach((action, actionIndex) => {
    const card = cards[actionIndex];
    if (!card) {
      return;
    }
    card.dataset.cherriActionIndex = String(actionIndex);
    bindActionBooleanParameters(card, action, actionIndex);
    bindPrimitiveActionParameters(card, action, actionIndex);
  });
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
  wireGraphicalEditing();
};

hostWindow.clearShortcutPreview = () => {
  const element = document.querySelector(selector);
  if (element) {
    element.innerHTML = '';
  }
};
