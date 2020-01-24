# Checkpoint

Checkpoint provides a simple means of recording and acting
on checkpoints in scripting environments and in particular
shell scripts. It allows the user to define appropriate
checkpoints in the execution of a script and to skip past
those points if the checkpoint has been previously 'completed', ie,
deemed as being done. The intent is to allow users to develop scripts
incrementally but without having to re-execute all prior
steps in a script whilst implementing/iterating on later ones.

```
set -e
source <(checkpoint use $0)
trap step EXIT

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
source statement.

## Limitations

A linear sequential control flow is currently the only supported
execution mode.

Currenly only unix shells are supported, in particular only `bash` and `zsh`
have been tested, but since little is required of the shell it should
work with all other unix-like shells. In particular, the source
step only sets an environment variable and defines the `step` shell function

## State Storage

The execution state is currently stored in the user's home directory
as files, under `$HOME/.checkpointstate/...`, but other state stores
are anticipated such as dynamodb to allow for execution from other
environments such as aws lambda.