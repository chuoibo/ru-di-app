const fs = require('node:fs');
const path = require('node:path');
function locate(start) {
  for (let dir = path.resolve(start); ; dir = path.dirname(dir)) {
    if (fs.existsSync(path.join(dir, 'apps/mobile/package.json'))) return dir;
    if (path.dirname(dir) === dir) throw new Error('Set PQ_SOURCE_ROOT when these tests are outside the repository.');
  }
}
const repo = process.env.PQ_SOURCE_ROOT ?? locate(__dirname);
const sourceRoot = process.env.PQ_CANDIDATE_ROOT ?? repo;
module.exports = { repo, sourceRoot };
