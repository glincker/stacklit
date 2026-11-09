# Languages

Stacklit indexes 48 languages and formats. Each row shows what is extracted. Any other language gets line counts and language detection only. Logos load from the thesvg icon set; languages without a logo there show no icon.

| Logo | Language | Extracts |
|------|----------|----------|
| <picture><source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/go/dark.svg"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/go/light.svg" alt="Go" width="24" height="24"></picture> | Go | imports, exports with signatures, struct fields, interface methods |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/typescript/default.svg" alt="TypeScript/JS" width="24" height="24"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/javascript/default.svg" alt="TypeScript/JS" width="24" height="24"> | TypeScript/JS | imports (ESM, CJS, dynamic), classes, interfaces, type aliases |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/python/default.svg" alt="Python" width="24" height="24"> | Python | imports, classes with methods, type hints, decorators |
| <picture><source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/rust/dark.svg"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/rust/light.svg" alt="Rust" width="24" height="24"></picture> | Rust | use/mod/crate, pub items with generics, trait methods |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/java/default.svg" alt="Java" width="24" height="24"> | Java | imports, public classes, method signatures with types |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/csharp/default.svg" alt="C#" width="24" height="24"> | C# | using directives, public types, method signatures |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/ruby/default.svg" alt="Ruby" width="24" height="24"> | Ruby | require, classes, modules, methods |
| <picture><source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/php/dark.svg"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/php/light.svg" alt="PHP" width="24" height="24"></picture> | PHP | namespace use, classes, traits, public methods |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/kotlin/default.svg" alt="Kotlin" width="24" height="24"> | Kotlin | imports, classes, objects, enum entries, functions with signatures |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/swift/default.svg" alt="Swift" width="24" height="24"> | Swift | imports, structs, classes, protocols |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/c/default.svg" alt="C/C++" width="24" height="24"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/c-plusplus/default.svg" alt="C/C++" width="24" height="24"> | C/C++ | includes, non-static functions, structs, unions, enums, classes, typedefs, namespaces |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/elixir/default.svg" alt="Elixir" width="24" height="24"> | Elixir | alias/import/require/use, modules, protocols, public def/defmacro, structs, @type/@callback |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/zig/default.svg" alt="Zig" width="24" height="24"> | Zig | @import, pub functions, pub structs/enums/unions with methods, pub constants |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/dart/default.svg" alt="Dart" width="24" height="24"> | Dart | imports/exports, classes, mixins, enums, extensions, typedefs, public functions and methods |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/scala/default.svg" alt="Scala" width="24" height="24"> | Scala | imports (with selectors), traits, classes, objects, enums, type aliases, defs and vals |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/lua/default.svg" alt="Lua" width="24" height="24"> | Lua | require, non-local functions, `M.fn` and `M:method` definitions |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/shell/default.svg" alt="Shell" width="24" height="24"> | Shell | source/`.` includes, function definitions |
|  | SQL | tables with columns, views, functions |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/godot/default.svg" alt="GDScript" width="24" height="24"> | GDScript | class_name, signals, constants, enums, public funcs, inner classes, preload/load paths |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/vue/default.svg" alt="Vue / Svelte" width="24" height="24"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/svelte/default.svg" alt="Vue / Svelte" width="24" height="24"> | Vue / Svelte | script block imports, exports, types (parsed as TypeScript) |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/terraform/default.svg" alt="Terraform/HCL" width="24" height="24"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/hcl/default.svg" alt="Terraform/HCL" width="24" height="24"> | Terraform/HCL | resources, data sources, variables, outputs, modules, providers, module sources |
|  | Protobuf | imports, messages (nested), enums, services with rpc signatures |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/haskell/default.svg" alt="Haskell" width="24" height="24"> | Haskell | imports, signatures (honors the module export list), data/newtype/class declarations |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/ocaml/default.svg" alt="OCaml" width="24" height="24"> | OCaml | open/include, let bindings, types with constructors and fields, modules, exceptions |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/clojure/default.svg" alt="Clojure" width="24" height="24"> | Clojure | ns requires/imports, `defn`/`def`/`defmacro`, protocols and records |
| <picture><source media="(prefers-color-scheme: dark)" srcset="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/r/dark.svg"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/r/light.svg" alt="R" width="24" height="24"></picture> | R | library/require/source, non-dot functions with parameters, top-level constants |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/perl/default.svg" alt="Perl" width="24" height="24"> | Perl | use/require/parent, packages, public subs |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/julia/default.svg" alt="Julia" width="24" height="24"> | Julia | using/import, exported functions, structs, abstract types, constants |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/objectivec/default.svg" alt="Objective-C" width="24" height="24"> | Objective-C | #import, `@interface`, `@protocol`, methods and properties |
|  | Groovy | imports, classes, interfaces, enums, top-level `def` functions |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/nim/default.svg" alt="Nim" width="24" height="24"> | Nim | imports, star-exported procs, funcs, types with fields, constants |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/erlang/default.svg" alt="Erlang" width="24" height="24"> | Erlang | includes, exported functions as name/arity, records, types |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/fsharp/default.svg" alt="F#" width="24" height="24"> | F# | open, modules, types (unions, records, members), non-private lets |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/fortran/default.svg" alt="Fortran" width="24" height="24"> | Fortran | use, modules, functions, subroutines, derived types |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/elm/default.svg" alt="Elm" width="24" height="24"> | Elm | imports, exposed annotated values and types |
|  | Solidity | imports, contracts, interfaces, libraries, public functions, events, structs |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/powershell/default.svg" alt="PowerShell" width="24" height="24"> | PowerShell | Import-Module/using/dot-sourcing, functions with params, classes |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/gleam/default.svg" alt="Gleam" width="24" height="24"> | Gleam | imports, pub functions, types with constructors, constants |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/cmake/default.svg" alt="CMake" width="24" height="24"> | CMake | include/add_subdirectory/find_package, project, library/executable/custom targets, functions, macros |
|  | Makefile | includes, explicit targets with prerequisites |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/docker/default.svg" alt="Dockerfile" width="24" height="24"> | Dockerfile | base images, build stages, exposed ports |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/yaml/default.svg" alt="YAML / TOML / JSON" width="24" height="24"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/toml/default.svg" alt="YAML / TOML / JSON" width="24" height="24"><img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/json/default.svg" alt="YAML / TOML / JSON" width="24" height="24"> | YAML / TOML / JSON | top-level keys (capped, see below) |
| <img src="https://cdn.jsdelivr.net/gh/glincker/thesvg@main/public/icons/graphql/default.svg" alt="GraphQL" width="24" height="24"> | GraphQL | object/interface/enum/input/union/scalar types with fields, named operations and fragments |

## Limits and parser notes

- **Config files** (YAML, TOML, JSON) report at most `STACKLIT_CONFIG_KEY_CAP` top-level keys (default 40). Files above `STACKLIT_CONFIG_MAX_BYTES` (default 262144) skip key extraction.
- **Skipped files**: lockfiles and the generated `stacklit.json` are not indexed.
- **Parsers**: Go uses the Go standard library. Every other row uses tree-sitter.
- **SQL**: `$$` function bodies are masked before parsing, so the statements after them still extract.
- **Groovy** falls back to line patterns where the grammar fails.
- **`.m` files** are treated as Objective-C.

## Symbol graph

The symbol graph (`stacklit graph`, `query`, `impact`, `slice`) covers 20 languages. The per-language table is in [ARCHITECTURE.md](ARCHITECTURE.md#symbol-graph-extraction). The other languages still get the module index.

## Monorepos

Monorepos are detected automatically:

- pnpm, npm and yarn workspaces
- Go workspaces
- Turborepo, Nx and Lerna
- Cargo workspaces
- `apps/`, `packages/`, `services/` and `libs/` directories
