# ACN E2E

## Objectives
- Steps are reusable
- Steps parameters are saved to the context of the job
- Once written to the job context, the values are immutable
- Cluster resources used in code should be able to be generated to yaml for easy manual repro
- Avoid shell/ps calls wherever possible and use go libraries for typed parameters (avoid capturing error codes/stderr/stdout)

---
## Starter Example:

When authoring tests, make sure to prefix the test name with `TestE2E` so that it is skipped by existing pipeline unit test framework.
For reference, see the `test-all` recipe in the root [Makefile](../../Makefile).


For sample test, please check out:
[the Hubble E2E.](./scenarios/hubble/index_test.go)


## acndev CLI

The `acndev` CLI is a tool for manually interacting with E2E steps for quick access. 

It is used to create and manage clusters, but **not** to author tests with, and should **not** be referenced in pipeline yaml. Please stick to using tests with `TestE2E` prefix for authoring tests.
