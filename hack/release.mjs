import { execFileSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

const upstreamTag = /^v(\d+)\.(\d+)\.(\d+)$/;
const patchTag = /^v(\d+)\.(\d+)\.(\d+)-nsk\.(\d+)$/;
const conventionalCommit = /^(\w+)(?:\([^)]+\))?(!)?: (.+)$/;
const releaseTypes = new Set(['feat', 'fix', 'perf', 'build']);

function git(...args) {
  try {
    return execFileSync('git', args, {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'pipe'],
    }).trim();
  } catch {
    // Push arguments can contain the release token; never include them in errors.
    throw new Error(`Git ${args[0]} failed`);
  }
}

function versionParts(tag) {
  return upstreamTag.exec(tag)?.slice(1).map(Number);
}

function compareVersions(a, b) {
  const left = versionParts(a);
  const right = versionParts(b);
  for (let i = 0; i < 3; i += 1) {
    if (left[i] !== right[i]) return left[i] - right[i];
  }
  return 0;
}

function releaseWorthy(message) {
  const match = conventionalCommit.exec(message.split('\n', 1)[0]);
  return Boolean(match && (releaseTypes.has(match[1]) || match[2])) ||
    /(?:^|\n)BREAKING CHANGE: /m.test(message);
}

export function planRelease({ reachableTags, allTags, commits }) {
  const upstream = reachableTags.filter((tag) => upstreamTag.test(tag))
    .sort(compareVersions).at(-1);
  if (!upstream) throw new Error('No reachable upstream release tag');

  const base = upstream.slice(1);
  const prefix = `v${base}-nsk.`;
  const reachablePatches = reachableTags.filter((tag) =>
    patchTag.test(tag) && tag.startsWith(prefix));
  const previousTag = reachablePatches.sort((a, b) =>
    Number(a.slice(prefix.length)) - Number(b.slice(prefix.length))).at(-1) ?? upstream;

  if (!commits.some(releaseWorthy)) return null;

  const highestTick = Math.max(0, ...allTags.filter((tag) =>
    patchTag.test(tag) && tag.startsWith(prefix))
    .map((tag) => Number(tag.slice(prefix.length))));
  return { version: `${base}-nsk.${highestTick + 1}`, previousTag };
}

function commitsSince(tag) {
  return git('log', '--format=%B%x00', `${tag}..HEAD`)
    .split('\0').map((message) => message.trim()).filter(Boolean);
}

function currentPlan() {
  const reachableTags = git('tag', '--merged', 'HEAD').split('\n').filter(Boolean);
  const allTags = git('tag', '--list').split('\n').filter(Boolean);
  const upstream = reachableTags.filter((tag) => upstreamTag.test(tag))
    .sort(compareVersions).at(-1);
  if (!upstream) throw new Error('No reachable upstream release tag');
  const prefix = `${upstream}-nsk.`;
  const previousTag = reachableTags.filter((tag) =>
    patchTag.test(tag) && tag.startsWith(prefix))
    .sort((a, b) => Number(a.slice(prefix.length)) - Number(b.slice(prefix.length)))
    .at(-1) ?? upstream;
  const commits = commitsSince(previousTag);
  return { plan: planRelease({ reachableTags, allTags, commits }), commits };
}

export function releaseNotes(version, commits) {
  const changes = commits.filter(releaseWorthy).map((message) =>
    `- ${message.split('\n', 1)[0]}`);
  return `## [${version}]\n\n${changes.join('\n')}\n`;
}

async function publish() {
  const { plan, commits } = currentPlan();
  if (!plan || plan.version !== process.env.VERSION) {
    throw new Error(`Release plan changed: expected ${process.env.VERSION}, got ${plan?.version}`);
  }
  const tag = `v${plan.version}`;
  const source = git('rev-parse', 'HEAD');
  const remote = git('ls-remote', 'origin', 'refs/heads/main').split('\t')[0];
  if (remote !== source) throw new Error('main moved since the image build');
  if (git('tag', '--list', tag)) throw new Error(`${tag} already exists`);

  const token = process.env.GITLAB_TOKEN;
  const project = process.env.CI_PROJECT_ID;
  const server = process.env.CI_SERVER_URL;
  if (!token || !project || !server || !process.env.CI_PROJECT_URL) {
    throw new Error('Missing GitLab release credentials');
  }

  const notes = releaseNotes(plan.version, commits);
  const changelog = readFileSync('CHANGELOG.md', 'utf8');
  const marker = '## [Unreleased]\n';
  if (!changelog.includes(marker)) throw new Error('Missing Unreleased changelog section');
  writeFileSync('CHANGELOG.md', changelog.replace(marker, `${marker}\n${notes}\n`));
  git('config', 'user.name', 'Release Bot');
  git('config', 'user.email', 'release@nsk.io');
  git('add', 'CHANGELOG.md');
  git('commit', '-m', `chore(release): ${plan.version} [skip ci]`);
  git('tag', tag);

  const url = new URL(process.env.CI_PROJECT_URL);
  url.username = 'oauth2';
  url.password = token;
  git('push', '--atomic', url.toString(), 'HEAD:refs/heads/main', `refs/tags/${tag}`);

  const response = await fetch(`${server}/api/v4/projects/${project}/releases`, {
    method: 'POST',
    headers: { 'PRIVATE-TOKEN': token, 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: tag, tag_name: tag, description: notes }),
  });
  if (!response.ok) throw new Error(`GitLab release failed: HTTP ${response.status}`);
  process.stdout.write(`Published ${tag}\n`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  if (process.argv[2] === 'version') {
    const { plan } = currentPlan();
    if (!plan) throw new Error('No release-worthy conventional commits');
    process.stdout.write(`VERSION=${plan.version}\n`);
  } else if (process.argv[2] === 'publish') {
    await publish();
  } else {
    throw new Error('Usage: node hack/release.mjs version|publish');
  }
}
