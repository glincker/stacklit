# Languages

Stacklit indexes 48 languages and formats. Each row shows what is extracted. Any other language gets line counts and language detection only.

| Language | Extracts |
|----------|----------|
| Go | imports, exports with signatures, struct fields, interface methods |
| TypeScript/JS | imports (ESM, CJS, dynamic), classes, interfaces, type aliases |
| Python | imports, classes with methods, type hints, decorators |
| Rust | use/mod/crate, pub items with generics, trait methods |
| Java | imports, public classes, method signatures with types |
| C# | using directives, public types, method signatures |
| Ruby | require, classes, modules, methods |
| PHP | namespace use, classes, traits, public methods |
| Kotlin | imports, classes, objects, enum entries, functions with signatures |
| Swift | imports, structs, classes, protocols |
| C/C++ | includes, non-static functions, structs, unions, enums, classes, typedefs, namespaces |
| Elixir | alias/import/require/use, modules, protocols, public def/defmacro, structs, @type/@callback |
| Zig | @import, pub functions, pub structs/enums/unions with methods, pub constants |
| Dart | imports/exports, classes, mixins, enums, extensions, typedefs, public functions and methods |
| Scala | imports (with selectors), traits, classes, objects, enums, type aliases, defs and vals |
| Lua | require, non-local functions, `M.fn` and `M:method` definitions |
| Shell | source/`.` includes, function definitions |
| SQL | tables with columns, views, functions |
| GDScript | class_name, signals, constants, enums, public funcs, inner classes, preload/load paths |
| Vue / Svelte | script block imports, exports, types (parsed as TypeScript) |
| Terraform/HCL | resources, data sources, variables, outputs, modules, providers, module sources |
| Protobuf | imports, messages (nested), enums, services with rpc signatures |
| Haskell | imports, signatures (honors the module export list), data/newtype/class declarations |
| OCaml | open/include, let bindings, types with constructors and fields, modules, exceptions |
| Clojure | ns requires/imports, `defn`/`def`/`defmacro`, protocols and records |
| R | library/require/source, non-dot functions with parameters, top-level constants |
| Perl | use/require/parent, packages, public subs |
| Julia | using/import, exported functions, structs, abstract types, constants |
| Objective-C | #import, `@interface`, `@protocol`, methods and properties |
| Groovy | imports, classes, interfaces, enums, top-level `def` functions |
| Nim | imports, star-exported procs, funcs, types with fields, constants |
| Erlang | includes, exported functions as name/arity, records, types |
| F# | open, modules, types (unions, records, members), non-private lets |
| Fortran | use, modules, functions, subroutines, derived types |
| Elm | imports, exposed annotated values and types |
| Solidity | imports, contracts, interfaces, libraries, public functions, events, structs |
| PowerShell | Import-Module/using/dot-sourcing, functions with params, classes |
| Gleam | imports, pub functions, types with constructors, constants |
| CMake | include/add_subdirectory/find_package, project, library/executable/custom targets, functions, macros |
| Makefile | includes, explicit targets with prerequisites |
| Dockerfile | base images, build stages, exposed ports |
| YAML / TOML / JSON | top-level keys (capped, see below) |
| GraphQL | object/interface/enum/input/union/scalar types with fields, named operations and fragments |

Config files (YAML, TOML, JSON) report at most `STACKLIT_CONFIG_KEY_CAP` top-level keys (default 40) and skip key extraction above `STACKLIT_CONFIG_MAX_BYTES` (default 262144). Lockfiles and the generated `stacklit.json` are not indexed. Go is parsed with the Go standard library; every other row uses tree-sitter. SQL `$$` function bodies are masked before parsing so later statements still extract. Grammar-specific caveats: Groovy extraction falls back to line patterns where the grammar errors out, and `.m` files are treated as Objective-C.

Any other language gets basic support (line count + language detection).

The symbol graph (`stacklit graph`, `query`, `impact`, `slice`) covers 20 languages; the per-language table is in [ARCHITECTURE.md](ARCHITECTURE.md#symbol-graph-extraction). The other languages still get the module index.

Monorepos are detected automatically: pnpm, npm and yarn workspaces, Go workspaces, Turborepo, Nx, Lerna, Cargo workspaces, and `apps/`, `packages/`, `services/`, `libs/` conventions.
