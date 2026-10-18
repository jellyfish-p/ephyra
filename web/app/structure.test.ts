import { existsSync, readFileSync, readdirSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

/**
 * Structural invariants that fail *silently*.
 *
 * Every check here is for a mistake that typechecks, builds, and passes review,
 * then shows up only as a blank page or a dead button in a browser. Each one is
 * cheap to assert, and each was a real bug in this app at least once.
 *
 * This file lives at the root of `app/` on purpose: Nuxt treats `.ts` files
 * under `app/pages/` as pages, so a test there would register a bogus route.
 */

const APP_DIR = fileURLToPath(new URL('.', import.meta.url))
const PAGES_DIR = join(APP_DIR, 'pages')

function walk(dir: string, extensions: string[]): string[] {
  const out: string[] = []
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name)
    if (entry.isDirectory()) out.push(...walk(full, extensions))
    else if (extensions.some((ext) => entry.name.endsWith(ext))) out.push(full)
  }
  return out
}

const pageFiles = walk(PAGES_DIR, ['.vue'])
const appFiles = walk(APP_DIR, ['.vue', '.ts']).filter((f) => !f.endsWith('.test.ts'))
const read = (file: string) => readFileSync(file, 'utf8')
const label = (file: string) => relative(APP_DIR, file).replace(/\\/g, '/')

/** A page that has a sibling directory of the same name owns those child pages. */
function parentPages(): string[] {
  return pageFiles.filter((file) => {
    const dir = file.replace(/\.vue$/, '')
    return existsSync(dir) && statSync(dir).isDirectory()
  })
}

describe('page structure', () => {
  it('renders <NuxtPage /> in every page that has child pages', () => {
    // Nuxt refuses to display a child route unless its parent renders
    // `<NuxtPage />`. It only emits a console warning (NUXT_E4016): the build
    // stays green and the navigation simply appears to do nothing.
    const parents = parentPages()
    expect(parents.length).toBeGreaterThan(0)

    const missing = parents.filter((file) => !/<NuxtPage\s*\/>/.test(read(file)))
    expect(missing.map(label)).toEqual([])
  })

  it('has an index page for every parent, so the bare parent path still resolves', () => {
    const missing = parentPages().filter(
      (file) => !existsSync(join(file.replace(/\.vue$/, ''), 'index.vue')),
    )
    expect(missing.map(label)).toEqual([])
  })
})

describe('UI conventions', () => {
  it('uses UTable cell slots that actually exist', () => {
    // Nuxt UI v4 names cell slots `<columnId>-cell`. The v2-era `<columnId>-data`
    // name compiles fine and then never renders, so the cell comes out empty —
    // and a column that was never declared cannot render its slot at all.
    const offenders = appFiles.filter((file) => /#[\w-]+-data=/.test(read(file)))
    expect(offenders.map(label)).toEqual([])
  })

  it('never gives a select item an empty-string value', () => {
    // Reka UI reserves `''` for clearing a select and throws on a
    // `<SelectItem value="">`, which takes the whole page down to a 500. Use a
    // named sentinel and translate it at the query boundary instead.
    const offenders = appFiles.filter((file) => {
      const source = read(file)
      return source.includes('USelect') && /value:\s*''/.test(source)
    })
    expect(offenders.map(label)).toEqual([])
  })
})
