import assert from 'node:assert/strict';
import test from 'node:test';

import { planRelease, releaseNotes } from './release.mjs';

test('first fork release keeps the upstream baseline', () => {
  assert.deepEqual(
    planRelease({
      reachableTags: ['v0.8.2', 'v0.9.0', 'v0.9.1'],
      allTags: ['v0.8.2', 'v0.9.0', 'v0.9.1'],
      commits: ['feat(capmox): add disks'],
    }),
    { version: '0.9.1-nsk.1', previousTag: 'v0.9.1' },
  );
});

test('a second patch set increments without changing the upstream baseline', () => {
  assert.deepEqual(
    planRelease({
      reachableTags: ['v0.9.1', 'v0.9.1-nsk.1'],
      allTags: ['v0.9.1', 'v0.9.1-nsk.1'],
      commits: ['fix(vmservice): handle unchanged config'],
    }),
    { version: '0.9.1-nsk.2', previousTag: 'v0.9.1-nsk.1' },
  );
});

test('a rebase resets the patch set on the new upstream baseline', () => {
  assert.deepEqual(
    planRelease({
      reachableTags: ['v0.9.1', 'v0.9.2'],
      allTags: ['v0.9.1', 'v0.9.1-nsk.2', 'v0.9.2'],
      commits: ['fix(capmox): port disk patch'],
    }),
    { version: '0.9.2-nsk.1', previousTag: 'v0.9.2' },
  );
});

test('never reuses a patch-set tag after history is rewritten', () => {
  assert.deepEqual(
    planRelease({
      reachableTags: ['v0.9.1'],
      allTags: ['v0.9.1', 'v0.9.1-nsk.1'],
      commits: ['fix(capmox): rebuilt patch'],
    }),
    { version: '0.9.1-nsk.2', previousTag: 'v0.9.1' },
  );
});

test('non-release commits cannot publish an image', () => {
  assert.equal(
    planRelease({
      reachableTags: ['v0.9.1', 'v0.9.1-nsk.1'],
      allTags: ['v0.9.1', 'v0.9.1-nsk.1'],
      commits: ['docs: correct README', 'chore(release): 0.9.1-nsk.1'],
    }),
    null,
  );
});

test('fails closed when there is no upstream baseline', () => {
  assert.throws(
    () => planRelease({ reachableTags: [], allTags: [], commits: ['feat: disks'] }),
    /upstream release tag/,
  );
});

test('release notes contain only release-worthy fork commits', () => {
  assert.equal(
    releaseNotes('0.9.1-nsk.1', [
      'docs: clarify installation',
      'feat(capmox): add disks\n\nExtra detail',
      'fix(vmservice): handle unchanged config',
    ]),
    '## [0.9.1-nsk.1]\n\n' +
      '- feat(capmox): add disks\n' +
      '- fix(vmservice): handle unchanged config\n',
  );
});
