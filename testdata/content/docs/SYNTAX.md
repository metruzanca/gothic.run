# gothic syntax

This is the reference for the gothic view language.

## Views

A `.gth` file contains ordinary Go followed by views.

### Pure components

A pure component takes exactly one parameter named `props`.

```templ
view Badge(props BadgeProps) {
	<span>{ props.Text }</span>
}
```
