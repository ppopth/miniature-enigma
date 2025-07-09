# Simple two nodes

This simple example connects two nodes to each other and performs basic operations.

## Build

From the `eth-ec-broadcast/examples` directory run the following:

```
> cd simple/
> go build
```

## Run

To run the listening node.
```
> ./simple -l 8001
```

To run the connecting node.
```
> ./simple -c 127.0.0.1:8001
```
