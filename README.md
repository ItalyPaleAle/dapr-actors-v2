# Dapr Actors v2 proposal and POC

This repo contains a proposal and initial proof-of-concept for Dapr Actors v2.

## Proposal

You can read the proposal in the [Proposal.md](./Proposal.md) file

## Proof-of-concept

This POC doesn't update the Actor SDK yet, so you don't have the ability to define a struct an have the methods exported automatically. For now, you need to use the `OnActorInvokeV2` handler and manually parse the method being invoked.

> Note: This POC currently works with only a single component, `state.postgresql`.

### Set up the environment

First, clone this repo:

```sh
git clone https://github.com/ItalyPaleAle/dapr-actors-v2 dapr-actors-v2
cd dapr-actors-v2
```

Clone dapr/dapr from my fork:

```sh
git clone https://github.com/ItalyPaleAle/dapr dapr
(cd dapr && git checkout actors-v2)
```

Clone dapr/components-contrib from my fork:

```sh
git clone https://github.com/ItalyPaleAle/dapr-components-contrib components-contrib
(cd components-contrib && git checkout actors-v2)
```

Clone the dapr/go-sdk from my fork:

```sh
git clone https://github.com/ItalyPaleAle/dapr-go-sdk go-sdk
(cd go-sdk && git checkout actors-v2)
```

### Build Dapr

> You need to have the Dapr CLI installed and have already executed `dapr init`

Build Dapr and "install" the binary:

```sh
./build-dapr.sh
```

### Start Postgres

In a separate terminal, run:

```sh
docker run -n postgresdev -p 5432:5432 -e POSTGRES_PASSWORD=mysecretpassword --rm postgres
```

### Start the app

```sh
cd actor
dapr run \
  --app-id dev \
  --app-protocol grpc \
  --app-port 9001 \
  --log-level debug \
  --resources-path ./config \
  go run ./dev/
```

You should see two actors being invoked.

One is invoked by two goroutines at the same time, and one of the calls is added to the queue.
