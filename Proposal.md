# Dapr Actors V2

## Overview

This document contains a proposal for a v2 of the Actors building block in Dapr, which aims at solving a number of problems, first and foremost the dependency on a Placement service, with a different design.

Actors v2 is not backwards-compatible with "v1", but most differences can be smoothed out in the Dapr SDKs, so for most users the change could be transparent. It's also possible to continue running the actor "v1" and v2 runtimes side-by-side.

## Background

### Business problem

The current implementation for Dapr Actors requires two pieces to function properly:

- A state store component that is capable of storing actor state (requirements are support for ETags and "transactions" in the sense of batched operations executed atomically)
- The Placement service, which is necessary to ensure that a single actor (with a given type + ID combination) is currently active on a single node only, so that the node can enforce the actor can only be processing a single message at a time

The Placement service can be run either as single-instance (in which case requires storing data on disk and so requires a PersistentVolume) or in a HA configuration, which uses Raft to ensure the state is replicated across all instances. Both options have been the cause of multiple operational issues, especially due to Raft. Over the last few months, a number of proposals to replace Raft have been surfaced.

Other issues with the current Actors include:

- Scalability issues with actor reminders, since they're currently executing by the apps
- Lack of a "DeleteAll" method to remove all state belonging to a given actor in a single call (e.g. for garbage collection)
- The concept of "active" actors means that they are tied to a specific node, and that adds complexities such as: periodically "un-mounting" actors that have been inactive for a period of time, rebalancing in case of uneven distribution, and handling failovers.

### High-level solution overview

We have made, and continue to advance, proposals to make incremental improvements to the issues that are surfacing. However, at the same time this proposal tries to look at the problem in a different way and look for a solution for the core issue.

The Actors v2 runtime differs from "v1" in these key aspects:

- An actor can be scheduled on any node that is capable of running it, and it's not tied to a single node. For example, if app X is capable of running actors of type Y, and there are multiple instances of app X, any one of them can pick up any invocation for actors of type Y, regardless of the actor ID. This removes the need for the Placement service to exist.
- Actors store all of their state in a single key in the database. Multiple key-values can be stored in the same key and are then unserialized and re-serialized by the actor runtime.
- We ensure that 2 actors of with the same type and ID cannot run at the same time by leveraging locks in the state stores:
   - If the state store supports transactions with row-level blocking (pretty much any relational database except SQLite) we can lock on the row that contains the state for the individual actor.
   - Other state stores that support object-level locking can be used, such as Azure Blob Storage, although performance may not be on-par with the above.
   - Otherwise, we can use a Lock component to acquire a lock for a specific actor type and ID.
- Actor re-entrancy is supported but only with actors that can be hosted by the same app (e.g. if app X can run actors of type A and B only, there can be re-entrancy such as A->A or A->B->A, but not with an actor C).
- Actor reminders and timers are executed by a separate, dedicated service.

Because there's no Placement and no need for maintaining locks within a Dapr sidecar (which comes with additional problems such as failovers and rebalancing), the Actors v2 runtime is significantly simpler in the architecture and implementation.

## Related Items

### Related proposals 

- dapr/dapr#5403: proposal for improving actor reminder scalability

### Related issues 

- Scalability of actor reminders:
    - dapr/dapr#5671
- Operational issues with Placement:
    - dapr/dapr#5448
    - dapr/dapr#4892
    - dapr/dapr#5583

## Expectations and alternatives

* **What is in scope for this proposal?**
    - This proposal is an attempt at solving many issues with Dapr actors by leveraging a transactional database or a lock instead of the Placement service, building a v2 runtime for actors.
* **What is deliberately *not* in scope?**
* **What alternatives have been considered, and why do they not solve the problem?**
   - We can continue to make constant, incremental improvements to the actors "v1" runtime.
