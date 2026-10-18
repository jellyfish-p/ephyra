// Generates app/types/generated.ts from the backend's OpenAPI document.
//
//   npm run api:generate   # write
//   npm run api:check      # fail if stale, which is what CI runs
//
// The document is produced by the Go generator (server/cmd/openapi), so these
// types follow the handlers rather than a second hand-written copy of them.
// Point OPENAPI_SCHEMA at another file to generate from a deployed spec.
import { readFile, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'
import openapiTS, { COMMENT_HEADER, astToString } from 'openapi-typescript'
import prettier from 'prettier'

const check = process.argv.includes('--check')

const schema = process.env.OPENAPI_SCHEMA
  ? pathToFileURL(resolve(process.env.OPENAPI_SCHEMA))
  : new URL('../../server/docs/openapi.json', import.meta.url)
const target = new URL('../app/types/generated.ts', import.meta.url)

// openapi-typescript emits four-space indentation of its own, so run the result
// through the repo's prettier config. Without this the file would be the one
// thing in the tree that format:check rejects. Prettier 3 does not pick the
// config up from the filepath on its own, hence the explicit resolveConfig.
const targetPath = fileURLToPath(target)
const options = await prettier.resolveConfig(targetPath)
const generated = await prettier.format(COMMENT_HEADER + astToString(await openapiTS(schema)), {
  ...options,
  filepath: targetPath,
})

if (check) {
  const existing = await readFile(target, 'utf8').catch(() => '')
  if (existing !== generated) {
    console.error(
      'app/types/generated.ts is out of date; run: npm run api:generate\n' +
        '(if the backend changed, regenerate the spec first: cd ../server && go run ./cmd/openapi)',
    )
    process.exit(1)
  }
  console.log('app/types/generated.ts is up to date')
} else {
  await writeFile(target, generated)
  console.log(`wrote app/types/generated.ts (${generated.length} bytes)`)
}
