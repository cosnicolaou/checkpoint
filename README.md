# Checkpoint

[![CircleCI](https://circleci.com/gh/circleci/circleci-docs.svg?style=svg)](https://circleci.com/gh/circleci/circleci-docs)
[![Go Report Card](https://goreportcard.com/badge/github.com/cosnicolaou/checkpoint)](https://goreportcard.com/report/github.com/cosnicolaou/checkpoint)

Checkpoint provides a simple means of recording and acting
on checkpoints in scripting environments and in particular
shell scripts. It allows the user to define appropriate
checkpoints in the execution of a script and to skip past
those points if the checkpoint has been previously 'completed', ie,
deemed as being done. The intent is to allow users to develop scripts
incrementally but without having to re-execute all prior
steps in a script whilst implementing/iterating on later ones.

```sh
set -e
source <(checkpoint use $0)
trap completed EXIT

completed step1 || <action>
completed step2 || <action>

if ! completed step3; then
   <action>
   completed step4 || <action>
fi
```

Reaching each step implicitly transitions the currently active one to being
complete; alternatively the current step can be explicitly marked
as complete using `completed` without an argument. Note that `completed` is
a shell function defined in the context of the shell via the
source statement. This shell function tests the exit status of the previous
command and will not execute the next step if that command failed.

Another anticipated common use case is to guard the execution of a script
based on the arrival or generation of new data.

```sh
proposed="date-for-example"

source <(checkpoint use $(basename $proposed))
trap completed EXIT

if completed ready; then
  echo "already-done"
  exit 0
fi

exit 0
```

## Limitations

A linear sequential control flow is currently the only supported
execution mode.

Currenly only unix shells are supported, in particular only `bash` and `zsh`
have been tested, but since little is required of the shell it should
work with all other unix-like shells. In particular, the source
step only sets an environment variable and defines the `step` shell function


## Session Management and Inspection

Simple checkpoint management is available to list and delete sessions.

```sh
checkpoint list
checkpoint delete c4518f9acbeb9d3ac4c7970e899460258cc0f7a923003b73bb0a28fa0f050f99
checkpoint delete c4518f9acbeb9d3ac4c7970e899460258cc0f7a923003b73bb0a28fa0f050f99 step1
```

The detailed metadata and state associated with a session is available in both
raw JSON form (`dump`) or as a summary (`state`).
```sh
checkpoint dump c4518f9acbeb9d3ac4c7970e899460258cc0f7a923003b73bb0a28fa0f050f99
checkpoint state c4518f9acbeb9d3ac4c7970e899460258cc0f7a923003b73bb0a28fa0f050f99
```

## State Storage

The execution state is currently stored in the user's home directory
as files, under `$HOME/.checkpointstate/...`, but other state stores
are anticipated such as dynamodb to allow for execution from other
environments such as aws lambda.
