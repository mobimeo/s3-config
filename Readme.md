# s3-config

Welcome to s3-config!

This is not an official Reach-Now product.

## requirements

The project used `go mod` as package mgmt util, so a recent Go toolchain is required (1.12+).

## build

```
go mod tidy
go build
```

## init config file

```
./s3-config init
```

## release

note: this should run on a CI. it will publish the golang bins to NPM, so node projects can import it.

```
docker run --rm \
  -w data \
  -v $PWD:/data \
  -e NPM_TOKEN \
  node:lts \
  npm ci && npm publish --access restricted
```
