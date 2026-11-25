import test from 'node:test';
import assert from 'node:assert/strict';
import {
  primitiveParameterPresentation,
  uniquePrimitiveContainer,
  prepareEditableScalar,
} from '../src/primitive-parameter-editing.mjs';

const wait = (value) => primitiveParameterPresentation('is.workflow.actions.delay', 'WFDelayTime', value);

test('locked Wait display keeps its real scalar and singular/plural units separate', () => {
  for (const [value, suffix] of [[0, ' seconds'], [1, ' second'], [2, ' seconds'], [1.5, ' seconds'], [-2, ' seconds']]) {
    assert.deepEqual(wait(value), { raw: String(value), display: String(value) + suffix, suffix });
  }
});

test('decoration requires both the exact native action identity and parameter key', () => {
  for (const [action, key] of [
    ['is.workflow.actions.showresult', 'WFDelayTime'],
    ['is.workflow.actions.delay', 'OtherParameter'],
    ['is.workflow.actions.Delay', 'WFDelayTime'],
    ['', 'WFDelayTime'],
  ]) {
    assert.deepEqual(primitiveParameterPresentation(action, key, 2), { raw: '2', display: '2', suffix: '' });
  }
});

test('primitive text remains exact, including Unicode and words that look like units', () => {
  for (const value of ['שלום 🍒', '2 seconds', ' café ', '01', '1e2']) {
    assert.deepEqual(wait(value), { raw: value, display: value, suffix: '' });
  }
});

test('empty, markup, variable objects, booleans and nonfinite numbers remain unbound', () => {
  for (const value of ['', '<b>2</b>', 'left > right', {}, { Value: 2 }, [2], null, undefined, true, false, NaN, Infinity, -Infinity]) {
    assert.equal(wait(value), null);
  }
});

test('raw-value-only binding reproduces the actual missing Wait field', () => {
  const originalRenderedNode = { textContent: '2 seconds' };
  const candidates = [originalRenderedNode];
  assert.equal(candidates.filter((node) => node.textContent === String(2)).length, 0);
  assert.equal(uniquePrimitiveContainer(candidates, new Set(), wait(2)), originalRenderedNode);
});

test('exact unique display is required, without arbitrary unit stripping or whitespace coercion', () => {
  const correct = { textContent: '2 seconds' };
  for (const text of ['2', '2 second', '2 Seconds', ' 2 seconds', '2 seconds ', '2 seconds extra', '20 seconds']) {
    assert.equal(uniquePrimitiveContainer([{ textContent: text }], new Set(), wait(2)), null);
  }
  assert.equal(uniquePrimitiveContainer([correct], new Set(), wait(2)), correct);
});

test('duplicate displays or an already-owned node cannot be relabelled to another parameter', () => {
  const first = { textContent: '2 seconds' };
  const second = { textContent: '2 seconds' };
  assert.equal(uniquePrimitiveContainer([first, second], new Set(), wait(2)), null);
  assert.equal(uniquePrimitiveContainer([first], new Set([first]), wait(2)), null);
  assert.equal(uniquePrimitiveContainer([first, second], new Set([first]), wait(2)), second);
});

function ownedContainer(text) {
  const created = [];
  const document = {
    createElement(tag) {
      assert.equal(tag, 'span');
      const node = { tag, textContent: '' };
      created.push(node);
      return node;
    },
    createTextNode(value) {
      const node = { textContent: value, nodeType: 3 };
      created.push(node);
      return node;
    },
  };
  const container = {
    ownerDocument: document,
    textContent: text,
    children: null,
    replaceChildren(...children) { this.children = children; },
  };
  return { container, created };
}

test('only the numeric span becomes editable while renderer units remain a separate text node', () => {
  const { container, created } = ownedContainer('2 seconds');
  const scalar = prepareEditableScalar(container, wait(2));
  assert.equal(scalar, created[0]);
  assert.equal(scalar.textContent, '2');
  assert.equal(container.children.length, 2);
  assert.equal(container.children[1].nodeType, 3);
  assert.equal(container.children[1].textContent, ' seconds');
  scalar.textContent = 'invalid-number';
  scalar.textContent = wait(2).raw; // existing invalid-number reset restores only the scalar.
  assert.equal(container.children.map((node) => node.textContent).join(''), '2 seconds');
});

test('an altered display cannot be split after the matching observation', () => {
  const { container, created } = ownedContainer('different');
  assert.equal(prepareEditableScalar(container, wait(2)), null);
  assert.equal(created.length, 0);
});

test('ordinary text and numbers keep the existing actual DOM node', () => {
  for (const value of [2, 'שלום 🍒']) {
    const presentation = primitiveParameterPresentation('other.action', 'Value', value);
    const { container, created } = ownedContainer(String(value));
    assert.equal(prepareEditableScalar(container, presentation), container);
    assert.equal(created.length, 0);
  }
});