* **Are there any trade-offs being made?**
   - Although the actors v2 proposal makes certain things more flexible (e.g. being able to start an actor on any node capable of hosting that kind of actors), there are a few limitations (maximum size for actor state after serialization, re-entrancy limited to actors that can be scheduled on the same noide)

## Implementation Details

### State storage and locking

The main difference with actors "v1" is related to how state is stored, which is also how we acquire an exclusive lock in most cases.

All of the state for a given actor is stored in a single object (e.g. single row in a relational DB). The application can store and retrieve multiple keys, which are serialized by the runtime in a single key inside the database. State in the database is serialized as JSON or in an alternative format. Because this can lead to performance issues, there's a limit of 1MB for the entire state, after serialization. The key is `appId + "||" + actorType + "||" + actorID + "||state".

1. When an actor gets activated (e.g. because it receives a message or due to a reminder), the runtime loads the object containing the state for the given actor.
   - If the state store is capable of object-level/row-level locking, this is done in a transaction that also acquires a lock for that object/row automatically (which means that no other actor with the same type/ID can obtain).
   - Otherwise, we will use an external Lock component (the current APIs for the lock building block do not support this scenario and will need some changes - that's for a separate proposal)
2. The runtime calls the method on the actor, making the incoming request's data as well as all the state available.
  - When the app calls methods like `state.get(key)`, the Dapr Actors SDK returns the value that is in the state received by the runtime after un-serializing it. This is a synchronous operation.
  - Likewise, calls like `state.set(key, value)` or `state.delete(key)` are synchronous and modify only the in-memory state the SDK currently manages.
  - An actor can also call `state.deleteAll()` to have the entire state deleted
3. When the call is complete, the Actor SDK responds to Dapr with the response data as well as the updated state:
  - If the state is updated (i.e. if the actor called `state.set` or `state.delete` at least once), then the new state is serialized and stored.
  - If the actor called `state.deleteAll` the entire object/row is removed from the state store
  - After updating the value if necessary, the transaction is committed and the lock released.
  - In case of errors, the transaction is automatically rolled back.

### Actor invocation and re-entrancy

Thanks to the way locks are managed as described above, any app that can execute actors of a given type can respond to requests to invoke them. This means that, unlike with the "v1" runtime:

- If the app X that can execute actors of a given type is scaled horizontally, then any instance of that app can execute any actor (with any ID) at any time. For example, actor with ID "foo123" can be executed on instance 1 on the first call, and instance 3 on the second call. Thanks to the locking method, we still guarantee that the executions are performed in sequence and there's a "queue" (which is not orderly: when an actor ID becomes unlocked, if there are multiple callers competing for that, one of them will acquire the lock in a non-deterministic way).
- There's no concept of "idle" actors. An actor is "activated" (which means acquiring the lock) when a method comes in, and "deactivated" (lock released) once it's done.
- Because there are no "idle" actors, there's no need for failing over actors when a host app crashes, or rebalacing them.

With the "v1" runtime, callers can invoke actors by calling any Dapr sidecar (usually, their own) and specifying the name of the actor, for example:

```text
PUT /v1.0/actors/<actorType>/<actorId>/method/<methodName>
```

With the "v2" runtime, due to the lack of a centralized Placement service, callers will need to specify the ID of the app that can host that kind of actors:

```text
PUT /v2.0/actors/<appId>/<actorType>/<actorId>/method/<methodName>
```

Actor re-entrancy is supported but it requires all actors in the chain to be scheduled on the same node (process): because there's no Placement, an actor on a different node would not be able to know the address of the node that is hosting a specific actor ID otherwise. This means that re-entrancy can only work with actor types that are allowed by the same actor host.

For example, if app X can host actors of type A and B, the following re-entrant calls are possible: `A->A`, `A->B->A`. However, it's not possible to create a chain that use re-entrancy with actors of type C, because they cannot be scheduled on the same node.

### Reminders and timers

Reminders and timers are executed by a separate service in the Dapr control plane. The design for this can be found in dapr/dapr#5403.
