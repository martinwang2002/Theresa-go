# Theresa-go

This is golang backend for https://theresa.wiki, wiki for arknights

## features
* audio conversion from wav to ogg
* png image to webp
* 3d model & texture conversion from unity data to json file, which can be rendered by three.js

## Go mod update
Either
```
go get -u
go mod tidy
```
Or
vscode Go extension

## Development

### hot relaoding with `air`
You should install `air` and link `.air.conf` to the `.air.<platform>.conf` file.

### memory profile
`go tool pprof -http=: http://static.theresa.localhost:8000/debug/pprof/allocs`
