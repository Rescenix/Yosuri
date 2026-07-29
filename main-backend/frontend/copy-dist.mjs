import { cp, mkdir, readdir, rm } from 'node:fs/promises'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const source = join(here, '..', '..', 'main-frontend', 'beneficial-belt', 'dist')
const target = join(here, 'dist')

await mkdir(target, { recursive: true })
for (const entry of await readdir(target, { withFileTypes: true })) {
  if (entry.name === '.gitkeep') continue
  await rm(join(target, entry.name), { recursive: true, force: true })
}
await cp(source, target, { recursive: true })
