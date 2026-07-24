# Embedded entry templates

Each Current Capability owns exactly one `<capability-id>.tmpl` file. Templates
are parsed with Go `text/template` and may use only the compiled helper set:

- `section "Label"` writes a fixed English section heading.
- `field "Label" "path.to.value" "unit"` writes an optional scalar field.
- `requiredField` behaves like `field` but triggers complete generic fallback
  when the path is absent.
- `items "path.to.array"` returns ordered indexes and triggers fallback when
  the array is absent or has another type; `optionalItems` permits absence.
- `optionalObject "path.to.object"` guards a nullable or absent object and
  triggers fallback when a present non-null value has another type.
- `path "array" $index "field"` joins a provider path.
- `has "path"` tests optional provider content without consuming it.
- `inc $index` converts a zero-based index to a readable ordinal.
- `unit "temperature"` selects a unit from the provider response's known unit
  system. For endpoints with a unit parameter this is the invocation's
  effective unit; fixed-unit endpoints use their documented unit. Other
  supported keys are `speed`, `precipitation`, `visibility`, `pressure`,
  `percent`, `angle`, `distance`, `solar-radiation`, `energy`, and `altitude`.
  A literal reliably documented unit may also be passed directly.

Every scalar emitted through `field` is marked consumed. Any remaining provider
field is rendered once under `Additional fields`; Attribution paths are consumed
by the common renderer. Templates must not print Attribution themselves.
