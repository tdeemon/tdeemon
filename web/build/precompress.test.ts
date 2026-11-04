import { mkdtemp, readdir, readFile, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { brotliDecompressSync, gunzipSync } from 'node:zlib'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { compressAssets } from './precompress.ts'

describe('compressAssets', () => {
  let dir = ''
  beforeEach(async () => {
    dir = await mkdtemp(join(tmpdir(), 'precompress-'))
  })
  afterEach(async () => {
    await rm(dir, { recursive: true, force: true })
  })

  it('writes Brotli and gzip copies of the assets that compress', async () => {
    const script = 'export const answer = 42;\n'.repeat(200)
    const style = 'body { color: red; }\n'.repeat(100)
    await writeFile(join(dir, 'index-abc.js'), script)
    await writeFile(join(dir, 'index-abc.css'), style)
    // Fonts and images are compressed already.
    await writeFile(join(dir, 'font-abc.woff2'), 'x'.repeat(1000))
    await writeFile(join(dir, 'icon-abc.png'), 'x'.repeat(1000))

    const written = await compressAssets(dir)

    expect(written).toEqual([
      'index-abc.css.br',
      'index-abc.css.gz',
      'index-abc.js.br',
      'index-abc.js.gz',
    ])
    expect(brotliDecompressSync(await readFile(join(dir, 'index-abc.js.br'))).toString()).toBe(
      script,
    )
    expect(gunzipSync(await readFile(join(dir, 'index-abc.js.gz'))).toString()).toBe(script)
    expect(gunzipSync(await readFile(join(dir, 'index-abc.css.gz'))).toString()).toBe(style)
    expect((await readdir(dir)).toSorted()).toEqual([
      'font-abc.woff2',
      'icon-abc.png',
      'index-abc.css',
      'index-abc.css.br',
      'index-abc.css.gz',
      'index-abc.js',
      'index-abc.js.br',
      'index-abc.js.gz',
    ])
  })

  it('leaves a file whose copy would not be smaller', async () => {
    await writeFile(join(dir, 'tiny-abc.js'), 'a')

    expect(await compressAssets(dir)).toEqual([])
    expect(await readdir(dir)).toEqual(['tiny-abc.js'])
  })
})
