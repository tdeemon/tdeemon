// Precompressed copies of the built assets, for internal/webui to serve to
// a browser that takes them: name.br (Brotli) and name.gz (gzip) next to
// each script, stylesheet, WebAssembly module and SVG under assets/. An
// instance without a compressing proxy in front of it sends about a third
// of the bytes; with one, the proxy passes an encoded answer as it is.
import { readdir, readFile, writeFile } from 'node:fs/promises'
import { extname, join, resolve } from 'node:path'
import { promisify } from 'node:util'
import { brotliCompress, constants, gzip } from 'node:zlib'
import type { Plugin } from 'vite'

const brotli = promisify(brotliCompress)
const gz = promisify(gzip)

// The types that compress; fonts (woff2) and images are compressed already.
const compressible = new Set(['.js', '.css', '.wasm', '.svg', '.json'])

// compressAssets writes name.br and name.gz next to each compressible file
// in dir, the best each format does, and only where it is smaller than the
// file. It returns the names it wrote, sorted.
export async function compressAssets(dir: string): Promise<string[]> {
  const names = (await readdir(dir)).filter((name) => compressible.has(extname(name)))
  const written = await Promise.all(
    names.map(async (name) => {
      const data = await readFile(join(dir, name))
      const variants: [string, Buffer][] = [
        [
          `${name}.br`,
          await brotli(data, {
            params: {
              [constants.BROTLI_PARAM_QUALITY]: constants.BROTLI_MAX_QUALITY,
              [constants.BROTLI_PARAM_SIZE_HINT]: data.length,
            },
          }),
        ],
        [`${name}.gz`, await gz(data, { level: constants.Z_BEST_COMPRESSION })],
      ]
      const smaller = variants.filter(([, out]) => out.length < data.length)
      await Promise.all(smaller.map(([file, out]) => writeFile(join(dir, file), out)))
      return smaller.map(([file]) => file)
    }),
  )
  return written.flat().toSorted()
}

// precompress is the build's plugin: once the bundle and the service
// worker are written, it compresses the assets directory.
export function precompress(): Plugin {
  let assets = ''
  return {
    name: 'tdeemon:precompress',
    apply: 'build',
    enforce: 'post',
    configResolved(config) {
      assets = resolve(config.root, config.build.outDir, config.build.assetsDir)
    },
    async closeBundle() {
      await compressAssets(assets)
    },
  }
}
