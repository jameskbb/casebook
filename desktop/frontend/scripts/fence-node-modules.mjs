// Writes node_modules/go.mod so Go treats node_modules as a separate module.
// Some npm packages ship .go files; without this fence `go build ./...`,
// `go vet ./...` and `go test ./...` from the repository root would descend
// into them.
import { existsSync, mkdirSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const dir = join(root, 'node_modules')
if (!existsSync(dir)) mkdirSync(dir, { recursive: true })
writeFileSync(join(dir, 'go.mod'), 'module casebook.invalid/frontend-node-modules\n')
