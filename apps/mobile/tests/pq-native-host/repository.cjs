const fs = require('node:fs');
const path = require('node:path');
function locate(start) {
  for (let dir = path.resolve(start); ; dir = path.dirname(dir)) {
    if (fs.existsSync(path.join(dir, 'apps/mobile/package.json'))) return dir;
    if (path.dirname(dir) === dir) throw new Error('Set PQ_SOURCE_ROOT when these tests are outside the repository.');
  }
}
// A candidate is a complete source graph. Never fall back to a second tree.
const repo = fs.realpathSync(path.resolve(process.env.PQ_SOURCE_ROOT ?? locate(__dirname)));
const sourceRoot = fs.realpathSync(path.resolve(process.env.PQ_CANDIDATE_ROOT ?? repo));
if (sourceRoot !== repo) throw new Error('PQ source and dependency repository must be the same complete graph.');
for (const name of ['COMMENTS_SOURCE', 'PQ_HANDOFF_POSTDETAIL_SOURCE', 'PQ_HANDOFF_COMMENTS_SOURCE', 'PQ_MEDIA_COMMENTS_SOURCE', 'PQ_MEDIA_PICKER_SOURCE']) {
  const value = process.env[name];
  if (value !== undefined && !path.resolve(value).startsWith(repo + path.sep)) throw new Error('Source override leaves the complete graph: ' + name);
}
module.exports = { repo, sourceRoot };
