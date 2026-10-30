// Copies web/dist into internal/web/dist so go:embed can pick it up (plain read/write: works on odd filesystems).
import { readdirSync, rmSync, mkdirSync, readFileSync, writeFileSync, statSync } from 'node:fs';
const dst = '../internal/web/dist';
function copy(from, to) {
  mkdirSync(to, { recursive: true });
  for (const f of readdirSync(from)) {
    const s = `${from}/${f}`, d = `${to}/${f}`;
    if (statSync(s).isDirectory()) copy(s, d);
    else writeFileSync(d, readFileSync(s));
  }
}
mkdirSync(dst, { recursive: true });
for (const f of readdirSync(dst)) if (f !== '.gitkeep') rmSync(`${dst}/${f}`, { recursive: true, force: true });
copy('dist', dst);
writeFileSync(`${dst}/.gitkeep`, '');
