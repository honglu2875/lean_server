# lean_server

Easy. Just do

```bash
make build
make run
```

Wait for `Server starting on http://localhost:8080` and enjoy.

To clean up:
```bash
make clean
```

# A few simple tests

```bash
curl -X POST http://localhost:8080/repl -d '{"cmd": "def f (x : Unit) : Nat := by sorry"}'
```

```bash
curl -X POST http://localhost:8080/repl -d '{ "cmd" : "def f := 2" }'
```


# TODO

I'm already considering to scale it to full-fledged cluster containing many containers. To avoid excess storage, the current design is to build two images: `lean-provider` and `lean-repl` where `lean-provider` runs indefinitely (blocking!!) and only provides elan and lean binaries. `lean-repl` is on the other hand a small golang app that wraps the LEAN REPL and communicate by simple HTTP API.

A potential roadmap without involving more complicated setup like k8 is the following:
- [ ] decide a load-balancing strategy and implement
  - This could be as simple as round-robin
  - Or we could use a more sophisticated approach like least-connections or least-response-time
- [ ] make each server more robust
  - [ ] LEAN REPL process can crash in the face of bad JSON, for example. We could either perform more verifications or have a restart strategy.
  - [ ] I do not know if the output parsing logic can fail or not. Currently, I did a simple bracket counting loop to determine if the LEAN REPL ends the output or not. If this is not sufficient, we need to fix it.
    - This is easily fixable if we can modify the REPL itself but let's keep it simple.
  - [ ] add more tests
- [ ] add another layer of script that
  - spin up multiple containers that pin to a range of CPUs.
  - set up the ports all inside a docker network.
  - spin up the load balancer and expose one endpoint externally.
