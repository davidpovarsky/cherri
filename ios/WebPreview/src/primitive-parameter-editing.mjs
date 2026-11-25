/**
 * Keep an editable scalar separate from its renderer-owned decoration.
 *
 * The locked preview-shortcut Wait renderer formats WFDelayTime with an English
 * unit. Only that exact action/key representation is enrolled here. Arbitrary
 * suffixes, markup and ambiguous matching cards never acquire edit authority.
 * The native source map remains the final authority for every submitted edit.
 */
export function primitiveParameterPresentation(actionIdentifier, key, value) {
  if (typeof value === 'string') {
    if (value.length === 0 || value.includes('<') || value.includes('>')) {
      return null;
    }
    return { raw: value, display: value, suffix: '' };
  }
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return null;
  }
  const raw = String(value);
  const suffix = actionIdentifier === 'is.workflow.actions.delay' && key === 'WFDelayTime'
    ? (value === 1 ? ' second' : ' seconds')
    : '';
  return { raw, display: raw + suffix, suffix };
}

/** A repeated value is not enough to identify which parameter owns a DOM node. */
export function uniquePrimitiveContainer(candidates, used, presentation) {
  const matches = candidates.filter((element) =>
    !used.has(element) && element.textContent === presentation.display);
  return matches.length === 1 ? matches[0] : null;
}

/**
 * Split only the already verified complete display. Text nodes preserve literal
 * content; unit text stays outside the actual contentEditable field. On blur an
 * invalid scalar can be reset without removing or parsing a display decoration.
 */
export function prepareEditableScalar(container, presentation) {
  if (container.textContent !== presentation.display) {
    return null;
  }
  if (!presentation.suffix) {
    return container;
  }
  const document = container.ownerDocument;
  const scalar = document.createElement('span');
  scalar.textContent = presentation.raw;
  container.replaceChildren(scalar, document.createTextNode(presentation.suffix));
  return scalar;
}
